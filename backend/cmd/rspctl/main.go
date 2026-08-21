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

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
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
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(context.WithoutCancel(ctx))
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
		return seed(ctx, conn, options)
	case "bootstrap-admin":
		set := flag.NewFlagSet("bootstrap-admin", flag.ContinueOnError)
		subject := set.String("auth-subject", "", "Better Auth user id")
		email := set.String("email", "", "verified administrator email")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		return bootstrap(ctx, conn, *subject, *email)
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
	FallbackID    string
	FallbackEmail string
	Email         string
	Slug          string
	DisplayName   string
}

func resolveSeedUser(ctx context.Context, tx pgx.Tx, user seedUser) (string, error) {
	if user.Email == "" {
		_, err := tx.Exec(ctx, `INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,timezone_configured,is_test,revision) VALUES($1,$2,$3,$4,'active','Australia/Adelaide',true,true,1)`, user.FallbackID, user.Slug, user.DisplayName, user.FallbackEmail)
		if err != nil {
			return "", err
		}
		return user.FallbackID, nil
	}

	var userID string
	err := tx.QueryRow(ctx, `
SELECT u.id
FROM app.users u
WHERE lower(u.email)=lower($1)
  AND u.account_state='active'
  AND u.deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM app.user_auth_links l
    WHERE l.user_id=u.id AND l.active
  )
FOR UPDATE`, user.Email).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("verified active auth-linked seed user not found for %s", user.Email)
	}
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `UPDATE app.users SET slug=$2,display_name=$3,timezone='Australia/Adelaide',timezone_configured=true,is_test=false,revision=revision+1 WHERE id=$1`, userID, user.Slug, user.DisplayName)
	if err != nil {
		return "", err
	}
	return userID, nil
}

func seed(ctx context.Context, conn *pgx.Conn, options seedOptions) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.seasons WHERE id='dev-season')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		if options.AllowExisting {
			return nil
		}
		return errors.New("seed data already exists")
	}
	studentID, err := resolveSeedUser(ctx, tx, seedUser{FallbackID: "dev-student", FallbackEmail: "student@rsp.local", Email: options.StudentEmail, Slug: "dev-student", DisplayName: "Dev Student"})
	if err != nil {
		return err
	}
	mentorID, err := resolveSeedUser(ctx, tx, seedUser{FallbackID: "dev-mentor", FallbackEmail: "mentor@rsp.local", Email: options.MentorEmail, Slug: "dev-mentor", DisplayName: "Dev Mentor"})
	if err != nil {
		return err
	}
	coordinatorID, err := resolveSeedUser(ctx, tx, seedUser{FallbackID: "dev-coordinator", FallbackEmail: "coordinator@rsp.local", Email: options.CoordinatorEmail, Slug: "dev-coordinator", DisplayName: "Dev Coordinator"})
	if err != nil {
		return err
	}
	privileged := seedUser{FallbackID: "dev-director", FallbackEmail: "director@rsp.local", Email: options.DirectorEmail, Slug: "dev-director", DisplayName: "Dev Director"}
	privilegedRole := "director"
	privilegedAssignmentID := "dev-role-director"
	privilegedAuditAction := "development_seed.director_granted"
	if options.SiteAdminEmail != "" {
		privileged = seedUser{FallbackID: "dev-site-admin", FallbackEmail: "site-admin@rsp.local", Email: options.SiteAdminEmail, Slug: "dev-site-admin", DisplayName: "Dev Site Admin"}
		privilegedRole = "system_admin"
		privilegedAssignmentID = "dev-role-system-admin"
		privilegedAuditAction = "development_seed.site_admin_granted"
	}
	privilegedID, err := resolveSeedUser(ctx, tx, privileged)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location,image_url,resources_url,revision) VALUES('dev-season','dev-season','Development Season','open',$1,$2,'Adelaide','https://example.invalid/rsp-season','https://example.invalid/rsp-resources',1)`, []any{now.AddDate(0, 0, -7), now.AddDate(0, 0, 49)}},
		{`INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,revision) VALUES('dev-enrollment-student',$1,'dev-season','student','beginner','active',1),('dev-enrollment-mentor',$2,'dev-season','mentor','not_applicable','active',1),('dev-enrollment-coordinator',$3,'dev-season','coordinator','not_applicable','active',1)`, []any{studentID, mentorID, coordinatorID}},
		{`INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id,revision) VALUES('dev-mentorship','dev-season','dev-enrollment-mentor','dev-enrollment-student',1)`, nil},
		{`INSERT INTO app.problems(id,title,url,revision) VALUES('dev-problem','Two Sum','https://leetcode.com/problems/two-sum/',1)`, nil},
		{`INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium,revision) VALUES('dev-leetcode','dev-problem',1,'easy',false,1) ON CONFLICT (leetcode_number) DO UPDATE SET difficulty='easy',is_premium=false,deleted_at=NULL,revision=app.leetcode_problems.revision+1`, nil},
		{`DELETE FROM app.problems p WHERE p.id='dev-problem' AND NOT EXISTS(SELECT 1 FROM app.leetcode_problems l WHERE l.problem_id=p.id)`, nil},
		{fmt.Sprintf(`INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,activated_at,revision) SELECT $2,$1,'%s',CASE WHEN mfa_configured THEN 'active'::app.assignment_state ELSE 'pending_mfa'::app.assignment_state END,NULL,$3::timestamptz,CASE WHEN mfa_configured THEN $3::timestamptz ELSE NULL END,1 FROM app.users WHERE id=$1`, privilegedRole), []any{privilegedID, privilegedAssignmentID, now}},
		{fmt.Sprintf(`INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data,occurred_at) VALUES('dev-audit-coordinator-seed',NULL,'development_seed.coordinator_granted','enrollment','dev-enrollment-coordinator',jsonb_build_object('userId',$1::text,'seasonId','dev-season'),$3::timestamptz),('dev-audit-privileged-seed',NULL,'%s','global_role_assignment',$2,jsonb_build_object('userId',$4::text),$3::timestamptz)`, privilegedAuditAction), []any{coordinatorID, privilegedAssignmentID, now, privilegedID}},
	}
	for i, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return fmt.Errorf("seed statement %d: %w", i+1, err)
		}
	}
	return tx.Commit(ctx)
}
func bootstrap(ctx context.Context, conn *pgx.Conn, subject, email string) error {
	subject = strings.TrimSpace(subject)
	email = strings.TrimSpace(strings.ToLower(email))
	if subject == "" || email == "" {
		return errors.New("--auth-subject and --email are required")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7277500101)`); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.global_role_assignments WHERE role='system_admin'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errors.New("a System Admin has already been bootstrapped")
	}
	var userID string
	err = tx.QueryRow(ctx, `SELECT u.id FROM app.users u JOIN app.user_auth_links l ON l.user_id=u.id WHERE l.auth_subject=$1 AND lower(u.email)=lower($2) AND l.active AND u.account_state='active' FOR UPDATE`, subject, email).Scan(&userID)
	if err != nil {
		return fmt.Errorf("verified linked user not found: %w", err)
	}
	assignmentID := id.New()
	var assignmentState string
	err = tx.QueryRow(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,revision) VALUES($1,$2,'system_admin','pending_mfa',NULL,now(),1) RETURNING state::text`, assignmentID, userID).Scan(&assignmentState)
	if err != nil {
		return err
	}
	auditAction, err := bootstrapAuditAction(assignmentState)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data) VALUES($1,NULL,$2,'global_role_assignment',$3,jsonb_build_object('userId',$4,'state',$5))`, id.New(), auditAction, assignmentID, userID, assignmentState)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
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
