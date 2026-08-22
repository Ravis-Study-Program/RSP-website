package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/store"
)

var (
	httpStatusClasses      = [...]string{"1xx", "2xx", "3xx", "4xx", "5xx"}
	httpDurationBoundaries = [...]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	recommendationOutcomes = [...]string{"generated", "reused", "attempted", "dismissed", "unavailable"}
	workerResults          = [...]string{"success", "partial_failure", "failure"}
	migrationStates        = [...]string{"none", "planned", "applying", "applied", "verified", "rolled_back", "failed"}
)

type apiMetrics struct {
	mu                     sync.Mutex
	httpRequests           [len(httpStatusClasses)]uint64
	httpDurationBuckets    [len(httpDurationBoundaries) + 1]uint64
	httpDurationCount      uint64
	httpDurationSumSeconds float64
	recommendations        map[string]uint64
}

func newAPIMetrics() *apiMetrics {
	return &apiMetrics{recommendations: make(map[string]uint64, len(recommendationOutcomes))}
}

func (m *apiMetrics) observeHTTP(status int, elapsed time.Duration) {
	index := status/100 - 1
	if index < 0 || index >= len(httpStatusClasses) {
		index = len(httpStatusClasses) - 1
	}
	seconds := elapsed.Seconds()
	m.mu.Lock()
	m.httpRequests[index]++
	for bucket, boundary := range httpDurationBoundaries {
		if seconds <= boundary {
			m.httpDurationBuckets[bucket]++
		}
	}
	m.httpDurationBuckets[len(httpDurationBoundaries)]++
	m.httpDurationCount++
	m.httpDurationSumSeconds += seconds
	m.mu.Unlock()
}

func (m *apiMetrics) observeRecommendation(outcome string) {
	if !containsMetricLabel(recommendationOutcomes[:], outcome) {
		return
	}
	m.mu.Lock()
	m.recommendations[outcome]++
	m.mu.Unlock()
}

func (m *apiMetrics) render(ctx context.Context, writer io.Writer, repository store.Repository) {
	m.mu.Lock()
	requests := m.httpRequests
	buckets := m.httpDurationBuckets
	durationCount := m.httpDurationCount
	durationSum := m.httpDurationSumSeconds
	recommendations := make(map[string]uint64, len(m.recommendations))
	for outcome, count := range m.recommendations {
		recommendations[outcome] = count
	}
	m.mu.Unlock()

	snapshot := store.ObservabilitySnapshot{
		WorkerRuns:     map[string]uint64{},
		MigrationState: "none",
	}
	snapshotOK := true
	if source, ok := repository.(store.ObservabilitySource); ok {
		var err error
		snapshot, err = source.ObservabilitySnapshot(ctx)
		if err != nil {
			snapshot = store.ObservabilitySnapshot{WorkerRuns: map[string]uint64{}, MigrationState: "none"}
			snapshotOK = false
		}
	}

	fmt.Fprintln(writer, "# HELP rsp_http_requests_total Go API responses by bounded status class.")
	fmt.Fprintln(writer, "# TYPE rsp_http_requests_total counter")
	for index, class := range httpStatusClasses {
		fmt.Fprintf(writer, "rsp_http_requests_total{status_class=%q} %d\n", class, requests[index])
	}
	fmt.Fprintln(writer, "# HELP rsp_http_request_duration_seconds Go API response duration in seconds.")
	fmt.Fprintln(writer, "# TYPE rsp_http_request_duration_seconds histogram")
	for index, boundary := range httpDurationBoundaries {
		fmt.Fprintf(writer, "rsp_http_request_duration_seconds_bucket{le=%q} %d\n", strconv.FormatFloat(boundary, 'g', -1, 64), buckets[index])
	}
	fmt.Fprintf(writer, "rsp_http_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", buckets[len(httpDurationBoundaries)])
	fmt.Fprintf(writer, "rsp_http_request_duration_seconds_sum %s\n", strconv.FormatFloat(durationSum, 'g', -1, 64))
	fmt.Fprintf(writer, "rsp_http_request_duration_seconds_count %d\n", durationCount)
	fmt.Fprintln(writer, "# HELP rsp_db_pool_acquired_connections PostgreSQL connections currently acquired by the API.")
	fmt.Fprintln(writer, "# TYPE rsp_db_pool_acquired_connections gauge")
	fmt.Fprintf(writer, "rsp_db_pool_acquired_connections %d\n", snapshot.DBPoolAcquiredConnections)
	fmt.Fprintln(writer, "# HELP rsp_db_pool_idle_connections PostgreSQL connections currently idle in the API pool.")
	fmt.Fprintln(writer, "# TYPE rsp_db_pool_idle_connections gauge")
	fmt.Fprintf(writer, "rsp_db_pool_idle_connections %d\n", snapshot.DBPoolIdleConnections)
	fmt.Fprintln(writer, "# HELP rsp_worker_runs_total Completed LeetCode worker runs by bounded result.")
	fmt.Fprintln(writer, "# TYPE rsp_worker_runs_total counter")
	for _, result := range workerResults {
		fmt.Fprintf(writer, "rsp_worker_runs_total{result=%q} %d\n", result, snapshot.WorkerRuns[result])
	}
	fmt.Fprintln(writer, "# HELP rsp_recommendation_outcomes_total Recommendation lifecycle events by bounded outcome.")
	fmt.Fprintln(writer, "# TYPE rsp_recommendation_outcomes_total counter")
	for _, outcome := range recommendationOutcomes {
		fmt.Fprintf(writer, "rsp_recommendation_outcomes_total{outcome=%q} %d\n", outcome, recommendations[outcome])
	}
	fmt.Fprintln(writer, "# HELP rsp_migration_status Latest legacy migration state as a one-hot bounded label.")
	fmt.Fprintln(writer, "# TYPE rsp_migration_status gauge")
	state := snapshot.MigrationState
	if !containsMetricLabel(migrationStates[:], state) {
		state = "none"
	}
	for _, candidate := range migrationStates {
		value := 0
		if candidate == state {
			value = 1
		}
		fmt.Fprintf(writer, "rsp_migration_status{state=%q} %d\n", candidate, value)
	}
	fmt.Fprintln(writer, "# HELP rsp_observability_snapshot_success Whether durable operational state was read successfully.")
	fmt.Fprintln(writer, "# TYPE rsp_observability_snapshot_success gauge")
	if snapshotOK {
		fmt.Fprintln(writer, "rsp_observability_snapshot_success 1")
	} else {
		fmt.Fprintln(writer, "rsp_observability_snapshot_success 0")
	}
}

func containsMetricLabel(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader writes a response.
func (writer *statusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

// Write writes a response.
func (writer *statusWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

// Unwrap performs the operation.
func (writer *statusWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func statusClass(status int) string {
	index := status/100 - 1
	if index < 0 || index >= len(httpStatusClasses) {
		return "5xx"
	}
	return strings.Clone(httpStatusClasses[index])
}
