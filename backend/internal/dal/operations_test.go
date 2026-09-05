package dal

import "testing"

func TestBootstrapAuditActionMatchesPersistedAssignmentState(t *testing.T) {
	tests := map[string]string{
		"active":      "system_admin.bootstrap_activated",
		"pending_mfa": "system_admin.bootstrap_pending_mfa",
	}
	for state, expected := range tests {
		t.Run(state, func(t *testing.T) {
			action, err := bootstrapAuditAction(state)
			if err != nil {
				t.Fatal(err)
			}
			if action != expected {
				t.Fatalf("expected %q, got %q", expected, action)
			}
		})
	}
	if _, err := bootstrapAuditAction("revoked"); err == nil {
		t.Fatal("expected an unsupported assignment state to fail")
	}
}
