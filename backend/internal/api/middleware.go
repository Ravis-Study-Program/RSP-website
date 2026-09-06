package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/authn"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
)

type Authenticator interface {
	Authenticate(context.Context, string) (authz.Actor, error)
}

type AuthenticatorFunc func(context.Context, string) (authz.Actor, error)

func (f AuthenticatorFunc) Authenticate(ctx context.Context, token string) (authz.Actor, error) {
	return f(ctx, token)
}

// SubjectResolver resolves a validated token subject into a domain actor.
// Authentication needs only subject resolution, not every database operation.
type SubjectResolver interface {
	ResolveAuthSubject(context.Context, string) (authz.Actor, error)
}

type BearerAuthenticator struct {
	Validator *authn.Validator
	Subjects  SubjectResolver
}

func (b BearerAuthenticator) Authenticate(ctx context.Context, token string) (authz.Actor, error) {
	claims, err := b.Validator.Validate(ctx, token)
	if err != nil {
		return authz.Actor{}, err
	}

	actor, err := b.Subjects.ResolveAuthSubject(ctx, claims.Subject)
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
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeErrorResponse(w, http.StatusUnauthorized, "A valid access token is required.")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		actor, err := a.auth.Authenticate(r.Context(), token)
		if err != nil {
			if !errors.Is(err, authn.ErrInvalidToken) && !errors.Is(err, authn.ErrUnverified) && !errors.Is(err, authn.ErrAccountUnavailable) && !errors.Is(err, dal.ErrNotFound) {
				a.writeStoreErrorResponse(w, err)
				return
			}
			cause := err.Error()
			if len(cause) > 512 {
				cause = cause[:512]
			}
			a.logger.Warn("authentication failed", "requestId", requestIDFrom(r.Context()), "cause", cause)
			writeErrorResponse(w, http.StatusUnauthorized, "A valid access token is required.")
			return
		}
		if actor.AccountState != authz.AccountActive {
			writeErrorResponse(w, http.StatusForbidden, "This account cannot access the product.")
			return
		}
		if !actor.EmailVerified {
			writeErrorResponse(w, http.StatusForbidden, "Verify the account email before accessing the product.")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !a.validOrigin(r) {
			writeErrorResponse(w, http.StatusForbidden, "State-changing requests must come from the configured application origin.")
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
			writeErrorResponse(w, http.StatusTooManyRequests, "Wait before trying again.")
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
func writeErrorResponse(w http.ResponseWriter, status int, message string) {
	body := struct {
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	}{
		Message:   message,
		RequestID: w.Header().Get("X-Request-ID"),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeStoreErrorResponse translates storage failures into safe HTTP errors.
// Unexpected errors are logged with their request ID but are never exposed.
func (a *API) writeStoreErrorResponse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dal.ErrNotFound):
		writeErrorResponse(w, http.StatusNotFound, "The requested resource does not exist.")
	case errors.Is(err, dal.ErrConflict):
		writeErrorResponse(w, http.StatusConflict, "The requested operation cannot be completed.")
	case errors.Is(err, dal.ErrDuplicate):
		writeErrorResponse(w, http.StatusConflict, "A resource with that unique value already exists.")
	default:
		a.logger.Error(
			"storage operation failed",
			"requestId", w.Header().Get("X-Request-ID"),
			"error", err,
		)
		writeErrorResponse(w, http.StatusInternalServerError, "The request could not be completed.")
	}
}
