package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuth0NormalizeAndProposeCommands(t *testing.T) {
	directory := t.TempDir()
	exportPath := filepath.Join(directory, "export.ndjson")
	export := `{"user_id":"auth0|one","email":"one@example.test","email_verified":true,"app_metadata":{"userId":"legacy-one"}}
{"user_id":"google-oauth2|7","email":"two@example.test","email_verified":true,"identities":[{"provider":"google-oauth2","user_id":"7"}],"app_metadata":{"userId":"Unknown User"}}
`
	if err := os.WriteFile(exportPath, []byte(export), 0o600); err != nil {
		t.Fatal(err)
	}
	candidatesPath := filepath.Join(directory, "candidates.json")
	if err := os.WriteFile(candidatesPath, []byte(`[{"appUserId":"app-one","email":"ONE@example.test","legacyUserId":"legacy-one"},{"appUserId":"app-two","email":"two@example.test","legacyUserId":"legacy-two"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	usersPath := filepath.Join(directory, "users.json")
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	if code := run(ctx, []string{"auth0", "normalize", "--export", exportPath, "--out", usersPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("normalize code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "auth0Users=2 identities=2 blocked=0 withLegacyUserId=1") {
		t.Fatalf("normalize output = %s", stdout.String())
	}

	stdout.Reset()
	proposedPath := filepath.Join(directory, "proposed.json")
	reportPath := filepath.Join(directory, "report.json")
	args := []string{"auth0", "propose", "--auth0-users", usersPath, "--app-candidates", candidatesPath, "--out", proposedPath, "--report", reportPath}
	if code := run(ctx, args, &stdout, &stderr); code != 0 {
		t.Fatalf("propose code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "proposed=1 manual=1") {
		t.Fatalf("propose output = %s", stdout.String())
	}
	proposed, err := os.ReadFile(proposedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proposed), `"appUserId": "app-one"`) || strings.Contains(string(proposed), "app-two") {
		t.Fatalf("proposed resolutions = %s", proposed)
	}
}

func TestAuth0ApplyRejectsDriftedInputsBeforeConnecting(t *testing.T) {
	testdata, err := filepath.Abs(filepath.Join("..", "migration", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	planArgs := []string{
		"auth0", "plan",
		"--auth0-users", filepath.Join(testdata, "auth0_users.json"),
		"--app-candidates", filepath.Join(testdata, "auth0_app_candidates.json"),
		"--resolutions", filepath.Join(testdata, "auth0_resolutions.json"),
		"--plan", planPath,
	}
	if code := run(ctx, planArgs, &stdout, &stderr); code != 0 {
		t.Fatalf("plan code=%d stderr=%s", code, stderr.String())
	}
	stderr.Reset()
	// Omitting the approved resolutions changes the plan, so apply must stop
	// with the drift exit code before it ever opens a database connection.
	applyArgs := []string{
		"auth0", "apply",
		"--plan", planPath,
		"--auth0-users", filepath.Join(testdata, "auth0_users.json"),
		"--app-candidates", filepath.Join(testdata, "auth0_app_candidates.json"),
		"--run-id", "01900000-0000-7000-8000-000000000001",
		"--target-dsn", "postgres://unreachable.invalid/rsp",
	}
	if code := run(ctx, applyArgs, &stdout, &stderr); code != 3 || !strings.Contains(stderr.String(), "do not reproduce the approved plan checksum") {
		t.Fatalf("drifted apply code=%d stderr=%s", code, stderr.String())
	}
}
