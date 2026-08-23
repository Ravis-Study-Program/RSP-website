package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/observability"
	"github.com/magedmg/RSP-website/backend/internal/store"
)

type observableRepository struct {
	*store.Memory
	snapshot observability.Snapshot
	err      error
}

func (repository *observableRepository) ObservabilitySnapshot(context.Context) (observability.Snapshot, error) {
	return repository.snapshot, repository.err
}

func TestMetricsMatchDashboardSeriesWithBoundedLabels(t *testing.T) {
	metrics := newAPIMetrics()
	metrics.observeHTTP(http.StatusCreated, 12*time.Millisecond)
	metrics.observeHTTP(http.StatusServiceUnavailable, 2*time.Second)
	for _, outcome := range []string{"generated", "reused", "attempted", "dismissed", "unavailable", "user-123"} {
		metrics.observeRecommendation(outcome)
	}
	repository := &observableRepository{
		Memory: store.NewMemory(),
		snapshot: observability.Snapshot{
			DBPoolAcquiredConnections: 3,
			DBPoolIdleConnections:     7,
			WorkerRuns: map[string]uint64{
				"success":         8,
				"partial_failure": 1,
				"failure":         2,
			},
			MigrationState: "verified",
		},
	}
	var output strings.Builder
	metrics.render(context.Background(), &output, repository)
	body := output.String()
	for _, expected := range []string{
		`rsp_http_requests_total{status_class="2xx"} 1`,
		`rsp_http_requests_total{status_class="5xx"} 1`,
		`rsp_http_request_duration_seconds_bucket{le="+Inf"} 2`,
		`rsp_http_request_duration_seconds_count 2`,
		`rsp_db_pool_acquired_connections 3`,
		`rsp_db_pool_idle_connections 7`,
		`rsp_worker_runs_total{result="success"} 8`,
		`rsp_worker_runs_total{result="partial_failure"} 1`,
		`rsp_worker_runs_total{result="failure"} 2`,
		`rsp_recommendation_outcomes_total{outcome="attempted"} 1`,
		`rsp_migration_status{state="verified"} 1`,
		`rsp_migration_status{state="applied"} 0`,
		`rsp_observability_snapshot_success 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q in metrics:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "user-123") {
		t.Fatal("unbounded recommendation label was exported")
	}
}

func TestMetricsSnapshotFailureIsSafeAndDoesNotLeakError(t *testing.T) {
	metrics := newAPIMetrics()
	repository := &observableRepository{Memory: store.NewMemory(), err: errors.New("private@example.com")}
	var output strings.Builder
	metrics.render(context.Background(), &output, repository)
	body := output.String()
	if !strings.Contains(body, `rsp_migration_status{state="none"} 1`) ||
		!strings.Contains(body, "rsp_observability_snapshot_success 0") {
		t.Fatalf("missing safe failure state:\n%s", body)
	}
	if strings.Contains(body, "private@example.com") {
		t.Fatal("snapshot error leaked into metrics")
	}
}

func TestMetricsEndpointRecordsResponseClassesAndLatency(t *testing.T) {
	fixture := newFixture()
	if response := request(t, fixture, http.MethodGet, "/api/v2/health/live", "", ""); response.Code != http.StatusOK {
		t.Fatal(response.Code)
	}
	if response := request(t, fixture, http.MethodGet, "/api/v2/me", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatal(response.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v2/metrics", nil)
	response := httptest.NewRecorder()
	fixture.handler.ServeHTTP(response, request)
	body := response.Body.String()
	for _, expected := range []string{
		`rsp_http_requests_total{status_class="2xx"} 1`,
		`rsp_http_requests_total{status_class="4xx"} 1`,
		`rsp_http_request_duration_seconds_count 2`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in endpoint metrics:\n%s", expected, body)
		}
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("content type = %q", contentType)
	}
}
