package main

import (
	"strings"
	"testing"
)

func TestParseSeedOptionsNormalizesExistingUsers(t *testing.T) {
	options, err := parseSeedOptions([]string{
		"--student-email", " Student.E2E@Example.Test ",
		"--coordinator-email", "coordinator.e2e@example.test",
		"--director-email", "director.e2e@example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.StudentEmail != "student.e2e@example.test" {
		t.Fatalf("unexpected student email %q", options.StudentEmail)
	}
	if options.MentorEmail != "" {
		t.Fatalf("unexpected mentor email %q", options.MentorEmail)
	}
}

func TestParseSeedOptionsRejectsDuplicateRoleIdentity(t *testing.T) {
	_, err := parseSeedOptions([]string{
		"--student-email", "same@example.test",
		"--director-email", "SAME@example.test",
	})
	if err == nil || !strings.Contains(err.Error(), "must be distinct") {
		t.Fatalf("expected distinct-email error, got %v", err)
	}
}

func TestParseSeedOptionsAcceptsSiteAdminAndRejectsMixedPrivilegedRoles(t *testing.T) {
	options, err := parseSeedOptions([]string{"--site-admin-email", "site-admin@example.test", "--allow-existing"})
	if err != nil {
		t.Fatal(err)
	}
	if options.SiteAdminEmail != "site-admin@example.test" || options.DirectorEmail != "" || !options.AllowExisting {
		t.Fatalf("unexpected privileged seed options: %+v", options)
	}
	if _, err := parseSeedOptions([]string{
		"--director-email", "director@example.test",
		"--site-admin-email", "site-admin@example.test",
	}); err == nil || !strings.Contains(err.Error(), "either --director-email or --site-admin-email") {
		t.Fatalf("expected mixed privileged-role error, got %v", err)
	}
}

func TestParseSeedOptionsRejectsMalformedEmailAndArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--student-email", "not-an-email"},
		{"unexpected"},
	} {
		if _, err := parseSeedOptions(args); err == nil {
			t.Fatalf("expected %v to fail", args)
		}
	}
}
