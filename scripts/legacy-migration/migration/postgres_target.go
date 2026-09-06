package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type PostgresTarget struct {
	ConnectionString string
	Now              func() time.Time
}

func (target *PostgresTarget) Begin(ctx context.Context) (TargetTx, error) {
	repository, err := connectMigration(ctx, target.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("connect target: %w", err)
	}

	tx, err := repository.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		_ = repository.Close(context.Background())
		return nil, err
	}

	now := target.Now
	if now == nil {
		now = time.Now
	}
	return &postgresTx{repository: repository, tx: tx, now: now}, nil
}

type postgresTx struct {
	repository *pgx.Conn
	tx         pgx.Tx
	now        func() time.Time
	locked     bool
	completed  bool
}

func (tx *postgresTx) AcquireAdvisoryLock(ctx context.Context, key int64) error {
	if key != AdvisoryLockKey {
		return fmt.Errorf("unexpected advisory lock key %d", key)
	}
	if _, err := tx.tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key); err != nil {
		return err
	}

	tx.locked = true
	return nil
}

func (tx *postgresTx) HasRun(ctx context.Context, runID string) (bool, error) {
	var exists bool
	err := tx.tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM migration.runs WHERE id = $1)", runID).Scan(&exists)
	return exists, err
}

// Apply applies the operation.
func (tx *postgresTx) Apply(ctx context.Context, prepared PreparedImport) error {
	if !tx.locked {
		return errors.New("migration advisory lock is not held")
	}
	manifest := prepared.Manifest
	if _, err := tx.tx.Exec(ctx, `
INSERT INTO migration.runs (
  id, manifest_checksum, source_schema_fingerprint, source_snapshot_at,
  state, started_at
) VALUES ($1, $2, $3, $4, 'applying', $5)`,
		manifest.RunID, manifest.Checksum, manifest.SourceSchemaFingerprint,
		manifest.SourceSnapshotAt, tx.now().UTC()); err != nil {
		return fmt.Errorf("record migration run: %w", err)
	}

	for _, table := range prepared.Tables {
		for _, row := range table.Rows {
			if err := insertPreparedRow(ctx, tx.tx, table.Name, row.Values); err != nil {
				return fmt.Errorf("insert %s/%s: %w", table.Name, row.ID, err)
			}
		}
	}
	for _, table := range manifest.Tables {
		if _, err := tx.tx.Exec(ctx, `
INSERT INTO migration.source_tables (
  run_id, source_table, source_count, imported_count,
  source_ids_checksum, transformed_checksum
) VALUES ($1, $2, $3, $4, $5, $6)`,
			manifest.RunID, table.SourceTable, table.SourceCount,
			table.TransformedCount, table.SourceIDsChecksum, table.TransformedChecksum); err != nil {
			return err
		}
	}
	preparedLookup := map[string]Row{}
	for _, table := range prepared.Tables {
		for _, row := range table.Rows {
			preparedLookup[table.Name+"\x00"+row.ID] = row.Values
		}
	}
	for _, item := range prepared.Provenance {
		values := preparedLookup[item.TargetTable+"\x00"+item.TargetID]
		if values == nil {
			// Exact duplicate pure joins intentionally share the first target row.
			values = Row{}
		}
		transformedData, err := json.Marshal(values)
		if err != nil {
			return fmt.Errorf("encode provenance %s/%s: %w", item.SourceTable, item.SourceID, err)
		}
		if _, err := tx.tx.Exec(ctx, `
INSERT INTO migration.row_provenance (
  run_id, source_table, source_id, target_table, target_id,
  source_checksum, transformed_checksum, transformed_data, duplicate_group
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			manifest.RunID, item.SourceTable, item.SourceID, item.TargetTable,
			item.TargetID, item.SourceChecksum, item.TransformedChecksum, transformedData,
			nilIfEmpty(item.DuplicateGroup)); err != nil {
			return err
		}
	}
	for _, item := range manifest.Anomalies {
		contextValue := item.Context
		if contextValue == nil {
			contextValue = Row{}
		}
		encodedContext, err := json.Marshal(contextValue)
		if err != nil {
			return fmt.Errorf("encode anomaly %s/%s: %w", item.Code, item.SourceID, err)
		}
		if _, err := tx.tx.Exec(ctx, `
INSERT INTO migration.anomalies (
  run_id, code, source_table, source_id, severity, detail, context,
  resolution_checksum
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			manifest.RunID, item.Code, item.SourceTable, item.SourceID,
			item.Severity, item.Detail, encodedContext, nilIfEmpty(item.ResolvedByChecksum)); err != nil {
			return err
		}
	}
	for _, item := range manifest.Resolutions {
		value := item.Value
		if value == nil {
			value = Row{}
		}
		encodedValue, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("encode resolution %s/%s: %w", item.AnomalyCode, item.SourceID, err)
		}
		if _, err := tx.tx.Exec(ctx, `
INSERT INTO migration.resolutions (
  run_id, anomaly_code, source_table, source_id, action, value,
  resolution_file_checksum
) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			manifest.RunID, item.AnomalyCode, item.SourceTable, item.SourceID,
			item.Action, encodedValue, manifest.ResolutionChecksum); err != nil {
			return err
		}
	}
	for _, item := range manifest.AutoFixes {
		beforeValue, err := jsonValue(item.Before)
		if err != nil {
			return fmt.Errorf("encode auto-fix %s before value: %w", item.Code, err)
		}

		afterValue, err := jsonValue(item.After)
		if err != nil {
			return fmt.Errorf("encode auto-fix %s after value: %w", item.Code, err)
		}
		if _, err := tx.tx.Exec(ctx, `
INSERT INTO migration.auto_fixes (
  run_id, source_table, source_id, code, before_value, after_value
) VALUES ($1, $2, $3, $4, $5, $6)`,
			manifest.RunID, item.SourceTable, item.SourceID, item.Code,
			beforeValue, afterValue); err != nil {
			return err
		}
	}
	if _, err := tx.tx.Exec(ctx, `
UPDATE migration.runs
SET state = 'applied', finished_at = $2
WHERE id = $1 AND state = 'applying'`, manifest.RunID, tx.now().UTC()); err != nil {
		return err
	}
	return nil
}

func insertPreparedRow(ctx context.Context, tx pgx.Tx, table string, values Row) error {
	if !containsString(targetOrder, table) {
		return fmt.Errorf("target table %q is not allowed", table)
	}
	columns := make([]string, 0, len(values))
	for column := range values {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	encoded, err := json.Marshal(values)
	if err != nil {
		return err
	}

	quotedColumns := make([]string, 0, len(columns))
	selectedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted := pgx.Identifier{column}.Sanitize()
		quotedColumns = append(quotedColumns, quoted)
		selectedColumns = append(selectedColumns, "source_row."+quoted)
	}
	qualifiedTable := "app." + pgx.Identifier{table}.Sanitize()
	query := "INSERT INTO " + qualifiedTable + " (" + strings.Join(quotedColumns, ",") + ") SELECT " + strings.Join(selectedColumns, ",") + " FROM jsonb_populate_record(NULL::" + qualifiedTable + ", $1::jsonb) AS source_row"
	_, err = tx.Exec(ctx, query, encoded)
	return err
}
