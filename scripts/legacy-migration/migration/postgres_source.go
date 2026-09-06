package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type PostgresSource struct {
	ConnectionString string
}

func (source *PostgresSource) Snapshot(ctx context.Context, options SnapshotOptions) (Snapshot, error) {
	if !options.ReadOnly || options.Isolation != IsolationRepeatableRead {
		return Snapshot{}, errors.New("legacy source requires READ ONLY, REPEATABLE READ")
	}
	repository, err := connectMigration(ctx, source.ConnectionString)
	if err != nil {
		return Snapshot{}, fmt.Errorf("connect legacy source: %w", err)
	}

	defer repository.Close(context.Background())
	tx, err := repository.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin legacy snapshot: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
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
