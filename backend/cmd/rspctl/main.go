// Package main provides administrative commands for the RSP backend.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"strings"
	"time"

	"database/sql"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/worker"
	"gorm.io/gorm"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "rspctl:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: rspctl seed|bootstrap-admin")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}

	repository, err := dal.Open(ctx, dsn)
	if err != nil {
		return err
	}

	defer repository.Close()
	switch args[0] {
	case "seed":
		appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
		if appEnv != "development" && appEnv != "test" {
			return errors.New("seed is allowed only when APP_ENV=development or APP_ENV=test")
		}
		options, err := parseSeedOptions(args[1:])
		if err != nil {
			return err
		}
		return seed(ctx, repository.DB, options)
	case "bootstrap-admin":
		set := flag.NewFlagSet("bootstrap-admin", flag.ContinueOnError)
		subject := set.String("auth-subject", "", "Better Auth user id")
		email := set.String("email", "", "verified administrator email")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		return bootstrap(ctx, repository.DB, *subject, *email)
	default:
		return errors.New("usage: rspctl seed|bootstrap-admin")
	}
}

type seedOptions struct {
	StudentEmail     string
	MentorEmail      string
	CoordinatorEmail string
	DirectorEmail    string
	SiteAdminEmail   string
	AllowExisting    bool
}

func parseSeedOptions(args []string) (seedOptions, error) {
	set := flag.NewFlagSet("seed", flag.ContinueOnError)
	var options seedOptions
	set.StringVar(&options.StudentEmail, "student-email", "", "existing verified Better Auth user to enrol as the development student")
	set.StringVar(&options.MentorEmail, "mentor-email", "", "existing verified Better Auth user to enrol as the development mentor")
	set.StringVar(&options.CoordinatorEmail, "coordinator-email", "", "existing verified Better Auth user to enrol as the development coordinator")
	set.StringVar(&options.DirectorEmail, "director-email", "", "existing verified Better Auth user to grant the development Director role")
	set.StringVar(&options.SiteAdminEmail, "site-admin-email", "", "existing verified Better Auth user to grant the development Site Admin role")
	set.BoolVar(&options.AllowExisting, "allow-existing", false, "succeed when the development season already exists")
	if err := set.Parse(args); err != nil {
		return seedOptions{}, err
	}
	if set.NArg() != 0 {
		return seedOptions{}, errors.New("seed does not accept positional arguments")
	}

	values := []*string{
		&options.StudentEmail,
		&options.MentorEmail,
		&options.CoordinatorEmail,
		&options.DirectorEmail,
		&options.SiteAdminEmail,
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized, err := normalizeSeedEmail(*value)
		if err != nil {
			return seedOptions{}, err
		}

		*value = normalized
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			return seedOptions{}, fmt.Errorf("seed role emails must be distinct: %s", normalized)
		}
		seen[normalized] = struct{}{}
	}

	if options.DirectorEmail != "" && options.SiteAdminEmail != "" {
		return seedOptions{}, errors.New("seed accepts either --director-email or --site-admin-email, not both")
	}
	return options, nil
}

func normalizeSeedEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}

	parsed, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(parsed.Address, value) {
		return "", fmt.Errorf("invalid seed email %q", value)
	}

	return value, nil
}

type seedUser struct {
	FallbackEmail string
	Email         string
	Slug          string
	DisplayName   string
}

func resolveSeedUser(ctx context.Context, tx *gorm.DB, user seedUser) (string, error) {
	if user.Email == "" {
		var userID string
		err := tx.WithContext(ctx).Raw(`INSERT INTO app.users(slug,display_name,email,account_state,timezone,timezone_configured,is_test,revision) VALUES($1,$2,$3,'active','Australia/Adelaide',true,true,1) RETURNING id`, user.Slug, user.DisplayName, user.FallbackEmail).Row().Scan(&userID)
		if err != nil {
			return "", err
		}
		return userID, nil
	}

	var userID string
	err := tx.WithContext(ctx).Raw(`
SELECT u.id
FROM app.users u
WHERE lower(u.email)=lower($1)
  AND u.account_state='active'
  AND u.deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM app.user_auth_links l
    WHERE l.user_id=u.id AND l.active
  )
FOR UPDATE`, user.Email).Row().Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("verified active auth-linked seed user not found for %s", user.Email)
	}
	if err != nil {
		return "", err
	}

	err = tx.WithContext(ctx).Exec(`UPDATE app.users SET slug=$2,display_name=$3,timezone='Australia/Adelaide',timezone_configured=true,is_test=false,revision=revision+1 WHERE id=$1`, userID, user.Slug, user.DisplayName).Error
	if err != nil {
		return "", err
	}

	return userID, nil
}

func seed(ctx context.Context, db *gorm.DB, options seedOptions) error {
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer tx.Rollback()
	// The worker may populate the catalogue as soon as the test stack starts.
	// Use its lock so the fixture can safely reuse problem 1 instead of racing
	// the catalogue sync into a unique-key violation.
	if err := tx.WithContext(ctx).Exec(`SELECT pg_advisory_xact_lock($1)`, worker.LeetCodeAdvisoryLock).Error; err != nil {
		return err
	}
	var exists bool
	if err := tx.WithContext(ctx).Raw(`SELECT EXISTS(SELECT 1 FROM app.seasons WHERE slug='dev-season')`).Row().Scan(&exists); err != nil {
		return err
	}

	if exists {
		if options.AllowExisting {
			return nil
		}
		return errors.New("seed data already exists")
	}

	studentID, err := resolveSeedUser(ctx, tx, seedUser{FallbackEmail: "student@rsp.local", Email: options.StudentEmail, Slug: "dev-student", DisplayName: "Dev Student"})
	if err != nil {
		return err
	}

	mentorID, err := resolveSeedUser(ctx, tx, seedUser{FallbackEmail: "mentor@rsp.local", Email: options.MentorEmail, Slug: "dev-mentor", DisplayName: "Dev Mentor"})
	if err != nil {
		return err
	}

	coordinatorID, err := resolveSeedUser(ctx, tx, seedUser{FallbackEmail: "coordinator@rsp.local", Email: options.CoordinatorEmail, Slug: "dev-coordinator", DisplayName: "Dev Coordinator"})
	if err != nil {
		return err
	}

	privileged := seedUser{FallbackEmail: "director@rsp.local", Email: options.DirectorEmail, Slug: "dev-director", DisplayName: "Dev Director"}
	privilegedRole := "director"
	privilegedAuditAction := "development_seed.director_granted"
	if options.SiteAdminEmail != "" {
		privileged = seedUser{FallbackEmail: "site-admin@rsp.local", Email: options.SiteAdminEmail, Slug: "dev-site-admin", DisplayName: "Dev Site Admin"}
		privilegedRole = "system_admin"
		privilegedAuditAction = "development_seed.site_admin_granted"
	}

	privilegedID, err := resolveSeedUser(ctx, tx, privileged)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	var seasonID string
	if err := tx.WithContext(ctx).Raw(`INSERT INTO app.seasons(slug,name,status,start_at,end_at,location,image_url,resources_url,revision) VALUES('dev-season','Development Season','open',$1,$2,'Adelaide','https://example.invalid/rsp-season','https://example.invalid/rsp-resources',1) RETURNING id`, now.AddDate(0, 0, -7), now.AddDate(0, 0, 49)).Row().Scan(&seasonID); err != nil {
		return fmt.Errorf("seed season: %w", err)
	}

	var studentEnrollmentID, mentorEnrollmentID, coordinatorEnrollmentID string
	for _, item := range []struct {
		userID, role, level string
		out                 *string
	}{
		{studentID, "student", "beginner", &studentEnrollmentID},
		{mentorID, "mentor", "not_applicable", &mentorEnrollmentID},
		{coordinatorID, "coordinator", "not_applicable", &coordinatorEnrollmentID},
	} {
		if err := tx.WithContext(ctx).Raw(`INSERT INTO app.enrollments(user_id,season_id,role,student_level,state,revision) VALUES($1,$2,$3,$4,'active',1) RETURNING id`, item.userID, seasonID, item.role, item.level).Row().Scan(item.out); err != nil {
			return fmt.Errorf("seed enrollment: %w", err)
		}
	}
	// The coordinator insert trigger starts every new coordinator assignment in
	// pending_mfa. These users completed MFA before the seed ran, so reconcile
	// the fixture without waiting for another identity event.
	if err := tx.WithContext(ctx).Exec(`
UPDATE app.enrollments e
SET assignment_state='active',activated_at=$2,revision=e.revision+1
FROM app.users u
WHERE e.id=$1
  AND e.user_id=u.id
  AND e.role='coordinator'
  AND e.state='active'
  AND e.assignment_state='pending_mfa'
  AND u.mfa_configured`, coordinatorEnrollmentID, now).Error; err != nil {
		return fmt.Errorf("seed coordinator role: %w", err)
	}
	if err := tx.WithContext(ctx).Exec(`INSERT INTO app.mentorships(season_id,mentor_enrollment_id,student_enrollment_id,revision) VALUES($1,$2,$3,1)`, seasonID, mentorEnrollmentID, studentEnrollmentID).Error; err != nil {
		return fmt.Errorf("seed mentorship: %w", err)
	}
	if err := seedLeetcodeProblem(ctx, tx); err != nil {
		return fmt.Errorf("seed leetcode problem: %w", err)
	}

	var privilegedAssignmentID string
	if err := tx.WithContext(ctx).Raw(`
INSERT INTO app.global_role_assignments(user_id,role,state,granted_by_user_id,granted_at,activated_at,revision)
SELECT $1,$2::app.global_role,'pending_mfa'::app.assignment_state,NULL,$3::timestamptz,NULL::timestamptz,1
FROM app.users
WHERE id=$1
RETURNING id`, privilegedID, privilegedRole, now).Row().Scan(&privilegedAssignmentID); err != nil {
		return fmt.Errorf("seed privileged role: %w", err)
	}
	if err := tx.WithContext(ctx).Exec(`INSERT INTO app.audit_events(actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES(NULL,'development_seed.coordinator_granted','enrollment',$1,jsonb_build_object('userId',$2::text,'seasonId',$3::text),$4), (NULL,$5,'global_role_assignment',$6,jsonb_build_object('userId',$7::text),$4)`, coordinatorEnrollmentID, coordinatorID, seasonID, now, privilegedAuditAction, privilegedAssignmentID, privilegedID).Error; err != nil {
		return fmt.Errorf("seed audit: %w", err)
	}

	return tx.Commit().Error
}

func seedLeetcodeProblem(ctx context.Context, tx *gorm.DB) error {
	const leetcodeNumber = 1
	var problemID, leetcodeID string
	err := tx.WithContext(ctx).Raw(`
SELECT p.id, l.id
FROM app.leetcode_problems AS l
JOIN app.problems AS p ON p.id=l.problem_id
WHERE l.leetcode_number=$1
FOR UPDATE OF p, l`, leetcodeNumber).Row().Scan(&problemID, &leetcodeID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if err := tx.WithContext(ctx).Raw(`INSERT INTO app.problems(title,url,revision) VALUES('Two Sum','https://leetcode.com/problems/two-sum/',1) RETURNING id`).Row().Scan(&problemID); err != nil {
			return err
		}
		err = tx.WithContext(ctx).Exec(`INSERT INTO app.leetcode_problems(problem_id,leetcode_number,difficulty,is_premium,revision) VALUES($1,$2,'easy',false,1)`, problemID, leetcodeNumber).Error
		return err
	case err != nil:
		return err
	default:
		if err := tx.WithContext(ctx).Exec(`UPDATE app.problems SET title='Two Sum',url='https://leetcode.com/problems/two-sum/',deleted_at=NULL,revision=revision+1 WHERE id=$1`, problemID).Error; err != nil {
			return err
		}
		err = tx.WithContext(ctx).Exec(`UPDATE app.leetcode_problems SET difficulty='easy',is_premium=false,deleted_at=NULL,revision=revision+1 WHERE id=$1`, leetcodeID).Error
		return err
	}
}

func bootstrap(ctx context.Context, db *gorm.DB, subject, email string) error {
	subject = strings.TrimSpace(subject)
	email = strings.TrimSpace(strings.ToLower(email))
	if subject == "" || email == "" {
		return errors.New("--auth-subject and --email are required")
	}

	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer tx.Rollback()
	if err := tx.WithContext(ctx).Exec(`SELECT pg_advisory_xact_lock(7277500101)`).Error; err != nil {
		return err
	}

	var count int
	if err := tx.WithContext(ctx).Raw(`SELECT count(*) FROM app.global_role_assignments WHERE role='system_admin'`).Row().Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errors.New("a System Admin has already been bootstrapped")
	}

	var userID string
	err := tx.WithContext(ctx).Raw(`SELECT u.id FROM app.users u JOIN app.user_auth_links l ON l.user_id=u.id WHERE l.auth_subject=$1 AND lower(u.email)=lower($2) AND l.active AND u.account_state='active' FOR UPDATE`, subject, email).Row().Scan(&userID)
	if err != nil {
		return fmt.Errorf("verified linked user not found: %w", err)
	}

	assignmentID := id.New()
	var assignmentState string
	err = tx.WithContext(ctx).Raw(`INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,revision) VALUES($1,$2,'system_admin','pending_mfa',NULL,now(),1) RETURNING state::text`, assignmentID, userID).Row().Scan(&assignmentState)
	if err != nil {
		return err
	}

	auditAction, err := bootstrapAuditAction(assignmentState)
	if err != nil {
		return err
	}

	err = tx.WithContext(ctx).Exec(`INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data) VALUES($1,NULL,$2,'global_role_assignment',$3,jsonb_build_object('userId',$4,'state',$5))`, id.New(), auditAction, assignmentID, userID, assignmentState).Error
	if err != nil {
		return err
	}

	return tx.Commit().Error
}

func bootstrapAuditAction(assignmentState string) (string, error) {
	switch assignmentState {
	case "active":
		return "system_admin.bootstrap_activated", nil
	case "pending_mfa":
		return "system_admin.bootstrap_pending_mfa", nil
	default:
		return "", fmt.Errorf("unexpected bootstrap assignment state %q", assignmentState)
	}
}
