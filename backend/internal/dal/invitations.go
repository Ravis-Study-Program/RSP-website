package dal

import (
	"context"
	"errors"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/id"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/program"
	"github.com/jackc/pgx/v5"
	"time"
)

const invitationColumns = `i.id,i.season_id,s.name,s.slug,i.name,i.email,i.role::text,
 CASE WHEN i.accepted_at IS NOT NULL THEN 'accepted' WHEN i.cancelled_at IS NOT NULL THEN 'cancelled' WHEN i.expires_at<=now() THEN 'expired' ELSE 'pending' END,
 i.expires_at,i.sent_at,i.delivery_id,i.mentor_user_id,i.accepted_by,i.profile_slug`

func scanInvitation(row pgx.Row) (program.Invitation, error) {
	var v program.Invitation
	err := row.Scan(&v.ID, &v.SeasonID, &v.SeasonName, &v.SeasonSlug, &v.Name, &v.Email, &v.Role, &v.Status, &v.ExpiresAt, &v.SentAt, &v.DeliveryID, &v.MentorUserID, &v.AcceptedBy, &v.ProfileSlug)
	return v, noRows(err)
}

// Taking the season lock serializes enrollment/invitation writes with closure
// and empty-season deletion.
func lockOpenSeason(ctx context.Context, tx pgx.Tx, seasonID string) error {
	var status string
	if err := tx.QueryRow(ctx, `SELECT status::text FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, seasonID).Scan(&status); err != nil {
		return noRows(err)
	}
	if status != "open" {
		return ErrConflict
	}
	return nil
}

func (p *Store) ListInvitations(ctx context.Context, seasonID, boundary string, limit int) ([]program.Invitation, bool, int64, error) {
	var total int64
	if err := p.conn().QueryRow(ctx, `SELECT count(*) FROM app.season_invitations WHERE season_id=$1`, seasonID).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.conn().Query(ctx, `SELECT `+invitationColumns+` FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id WHERE i.season_id=$1 AND ($2='' OR i.id>NULLIF($2,'')::uuid) ORDER BY i.id LIMIT $3`, seasonID, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items := []program.Invitation{}
	for rows.Next() {
		v, err := scanInvitation(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, v)
	}
	items, more := finishPage(items, limit, "forward")
	return items, more, total, rows.Err()
}

func (p *Store) CreateInvitation(ctx context.Context, v program.Invitation, hash, actorID string, at time.Time) (program.Invitation, error) {
	tx, err := p.conn().Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(context.Background())
	if err = lockOpenSeason(ctx, tx, v.SeasonID); err != nil {
		return v, err
	}
	var enrolled bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.enrollments e JOIN app.user_contacts c ON c.user_id=e.user_id WHERE e.season_id=$1 AND lower(c.email)=$2)`, v.SeasonID, v.Email).Scan(&enrolled); err != nil {
		return v, err
	}
	if enrolled {
		return v, ErrDuplicate
	}
	if v.MentorUserID != nil {
		// The mentor must be mentoring in this season when invited.
		var mentors bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.enrollments WHERE season_id=$1 AND user_id=$2 AND role IN ('mentor','coordinator') AND state='active' AND deleted_at IS NULL)`, v.SeasonID, *v.MentorUserID).Scan(&mentors); err != nil {
			return v, err
		}
		if !mentors {
			return v, ErrInvalidMentor
		}
	}
	if v.MentorInvitationID != nil {
		// The mentor's invitation must be a live one for a mentor or
		// coordinator in this season.
		var live bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_invitations WHERE id=$1 AND season_id=$2 AND role IN ('mentor','coordinator') AND cancelled_at IS NULL)`, *v.MentorInvitationID, v.SeasonID).Scan(&live); err != nil {
			return v, err
		}
		if !live {
			return v, ErrInvalidMentor
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO app.season_invitations(id,season_id,name,email,role,token_hash,delivery_id,expires_at,created_by,mentor_user_id,profile_slug,mentor_invitation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, v.ID, v.SeasonID, v.Name, v.Email, v.Role, hash, v.DeliveryID, v.ExpiresAt, actorID, v.MentorUserID, v.ProfileSlug, v.MentorInvitationID); err != nil {
		return v, mapDatabaseError(err)
	}
	if err = appendAuditTx(ctx, tx, newAudit(actorID, "invitation.created", "invitation", v.ID, map[string]any{"seasonId": v.SeasonID, "role": v.Role}, at)); err != nil {
		return v, err
	}
	v, err = scanInvitation(tx.QueryRow(ctx, `SELECT `+invitationColumns+` FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id WHERE i.id=$1`, v.ID))
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) ChangeInvitation(ctx context.Context, seasonID, inviteID, hash, deliveryID, actorID string, cancel bool, at time.Time) (program.Invitation, error) {
	tx, err := p.conn().Begin(ctx)
	if err != nil {
		return program.Invitation{}, err
	}
	defer tx.Rollback(context.Background())
	if err = lockOpenSeason(ctx, tx, seasonID); err != nil {
		return program.Invitation{}, err
	}
	v, err := scanInvitation(tx.QueryRow(ctx, `SELECT `+invitationColumns+` FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id WHERE i.id=$1 AND i.season_id=$2 AND i.accepted_at IS NULL AND i.cancelled_at IS NULL FOR UPDATE OF i`, inviteID, seasonID))
	if err != nil {
		return v, err
	}
	action := "invitation.resent"
	if cancel {
		action = "invitation.cancelled"
		_, err = tx.Exec(ctx, `UPDATE app.season_invitations SET cancelled_at=$2 WHERE id=$1`, inviteID, at)
		v.Status = "cancelled"
	} else {
		v.DeliveryID = deliveryID
		v.ExpiresAt = at.Add(7 * 24 * time.Hour)
		v.SentAt = nil
		v.Status = "pending"
		_, err = tx.Exec(ctx, `UPDATE app.season_invitations SET token_hash=$2,delivery_id=$3,expires_at=$4,sent_at=NULL WHERE id=$1`, inviteID, hash, deliveryID, v.ExpiresAt)
	}
	if err != nil {
		return v, mapDatabaseError(err)
	}
	if err = appendAuditTx(ctx, tx, newAudit(actorID, action, "invitation", v.ID, map[string]any{"seasonId": seasonID}, at)); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
func (p *Store) MarkInvitationSent(ctx context.Context, inviteID, deliveryID string, at time.Time) error {
	_, err := p.conn().Exec(ctx, `UPDATE app.season_invitations SET sent_at=$3 WHERE id=$1 AND delivery_id=$2`, inviteID, deliveryID, at)
	return err
}

// SeasonMemberByEmail is an active member of a season found by email, with
// the details an import may change.
type SeasonMemberByEmail struct {
	EnrollmentID string
	UserID       string
	Role         string
	Name         string
	Slug         string
	// The student's current mentorship, if any.
	MentorshipID string
	MentorUserID string
	MentorName   string
}

// FindSeasonMemberByEmail finds the active member of a season with this
// email address; found is false when there is none.
func (p *Store) FindSeasonMemberByEmail(ctx context.Context, seasonID, email string) (SeasonMemberByEmail, bool, error) {
	var m SeasonMemberByEmail
	err := p.conn().QueryRow(ctx, `SELECT e.id::text, e.user_id::text, e.role::text, coalesce(pr.display_name,''), coalesce(pr.slug,''),
 coalesce(ms.id::text,''), coalesce(mentor.user_id::text,''), coalesce(mp.display_name,'')
 FROM app.enrollments e JOIN app.user_contacts c ON c.user_id=e.user_id LEFT JOIN app.user_profiles pr ON pr.user_id=e.user_id
 LEFT JOIN app.mentorships ms ON ms.student_enrollment_id=e.id AND ms.ended_at IS NULL AND ms.deleted_at IS NULL
 LEFT JOIN app.enrollments mentor ON mentor.id=ms.mentor_enrollment_id
 LEFT JOIN app.user_profiles mp ON mp.user_id=mentor.user_id
 WHERE e.season_id=$1 AND lower(c.email)=lower($2) AND e.state='active' AND e.deleted_at IS NULL
 LIMIT 1`, seasonID, email).Scan(&m.EnrollmentID, &m.UserID, &m.Role, &m.Name, &m.Slug, &m.MentorshipID, &m.MentorUserID, &m.MentorName)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, false, nil
	}
	return m, err == nil, err
}

// AccountExistsByEmail reports whether an active account uses this email.
func (p *Store) AccountExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := p.conn().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.user_contacts c JOIN app.users u ON u.id=c.user_id
 WHERE lower(c.email)=lower($1) AND u.deleted_at IS NULL)`, email).Scan(&exists)
	return exists, err
}

// ActiveAccountIDByEmail is the id of the active account with this email, or
// "" when there is none.
func (p *Store) ActiveAccountIDByEmail(ctx context.Context, email string) (string, error) {
	var userID string
	err := p.conn().QueryRow(ctx, `SELECT u.id::text FROM app.user_contacts c JOIN app.users u ON u.id=c.user_id
 WHERE lower(c.email)=lower($1) AND u.account_state='active' AND u.deleted_at IS NULL LIMIT 1`, email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return userID, err
}

// AccountSlugByEmail is the username of the account with this email, or "".
func (p *Store) AccountSlugByEmail(ctx context.Context, email string) (string, error) {
	var slug string
	err := p.conn().QueryRow(ctx, `SELECT lower(pr.slug) FROM app.user_contacts c JOIN app.users u ON u.id=c.user_id
 JOIN app.user_profiles pr ON pr.user_id=u.id WHERE lower(c.email)=lower($1) AND u.deleted_at IS NULL LIMIT 1`, email).Scan(&slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return slug, err
}

// SeasonMentor is someone who can mentor in a season: an active mentor or
// coordinator, with the name and email an import may use to name them.
type SeasonMentor struct {
	UserID string
	Name   string
	Email  string
	Slug   string
}

// SeasonMentors lists a season's active mentors and coordinators.
func (p *Store) SeasonMentors(ctx context.Context, seasonID string) ([]SeasonMentor, error) {
	rows, err := p.conn().Query(ctx, `SELECT e.user_id::text, coalesce(pr.display_name,''), coalesce(lower(c.email),''), coalesce(lower(pr.slug),'')
 FROM app.enrollments e LEFT JOIN app.user_profiles pr ON pr.user_id=e.user_id LEFT JOIN app.user_contacts c ON c.user_id=e.user_id
 WHERE e.season_id=$1 AND e.role IN ('mentor','coordinator') AND e.state='active' AND e.deleted_at IS NULL`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	mentors := []SeasonMentor{}
	for rows.Next() {
		var m SeasonMentor
		if err := rows.Scan(&m.UserID, &m.Name, &m.Email, &m.Slug); err != nil {
			return nil, err
		}
		mentors = append(mentors, m)
	}
	return mentors, rows.Err()
}

// ProfileSlugOwner is the user whose profile uses this slug, or "" if none.
func (p *Store) ProfileSlugOwner(ctx context.Context, slug string) (string, error) {
	var owner string
	err := p.conn().QueryRow(ctx, `SELECT user_id::text FROM app.user_profiles WHERE lower(slug)=lower($1)`, slug).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return owner, err
}

// PendingInvitationExists reports whether this email has an open invitation
// to the season.
func (p *Store) PendingInvitationExists(ctx context.Context, seasonID, email string) (bool, error) {
	var exists bool
	err := p.conn().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_invitations WHERE season_id=$1 AND email=lower($2)
 AND accepted_at IS NULL AND cancelled_at IS NULL AND expires_at>now())`, seasonID, email).Scan(&exists)
	return exists, err
}

// InvitationInvitee is the name and email a live invitation was sent to.
func (p *Store) InvitationInvitee(ctx context.Context, hash string, at time.Time) (string, string, error) {
	var name, email string
	err := p.conn().QueryRow(ctx, `SELECT i.name, i.email FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id
 WHERE i.token_hash=$1 AND i.expires_at>$2 AND i.accepted_at IS NULL AND i.cancelled_at IS NULL AND s.deleted_at IS NULL`, hash, at).Scan(&name, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return name, email, err
}

// InvitationSentTo reports whether a live invitation with this token was sent
// to this email address.
func (p *Store) InvitationSentTo(ctx context.Context, hash, email string, at time.Time) (bool, error) {
	var matches bool
	err := p.conn().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id
 WHERE i.token_hash=$1 AND i.email=lower($2) AND i.expires_at>$3 AND i.accepted_at IS NULL AND i.cancelled_at IS NULL AND s.deleted_at IS NULL)`, hash, email, at).Scan(&matches)
	return matches, err
}

func (p *Store) InvitationForUser(ctx context.Context, hash, userID string) (program.Invitation, error) {
	return scanInvitation(p.conn().QueryRow(ctx, `SELECT `+invitationColumns+` FROM app.season_invitations i JOIN app.seasons s ON s.id=i.season_id JOIN app.user_contacts c ON lower(c.email)=i.email JOIN app.users u ON u.id=c.user_id
 WHERE i.token_hash=$1 AND c.user_id=$2 AND u.account_state='active' AND u.deleted_at IS NULL AND i.expires_at>now() AND i.accepted_at IS NULL AND i.cancelled_at IS NULL AND s.deleted_at IS NULL`, hash, userID))
}
func (p *Store) AcceptInvitation(ctx context.Context, hash, userID string, at time.Time) (program.EnrollmentRecord, error) {
	v, err := p.InvitationForUser(ctx, hash, userID)
	if err != nil {
		return program.EnrollmentRecord{}, err
	}
	tx, err := p.conn().Begin(ctx)
	if err != nil {
		return program.EnrollmentRecord{}, err
	}
	defer tx.Rollback(context.Background())
	if err = lockOpenSeason(ctx, tx, v.SeasonID); err != nil {
		return program.EnrollmentRecord{}, err
	}
	var inviteID string
	err = tx.QueryRow(ctx, `SELECT i.id FROM app.season_invitations i JOIN app.user_contacts c ON lower(c.email)=i.email JOIN app.users u ON u.id=c.user_id WHERE i.token_hash=$1 AND c.user_id=$2 AND u.account_state='active' AND u.deleted_at IS NULL AND i.expires_at>$3 AND i.accepted_at IS NULL AND i.cancelled_at IS NULL FOR UPDATE OF i,c,u`, hash, userID, at).Scan(&inviteID)
	if err != nil {
		return program.EnrollmentRecord{}, noRows(err)
	}
	member := program.EnrollmentRecord{ID: id.New(), SeasonID: v.SeasonID, SeasonSlug: v.SeasonSlug, UserID: userID, Role: v.Role, State: "active", StudentLevel: "not_applicable", AssignmentState: "active"}
	if member.Role == "student" {
		member.StudentLevel = "beginner"
	}
	err = tx.QueryRow(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state) VALUES($1,$2,$3,$4,$5,'active') RETURNING assignment_state::text`, member.ID, member.UserID, member.SeasonID, member.Role, member.StudentLevel).Scan(&member.AssignmentState)
	if err != nil {
		return member, mapDatabaseError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE app.season_invitations SET accepted_at=$2,accepted_by=$3 WHERE id=$1`, inviteID, at, userID); err != nil {
		return member, err
	}
	// The username the invitation names goes to an account made for it (one
	// created after the invitation), if nobody else has taken it meanwhile.
	if v.ProfileSlug != nil {
		if _, err = tx.Exec(ctx, `UPDATE app.user_profiles p SET slug=$2
			FROM app.users u, app.season_invitations i
			WHERE p.user_id=$1 AND u.id=p.user_id AND i.id=$3 AND u.created_at>=i.created_at
			AND NOT EXISTS (SELECT 1 FROM app.user_profiles other WHERE lower(other.slug)=$2 AND other.user_id<>$1)`, userID, *v.ProfileSlug, inviteID); err != nil {
			return member, mapDatabaseError(err)
		}
	}
	// An invitation that named a mentor assigns them now, if they still mentor
	// in the season; otherwise the student is left for a coordinator to assign.
	if v.MentorUserID != nil {
		mentorshipID := id.New()
		tag, err := tx.Exec(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id)
			SELECT $1,$2,mentor.id,$4 FROM app.enrollments mentor
			WHERE mentor.season_id=$2 AND mentor.user_id=$3 AND mentor.role IN ('mentor','coordinator') AND mentor.state='active' AND mentor.deleted_at IS NULL`,
			mentorshipID, v.SeasonID, *v.MentorUserID, member.ID)
		if err != nil {
			return member, mapDatabaseError(err)
		}
		if tag.RowsAffected() == 1 {
			if err = appendAuditTx(ctx, tx, newAudit(userID, "mentorship.created", "mentorship", mentorshipID, map[string]any{"invitationId": inviteID}, at)); err != nil {
				return member, err
			}
		}
	}
	// A student whose mentor was invited in the same import: assign them now if
	// the mentor has already joined; otherwise it happens when the mentor does.
	if err = assignInvitedMentor(ctx, tx, inviteID, member.ID, v.SeasonID, userID, at); err != nil {
		return member, err
	}
	if err = appendAuditTx(ctx, tx, newAudit(userID, "invitation.accepted", "invitation", inviteID, map[string]any{"seasonId": v.SeasonID, "enrollmentId": member.ID, "role": member.Role}, at)); err != nil {
		return member, err
	}
	return member, tx.Commit(ctx)
}

// assignInvitedMentor makes the mentorships an accepted invitation settles.
// A student's invitation may name their mentor's invitation: if that mentor
// has already accepted, the student is assigned to them now. And when this
// invitation is a mentor's, every student whose invitation names it and who
// has already accepted is assigned to this mentor.
func assignInvitedMentor(ctx context.Context, tx pgx.Tx, inviteID, enrollmentID, seasonID, userID string, at time.Time) error {
	create := func(mentorEnrollment, studentEnrollment string) error {
		mentorshipID := id.New()
		tag, err := tx.Exec(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id)
			SELECT $1,$2,$3,$4
			WHERE NOT EXISTS (SELECT 1 FROM app.mentorships WHERE student_enrollment_id=$4 AND ended_at IS NULL AND deleted_at IS NULL)`,
			mentorshipID, seasonID, mentorEnrollment, studentEnrollment)
		if err != nil {
			return mapDatabaseError(err)
		}
		if tag.RowsAffected() == 1 {
			return appendAuditTx(ctx, tx, newAudit(userID, "mentorship.created", "mentorship", mentorshipID, map[string]any{"invitationId": inviteID}, at))
		}
		return nil
	}
	// This student's mentor, if they have joined.
	var mentorEnrollment *string
	err := tx.QueryRow(ctx, `SELECT me.id FROM app.season_invitations i
		JOIN app.season_invitations mi ON mi.id=i.mentor_invitation_id
		JOIN app.enrollments me ON me.user_id=mi.accepted_by AND me.season_id=$2
		 AND me.role IN ('mentor','coordinator') AND me.state='active' AND me.deleted_at IS NULL
		WHERE i.id=$1`, inviteID, seasonID).Scan(&mentorEnrollment)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if mentorEnrollment != nil {
		if err := create(*mentorEnrollment, enrollmentID); err != nil {
			return err
		}
	}
	// Students who accepted before this mentor did.
	rows, err := tx.Query(ctx, `SELECT se.id FROM app.season_invitations i
		JOIN app.enrollments se ON se.user_id=i.accepted_by AND se.season_id=$2 AND se.role='student' AND se.state='active' AND se.deleted_at IS NULL
		WHERE i.mentor_invitation_id=$1`, inviteID, seasonID)
	if err != nil {
		return err
	}
	var students []string
	for rows.Next() {
		var student string
		if err := rows.Scan(&student); err != nil {
			rows.Close()
			return err
		}
		students = append(students, student)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, student := range students {
		if err := create(enrollmentID, student); err != nil {
			return err
		}
	}
	return nil
}

// PendingMentorInvitation is a mentor or coordinator invited to a season who
// has not accepted yet.
type PendingMentorInvitation struct {
	ID, Name, Email string
	Slug            string
}

// PendingMentorInvitations lists the season's live invitations for mentors and
// coordinators, so a student can be invited with one of them as their mentor.
func (p *Store) PendingMentorInvitations(ctx context.Context, seasonID string) ([]PendingMentorInvitation, error) {
	rows, err := p.conn().Query(ctx, `SELECT id,name,email,COALESCE(profile_slug,'') FROM app.season_invitations
		WHERE season_id=$1 AND role IN ('mentor','coordinator') AND accepted_at IS NULL AND cancelled_at IS NULL AND expires_at>now()`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingMentorInvitation
	for rows.Next() {
		var v PendingMentorInvitation
		if err := rows.Scan(&v.ID, &v.Name, &v.Email, &v.Slug); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
