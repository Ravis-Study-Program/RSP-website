package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/magedmg/RSP-website/backend/internal/generated"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/model"
	"github.com/magedmg/RSP-website/backend/internal/platform/cursor"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/problem"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/practice"
	"github.com/magedmg/RSP-website/backend/internal/store"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

type Config struct {
	Store         store.Repository
	Authenticator Authenticator
	PublicOrigin  string
	CursorSecret  []byte
	Logger        *slog.Logger
	Ready         func() error
	SyncLeetCode  func() error
}
type API struct {
	store           store.Repository
	auth            Authenticator
	publicOrigin    string
	cursorSecret    []byte
	logger          *slog.Logger
	limiter         *ratelimit.Limiter
	ready           func() error
	syncLeetCode    func() error
	requests        atomic.Uint64
	mu              sync.Mutex
	weeks           map[string]model.Week
	enrollments     map[string]model.Enrollment
	mentorships     map[string]model.Mentorship
	mocks           map[string]mockinterviews.Interview
	recommendations map[string]practice.Recommendation
	dismissals      map[string][]practice.Dismissal
	mockService     mockinterviews.Service
}

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
	return &API{store: c.Store, auth: c.Authenticator, publicOrigin: c.PublicOrigin, cursorSecret: secret, logger: logger, limiter: ratelimit.New(), ready: c.Ready, syncLeetCode: c.SyncLeetCode, weeks: map[string]model.Week{}, enrollments: map[string]model.Enrollment{}, mentorships: map[string]model.Mentorship{}, mocks: map[string]mockinterviews.Interview{}, recommendations: map[string]practice.Recommendation{}, dismissals: map[string][]practice.Dismissal{}, mockService: mockinterviews.Service{Sanitize: cleaner.String}}
}
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v2/health/live", a.live)
	mux.HandleFunc("GET /api/v2/health/ready", a.readiness)
	mux.HandleFunc("GET /api/v2/metrics", a.metrics)
	mux.HandleFunc("GET /api/v2/openapi.json", a.openapi)
	mux.HandleFunc("GET /api/v2/me", a.protected(ratelimit.Read, a.me))
	mux.HandleFunc("PATCH /api/v2/me", a.protected(ratelimit.Write, a.updateMe))
	mux.HandleFunc("GET /api/v2/users", a.protected(ratelimit.Read, a.users))
	mux.HandleFunc("GET /api/v2/users/{id}", a.protected(ratelimit.Read, a.user))
	mux.HandleFunc("GET /api/v2/seasons", a.protected(ratelimit.Read, a.seasons))
	mux.HandleFunc("POST /api/v2/seasons", a.protected(ratelimit.Write, a.createSeason))
	mux.HandleFunc("GET /api/v2/seasons/{id}", a.protected(ratelimit.Read, a.season))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}", a.protected(ratelimit.Write, a.updateSeason))
	mux.HandleFunc("POST /api/v2/seasons/{id}/close", a.protected(ratelimit.Write, a.closeSeason))
	mux.HandleFunc("POST /api/v2/seasons/{id}/reopen", a.protected(ratelimit.Write, a.reopenSeason))
	mux.HandleFunc("GET /api/v2/seasons/{id}/weeks", a.protected(ratelimit.Read, a.listWeeks))
	mux.HandleFunc("POST /api/v2/seasons/{id}/weeks", a.protected(ratelimit.Write, a.createWeek))
	mux.HandleFunc("GET /api/v2/seasons/{id}/members", a.protected(ratelimit.Read, a.listMembers))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members", a.protected(ratelimit.Write, a.createMember))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/promote", a.protected(ratelimit.Write, a.promoteMember))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/remove", a.protected(ratelimit.Write, a.removeMember))
	mux.HandleFunc("GET /api/v2/seasons/{id}/mentorships", a.protected(ratelimit.Read, a.listMentorships))
	mux.HandleFunc("POST /api/v2/seasons/{id}/mentorships", a.protected(ratelimit.Write, a.createMentorship))
	mux.HandleFunc("GET /api/v2/leetcode-problems", a.protected(ratelimit.Read, a.problems))
	mux.HandleFunc("GET /api/v2/problem-attempts", a.protected(ratelimit.Read, a.attempts))
	mux.HandleFunc("POST /api/v2/problem-attempts", a.protected(ratelimit.Write, a.createAttempt))
	mux.HandleFunc("PATCH /api/v2/problem-attempts/{id}", a.protected(ratelimit.Write, a.updateAttempt))
	mux.HandleFunc("DELETE /api/v2/problem-attempts/{id}", a.protected(ratelimit.Write, a.deleteAttempt))
	mux.HandleFunc("GET /api/v2/recommendations/current", a.protected(ratelimit.Sensitive, a.recommendation))
	mux.HandleFunc("POST /api/v2/recommendations/current/dismiss", a.protected(ratelimit.Sensitive, a.dismissRecommendation))
	mux.HandleFunc("GET /api/v2/mock-interviews", a.protected(ratelimit.Read, a.listMocks))
	mux.HandleFunc("POST /api/v2/mock-interviews", a.protected(ratelimit.Write, a.createMock))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}", a.protected(ratelimit.Write, a.updateMock))
	mux.HandleFunc("DELETE /api/v2/mock-interviews/{id}", a.protected(ratelimit.Write, a.deleteMock))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}/rounds/{roundId}/review", a.protected(ratelimit.Write, a.reviewRound))
	mux.HandleFunc("POST /api/v2/admin/leetcode/sync", a.protected(ratelimit.Sensitive, a.adminSync))
	spec, err := generated.GetSwagger()
	if err != nil {
		panic(err)
	}
	validator := nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		Options:               openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		SilenceServersWarning: true,
		ErrorHandler: func(w http.ResponseWriter, message string, status int) {
			a.fail(w, &http.Request{URL: &url.URL{Path: "/api/v2"}}, status, "openapi_validation_failed", "Request validation failed", message, nil)
		},
	})
	root := http.NewServeMux()
	root.HandleFunc("POST /internal/auth/lifecycle-events", a.identityLifecycle)
	root.Handle("/", validator(mux))
	return requestContext(a.logger, a.requests.Add, root)
}
func requestContext(logger *slog.Logger, increment func(uint64) uint64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = id.New()
		}
		w.Header().Set("X-Request-ID", requestID)
		increment(1)
		defer func() {
			if recovered := recover(); recovered != nil {
				problem.Write(w, problem.Details{Type: "https://rsp.example/problems/internal_error", Title: "Internal server error", Status: 500, Code: "internal_error", RequestID: requestID})
				logger.Error("request panic", "requestId", requestID, "error", fmt.Sprint(recovered))
			}
			logger.Info("request", "requestId", requestID, "method", r.Method, "path", r.URL.Path, "durationMs", time.Since(started).Milliseconds())
		}()
		next.ServeHTTP(w, r)
	})
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
	if dec.Decode(&struct{}{}) == nil {
		return fmt.Errorf("multiple JSON values")
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
func (a *API) next(after string, binding string, more bool) *string {
	if !more || after == "" {
		return nil
	}
	v, _ := cursor.Encode(a.cursorSecret, after, binding)
	return &v
}
func (a *API) audit(r *http.Request, action, kind, subject string, data map[string]any) {
	actor := actorFrom(r.Context())
	actorID := actor.UserID
	_ = a.store.AppendAudit(r.Context(), model.AuditEvent{ID: id.New(), ActorID: &actorID, Action: action, SubjectType: kind, SubjectID: subject, Data: data, OccurredAt: time.Now().UTC()})
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
func (a *API) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "# TYPE rsp_http_requests_total counter\nrsp_http_requests_total %d\n", a.requests.Load())
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
func hasCategory(p model.Problem, c string) bool {
	for _, v := range p.Categories {
		if strings.EqualFold(v, c) {
			return true
		}
	}
	return false
}
