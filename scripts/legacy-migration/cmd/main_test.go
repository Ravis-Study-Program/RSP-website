package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixtureCommandLifecycle(t *testing.T) {
	fixture, err := filepath.Abs(filepath.Join("..", "migration", "testdata", "valid_snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()
	manifest := filepath.Join(directory, "manifest.json")
	state := filepath.Join(directory, "state.json")
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"legacy", "dry-run", "--source-fixture", fixture, "--manifest", manifest}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "sourceTables=17") {
		t.Fatalf("dry-run output = %s", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"legacy", "apply", "--source-fixture", fixture, "--manifest", manifest, "--target-state", state}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"legacy", "verify", "--manifest", manifest, "--target-state", state}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), `"foreignKeyErrors": 0`) {
		t.Fatalf("verify code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestAuth0FixturePlanCommand(t *testing.T) {
	testdata, err := filepath.Abs(filepath.Join("..", "migration", "testdata"))
	if err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(t.TempDir(), "auth0-plan.json")
	args := []string{
		"auth0", "plan",
		"--auth0-users", filepath.Join(testdata, "auth0_users.json"),
		"--app-candidates", filepath.Join(testdata, "auth0_app_candidates.json"),
		"--resolutions", filepath.Join(testdata, "auth0_resolutions.json"),
		"--plan", planPath,
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), args, &stdout, &stderr); code != 0 {
		t.Fatalf("auth0 plan code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	encoded, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}

	var plan struct {
		Items []struct {
			Auth0UserID         string   `json:"auth0UserId"`
			CandidateAppUserIDs []string `json:"candidateAppUserIds"`
			ResolvedAppUserID   string   `json:"resolvedAppUserId"`
			Status              string   `json:"status"`
		} `json:"items"`
		Reconciliation struct {
			Auth0UserCount         int            `json:"auth0UserCount"`
			ProviderIdentityCount  int            `json:"providerIdentityCount"`
			StatusCounts           map[string]int `json:"statusCounts"`
			UnresolvedAuth0UserIDs []string       `json:"unresolvedAuth0UserIds"`
		} `json:"reconciliation"`
		Checksum string `json:"checksum"`
	}
	if err := json.Unmarshal(encoded, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Reconciliation.Auth0UserCount != 4 || plan.Reconciliation.ProviderIdentityCount != 4 || len(plan.Checksum) != 64 {
		t.Fatalf("reconciliation = %+v checksum=%q", plan.Reconciliation, plan.Checksum)
	}
	if plan.Reconciliation.StatusCounts["matched"] != 1 || plan.Reconciliation.StatusCounts["password_reset_required"] != 1 || plan.Reconciliation.StatusCounts["requires_resolution"] != 2 || plan.Reconciliation.StatusCounts["imported"] != 0 {
		t.Fatalf("status counts = %v", plan.Reconciliation.StatusCounts)
	}
	foundAmbiguous, foundUnverified := false, false
	for _, item := range plan.Items {
		if item.Auth0UserID == "auth0|ambiguous" {
			foundAmbiguous = true
			if item.Status != "requires_resolution" || item.ResolvedAppUserID != "" || len(item.CandidateAppUserIDs) != 2 {
				t.Fatalf("ambiguous email was silently merged: %+v", item)
			}
		}
		if item.Auth0UserID == "auth0|unverified" {
			foundUnverified = true
			if item.Status != "requires_resolution" || item.ResolvedAppUserID != "" || len(item.CandidateAppUserIDs) != 0 {
				t.Fatalf("unverified email was used as an identity match: %+v", item)
			}
		}
	}
	if !foundAmbiguous || !foundUnverified {
		t.Fatalf("required no-merge fixtures missing: ambiguous=%v unverified=%v", foundAmbiguous, foundUnverified)
	}
}

func TestUsageAndMissingInputs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), nil, &stdout, &stderr); code != 2 {
		t.Fatalf("usage code = %d", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"legacy", "apply"}, &stdout, &stderr); code != 1 {
		t.Fatalf("missing input code = %d", code)
	}
}
