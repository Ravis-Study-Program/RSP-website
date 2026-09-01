package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
)

type requestIDKey struct{}

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

const maxRequestBodyBytes int64 = 1 << 20

// Config contains the dependencies needed by the HTTP API.
type Config struct {
	Store           Repository
	Authenticator   Authenticator
	PublicOrigin    string
	CursorSecret    []byte
	Logger          *slog.Logger
	Ready           func() error
	SyncLeetCode    func(context.Context, string, string) error
	SetAccountState func(context.Context, string, string, string, string) error
	GetMFAState     func(context.Context, string) (bool, error)
}

// API connects HTTP handlers to authentication, application services, and storage.
type API struct {
	store           Repository
	auth            Authenticator
	publicOrigin    string
	cursorSecret    []byte
	logger          *slog.Logger
	limiter         *ratelimit.Limiter
	ready           func() error
	syncLeetCode    func(context.Context, string, string) error
	setAccountState func(context.Context, string, string, string, string) error
	getMFAState     func(context.Context, string) (bool, error)
	telemetry       *apiMetrics
	mockService     mockinterviews.Application
}

// New constructs the HTTP API from its dependencies.
func New(c Config) *API {
	logger := c.Logger
	if logger == nil {
		logger = slog.Default()
	}
	secret := c.CursorSecret
	if len(secret) < 16 {
		secret = []byte("development-cursor-secret-change-me")
	}
	cleaner := sanitize.New()
	return &API{
		store:           c.Store,
		auth:            c.Authenticator,
		publicOrigin:    c.PublicOrigin,
		cursorSecret:    secret,
		logger:          logger,
		limiter:         ratelimit.New(),
		ready:           c.Ready,
		syncLeetCode:    c.SyncLeetCode,
		setAccountState: c.SetAccountState,
		getMFAState:     c.GetMFAState,
		telemetry:       newAPIMetrics(),
		mockService: mockinterviews.Application{
			Repository: c.Store,
			Rules:      mockinterviews.Service{Sanitize: cleaner.String},
			NewID:      id.New,
		},
	}
}

// Handler builds the request pipeline once when the server starts.
//
// Requests flow through requestContext, the body-size limit, the route mux,
// and finally the selected endpoint handler.
func (a *API) Handler() http.Handler {
	routes := http.NewServeMux()
	registerRoutes(routes, a)
	routes.HandleFunc("POST /internal/auth/lifecycle-events", a.identityLifecycle)

	return requestContext(a.logger, a.telemetry.observeHTTP, a.limitRequestBody(routes))
}

// requestContext is the outermost middleware, so it runs once for every request.
// It attaches the request ID and records the final status and duration after the
// selected endpoint handler returns.
func requestContext(logger *slog.Logger, observe func(int, time.Duration), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()

		// Reuse a safe caller-provided ID; otherwise generate one for this request.
		requestID := r.Header.Get("X-Request-ID")
		if !safeRequestID.MatchString(requestID) {
			requestID = id.New()
		}
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID))
		w.Header().Set("X-Request-ID", requestID)
		response := &statusWriter{ResponseWriter: w}

		// Deferred work runs after the selected endpoint returns or panics.
		defer func() {
			if recovered := recover(); recovered != nil {
				writeErrorResponse(response, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
				logger.Error("request panic", "requestId", requestID, "error", fmt.Sprint(recovered))
			}
			if response.status == 0 {
				response.status = http.StatusOK
			}
			elapsed := time.Since(started)
			observe(response.status, elapsed)
			logger.Info("request", "requestId", requestID, "method", r.Method, "path", r.URL.Path, "statusClass", statusClass(response.status), "durationMs", elapsed.Milliseconds())
		}()

		next.ServeHTTP(response, r)
	})
}

func requestIDFrom(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

// writeJSONResponse sends one JSON response with the supplied HTTP status.
func writeJSONResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// limitRequestBody rejects requests larger than the API accepts and also caps
// streaming bodies whose size was not declared in Content-Length.
func (a *API) limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		if r.ContentLength > maxRequestBodyBytes {
			writeErrorResponse(w, http.StatusBadRequest, "invalid_request_body", "The request body is too large or unreadable.")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// decodeJSON accepts exactly one JSON value and rejects unknown object fields.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return fmt.Errorf("malformed trailing JSON: %w", err)
	}
	return nil
}

func (a *API) page(r *http.Request, binding string) (int, string, error) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 100 {
			return 0, "", cursor.ErrInvalid
		}

		limit = v
	}
	encoded := r.URL.Query().Get("cursor")
	if encoded == "" {
		return limit, "", nil
	}
	after, err := cursor.Decode(a.cursorSecret, encoded, binding)
	return limit, after, err
}

func (a *API) live(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) readiness(w http.ResponseWriter, r *http.Request) {
	if a.ready != nil {
		if err := a.ready(); err != nil {
			writeErrorResponse(w, 503, "not_ready", "A required dependency is unavailable.")
			return
		}
	}
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	a.telemetry.render(r.Context(), w, a.store)
}
