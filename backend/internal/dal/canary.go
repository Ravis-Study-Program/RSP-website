package dal

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
)

// CanarySeasonSlug is the seeded season that canary self-service enrols into.
const CanarySeasonSlug = "dev-season"

// CanaryRemovedRole is the season role value for a kicked or withdrawn enrollment.
const CanaryRemovedRole = "removed"

const canaryReason = "Canary self-service access change"

type CanaryRoleState struct {
	Role  string `json:"role"`
	State string `json:"state"`
}

type CanarySeason struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// CanaryAccess is the caller's own access in a canary environment.
type CanaryAccess struct {
	Season        CanarySeason      `json:"season"`
	SeasonRole    *CanaryRoleState  `json:"seasonRole"`
	GlobalRoles   []CanaryRoleState `json:"globalRoles"`
	MFAConfigured bool              `json:"mfaConfigured"`
}

type SetCanaryAccessInput struct {
	UserID string
	// GlobalRoles is the complete wanted set; roles left out are revoked.
	GlobalRoles []string
	// SeasonRole is student, mentor, coordinator or CanaryRemovedRole.
	SeasonRole string
	ChangedAt  time.Time
}

// GetCanaryAccess reads the caller's global roles and canary season enrollment.
func (p *Store) GetCanaryAccess(ctx context.Context, userID string) (CanaryAccess, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return CanaryAccess{}, err
	}
	defer tx.Rollback(context.Background())
	return readCanaryAccess(ctx, tx, userID)
}

// SetCanaryAccess lets a canary user choose their own roles. Database triggers
// still hold privileged roles in pending_mfa until the user configures TOTP.
func (p *Store) SetCanaryAccess(ctx context.Context, input SetCanaryAccessInput) (CanaryAccess, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return CanaryAccess{}, err
	}
	defer tx.Rollback(context.Background())

	changedAt := input.ChangedAt.UTC()
	if err := setCanaryGlobalRoles(ctx, tx, input.UserID, input.GlobalRoles, changedAt); err != nil {
		return CanaryAccess{}, err
	}
	var seasonID string
	if err := tx.QueryRow(ctx, `SELECT id FROM app.seasons WHERE slug=$1 AND deleted_at IS NULL`, CanarySeasonSlug).Scan(&seasonID); err != nil {
		return CanaryAccess{}, noRows(err)
	}
	if err := lockOpenSeason(ctx, tx, seasonID); err != nil {
		return CanaryAccess{}, err
	}
	if err := setCanarySeasonRole(ctx, tx, input.UserID, seasonID, input.SeasonRole, changedAt); err != nil {
		return CanaryAccess{}, err
	}
	if err := appendAuditTx(ctx, tx, newAudit(input.UserID, "canary.access_changed", "user", input.UserID, map[string]any{"globalRoles": input.GlobalRoles, "seasonRole": input.SeasonRole}, changedAt)); err != nil {
		return CanaryAccess{}, err
	}

	access, err := readCanaryAccess(ctx, tx, input.UserID)
	if err != nil {
		return access, err
	}
	return access, tx.Commit(ctx)
}

func setCanaryGlobalRoles(ctx context.Context, tx pgx.Tx, userID string, wanted []string, changedAt time.Time) error {
	want := make(map[string]bool, len(wanted))
	for _, role := range wanted {
		want[role] = true
	}
	for _, role := range []string{"director", "system_admin"} {
		var err error
		if want[role] {
			// The MFA trigger decides between pending_mfa and active.
			_, err = tx.Exec(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at)
				VALUES($1,$2,$3,'pending_mfa',$2,$4)
				ON CONFLICT(user_id,role) DO UPDATE
				SET state='pending_mfa',granted_by_user_id=EXCLUDED.granted_by_user_id,
					granted_at=EXCLUDED.granted_at,activated_at=NULL,revoked_at=NULL
				WHERE app.global_role_assignments.state='revoked'`, id.New(), userID, role, changedAt)
		} else {
			_, err = tx.Exec(ctx, `UPDATE app.global_role_assignments
				SET state='revoked',revoked_at=$3,activated_at=NULL
				WHERE user_id=$1 AND role=$2 AND state<>'revoked'`, userID, role, changedAt)
		}
		if err != nil {
			return mapDatabaseError(err)
		}
	}
	return nil
}

func setCanarySeasonRole(ctx context.Context, tx pgx.Tx, userID, seasonID, role string, changedAt time.Time) error {
	var enrollmentID, currentRole, currentState string
	err := tx.QueryRow(ctx, `SELECT id,role::text,state::text FROM app.enrollments WHERE user_id=$1 AND season_id=$2 FOR UPDATE`, userID, seasonID).Scan(&enrollmentID, &currentRole, &currentState)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if role == CanaryRemovedRole {
		if !exists || currentState != "active" {
			return nil
		}
		return removeCanaryEnrollment(ctx, tx, enrollmentID, seasonID, userID, currentRole, changedAt)
	}

	studentLevel := "not_applicable"
	if role == "student" {
		studentLevel = "beginner"
	}
	if !exists {
		enrollmentID = id.New()
		// The coordinator MFA trigger corrects assignment_state and activated_at.
		if _, err := tx.Exec(ctx, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,assignment_state,activated_at)
			VALUES($1,$2,$3,$4,$5,'active','active',$6)`, enrollmentID, userID, seasonID, role, studentLevel, changedAt); err != nil {
			return mapDatabaseError(err)
		}
	} else if currentRole != role || currentState != "active" {
		if _, err := tx.Exec(ctx, `UPDATE app.enrollments
			SET role=$2::app.season_role,student_level=$3,state='active',deleted_at=NULL,state_changed_at=$4,
				assignment_state='active',activated_at=$4,
				completed_by_close_id=NULL,close_assignment_state=NULL,close_activated_at=NULL
			WHERE id=$1`, enrollmentID, role, studentLevel, changedAt); err != nil {
			return mapDatabaseError(err)
		}
		// Mirror UpdateEnrollmentRoleAndLevel: a role change ends the
		// mentorships the new role can no longer take part in.
		ended := "mentor_enrollment_id"
		if role != "student" {
			ended = "student_enrollment_id"
		}
		if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2 WHERE `+ended+`=$1 AND ended_at IS NULL AND deleted_at IS NULL`, enrollmentID, changedAt); err != nil {
			return err
		}
	} else {
		return nil
	}
	return linkCanaryMentorship(ctx, tx, seasonID, enrollmentID, role, changedAt)
}

// linkCanaryMentorship pairs a new student with the seeded mentor, or hands
// the seeded student to a new mentor or coordinator (a student has at most one
// active mentor), so relationship-scoped pages have someone to show.
func linkCanaryMentorship(ctx context.Context, tx pgx.Tx, seasonID, enrollmentID, role string, changedAt time.Time) error {
	counterpartSlug, counterpartRoles := "dev-student", []string{"student"}
	mentorColumn, studentColumn := "$3", "e.id"
	if role == "student" {
		counterpartSlug, counterpartRoles = "dev-mentor", []string{"mentor", "coordinator"}
		mentorColumn, studentColumn = "e.id", "$3"
	} else if _, err := tx.Exec(ctx, `UPDATE app.mentorships m SET ended_at=$3
		FROM app.enrollments e JOIN app.user_profiles p ON p.user_id=e.user_id
		WHERE m.student_enrollment_id=e.id AND e.season_id=$1 AND p.slug='dev-student'
		  AND m.mentor_enrollment_id<>$2 AND m.ended_at IS NULL AND m.deleted_at IS NULL`, seasonID, enrollmentID, changedAt); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id)
		SELECT $1,$2,`+mentorColumn+`,`+studentColumn+`
		FROM app.enrollments e
		JOIN app.user_profiles p ON p.user_id=e.user_id
		WHERE e.season_id=$2 AND p.slug=$4 AND e.role::text=ANY($5) AND e.state='active' AND e.deleted_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM app.mentorships m
		    WHERE m.mentor_enrollment_id=`+mentorColumn+` AND m.student_enrollment_id=`+studentColumn+`
		      AND m.ended_at IS NULL AND m.deleted_at IS NULL
		  )`, id.New(), seasonID, enrollmentID, counterpartSlug, counterpartRoles)
	return mapDatabaseError(err)
}

func removeCanaryEnrollment(ctx context.Context, tx pgx.Tx, enrollmentID, seasonID, userID, role string, changedAt time.Time) error {
	state := "kicked"
	if role != "student" {
		state = "withdrawn"
	}
	if _, err := tx.Exec(ctx, `UPDATE app.enrollments SET state=$2,state_changed_at=$3,assignment_state='revoked',activated_at=NULL WHERE id=$1`, enrollmentID, state, changedAt); err != nil {
		return mapDatabaseError(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.mentorships SET ended_at=$2 WHERE (student_enrollment_id=$1 OR mentor_enrollment_id=$1) AND ended_at IS NULL AND deleted_at IS NULL`, enrollmentID, changedAt); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO app.enrollment_removal_events(id,enrollment_id,season_id,subject_user_id,actor_user_id,resulting_state,reason,occurred_at) VALUES($1,$2,$3,$4,$4,$5,$6,$7)`, id.New(), enrollmentID, seasonID, userID, state, canaryReason, changedAt)
	return err
}

func readCanaryAccess(ctx context.Context, tx pgx.Tx, userID string) (CanaryAccess, error) {
	var access CanaryAccess
	if err := tx.QueryRow(ctx, `SELECT id,slug,name FROM app.seasons WHERE slug=$1 AND deleted_at IS NULL`, CanarySeasonSlug).Scan(&access.Season.ID, &access.Season.Slug, &access.Season.Name); err != nil {
		return access, noRows(err)
	}
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT mfa_configured FROM app.user_security WHERE user_id=$1),false)`, userID).Scan(&access.MFAConfigured); err != nil {
		return access, err
	}

	var role, state, assignmentState string
	err := tx.QueryRow(ctx, `SELECT role::text,state::text,assignment_state::text FROM app.enrollments WHERE user_id=$1 AND season_id=$2`, userID, access.Season.ID).Scan(&role, &state, &assignmentState)
	switch {
	case err == nil && state == "active":
		access.SeasonRole = &CanaryRoleState{Role: role, State: assignmentState}
	case err == nil:
		access.SeasonRole = &CanaryRoleState{Role: CanaryRemovedRole, State: state}
	case !errors.Is(err, pgx.ErrNoRows):
		return access, err
	}

	rows, err := tx.Query(ctx, `SELECT role::text,state::text FROM app.global_role_assignments WHERE user_id=$1 AND state<>'revoked' ORDER BY role`, userID)
	if err != nil {
		return access, err
	}
	access.GlobalRoles, err = pgx.CollectRows(rows, pgx.RowToStructByPos[CanaryRoleState])
	if access.GlobalRoles == nil {
		access.GlobalRoles = []CanaryRoleState{}
	}
	return access, err
}
