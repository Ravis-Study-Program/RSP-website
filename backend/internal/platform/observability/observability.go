// Package observability defines bounded operational reporting ports.
package observability

import "context"

// Snapshot contains bounded aggregate operational data.
type Snapshot struct {
	DBPoolAcquiredConnections int32
	DBPoolIdleConnections     int32
	WorkerRuns                map[string]uint64
}

// Source provides operational data to metrics and health adapters.
type Source interface {
	ObservabilitySnapshot(context.Context) (Snapshot, error)
}
