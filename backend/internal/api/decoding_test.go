package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/authz"
)

func TestDecodeJSONRequiresOneValueWithKnownFields(t *testing.T) {
	for _, body := range []string{`{"name":`, `{"unknown":true}`, `{"name":"one"} {"name":"two"}`, `{"name":"one"} trailing`} {
		var request struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(strings.NewReader(body), &request); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
	var request struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(strings.NewReader(`{"name":"one"}`), &request); err != nil || request.Name != "one" {
		t.Fatalf("decode: %#v %v", request, err)
	}
}

func TestStreamingRequestBodyLimit(t *testing.T) {
	handler := testHandler(AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) {
		return authz.Actor{UserID: "student", EmailVerified: true, AccountState: authz.AccountActive}, nil
	}))
	request := httptest.NewRequest(http.MethodPatch, "/api/v2/me", strings.NewReader(`{"name":"`+strings.Repeat("x", int(maxRequestBodyBytes))+`"}`))
	request.ContentLength = -1
	request.Header.Set("Authorization", "Bearer student")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
}
