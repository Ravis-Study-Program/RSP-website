package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/observability"
)

type observableSource struct {
	snapshot observability.Snapshot
	err      error
}

func (source *observableSource) ObservabilitySnapshot(context.Context) (observability.Snapshot, error) {
	return source.snapshot, source.err
}

func TestMetricsMatchDashboardSeriesWithBoundedLabels(t *testing.T) {
	metrics := newAPIMetrics()
	metrics.observeHTTP(http.StatusCreated, 12*time.Millisecond)
	metrics.observeHTTP(http.StatusServiceUnavailable, 2*time.Second)
	source := &observableSource{
		snapshot: observability.Snapshot{
			DBPoolAcquiredConnections: 3,
			DBPoolIdleConnections:     7,
			WorkerRuns: map[string]uint64{
				"success":         8,
				"partial_failure": 1,
				"failure":         2,
			},
		},
	}
	var output strings.Builder
	metrics.render(context.Background(), &output, source)
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
		`rsp_observability_snapshot_success 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q in metrics:\n%s", expected, body)
		}
	}
}

func TestMetricsSnapshotFailureIsSafeAndDoesNotLeakError(t *testing.T) {
	metrics := newAPIMetrics()
	source := &observableSource{err: errors.New("private@example.com")}
	var output strings.Builder
	metrics.render(context.Background(), &output, source)
	body := output.String()
	if !strings.Contains(body, "rsp_observability_snapshot_success 0") {
		t.Fatalf("missing safe failure state:\n%s", body)
	}
	if strings.Contains(body, "private@example.com") {
		t.Fatal("snapshot error leaked into metrics")
	}
}
