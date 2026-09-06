package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (tx *postgresTx) Verification(ctx context.Context, manifest Manifest) (Verification, error) {
	var storedChecksum, state string
	if err := tx.tx.QueryRow(ctx, "SELECT manifest_checksum, state::text FROM migration.runs WHERE id = $1", manifest.RunID).Scan(&storedChecksum, &state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Verification{}, ErrRunNotFound
		}
		return Verification{}, err
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
		sort.Slice(prepared, func(i, j int) bool { return preparedRowSortKey(prepared[i]) < preparedRowSortKey(prepared[j]) })
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
	keyColumn := "id"
	if column, ok := targetKeyColumn[table]; ok {
		keyColumn = column
	}
	where, args := "t."+pgx.Identifier{keyColumn}.Sanitize()+"=$1", []any{targetID}
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
	keyColumn := "id"
	if column, ok := targetKeyColumn[table]; ok {
		keyColumn = column
	}
	err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+qualified+" WHERE "+pgx.Identifier{keyColumn}.Sanitize()+" = $1)", id).Scan(&exists)
	return exists, err
}
