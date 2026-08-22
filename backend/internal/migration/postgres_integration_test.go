//go:build integration

package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgres18SourceAndTargetLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := postgrescontainer.Run(ctx, "postgres:18.6-alpine3.24",
		postgrescontainer.WithDatabase("rsp"),
		postgrescontainer.WithUsername("rsp"),
		postgrescontainer.WithPassword("rsp-migration-integration-password"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	adminDSN, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	adminDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()
	for _, role := range []string{"rsp_migration", "rsp_app", "rsp_auth"} {
		if _, err := adminDB.ExecContext(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := adminDB.ExecContext(ctx, `GRANT CREATE ON DATABASE rsp TO rsp_migration`); err != nil {
		t.Fatal(err)
	}
	if _, err := adminDB.ExecContext(ctx, `GRANT USAGE, CREATE ON SCHEMA public TO rsp_migration`); err != nil {
		t.Fatal(err)
	}

	migrationDSN := adminDSN + "&options=-c%20role%3Drsp_migration"
	migrationDB, err := sql.Open("pgx", migrationDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer migrationDB.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Abs("../../../db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(migrationDB, migrations); err != nil {
		t.Fatal(err)
	}
	fixture := validSnapshot(t)
	if err := seedPostgresLegacyFixture(ctx, adminDB, fixture); err != nil {
		t.Fatal(err)
	}

	source := &PostgresSource{ConnectionString: adminDSN}
	if _, err := source.Snapshot(ctx, SnapshotOptions{ReadOnly: false, Isolation: IsolationRepeatableRead}); err == nil {
		t.Fatal("PostgresSource accepted a writable snapshot")
	}
	snapshot, err := source.Snapshot(ctx, SnapshotOptions{ReadOnly: true, Isolation: IsolationRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Schema) != len(LegacyTables) || len(snapshot.Tables) != len(LegacyTables) {
		t.Fatalf("source schema=%d tables=%d, want %d", len(snapshot.Schema), len(snapshot.Tables), len(LegacyTables))
	}
	if len(snapshot.EFHistory) != len(fixture.EFHistory) || snapshot.CapturedAt.Location() != time.UTC {
		t.Fatalf("source EF history=%v capturedAt=%v", snapshot.EFHistory, snapshot.CapturedAt)
	}

	engine := NewEngine(fixedNow)
	prepared, err := engine.DryRun(ctx, source, nil)
	if err != nil {
		t.Fatal(err)
	}
	target := &PostgresTarget{ConnectionString: migrationDSN, Now: fixedNow}
	if err := engine.Apply(ctx, source, target, prepared.Manifest, nil); err != nil {
		t.Fatal(err)
	}
	assertPostgresRunState(t, ctx, adminDB, prepared.Manifest.RunID, "applied")
	verification, err := engine.Verify(ctx, target, prepared.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if verification.ForeignKeyErrors != 0 || len(verification.Counts) != len(LegacyTables) {
		t.Fatalf("verification = %+v", verification)
	}
	assertPostgresRunState(t, ctx, adminDB, prepared.Manifest.RunID, "verified")

	round := preparedSourceRow(prepared, "lc-round-1")
	if _, err := adminDB.ExecContext(ctx, `UPDATE app.leetcode_mock_interview_rounds SET coding_score=9 WHERE id=$1`, round.ID); err != nil {
		t.Fatal(err)
	}
	if err := engine.Rollback(ctx, target, prepared.Manifest.RunID); err == nil || !strings.Contains(err.Error(), "refuse rollback of changed migration targets") {
		t.Fatalf("tampered rollback error = %v", err)
	}
	assertPostgresRunState(t, ctx, adminDB, prepared.Manifest.RunID, "verified")
	if _, err := adminDB.ExecContext(ctx, `UPDATE app.leetcode_mock_interview_rounds SET coding_score=8 WHERE id=$1`, round.ID); err != nil {
		t.Fatal(err)
	}
	if err := engine.Rollback(ctx, target, prepared.Manifest.RunID); err != nil {
		t.Fatal(err)
	}
	assertPostgresRunState(t, ctx, adminDB, prepared.Manifest.RunID, "rolled_back")
	var appUsers, provenanceRows int
	if err := adminDB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM app.users), (SELECT count(*) FROM migration.row_provenance WHERE run_id=$1)`, prepared.Manifest.RunID).Scan(&appUsers, &provenanceRows); err != nil {
		t.Fatal(err)
	}
	if appUsers != 0 || provenanceRows == 0 {
		t.Fatalf("rollback appUsers=%d provenanceRows=%d", appUsers, provenanceRows)
	}
	if _, err := engine.Verify(ctx, target, prepared.Manifest); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("verify after rollback error = %v", err)
	}
}

func seedPostgresLegacyFixture(ctx context.Context, db *sql.DB, snapshot Snapshot) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE public."__EFMigrationsHistory" ("MigrationId" text PRIMARY KEY)`); err != nil {
		return err
	}
	for _, migrationID := range snapshot.EFHistory {
		if _, err := db.ExecContext(ctx, `INSERT INTO public."__EFMigrationsHistory" ("MigrationId") VALUES ($1)`, migrationID); err != nil {
			return err
		}
	}
	for _, table := range LegacyTables {
		columns := map[string]bool{}
		for _, idColumn := range legacyIDFields[table] {
			columns[idColumn] = true
		}
		for _, row := range snapshot.Tables[table] {
			for column := range row {
				columns[column] = true
			}
		}
		ordered := make([]string, 0, len(columns))
		for column := range columns {
			ordered = append(ordered, column)
		}
		sort.Strings(ordered)
		definitions := make([]string, 0, len(ordered)+1)
		for _, column := range ordered {
			definition := pgx.Identifier{column}.Sanitize() + " jsonb"
			if containsString(legacyIDFields[table], column) {
				definition += " NOT NULL"
			}
			definitions = append(definitions, definition)
		}
		primaryColumns := make([]string, 0, len(legacyIDFields[table]))
		for _, column := range legacyIDFields[table] {
			primaryColumns = append(primaryColumns, pgx.Identifier{column}.Sanitize())
		}
		definitions = append(definitions, "PRIMARY KEY ("+strings.Join(primaryColumns, ",")+")")
		qualified := "public." + pgx.Identifier{table}.Sanitize()
		if _, err := db.ExecContext(ctx, "CREATE TABLE "+qualified+" ("+strings.Join(definitions, ",")+")"); err != nil {
			return fmt.Errorf("create legacy table %s: %w", table, err)
		}
		for _, row := range snapshot.Tables[table] {
			encoded, err := json.Marshal(row)
			if err != nil {
				return err
			}
			query := "INSERT INTO " + qualified + " SELECT source_row.* FROM jsonb_populate_record(NULL::" + qualified + ", $1::jsonb) AS source_row"
			if _, err := db.ExecContext(ctx, query, encoded); err != nil {
				return fmt.Errorf("insert legacy table %s: %w", table, err)
			}
		}
	}
	return nil
}

func assertPostgresRunState(t *testing.T, ctx context.Context, db *sql.DB, runID, expected string) {
	t.Helper()
	var state string
	if err := db.QueryRowContext(ctx, `SELECT state::text FROM migration.runs WHERE id=$1`, runID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != expected {
		t.Fatalf("migration state = %q, want %q", state, expected)
	}
}
