package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/magedmg/RSP-website/backend/internal/generated"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/problem"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/store"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

//go:generate go run ./internal/genstrict

type requestIDKey struct{}

type rawBodyKey struct{}

type validationResponseWriter struct {
	http.ResponseWriter
	request *http.Request
}

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// Config represents a backend data structure.
type Config struct {
	Store           store.Repository
	Authenticator   Authenticator
	PublicOrigin    string
	CursorSecret    []byte
	Logger          *slog.Logger
	Ready           func() error
	SyncLeetCode    func(context.Context, string, string) error
	SetAccountState func(context.Context, string, string, string, string) error
	GetMFAState     func(context.Context, string) (bool, error)
}

// API represents a backend data structure.
type API struct {
	store           store.Repository
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
	mockService     mockinterviews.Service
}

type strictAdapter struct{}

type strictResponseKey struct{}

type strictManualResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newStrictManualResponse() *strictManualResponse {
	return &strictManualResponse{header: make(http.Header)}
}

// Header performs the operation.
func (response *strictManualResponse) Header() http.Header { return response.header }

// WriteHeader writes a response.
func (response *strictManualResponse) WriteHeader(status int) {
	if response.status == 0 {
		response.status = status
	}
}

// Write writes a response.
func (response *strictManualResponse) Write(body []byte) (int, error) {
	if response.status == 0 {
		response.status = http.StatusOK
	}
	return response.body.Write(body)
}

func (response *strictManualResponse) write(w http.ResponseWriter) error {
	for key, values := range response.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if response.body.Len() == 0 {
		return nil
	}
	_, err := w.Write(response.body.Bytes())
	return err
}

func strictResponse(ctx context.Context) (*strictManualResponse, error) {
	response, ok := ctx.Value(strictResponseKey{}).(*strictManualResponse)
	if !ok {
		return nil, fmt.Errorf("strict response capture is missing")
	}
	return response, nil
}

// New creates a new value.
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
	return &API{store: c.Store, auth: c.Authenticator, publicOrigin: c.PublicOrigin, cursorSecret: secret, logger: logger, limiter: ratelimit.New(), ready: c.Ready, syncLeetCode: c.SyncLeetCode, setAccountState: c.SetAccountState, getMFAState: c.GetMFAState, telemetry: newAPIMetrics(), mockService: mockinterviews.Service{Sanitize: cleaner.String}}
}

// Handler performs the operation.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux, a)
	spec, err := generated.GetSwagger()
	if err != nil {
		panic(err)
	}

	validatorOptions := &nethttpmiddleware.Options{
		Options:               openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		SilenceServersWarning: true,
		ErrorHandler: func(w http.ResponseWriter, message string, status int) {
			r := &http.Request{}
			if wrapped, ok := w.(*validationResponseWriter); ok {
				r = wrapped.request
			}
			a.fail(w, r, status, "openapi_validation_failed", "Request validation failed", message, nil)
		},
	}
	baseValidator := nethttpmiddleware.OapiRequestValidatorWithOptions(spec, validatorOptions)
	validator := func(next http.Handler) http.Handler {
		validated := baseValidator(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			validated.ServeHTTP(&validationResponseWriter{ResponseWriter: w, request: r}, r)
		})
	}
	strict := generated.NewStrictHandlerWithOptions(strictAdapter{}, []generated.StrictMiddlewareFunc{func(next generated.StrictHandlerFunc, _ string) generated.StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
			if raw, ok := ctx.Value(rawBodyKey{}).([]byte); ok {
				r.Body = io.NopCloser(bytes.NewReader(raw))
			}
			captured := newStrictManualResponse()
			mux.ServeHTTP(captured, r)
			return next(context.WithValue(ctx, strictResponseKey{}, captured), w, r, request)
		}
	}}, generated.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			a.fail(w, r, http.StatusBadRequest, "openapi_validation_failed", "Request validation failed", err.Error(), nil)
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			a.fail(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "The request could not be completed.", nil)
		},
	})
	strictMux := http.NewServeMux()
	strictRouter := generated.HandlerWithOptions(strict, generated.StdHTTPServerOptions{BaseURL: "/api/v2", BaseRouter: strictMux, ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
		a.fail(w, r, http.StatusBadRequest, "openapi_validation_failed", "Request validation failed", err.Error(), nil)
	}})
	captureBody := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			strictRouter.ServeHTTP(w, r)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			a.fail(w, r, http.StatusBadRequest, "invalid_request_body", "Invalid request body", "The request body is too large or unreadable.", nil)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(raw))
		strictRouter.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), rawBodyKey{}, raw)))
	})
	root := http.NewServeMux()
	root.HandleFunc("POST /internal/auth/lifecycle-events", a.identityLifecycle)
	root.Handle("/", validator(captureBody))
	return requestContext(a.logger, a.telemetry.observeHTTP, root)
}

func requestContext(logger *slog.Logger, observe func(int, time.Duration), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if !safeRequestID.MatchString(requestID) {
			requestID = id.New()
		}
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID))
		w.Header().Set("X-Request-ID", requestID)
		response := &statusWriter{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				problem.Write(response, problem.Details{Type: "https://rsp.example/problems/internal_error", Title: "Internal server error", Status: 500, Detail: "The request could not be completed.", Instance: r.URL.Path, Code: "internal_error", RequestID: requestID})
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (a *API) readiness(w http.ResponseWriter, r *http.Request) {
	if a.ready != nil {
		if err := a.ready(); err != nil {
			a.fail(w, r, 503, "not_ready", "Service unavailable", "A required dependency is unavailable.", nil)
			return
		}
	}
	writeJSON(w, 200, map[string]string{"status": "ready"})
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	a.telemetry.render(r.Context(), w, a.store)
}

func (a *API) openapi(w http.ResponseWriter, _ *http.Request) {
	spec, err := generated.GetSwagger()
	if err != nil {
		panic(err)
	}

	writeJSON(w, 200, spec)
}

func validation(a *API, w http.ResponseWriter, r *http.Request, detail string) {
	a.fail(w, r, 400, "validation_failed", "Validation failed", detail, nil)
}
