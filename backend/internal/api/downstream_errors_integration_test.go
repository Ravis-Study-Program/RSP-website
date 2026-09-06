//go:build integration

package api

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
)

type failingQueryTracer struct {
	match    string
	failures atomic.Int32
}

func (tracer *failingQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, query pgx.TraceQueryStartData) context.Context {
	if strings.Contains(query.SQL, tracer.match) {
		tracer.failures.Add(1)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		return cancelled
	}
	return ctx
}
func (*failingQueryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestDownstreamReadFailuresRemainServerErrors(t *testing.T) {
	fixture := newPostgresFixture(t)
	for _, test := range []struct {
		name, path, query string
		status            int
	}{
		{"current user season", "/api/v2/me", "FROM app.seasons", http.StatusInternalServerError},
		{"private relationship", "/api/v2/users/" + studentID + "/practice-settings", "FROM app.enrollments", http.StatusInternalServerError},
		{"metrics snapshot", "/api/v2/metrics", "FROM app.leetcode_sync_runs", http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			tracer := &failingQueryTracer{match: test.query}
			config := fixture.pool.Config().Copy()
			config.ConnConfig.Tracer = tracer
			pool, err := pgxpool.NewWithConfig(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			store := dal.New(pool)
			t.Cleanup(store.Close)
			actor := authz.Actor{UserID: otherID, EmailVerified: true, AccountState: authz.AccountActive, Enrollments: []authz.Enrollment{{SeasonID: seasonID, Role: authz.Coordinator, State: authz.Active}}}
			handler := New(Config{DB: store, Authenticator: AuthenticatorFunc(func(context.Context, string) (authz.Actor, error) { return actor, nil })}).Handler()
			response := testRequest(t, handler, http.MethodGet, test.path, "actor", "")
			if tracer.failures.Load() == 0 {
				t.Fatal("failure injection did not reach the intended downstream query")
			}
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "context canceled") || strings.Contains(response.Body.String(), "SELECT") {
				t.Fatal("database error leaked")
			}
			if test.status == http.StatusOK && !strings.Contains(response.Body.String(), "rsp_observability_snapshot_success 0") {
				t.Fatal("metrics did not report snapshot failure")
			}
		})
	}
}
