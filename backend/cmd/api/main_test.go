package main

import "testing"

func TestValidateProductionConfig(t *testing.T) {
	valid := map[string]string{
		"PUBLIC_ORIGIN":          "https://rsp.example",
		"CURSOR_SECRET":          "0123456789abcdef0123456789abcdef",
		"AUTH_INTERNAL_URL":      "http://auth:3001",
		"IDENTITY_SERVICE_TOKEN": "unit-test-token-000000000000000000",
	}
	getenv := func(key string) string { return valid[key] }
	if err := validateProductionConfig(getenv); err != nil {
		t.Fatalf("valid production configuration rejected: %v", err)
	}
	for _, test := range []struct {
		name, key, value string
	}{
		{"insecure public origin", "PUBLIC_ORIGIN", "http://rsp.example"},
		{"origin path", "PUBLIC_ORIGIN", "https://rsp.example/api"},
		{"short cursor secret", "CURSOR_SECRET", "short"},
		{"placeholder cursor secret", "CURSOR_SECRET", "development-cursor-secret-change-me"},
		{"missing auth internal URL", "AUTH_INTERNAL_URL", ""},
		{"short identity token", "IDENTITY_SERVICE_TOKEN", "short"},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := make(map[string]string, len(valid))
			for key, value := range valid {
				values[key] = value
			}
			values[test.key] = test.value
			if err := validateProductionConfig(func(key string) string { return values[key] }); err == nil {
				t.Fatal("invalid production configuration accepted")
			}
		})
	}
}
