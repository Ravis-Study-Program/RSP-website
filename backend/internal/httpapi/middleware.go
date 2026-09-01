package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authn"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
)

// Authenticator defines a backend interface.
type Authenticator interface {
	Authenticate(*http.Request) (authz.Actor, error)
}

// AuthenticatorFunc represents a backend value.
type AuthenticatorFunc func(*http.Request) (authz.Actor, error)

// Authenticate performs the operation.
func (f AuthenticatorFunc) Authenticate(r *http.Request) (authz.Actor, error) { return f(r) }

// SubjectResolver resolves a validated token subject into a domain actor.
// Authentication does not need the rest of the application's repository.
type SubjectResolver interface {
	ResolveAuthSubject(context.Context, string) (authz.Actor, error)
}

// BearerAuthenticator represents a backend data structure.
type BearerAuthenticator struct {
	Validator *authn.Validator
	Subjects  SubjectResolver
}

// Authenticate performs the operation.
func (b BearerAuthenticator) Authenticate(r *http.Request) (authz.Actor, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return authz.Actor{}, authn.ErrInvalidToken
	}

	claims, err := b.Validator.Validate(r.Context(), strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		return authz.Actor{}, err
	}

	actor, err := b.Subjects.ResolveAuthSubject(r.Context(), claims.Subject)
	if err != nil {
		return authz.Actor{}, err
	}

	if !claimsMatchActor(claims, actor) {
		return authz.Actor{}, authn.ErrInvalidToken
	}

	if claims.MFAVerified && claims.MFAVerifiedAt != nil {
		at := claims.MFAVerifiedAt.Time.UTC()
		actor.MFAAt = &at
	}
	return actor, nil
}

func claimsMatchActor(claims authn.Claims, actor authz.Actor) bool {
	return claims.AccountState == string(actor.AccountState) &&
		claims.SecurityVersion > 0 &&
		claims.SecurityVersion == actor.SecurityVersion
}

type actorKey struct{}

func actorFrom(ctx context.Context) authz.Actor { return ctx.Value(actorKey{}).(authz.Actor) }

func (a *API) protected(class ratelimit.Class, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, err := a.auth.Authenticate(r)
		if err != nil {
			cause := err.Error()
			if len(cause) > 512 {
				cause = cause[:512]
			}
			a.logger.Warn("authentication failed", "requestId", requestIDFrom(r.Context()), "cause", cause)
			writeErrorResponse(w, http.StatusUnauthorized, "authentication_required", "A valid access token is required.")
			return
		}
		if actor.AccountState != authz.AccountActive {
			writeErrorResponse(w, http.StatusForbidden, "account_unavailable", "This account cannot access the product.")
			return
		}
		if !actor.EmailVerified {
			writeErrorResponse(w, http.StatusForbidden, "email_verification_required", "Verify the account email before accessing the product.")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !a.validOrigin(r) {
			writeErrorResponse(w, http.StatusForbidden, "origin_rejected", "State-changing requests must come from the configured application origin.")
			return
		}

		result := a.limiter.Allow(actor.UserID, class)
		w.Header().Set("RateLimit-Limit", strconv.Itoa(result.Limit))
		w.Header().Set("RateLimit-Remaining", strconv.Itoa(result.Remaining))
		if !result.Allowed {
			seconds := int((result.RetryAfter + time.Second - 1) / time.Second)
			if seconds < 1 {
				seconds = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			writeErrorResponse(w, http.StatusTooManyRequests, "rate_limited", "Wait before trying again.")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, actor)))
	}
}

func (a *API) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || a.publicOrigin == "" || strings.TrimRight(origin, "/") == strings.TrimRight(a.publicOrigin, "/")
}

// writeErrorResponse sends one consistent JSON error response.
func writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
	body := struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	}{
		Code:      code,
		Message:   message,
		RequestID: w.Header().Get("X-Request-ID"),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
