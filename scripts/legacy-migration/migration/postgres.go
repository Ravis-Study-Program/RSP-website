package migration

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (tx *postgresTx) Commit(ctx context.Context) error {
	if tx.completed {
		return errors.New("transaction is already complete")
	}
	tx.completed = true
	commitErr := tx.tx.Commit(ctx)
	closeErr := tx.repository.Close(context.Background())
	if commitErr != nil {
		return commitErr
	}
	return closeErr
}

func (tx *postgresTx) Abort(ctx context.Context) error {
	if tx.completed {
		return nil
	}
	tx.completed = true
	rollbackErr := tx.tx.Rollback(context.Background())
	closeErr := tx.repository.Close(context.Background())
	if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
		return rollbackErr
	}
	return closeErr
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func jsonValue(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func connectMigration(ctx context.Context, dsn string) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.RuntimeParams["timezone"] = "UTC"
	return pgx.ConnectConfig(ctx, config)
}
