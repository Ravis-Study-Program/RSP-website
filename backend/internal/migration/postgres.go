package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type PostgresSource struct {
	ConnectionString string
}

func (source *PostgresSource) Snapshot(ctx context.Context, options SnapshotOptions) (Snapshot, error) {
	if !options.ReadOnly || options.Isolation != IsolationRepeatableRead {
		return Snapshot{}, errors.New("legacy source requires READ ONLY, REPEATABLE READ")
	}
	connection, err := pgx.Connect(ctx, source.ConnectionString)
	if err != nil {
		return Snapshot{}, fmt.Errorf("connect legacy source: %w", err)
	}
	defer connection.Close(context.WithoutCancel(ctx))
	tx, err := connection.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin legacy snapshot: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	snapshot := Snapshot{Tables: make(map[string][]Row)}
	if err := tx.QueryRow(ctx, "SELECT transaction_timestamp()").Scan(&snapshot.CapturedAt); err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot timestamp: %w", err)
	}
	snapshot.CapturedAt = snapshot.CapturedAt.UTC()
	snapshot.Schema, err = readLegacySchema(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.EFHistory, err = readEFHistory(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	available := map[string]bool{}
	for _, table := range snapshot.Schema {
		available[table.Name] = true
	}
	for _, table := range LegacyTables {
		if !available[table] {
			continue
		}
		rows, err := readLegacyTable(ctx, tx, table)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Tables[table] = rows
	}
	if err := tx.Commit(ctx); err != nil {
		return Snapshot{}, fmt.Errorf("commit legacy snapshot: %w", err)
	}
	committed = true
	return snapshot, nil
}

func readLegacySchema(ctx context.Context, tx pgx.Tx) ([]TableSchema, error) {
	rows, err := tx.Query(ctx, `
SELECT c.table_name, c.column_name, c.data_type, c.is_nullable = 'YES',
       COALESCE(pk.ordinal_position, 0)
FROM information_schema.columns AS c
LEFT JOIN (
  SELECT kcu.table_schema, kcu.table_name, kcu.column_name, kcu.ordinal_position
  FROM information_schema.table_constraints AS tc
  JOIN information_schema.key_column_usage AS kcu
    ON kcu.constraint_schema = tc.constraint_schema
   AND kcu.constraint_name = tc.constraint_name
  WHERE tc.constraint_type = 'PRIMARY KEY'
) AS pk
  ON pk.table_schema = c.table_schema
 AND pk.table_name = c.table_name
 AND pk.column_name = c.column_name
WHERE c.table_schema = 'public'
  AND c.table_name = ANY($1::text[])
ORDER BY c.table_name, c.ordinal_position`, LegacyTables)
	if err != nil {
		return nil, fmt.Errorf("read legacy schema: %w", err)
	}
	defer rows.Close()
	byName := map[string]*TableSchema{}
	order := make([]string, 0)
	for rows.Next() {
		var table, column, dataType string
		var nullable bool
		var primaryOrdinal int
		if err := rows.Scan(&table, &column, &dataType, &nullable, &primaryOrdinal); err != nil {
			return nil, err
		}
		if byName[table] == nil {
			byName[table] = &TableSchema{Name: table}
			order = append(order, table)
		}
		byName[table].Columns = append(byName[table].Columns, Column{Name: column, Type: dataType, Nullable: nullable})
		if primaryOrdinal > 0 {
			byName[table].PrimaryKey = append(byName[table].PrimaryKey, column)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]TableSchema, 0, len(order))
	for _, name := range order {
		result = append(result, *byName[name])
	}
	return result, nil
}

func readEFHistory(ctx context.Context, tx pgx.Tx) ([]string, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('public."__EFMigrationsHistory"') IS NOT NULL`).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return []string{}, nil
	}
	rows, err := tx.Query(ctx, `SELECT "MigrationId" FROM public."__EFMigrationsHistory" ORDER BY "MigrationId"`)
	if err != nil {
		return nil, fmt.Errorf("read EF migration history: %w", err)
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func readLegacyTable(ctx context.Context, tx pgx.Tx, table string) ([]Row, error) {
	if _, allowed := legacyIDFields[table]; !allowed {
		return nil, fmt.Errorf("legacy table %q is not allowed", table)
	}
	query := "SELECT to_jsonb(source_row) FROM public." + pgx.Identifier{table}.Sanitize() + " AS source_row"
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("read legacy table %s: %w", table, err)
	}
	defer rows.Close()
	result := make([]Row, 0)
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(strings.NewReader(string(encoded)))
		decoder.UseNumber()
		var row Row
		if err := decoder.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode %s row: %w", table, err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(result, func(left, right int) bool { return sourceID(table, result[left]) < sourceID(table, result[right]) })
	return result, nil
}

type PostgresTarget struct {
	ConnectionString string
	Now              func() time.Time
}

func (target *PostgresTarget) Begin(ctx context.Context) (TargetTx, error) {
	connection, err := pgx.Connect(ctx, target.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("connect target: %w", err)
	}
	tx, err := connection.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		_ = connection.Close(context.WithoutCancel(ctx))
		return nil, err
	}
	now := target.Now
	if now == nil {
		now = time.Now
	}
	return &postgresTx{connection: connection, tx: tx, now: now}, nil
}

type postgresTx struct {
	connection *pgx.Conn
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

func (tx *postgresTx) Verification(ctx context.Context, manifest Manifest) (Verification, error) {
	var storedChecksum, state string
	if err := tx.tx.QueryRow(ctx, "SELECT manifest_checksum, state::text FROM migration.runs WHERE id = $1", manifest.RunID).Scan(&storedChecksum, &state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Verification{}, ErrRunNotFound
		}
		return Verification{}, err
	}
	if state == "rolled_back" {
		return Verification{}, ErrRunNotFound
	}
	if storedChecksum != manifest.Checksum {
		return Verification{}, ErrManifestChecksum
	}
	verification := Verification{RunID: manifest.RunID, ManifestChecksum: manifest.Checksum, Counts: map[string]int{}, Checksums: map[string]string{}}
	rowsBySource, err := tx.readCurrentPreparedRows(ctx, manifest.RunID)
	if err != nil {
		return Verification{}, err
	}
	for _, expected := range manifest.Tables {
		prepared := rowsBySource[expected.SourceTable]
		sort.Slice(prepared, func(i, j int) bool {
			if prepared[i].ID == prepared[j].ID {
				return prepared[i].SourceID < prepared[j].SourceID
			}
			return prepared[i].ID < prepared[j].ID
		})
		checksum, checksumErr := Checksum(prepared)
		if checksumErr != nil {
			return Verification{}, checksumErr
		}
		verification.Counts[expected.SourceTable] = len(prepared)
		verification.Checksums[expected.SourceTable] = checksum
		if len(prepared) != expected.TransformedCount || checksum != expected.TransformedChecksum {
			return Verification{}, fmt.Errorf("verification mismatch for %s: count %d/%d checksum %s/%s", expected.SourceTable, len(prepared), expected.TransformedCount, checksum, expected.TransformedChecksum)
		}
	}
	var missingTargets int
	provenanceRows, err := tx.tx.Query(ctx, `
SELECT DISTINCT target_table, target_id
FROM migration.row_provenance
WHERE run_id = $1
ORDER BY target_table, target_id`, manifest.RunID)
	if err != nil {
		return Verification{}, err
	}
	type targetReference struct{ table, id string }
	targets := make([]targetReference, 0)
	for provenanceRows.Next() {
		var table, id string
		if err := provenanceRows.Scan(&table, &id); err != nil {
			provenanceRows.Close()
			return Verification{}, err
		}
		targets = append(targets, targetReference{table: table, id: id})
	}
	if err := provenanceRows.Err(); err != nil {
		provenanceRows.Close()
		return Verification{}, err
	}
	provenanceRows.Close()
	for _, target := range targets {
		exists, err := targetRowExists(ctx, tx.tx, target.table, target.id)
		if err != nil {
			return Verification{}, err
		}
		if !exists {
			missingTargets++
		}
	}
	verification.ForeignKeyErrors = missingTargets
	if missingTargets == 0 {
		if _, err := tx.tx.Exec(ctx, "SET CONSTRAINTS ALL IMMEDIATE"); err != nil {
			return Verification{}, err
		}
		if _, err := tx.tx.Exec(ctx, "UPDATE migration.runs SET state = 'verified', finished_at = $2 WHERE id = $1 AND state IN ('applied', 'verified')", manifest.RunID, tx.now().UTC()); err != nil {
			return Verification{}, err
		}
	}
	return verification, nil
}

func (tx *postgresTx) readCurrentPreparedRows(ctx context.Context, runID string) (map[string][]PreparedRow, error) {
	rows, err := tx.tx.Query(ctx, `
SELECT source_table, source_id, target_table, target_id, transformed_data
FROM migration.row_provenance
WHERE run_id=$1
ORDER BY source_table,target_table,target_id,(transformed_data='{}'::jsonb),source_id`, runID)
	if err != nil {
		return nil, err
	}
	type provenanceRecord struct {
		sourceTable, sourceID, targetTable, targetID string
		transformedData                              []byte
	}
	records := make([]provenanceRecord, 0)
	for rows.Next() {
		var record provenanceRecord
		if err := rows.Scan(&record.sourceTable, &record.sourceID, &record.targetTable, &record.targetID, &record.transformedData); err != nil {
			rows.Close()
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := map[string][]PreparedRow{}
	seenTargets := map[string]bool{}
	for _, record := range records {
		targetKey := record.sourceTable + "\x00" + record.targetTable + "\x00" + record.targetID
		if seenTargets[targetKey] {
			// Exact duplicate pure joins are represented multiple times in
			// provenance, but only once in the transformed target row set.
			continue
		}
		seenTargets[targetKey] = true
		var expected Row
		if err := json.Unmarshal(record.transformedData, &expected); err != nil {
			return nil, err
		}
		current, err := readTargetRow(ctx, tx.tx, record.targetTable, record.targetID, expected)
		if err != nil {
			return nil, err
		}
		result[record.sourceTable] = append(result[record.sourceTable], PreparedRow{ID: record.targetID, SourceTable: record.sourceTable, SourceID: record.sourceID, Values: current})
	}
	return result, nil
}

func readTargetRow(ctx context.Context, tx pgx.Tx, table, targetID string, expected Row) (Row, error) {
	if !containsString(targetOrder, table) {
		return nil, fmt.Errorf("target table %q is not allowed", table)
	}
	columns := make([]string, 0, len(expected))
	for column := range expected {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	comparisons := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted := pgx.Identifier{column}.Sanitize()
		comparisons = append(comparisons, "t."+quoted+" IS NOT DISTINCT FROM expected."+quoted)
	}
	qualified := "app." + pgx.Identifier{table}.Sanitize()
	where, args := "t.id=$1", []any{targetID}
	if table == "leetcode_problem_category_mappings" {
		parts := strings.SplitN(targetID, "|", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid category mapping id %q", targetID)
		}
		where, args = "t.leetcode_problem_id=$1 AND t.category_id=$2", []any{parts[0], parts[1]}
	}
	encoded, err := json.Marshal(expected)
	if err != nil {
		return nil, err
	}
	args = append(args, encoded)
	var matches bool
	query := "SELECT (" + strings.Join(comparisons, " AND ") + ") FROM " + qualified + " t CROSS JOIN jsonb_populate_record(NULL::" + qualified + ", $" + strconv.Itoa(len(args)) + "::jsonb) expected WHERE " + where
	if err := tx.QueryRow(ctx, query, args...).Scan(&matches); err != nil {
		return nil, noRowsMigration(err)
	}
	if !matches {
		return nil, fmt.Errorf("migration target row mismatch for %s/%s", table, targetID)
	}
	return expected, nil
}

func noRowsMigration(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("migration target row is missing: %w", err)
	}
	return err
}

func targetRowExists(ctx context.Context, tx pgx.Tx, table, id string) (bool, error) {
	if !containsString(targetOrder, table) {
		return false, fmt.Errorf("target table %q is not allowed", table)
	}
	qualified := "app." + pgx.Identifier{table}.Sanitize()
	var exists bool
	if table == "leetcode_problem_category_mappings" {
		parts := strings.SplitN(id, "|", 2)
		if len(parts) != 2 {
			return false, fmt.Errorf("invalid category mapping id %q", id)
		}
		err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+qualified+" WHERE leetcode_problem_id = $1 AND category_id = $2)", parts[0], parts[1]).Scan(&exists)
		return exists, err
	}
	err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+qualified+" WHERE id = $1)", id).Scan(&exists)
	return exists, err
}

func (tx *postgresTx) RollbackRun(ctx context.Context, runID string) error {
	if !tx.locked {
		return errors.New("migration advisory lock is not held")
	}
	var state string
	if err := tx.tx.QueryRow(ctx, `SELECT state::text FROM migration.runs WHERE id=$1 FOR UPDATE`, runID).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRunNotFound
		}
		return err
	}
	if state != "applied" && state != "verified" {
		return ErrRunNotFound
	}
	// Refuse to destroy any row that has changed since import. This also proves
	// every provenance target still exists before the first DELETE executes.
	if _, err := tx.readCurrentPreparedRows(ctx, runID); err != nil {
		return fmt.Errorf("refuse rollback of changed migration targets: %w", err)
	}
	if _, err := tx.tx.Exec(ctx, `SELECT set_config('rsp.migration_rollback','on',true)`); err != nil {
		return err
	}
	for index := len(targetOrder) - 1; index >= 0; index-- {
		table := targetOrder[index]
		rows, err := tx.tx.Query(ctx, `
SELECT DISTINCT target_id
FROM migration.row_provenance
WHERE run_id = $1 AND target_table = $2
ORDER BY target_id`, runID, table)
		if err != nil {
			return err
		}
		ids := make([]string, 0)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		qualified := "app." + pgx.Identifier{table}.Sanitize()
		for _, id := range ids {
			if table == "leetcode_problem_category_mappings" {
				parts := strings.SplitN(id, "|", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid category mapping id %q", id)
				}
				if _, err := tx.tx.Exec(ctx, "DELETE FROM "+qualified+" WHERE leetcode_problem_id = $1 AND category_id = $2", parts[0], parts[1]); err != nil {
					return err
				}
			} else if _, err := tx.tx.Exec(ctx, "DELETE FROM "+qualified+" WHERE id = $1", id); err != nil {
				return err
			}
		}
	}
	command, err := tx.tx.Exec(ctx, `
UPDATE migration.runs
SET state = 'rolled_back', rolled_back_at = $2
WHERE id = $1 AND state IN ('applied', 'verified')`, runID, tx.now().UTC())
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrRunNotFound
	}
	return nil
}

func (tx *postgresTx) Commit(ctx context.Context) error {
	if tx.completed {
		return errors.New("transaction is already complete")
	}
	tx.completed = true
	commitErr := tx.tx.Commit(ctx)
	closeErr := tx.connection.Close(context.WithoutCancel(ctx))
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
	rollbackErr := tx.tx.Rollback(ctx)
	closeErr := tx.connection.Close(context.WithoutCancel(ctx))
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
