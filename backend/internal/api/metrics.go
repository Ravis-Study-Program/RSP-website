package api

import (
	"fmt"

	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/observability"
)

var (
	httpStatusClasses      = [...]string{"1xx", "2xx", "3xx", "4xx", "5xx"}
	httpDurationBoundaries = [...]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	workerResults          = [...]string{"success", "partial_failure", "failure"}
)

type apiMetrics struct {
	mu                     sync.Mutex
	httpRequests           [len(httpStatusClasses)]uint64
	httpDurationBuckets    [len(httpDurationBoundaries) + 1]uint64
	httpDurationCount      uint64
	httpDurationSumSeconds float64
}

func newAPIMetrics() *apiMetrics {
	return &apiMetrics{}
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

type httpMetricsSnapshot struct {
	Requests           [len(httpStatusClasses)]uint64
	DurationBuckets    [len(httpDurationBoundaries) + 1]uint64
	DurationCount      uint64
	DurationSumSeconds float64
}

func (m *apiMetrics) snapshot() httpMetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return httpMetricsSnapshot{Requests: m.httpRequests, DurationBuckets: m.httpDurationBuckets, DurationCount: m.httpDurationCount, DurationSumSeconds: m.httpDurationSumSeconds}
}

func formatMetrics(httpSnapshot httpMetricsSnapshot, snapshot observability.Snapshot, snapshotOK bool) string {
	var output strings.Builder
	writer := &output

	fmt.Fprintln(writer, "# HELP rsp_http_requests_total Go API responses by bounded status class.")
	fmt.Fprintln(writer, "# TYPE rsp_http_requests_total counter")
	for index, class := range httpStatusClasses {
		fmt.Fprintf(writer, "rsp_http_requests_total{status_class=%q} %d\n", class, httpSnapshot.Requests[index])
	}
	fmt.Fprintln(writer, "# HELP rsp_http_request_duration_seconds Go API response duration in seconds.")
	fmt.Fprintln(writer, "# TYPE rsp_http_request_duration_seconds histogram")
	for index, boundary := range httpDurationBoundaries {
		fmt.Fprintf(writer, "rsp_http_request_duration_seconds_bucket{le=%q} %d\n", strconv.FormatFloat(boundary, 'g', -1, 64), httpSnapshot.DurationBuckets[index])
	}
	fmt.Fprintf(writer, "rsp_http_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", httpSnapshot.DurationBuckets[len(httpDurationBoundaries)])
	fmt.Fprintf(writer, "rsp_http_request_duration_seconds_sum %s\n", strconv.FormatFloat(httpSnapshot.DurationSumSeconds, 'g', -1, 64))
	fmt.Fprintf(writer, "rsp_http_request_duration_seconds_count %d\n", httpSnapshot.DurationCount)
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
	fmt.Fprintln(writer, "# HELP rsp_observability_snapshot_success Whether durable operational state was read successfully.")
	fmt.Fprintln(writer, "# TYPE rsp_observability_snapshot_success gauge")
	if snapshotOK {
		fmt.Fprintln(writer, "rsp_observability_snapshot_success 1")
	} else {
		fmt.Fprintln(writer, "rsp_observability_snapshot_success 0")
	}
	return output.String()
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

func (writer *statusWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func statusClass(status int) string {
	index := status/100 - 1
	if index < 0 || index >= len(httpStatusClasses) {
		return "5xx"
	}
	return strings.Clone(httpStatusClasses[index])
}
