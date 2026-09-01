package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/platform/dbtable"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/worker"
	"gorm.io/gorm"
)

// WorkerState uses a GORM handle pinned to one SQL connection. Advisory locks
// are session-scoped, so TryLock and Unlock must run on that same connection.
type WorkerState struct {
	DB          *gorm.DB
	ManualRunID string
}

func (p *WorkerState) RunPendingManual(ctx context.Context, scheduler worker.Scheduler) (bool, error) {
	var runID string
	result := p.DB.WithContext(ctx).Table(dbtable.LeetcodeSyncRuns).
		Select("id").
		Where("trigger_kind = ? AND finished_at IS NULL", "manual").
		Order("started_at, id").
		Limit(1).
		Scan(&runID)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}

	p.ManualRunID = runID
	defer func() { p.ManualRunID = "" }()
	return true, scheduler.Run(ctx)
}

func (p *WorkerState) TryLock(ctx context.Context, key int64) (bool, error) {
	var locked bool
	err := p.DB.WithContext(ctx).Raw("SELECT pg_try_advisory_lock(?)", key).Scan(&locked).Error
	return locked, err
}

func (p *WorkerState) Unlock(ctx context.Context, key int64) error {
	return p.DB.WithContext(ctx).Exec("SELECT pg_advisory_unlock(?)", key).Error
}

func (p *WorkerState) LastSuccess(ctx context.Context, _ string) (*time.Time, error) {
	var at sql.NullTime
	err := p.DB.WithContext(ctx).Table(dbtable.LeetcodeSyncRuns).
		Select("max(finished_at)").Where("succeeded").Scan(&at).Error
	if err != nil || !at.Valid {
		return nil, err
	}
	return &at.Time, nil
}

func (p *WorkerState) LastAttempt(ctx context.Context, _ string) (*time.Time, error) {
	var at sql.NullTime
	err := p.DB.WithContext(ctx).Table(dbtable.LeetcodeSyncRuns).
		Select("max(started_at)").Where("trigger_kind IN ?", []string{"schedule", "catch_up"}).Scan(&at).Error
	if err != nil || !at.Valid {
		return nil, err
	}
	return &at.Time, nil
}

func (p *WorkerState) Record(ctx context.Context, run worker.Run) error {
	errorSummary := nullableString(run.Error)
	if p.ManualRunID != "" {
		return p.DB.WithContext(ctx).Table(dbtable.LeetcodeSyncRuns).
			Where("id = ?", p.ManualRunID).
			Updates(map[string]any{
				"finished_at": run.FinishedAt.UTC(), "succeeded": run.Error == "",
				"fetched_count": run.Report.Fetched, "changed_count": run.Report.Inserted + run.Report.Updated,
				"error_summary": errorSummary,
			}).Error
	}

	trigger := run.TriggerKind
	if trigger != "catch_up" {
		trigger = "schedule"
	}
	return p.DB.WithContext(ctx).Table(dbtable.LeetcodeSyncRuns).Create(map[string]any{
		"id": id.New(), "trigger_kind": trigger, "started_at": run.StartedAt.UTC(),
		"finished_at": run.FinishedAt.UTC(), "succeeded": run.Error == "",
		"fetched_count": run.Report.Fetched, "changed_count": run.Report.Inserted + run.Report.Updated,
		"error_summary": errorSummary,
	}).Error
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
