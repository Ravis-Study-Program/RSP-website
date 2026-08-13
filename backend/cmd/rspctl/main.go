package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
		return seed(ctx, conn)
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
func seed(ctx context.Context, conn *pgx.Conn) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM app.users WHERE id='dev-student')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errors.New("seed data already exists")
	}
	now := time.Now().UTC()
	statements := []string{`INSERT INTO app.users(id,slug,display_name,email,account_state,timezone,is_test,revision) VALUES('dev-student','dev-student','Dev Student','student@rsp.local','active','Australia/Adelaide',true,1),('dev-mentor','dev-mentor','Dev Mentor','mentor@rsp.local','active','Australia/Adelaide',true,1),('dev-admin','dev-admin','Dev Admin','admin@rsp.local','active','Australia/Adelaide',true,1)`, `INSERT INTO app.seasons(id,slug,name,status,start_at,end_at,location,image_url,resources_url,revision) VALUES('dev-season','dev-season','Development Season','open',$1,$2,'Adelaide','https://example.invalid/rsp-season','https://example.invalid/rsp-resources',1)`, `INSERT INTO app.enrollments(id,user_id,season_id,role,student_level,state,revision) VALUES('dev-enrollment-student','dev-student','dev-season','student','beginner','active',1),('dev-enrollment-mentor','dev-mentor','dev-season','mentor','not_applicable','active',1),('dev-enrollment-admin','dev-admin','dev-season','coordinator','not_applicable','active',1)`, `INSERT INTO app.mentorships(id,season_id,mentor_enrollment_id,student_enrollment_id,revision) VALUES('dev-mentorship','dev-season','dev-enrollment-mentor','dev-enrollment-student',1)`, `INSERT INTO app.problems(id,title,url,revision) VALUES('dev-problem','Two Sum','https://leetcode.com/problems/two-sum/',1)`, `INSERT INTO app.leetcode_problems(id,problem_id,leetcode_number,difficulty,is_premium,revision) VALUES('dev-leetcode','dev-problem',1,'easy',false,1)`}
	for i, statement := range statements {
		var e error
		if i == 1 {
			_, e = tx.Exec(ctx, statement, now.AddDate(0, 0, -7), now.AddDate(0, 0, 49))
		} else {
			_, e = tx.Exec(ctx, statement)
		}
		if e != nil {
			return e
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
	_, err = tx.Exec(ctx, `INSERT INTO app.global_role_assignments(id,user_id,role,state,granted_by_user_id,granted_at,revision) VALUES($1,$2,'system_admin','pending_mfa',NULL,now(),1)`, assignmentID, userID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.audit_events(id,actor_user_id,action,subject_type,subject_id,data) VALUES($1,NULL,'system_admin.bootstrap_pending_mfa','global_role_assignment',$2,jsonb_build_object('userId',$3))`, id.New(), assignmentID, userID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
