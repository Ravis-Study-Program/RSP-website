package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"net/http"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authadmin"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/program"
)

func invitationToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashInvitationToken(token), nil
}
func hashInvitationToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func (a *API) authorizeInvitations(w http.ResponseWriter, r *http.Request, write bool) bool {
	actor := actorFrom(r.Context())
	seasonID := r.PathValue("id")
	if !actor.IsSeasonAdmin(seasonID) || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Season administrator access with recent MFA is required.")
		return false
	}
	season, err := a.db.GetSeason(r.Context(), seasonID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return false
	}
	if write && season.Status != "open" {
		writeErrorResponse(w, http.StatusConflict, "Reopen this season before changing invitations.")
		return false
	}
	return true
}
func (a *API) listInvitations(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeInvitations(w, r, false) {
		return
	}
	binding := "invitations|" + r.PathValue("id")
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil {
		writeErrorResponse(w, 400, "Invalid invitation cursor.")
		return
	}
	items, more, total, err := a.db.ListInvitations(r.Context(), r.PathValue("id"), boundary, limit)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	pageInfo, err := pageInfoForKeyset(a.cursorSecret, binding, "forward", boundary, items, more, func(v program.Invitation) string { return v.ID })
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, Page[program.Invitation]{Items: items, PageInfo: pageInfo, TotalCount: total})
}
func (a *API) sendInvitationEmail(r *http.Request, v *program.Invitation, token string) {
	if a.sendInvitation == nil {
		return
	}
	// Inside an import's transaction, send only once it has been committed.
	if a.afterCommit != nil {
		*a.afterCommit = append(*a.afterCommit, func(root *API) { root.sendInvitationEmail(r, v, token) })
		return
	}
	inviterName := ""
	if inviter, err := a.db.GetUser(r.Context(), actorFrom(r.Context()).UserID); err == nil {
		inviterName = inviter.Name
	}
	err := a.sendInvitation(r.Context(), authadmin.InvitationEmailInput{InviterName: inviterName, ID: v.DeliveryID, To: v.Email, Name: v.Name, Season: v.SeasonName, Role: v.Role, URL: strings.TrimRight(a.publicOrigin, "/") + "/invitations/accept#token=" + token})
	if err != nil {
		a.logger.Error("invitation email enqueue failed", "invitationId", v.ID)
		return
	}
	at := time.Now().UTC()
	if err = a.db.MarkInvitationSent(r.Context(), v.ID, v.DeliveryID, at); err != nil {
		a.logger.Error("invitation delivery receipt failed", "invitationId", v.ID)
		return
	}
	v.SentAt = &at
}

// invitationInput is one invitation to create, from the form or an import.
type invitationInput struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	MentorUserID string `json:"mentorUserId"`
	// MentorInvitationID names a mentor invited in the same import, who has
	// no account in the season yet.
	MentorInvitationID string `json:"-"`
	// Slug is the username to give the person, if their account is new.
	Slug string `json:"slug"`
}

// inviteOne validates and creates one invitation in a season, then queues its
// email. A rejected input returns its status and message; a storage failure
// returns the error.
func (a *API) inviteOne(r *http.Request, seasonID string, in invitationInput) (program.Invitation, int, string, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !validEmailAddress(in.Email) || len(in.Name) < 1 || len(in.Name) > 100 {
		return program.Invitation{}, 400, "Enter a name and a valid email address.", nil
	}
	if in.Role != "student" && in.Role != "mentor" && in.Role != "coordinator" {
		return program.Invitation{}, 400, "Choose student, mentor or coordinator.", nil
	}
	var mentorUserID *string
	if mentor := strings.TrimSpace(in.MentorUserID); mentor != "" {
		if in.Role != "student" {
			return program.Invitation{}, 400, "Only a student can be invited with a mentor.", nil
		}
		if !uuidSelection(mentor) {
			return program.Invitation{}, 400, "mentorUserId must be a user id.", nil
		}
		mentorUserID = &mentor
	}
	if in.Role == "coordinator" && !canGrantCoordinator(actorFrom(r.Context()), seasonID) {
		return program.Invitation{}, 403, "Coordinator access cannot be granted.", nil
	}
	var mentorInvitationID *string
	if in.MentorInvitationID != "" {
		mentorInvitationID = &in.MentorInvitationID
	}
	var profileSlug *string
	if slug := strings.ToLower(strings.TrimSpace(in.Slug)); slug != "" {
		if problem := a.slugProblem(r, slug, ""); problem != "" {
			return program.Invitation{}, 400, problem, nil
		}
		profileSlug = &slug
	}
	token, hash, err := invitationToken()
	if err != nil {
		return program.Invitation{}, 0, "", err
	}
	at := time.Now().UTC()
	v, err := a.db.CreateInvitation(r.Context(), program.Invitation{ID: id.New(), SeasonID: seasonID, Name: in.Name, Email: in.Email, Role: in.Role, MentorUserID: mentorUserID, ProfileSlug: profileSlug, MentorInvitationID: mentorInvitationID, DeliveryID: id.New(), ExpiresAt: at.Add(7 * 24 * time.Hour)}, hash, actorFrom(r.Context()).UserID, at)
	if err != nil {
		return program.Invitation{}, 0, "", err
	}
	a.sendInvitationEmail(r, &v, token)
	return v, 0, "", nil
}

func (a *API) createInvitation(w http.ResponseWriter, r *http.Request) {
	if !a.authorizeInvitations(w, r, true) {
		return
	}
	var request invitationInput
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, 400, err.Error())
		return
	}
	v, status, message, err := a.inviteOne(r, r.PathValue("id"), request)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if message != "" {
		writeErrorResponse(w, status, message)
		return
	}
	writeJSONResponse(w, 201, v)
}

// maxInvitationBatch bounds one import request.
const maxInvitationBatch = 200

type invitationBatchRow struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
	// Mentor names a student's mentor by email, username or full name; empty
	// for none.
	Mentor string `json:"mentor"`
	// Slug is an optional username for the person.
	Slug string `json:"slug"`
}

// importChange is one field an import changes (or would change) for someone
// already in the season.
type importChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type invitationBatchResult struct {
	Row int `json:"row"`
	// Outcome is, for a dry run, invite, update or unchanged; once sent,
	// invited, updated or unchanged; or failed either way.
	Outcome string `json:"outcome"`
	// Person is who the email matched: new (no account), account (an account
	// not in the season) or member (already in the season).
	Person     string              `json:"person,omitempty"`
	Invitation *program.Invitation `json:"invitation,omitempty"`
	Role       string              `json:"role,omitempty"`
	// Slug is the username a new person is given: the one written in the row,
	// or else one made from their name.
	Slug string `json:"slug,omitempty"`
	// Changes are the fields that change for a member; Skipped are those the
	// sheet differs on that this administrator may not change.
	Changes []importChange `json:"changes,omitempty"`
	Skipped []importChange `json:"skipped,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// memberPlan is what an import does to someone already in the season.
type memberPlan struct {
	member    dal.SeasonMemberByEmail
	email     string
	role      string
	mentorID  string
	name      string
	slug      string
	changes   []importChange
	skipped   []importChange
	setRole   bool
	setMentor bool
	setName   bool
	setSlug   bool
}

// previewInvitationBatch says what createInvitationBatch would do with the
// same rows, without changing anything or sending email.
func (a *API) previewInvitationBatch(w http.ResponseWriter, r *http.Request) {
	a.runInvitationBatch(w, r, true)
}

func (a *API) createInvitationBatch(w http.ResponseWriter, r *http.Request) {
	a.runInvitationBatch(w, r, false)
}

// runInvitationBatch invites many people to one season, as from a
// spreadsheet, matching each row to a person by email. Someone already in
// the season is updated instead: their role and mentor, and (for a System
// Admin) their name and username. A dry run changes nothing and says what
// would happen to each row. Each row succeeds or fails on its own, so one bad
// row does not stop the rest.
func (a *API) runInvitationBatch(w http.ResponseWriter, r *http.Request, dryRun bool) {
	if !a.authorizeInvitations(w, r, true) {
		return
	}
	var request struct {
		Invitations []invitationBatchRow `json:"invitations"`
	}
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, 400, err.Error())
		return
	}
	if len(request.Invitations) == 0 || len(request.Invitations) > maxInvitationBatch {
		writeErrorResponse(w, 400, fmt.Sprintf("Send between 1 and %d invitations at once.", maxInvitationBatch))
		return
	}
	season, err := a.db.GetSeason(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if dryRun {
		results, err := a.importRows(r, season, request.Invitations, true)
		if err != nil {
			a.writeStoreErrorResponse(w, err)
			return
		}
		writeJSONResponse(w, 200, map[string]any{"results": results, "applied": false})
		return
	}
	// The whole import is one transaction: if any row fails, none of them is
	// kept, and the emails are sent only once everything has been saved.
	var results []invitationBatchResult
	var afterCommit []func(*API)
	err = a.db.WithTx(r.Context(), func(tx *dal.Store) error {
		inTx := *a
		inTx.db, inTx.afterCommit = tx, &afterCommit
		var err error
		if results, err = inTx.importRows(r, season, request.Invitations, false); err != nil {
			return err
		}
		for _, result := range results {
			if result.Outcome == "failed" {
				return errBatchRejected
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, errBatchRejected):
		// Say what each row would have done, and that nothing was saved.
		for i := range results {
			results[i].Invitation = nil
			switch results[i].Outcome {
			case "invited":
				results[i].Outcome = "invite"
			case "updated":
				results[i].Outcome = "update"
			}
		}
		writeJSONResponse(w, 200, map[string]any{"results": results, "applied": false})
	case err != nil:
		a.writeStoreErrorResponse(w, err)
	default:
		for _, send := range afterCommit {
			send(a)
		}
		writeJSONResponse(w, 200, map[string]any{"results": results, "applied": true})
	}
}

// errBatchRejected cancels an import's transaction because a row failed.
var errBatchRejected = errors.New("an import row failed")

// importRows plans each row of an import and, unless dryRun, carries it out.
// Rows that make someone a mentor go first, so a student later in the same
// sheet can name them as their mentor.
func (a *API) importRows(r *http.Request, season program.SeasonRecord, rows []invitationBatchRow, dryRun bool) ([]invitationBatchResult, error) {
	mentors, err := a.db.SeasonMentors(r.Context(), season.ID)
	if err != nil {
		return nil, err
	}
	slugs := newSlugAllocator(rows)
	results := make([]invitationBatchResult, len(rows))
	mentoring := func(row invitationBatchRow) bool {
		role := strings.ToLower(strings.TrimSpace(row.Role))
		return role == "mentor" || role == "coordinator"
	}
	for i, row := range rows {
		if mentoring(row) {
			results[i] = a.importRow(r, season, mentors, nil, slugs, i, row, dryRun)
		}
	}
	invitedBefore, err := a.db.PendingMentorInvitations(r.Context(), season.ID)
	if err != nil {
		return nil, err
	}
	pending := pendingMentors(rows, results)
	for _, earlier := range invitedBefore {
		pending = append(pending, pendingMentor{InvitationID: earlier.ID, Email: earlier.Email, Slug: strings.ToLower(earlier.Slug), Name: earlier.Name})
	}
	if mentors, err = a.mentorsAfter(r, season.ID, rows, results, mentors, dryRun); err != nil {
		return nil, err
	}
	for i, row := range rows {
		if !mentoring(row) {
			results[i] = a.importRow(r, season, mentors, pending, slugs, i, row, dryRun)
		}
	}
	return results, nil
}

// mentorsAfter is the season's mentors once the mentor rows have run: read
// again after real changes, or, for a dry run, with the members those rows
// would make mentors added.
func (a *API) mentorsAfter(r *http.Request, seasonID string, rows []invitationBatchRow, results []invitationBatchResult, mentors []dal.SeasonMentor, dryRun bool) ([]dal.SeasonMentor, error) {
	if !dryRun {
		return a.db.SeasonMentors(r.Context(), seasonID)
	}
	for i, result := range results {
		if result.Person != "member" || (result.Outcome != "update" && result.Outcome != "unchanged") {
			continue
		}
		member, found, err := a.db.FindSeasonMemberByEmail(r.Context(), seasonID, strings.TrimSpace(rows[i].Email))
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		// The member as a mentor after this row: renamed or re-slugged if the
		// row changes those, whether they mentor already or become a mentor.
		mentor := dal.SeasonMentor{UserID: member.UserID, Name: member.Name, Email: strings.ToLower(strings.TrimSpace(rows[i].Email)), Slug: strings.ToLower(member.Slug)}
		for _, change := range result.Changes {
			switch change.Field {
			case "name":
				mentor.Name = change.To
			case "slug":
				mentor.Slug = change.To
			}
		}
		replaced := false
		for j := range mentors {
			if mentors[j].UserID == mentor.UserID {
				mentors[j], replaced = mentor, true
			}
		}
		if !replaced {
			mentors = append(mentors, mentor)
		}
	}
	return mentors, nil
}

// importRow plans one row and, unless this is a dry run, carries it out.
func (a *API) importRow(r *http.Request, season program.SeasonRecord, mentors []dal.SeasonMentor, pending []pendingMentor, slugs *slugAllocator, index int, row invitationBatchRow, dryRun bool) invitationBatchResult {
	result := invitationBatchResult{Row: index, Outcome: "failed"}
	input := invitationInput{Name: strings.TrimSpace(row.Name), Email: strings.ToLower(strings.TrimSpace(row.Email)), Role: strings.ToLower(strings.TrimSpace(row.Role)), Slug: strings.ToLower(strings.TrimSpace(row.Slug))}
	result.Role = input.Role
	usesInvitedMentor := false
	if input.Role != "student" && input.Role != "mentor" && input.Role != "coordinator" {
		result.Error = "Choose student, mentor or coordinator."
		return result
	}
	if named := strings.TrimSpace(row.Mentor); named != "" {
		if input.Role != "student" {
			result.Error = "Only a student can be given a mentor."
			return result
		}
		mentorID, problem := matchMentor(mentors, named)
		if strings.HasPrefix(problem, "More than one") {
			result.Error = problem
			return result
		}
		if problem != "" {
			// Not a mentor in the season yet, but perhaps one this import invites.
			invited, found, ambiguous := matchPendingMentor(pending, named)
			switch {
			case ambiguous:
				result.Error = fmt.Sprintf("More than one mentor is called %q; use their username or email.", named)
				return result
			case !found:
				result.Error = problem
				return result
			case invited.Failed:
				result.Error = fmt.Sprintf("The row for their mentor %q has an error.", named)
				return result
			}
			input.MentorInvitationID, usesInvitedMentor = invited.InvitationID, true
		} else {
			input.MentorUserID = mentorID
		}
	}
	member, found, err := a.db.FindSeasonMemberByEmail(r.Context(), season.ID, input.Email)
	if err != nil {
		result.Error = invitationStoreProblem(a, err)
		return result
	}
	if found {
		result.Person = "member"
		result.Slug = strings.ToLower(member.Slug)
		if usesInvitedMentor {
			result.Error = "Their mentor has not joined yet; set the mentor once they have."
			return result
		}
		plan, problem := a.planMemberUpdate(r, season, mentors, member, input)
		result.Changes, result.Skipped = plan.changes, plan.skipped
		switch {
		case problem != "":
			result.Error = problem
		case len(plan.changes) == 0:
			result.Outcome = "unchanged"
		case dryRun:
			result.Outcome = "update"
		default:
			if problem := a.applyMemberUpdate(r, season, plan); problem != "" {
				result.Error = problem
			} else {
				result.Outcome = "updated"
			}
		}
		return result
	}
	exists, err := a.db.AccountExistsByEmail(r.Context(), input.Email)
	if err != nil {
		result.Error = invitationStoreProblem(a, err)
		return result
	}
	result.Person = "new"
	if exists {
		result.Person = "account"
	}
	// Someone new always gets a username: the one given, or one made from
	// their name that no one else in this import or on the site has.
	if exists {
		// An existing account keeps its own username.
		slug, err := a.db.AccountSlugByEmail(r.Context(), input.Email)
		if err != nil {
			result.Error = invitationStoreProblem(a, err)
			return result
		}
		result.Slug = slug
	}
	if !exists && slugs != nil {
		if input.Slug == "" {
			generated, err := slugs.next(r, a, input.Name, input.Email)
			if err != nil {
				result.Error = invitationStoreProblem(a, err)
				return result
			}
			input.Slug = generated
		}
		result.Slug = input.Slug
	}
	if dryRun {
		if problem := a.checkInvitation(r, season.ID, input); problem != "" {
			result.Error = problem
		} else {
			result.Outcome = "invite"
		}
		return result
	}
	v, _, message, err := a.inviteOne(r, season.ID, input)
	switch {
	case err != nil:
		result.Error = invitationStoreProblem(a, err)
	case message != "":
		result.Error = message
	default:
		result.Outcome = "invited"
		result.Invitation = &v
	}
	return result
}

// checkInvitation finds, without changing anything, why an invitation could
// not be created; empty when it could.
func (a *API) checkInvitation(r *http.Request, seasonID string, in invitationInput) string {
	if !validEmailAddress(in.Email) || len(in.Name) < 1 || len(in.Name) > 100 {
		return "Enter a name and a valid email address."
	}
	if in.Role == "coordinator" && !canGrantCoordinator(actorFrom(r.Context()), seasonID) {
		return "Coordinator access cannot be granted."
	}
	if in.Slug != "" {
		if problem := a.slugProblem(r, in.Slug, ""); problem != "" {
			return problem
		}
	}
	pending, err := a.db.PendingInvitationExists(r.Context(), seasonID, in.Email)
	if err != nil {
		return invitationStoreProblem(a, err)
	}
	if pending {
		return "Already invited to this season."
	}
	return ""
}

// slugProblem says why a username cannot be used (by anyone but ownerID).
func (a *API) slugProblem(r *http.Request, slug, ownerID string) string {
	if len(slug) < 2 || len(slug) > 50 || !validSlug(slug) || strings.Contains(slug, "--") {
		return "Usernames are 2–50 lowercase letters, numbers and single hyphens."
	}
	owner, err := a.db.ProfileSlugOwner(r.Context(), slug)
	if err != nil {
		return invitationStoreProblem(a, err)
	}
	if owner != "" && owner != ownerID {
		return fmt.Sprintf("The username %q is taken.", slug)
	}
	return ""
}

// planMemberUpdate works out what an import changes for someone already in
// the season. Blank mentor and username cells leave those as they are.
func (a *API) planMemberUpdate(r *http.Request, season program.SeasonRecord, mentors []dal.SeasonMentor, member dal.SeasonMemberByEmail, in invitationInput) (memberPlan, string) {
	plan := memberPlan{member: member, email: in.Email, role: in.Role, mentorID: in.MentorUserID, name: in.Name, slug: in.Slug}
	actor := actorFrom(r.Context())
	if in.Role != member.Role {
		if (in.Role == "coordinator" || member.Role == "coordinator") && !canGrantCoordinator(actor, season.ID) {
			return plan, "Coordinator access cannot be changed."
		}
		plan.setRole = true
		plan.changes = append(plan.changes, importChange{"role", member.Role, in.Role})
	}
	// A role change away from student ends their mentorship.
	if plan.setRole && member.Role == "student" && member.MentorUserID != "" && in.MentorUserID == "" {
		plan.changes = append(plan.changes, importChange{"mentor", member.MentorName, ""})
	}
	if in.MentorUserID != "" && (in.MentorUserID != member.MentorUserID || plan.setRole) {
		plan.setMentor = true
		plan.changes = append(plan.changes, importChange{"mentor", member.MentorName, mentorName(mentors, in.MentorUserID)})
	}
	profileAdmin := actor.HasGlobalRole(authz.SystemAdmin)
	if in.Name != "" && in.Name != member.Name {
		change := importChange{"name", member.Name, in.Name}
		if profileAdmin {
			plan.setName = true
			plan.changes = append(plan.changes, change)
		} else {
			plan.skipped = append(plan.skipped, change)
		}
	}
	if in.Slug != "" && in.Slug != member.Slug {
		change := importChange{"slug", member.Slug, in.Slug}
		if !profileAdmin {
			plan.skipped = append(plan.skipped, change)
		} else if problem := a.slugProblem(r, in.Slug, member.UserID); problem != "" {
			return plan, problem
		} else {
			plan.setSlug = true
			plan.changes = append(plan.changes, change)
		}
	}
	return plan, ""
}

func mentorName(mentors []dal.SeasonMentor, userID string) string {
	for _, m := range mentors {
		if m.UserID == userID {
			return m.Name
		}
	}
	return ""
}

// applyMemberUpdate carries out a member's planned changes, then emails them
// if their role changed.
func (a *API) applyMemberUpdate(r *http.Request, season program.SeasonRecord, plan memberPlan) string {
	actorID := actorFrom(r.Context()).UserID
	now := time.Now().UTC()
	member := plan.member
	if plan.setRole {
		level := "not_applicable"
		if plan.role == "student" {
			level = "beginner"
		}
		if _, err := a.db.UpdateEnrollmentRoleAndLevel(r.Context(), dal.UpdateEnrollmentRoleAndLevelInput{
			SeasonID: season.ID, EnrollmentID: member.EnrollmentID, Role: plan.role, StudentLevel: level, ActorID: actorID, ChangedAt: now,
		}); err != nil {
			return invitationStoreProblem(a, err)
		}
		// Changing role ends the student's mentorship; any new one is created below.
		member.MentorshipID = ""
	}
	if plan.setMentor {
		var err error
		if member.MentorshipID != "" {
			_, err = a.db.UpdateMentorship(r.Context(), dal.UpdateMentorshipInput{
				SeasonID: season.ID, MentorshipID: member.MentorshipID, MentorUserID: plan.mentorID, StudentUserID: member.UserID, ActorID: actorID, ChangedAt: now,
			})
		} else {
			_, err = a.db.CreateMentorship(r.Context(), program.MentorshipRecord{ID: id.New(), SeasonID: season.ID, MentorUserID: plan.mentorID, StudentUserID: member.UserID}, actorID, now)
		}
		if err != nil {
			return invitationStoreProblem(a, err)
		}
	}
	if plan.setName || plan.setSlug {
		profile, err := a.db.GetAdminProfile(r.Context(), member.UserID)
		if err != nil {
			return invitationStoreProblem(a, err)
		}
		if plan.setName {
			profile.Name = plan.name
		}
		if plan.setSlug {
			profile.Slug = plan.slug
		}
		if _, err := a.db.UpdateAdminProfile(r.Context(), profile, actorID, now); err != nil {
			return invitationStoreProblem(a, err)
		}
	}
	// A student becoming a mentor is not emailed; other role changes are.
	promotedToMentor := member.Role == "student" && plan.role == "mentor"
	if plan.setRole && a.sendRoleChange != nil && !promotedToMentor {
		if a.afterCommit != nil {
			role, previous, email := plan.role, member.Role, plan.email
			name := plan.name
			if !plan.setName || name == "" {
				name = member.Name
			}
			enrollmentID := member.EnrollmentID
			*a.afterCommit = append(*a.afterCommit, func(root *API) {
				if err := root.sendRoleChange(r.Context(), authadmin.RoleChangeEmailInput{
					ID: id.New(), To: email, Name: name, Season: season.Name,
					Role: role, PreviousRole: previous,
					URL: strings.TrimRight(root.publicOrigin, "/") + "/seasons/" + season.Slug,
				}); err != nil {
					root.logger.Error("role change email enqueue failed", "enrollmentId", enrollmentID)
				}
			})
			return ""
		}
		name := plan.name
		if !plan.setName || name == "" {
			name = member.Name
		}
		if err := a.sendRoleChange(r.Context(), authadmin.RoleChangeEmailInput{
			ID: id.New(), To: plan.email, Name: name, Season: season.Name,
			Role: plan.role, PreviousRole: member.Role,
			URL: strings.TrimRight(a.publicOrigin, "/") + "/seasons/" + season.Slug,
		}); err != nil {
			a.logger.Error("role change email enqueue failed", "enrollmentId", member.EnrollmentID)
		}
	}
	return ""
}

// matchMentor finds the season mentor an import names, by email, username or
// full name (ignoring case). It explains when there is no match, or more than
// one.
func matchMentor(mentors []dal.SeasonMentor, named string) (string, string) {
	lowered := strings.ToLower(strings.TrimPrefix(named, "@"))
	for _, m := range mentors {
		if m.Email != "" && m.Email == lowered || m.Slug != "" && m.Slug == lowered {
			return m.UserID, ""
		}
	}
	var found []string
	for _, m := range mentors {
		if sameName(m.Name, named) {
			found = append(found, m.UserID)
		}
	}
	switch len(found) {
	case 1:
		return found[0], ""
	case 0:
		return "", fmt.Sprintf("No mentor called %q in this season.", named)
	default:
		return "", fmt.Sprintf("More than one mentor is called %q; use their username or email.", named)
	}
}

// pendingMentor is someone this import invites as a mentor or coordinator, who
// can be named as a mentor by a later row though they have no account here yet.
type pendingMentor struct {
	InvitationID string
	Email, Slug  string
	Name         string
	// Failed is set when the mentor's own row has an error, so the student
	// is told to fix that rather than that the mentor does not exist.
	Failed bool
}

// pendingMentors lists the mentor rows that invite someone new.
func pendingMentors(rows []invitationBatchRow, results []invitationBatchResult) []pendingMentor {
	var pending []pendingMentor
	for i, result := range results {
		if result.Person == "member" {
			continue
		}
		failed := result.Outcome == "failed"
		if !failed && result.Outcome != "invited" && result.Outcome != "invite" {
			continue
		}
		mentor := pendingMentor{
			Email:  strings.ToLower(strings.TrimSpace(rows[i].Email)),
			Slug:   result.Slug,
			Name:   strings.TrimSpace(rows[i].Name),
			Failed: failed,
		}
		if result.Invitation != nil {
			mentor.InvitationID = result.Invitation.ID
		}
		pending = append(pending, mentor)
	}
	return pending
}

// matchPendingMentor finds the invited mentor a row names by email, slug or
// full name.
func matchPendingMentor(pending []pendingMentor, named string) (found pendingMentor, ok, ambiguous bool) {
	lowered := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(named), "@"))
	// Anyone the text names by email, slug or full name; the same person listed
	// on several rows (the same email) counts once.
	var matches []pendingMentor
	seen := map[string]int{}
	for _, p := range pending {
		if !(p.Email != "" && p.Email == lowered || p.Slug != "" && p.Slug == lowered || sameName(p.Name, named)) {
			continue
		}
		key := "email:" + p.Email
		if p.Email == "" {
			key = "invitation:" + p.InvitationID
		}
		if at, again := seen[key]; again {
			if matches[at].InvitationID == "" || matches[at].Failed && !p.Failed {
				matches[at] = p
			}
			continue
		}
		seen[key] = len(matches)
		matches = append(matches, p)
	}
	if len(matches) == 1 {
		return matches[0], true, false
	}
	return pendingMentor{}, false, len(matches) > 1
}

// invitationStoreProblem says why storing one imported invitation failed.
func invitationStoreProblem(a *API, err error) string {
	switch {
	case errors.Is(err, dal.ErrDuplicate):
		return "Already a member of, or invited to, this season."
	case errors.Is(err, dal.ErrInvalidMentor):
		return err.Error()
	case errors.Is(err, dal.ErrConflict):
		return "This season is not open for invitations."
	default:
		a.logger.Error("invitation import row failed", "error", err)
		return "This invitation could not be created."
	}
}

func (a *API) resendInvitation(w http.ResponseWriter, r *http.Request) {
	a.changeInvitation(w, r, false)
}
func (a *API) cancelInvitation(w http.ResponseWriter, r *http.Request) {
	a.changeInvitation(w, r, true)
}
func (a *API) changeInvitation(w http.ResponseWriter, r *http.Request, cancel bool) {
	if !a.authorizeInvitations(w, r, true) {
		return
	}
	token, hash, err := invitationToken()
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	v, err := a.db.ChangeInvitation(r.Context(), r.PathValue("id"), r.PathValue("invitationId"), hash, id.New(), actorFrom(r.Context()).UserID, cancel, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if !cancel {
		a.sendInvitationEmail(r, &v, token)
	}
	writeJSONResponse(w, 200, v)
}
func invitationRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r.Body, &request); err != nil || len(request.Token) != 43 {
		writeErrorResponse(w, 400, "This invitation link is invalid.")
		return "", false
	}
	return hashInvitationToken(request.Token), true
}

// getInvitationInvitee tells a signed-out visitor holding an invitation link
// who it was sent to, so sign-up can start with that name and email. The token
// is a 256-bit secret delivered only to that inbox.
func (a *API) getInvitationInvitee(w http.ResponseWriter, r *http.Request) {
	if !a.validOrigin(r) {
		writeErrorResponse(w, http.StatusForbidden, "Requests must come from the configured application origin.")
		return
	}
	hash, ok := invitationRequest(w, r)
	if !ok {
		return
	}
	name, email, err := a.db.InvitationInvitee(r.Context(), hash, time.Now().UTC())
	if errors.Is(err, dal.ErrNotFound) {
		writeErrorResponse(w, http.StatusNotFound, "This invitation is expired, cancelled or already used.")
		return
	}
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]string{"name": name, "email": email})
}

func (a *API) previewInvitation(w http.ResponseWriter, r *http.Request) {
	hash, ok := invitationRequest(w, r)
	if !ok {
		return
	}
	v, err := a.db.InvitationForUser(r.Context(), hash, actorFrom(r.Context()).UserID)
	if errors.Is(err, dal.ErrNotFound) {
		writeErrorResponse(w, 404, "This invitation is expired, cancelled, already used, or belongs to another email address.")
		return
	}
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, v)
}
func (a *API) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	hash, ok := invitationRequest(w, r)
	if !ok {
		return
	}
	v, err := a.db.AcceptInvitation(r.Context(), hash, actorFrom(r.Context()).UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, 200, v)
}

// sameName compares full names ignoring case and any run of whitespace, so a
// pasted double space or non-breaking space does not hide a match.
func sameName(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

// slugAllocator hands out the usernames of people an import invites. A name
// written in a row is theirs (and an error if taken); otherwise one is made
// from their name, numbered (maddie-frew2, maddie-frew3…) when taken, whether
// by someone already on the site or by another row of the same import. The
// same person listed again keeps the same one.
type slugAllocator struct {
	owner map[string]string // slug → email of the row that has it
	made  map[string]string // email → the slug made for them
}

func newSlugAllocator(rows []invitationBatchRow) *slugAllocator {
	a := &slugAllocator{owner: map[string]string{}, made: map[string]string{}}
	for _, row := range rows {
		if slug := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(row.Slug), "@")); slug != "" {
			if _, taken := a.owner[slug]; !taken {
				a.owner[slug] = strings.ToLower(strings.TrimSpace(row.Email))
			}
		}
	}
	return a
}

func (al *slugAllocator) next(r *http.Request, a *API, name, email string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(email))
	if slug, ok := al.made[key]; ok {
		return slug, nil
	}
	base := dal.NameSlug(name)
	for n := 1; ; n++ {
		candidate := base
		if n > 1 {
			candidate = fmt.Sprintf("%s%d", base, n)
		}
		if _, taken := al.owner[candidate]; taken {
			continue
		}
		owner, err := a.db.ProfileSlugOwner(r.Context(), candidate)
		if err != nil {
			return "", err
		}
		if owner != "" {
			continue
		}
		al.owner[candidate], al.made[key] = strings.ToLower(strings.TrimSpace(email)), candidate
		return candidate, nil
	}
}
