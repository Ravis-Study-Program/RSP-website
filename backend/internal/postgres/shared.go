package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/magedmg/RSP-website/backend/internal/platform/dbtable"
	"github.com/magedmg/RSP-website/backend/internal/platform/observability"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrDuplicate = errors.New("duplicate")
)

// Postgres owns the GORM handle and its underlying database/sql pool. Goose,
// rather than GORM AutoMigrate, remains responsible for the schema.
type Postgres struct {
	DB  *gorm.DB
	SQL *sql.DB
}

func finishPostgresPage[T any](items []T, limit int, direction string) ([]T, bool) {
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if direction == "backward" {
		for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
			items[left], items[right] = items[right], items[left]
		}
	}
	return items, more
}

// Open creates one database/sql pool and lets GORM use it. pgx remains the
// PostgreSQL driver underneath GORM.
func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.RuntimeParams["timezone"] = "UTC"

	sqlDB := stdlib.OpenDB(*config)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return &Postgres{DB: db, SQL: sqlDB}, nil
}

func (p *Postgres) Close() error { return p.SQL.Close() }

func (p *Postgres) Ping(ctx context.Context) error { return p.SQL.PingContext(ctx) }

// PinnedConnection returns a GORM handle that always uses one physical SQL
// connection. Session-scoped PostgreSQL features, such as advisory locks, must
// be acquired and released through this same handle.
func (p *Postgres) PinnedConnection(ctx context.Context) (*gorm.DB, func() error, error) {
	connection, err := p.SQL.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	db := p.DB.Session(&gorm.Session{NewDB: true})
	db.ConnPool = connection
	return db, connection.Close, nil
}

func (p *Postgres) ObservabilitySnapshot(ctx context.Context) (observability.Snapshot, error) {
	pool := p.SQL.Stats()
	snapshot := observability.Snapshot{
		DBPoolAcquiredConnections: int32(pool.InUse),
		DBPoolIdleConnections:     int32(pool.Idle),
		WorkerRuns: map[string]uint64{
			"success":         0,
			"partial_failure": 0,
			"failure":         0,
		},
		MigrationState: "none",
	}

	var counts []struct {
		Result   string
		RunCount int64
	}
	if err := p.DB.WithContext(ctx).Table(dbtable.LeetcodeSyncRuns).
		Select("CASE WHEN succeeded THEN 'success' WHEN error_summary IS NOT NULL THEN 'failure' ELSE 'partial_failure' END AS result, count(*) AS run_count").
		Where("finished_at IS NOT NULL").
		Group("result").
		Scan(&counts).Error; err != nil {
		return observability.Snapshot{}, err
	}
	for _, count := range counts {
		if _, bounded := snapshot.WorkerRuns[count.Result]; bounded {
			snapshot.WorkerRuns[count.Result] = uint64(count.RunCount)
		}
	}

	result := p.DB.WithContext(ctx).Table(dbtable.MigrationRuns).
		Select("state::text").
		Order("started_at DESC, id DESC").
		Limit(1).
		Scan(&snapshot.MigrationState)
	if result.Error != nil {
		return observability.Snapshot{}, result.Error
	}
	if result.RowsAffected == 0 {
		snapshot.MigrationState = "none"
	}
	return snapshot, nil
}

func begin(ctx context.Context, db *gorm.DB) (*gorm.DB, error) {
	tx := db.WithContext(ctx).Begin()
	return tx, tx.Error
}

func rollback(tx *gorm.DB) { _ = tx.Rollback().Error }

func commit(tx *gorm.DB) error { return tx.Commit().Error }

func noRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) classifyRevision(ctx context.Context, db *gorm.DB, query string, args ...any) error {
	var revision int64
	err := db.WithContext(ctx).Raw(query, args...).Row().Scan(&revision)
	if err == nil {
		return ErrConflict
	}
	return noRows(err)
}

func mapPostgresError(err error) error {
	if isUnique(err) {
		return ErrDuplicate
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514" || pgErr.Code == "23502") {
		return ErrConflict
	}
	return noRows(err)
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
