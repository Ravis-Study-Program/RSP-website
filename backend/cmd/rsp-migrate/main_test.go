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
	fixture, err := filepath.Abs(filepath.Join("..", "..", "internal", "migration", "testdata", "valid_snapshot.json"))
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
	if !strings.Contains(stdout.String(), "sourceTables=18") {
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
	runID := extractRunID(t, manifest)
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"legacy", "rollback", "--run-id", runID, "--target-state", state}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rollback code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
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

func extractRunID(t *testing.T, path string) string {
	t.Helper()
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		RunID string `json:"runId"`
	}
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	return document.RunID
}
