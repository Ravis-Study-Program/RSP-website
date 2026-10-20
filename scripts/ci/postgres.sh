#!/usr/bin/env bash
# Migrate the CI PostgreSQL database and prove the migrations behave.
#
#   scripts/ci/postgres.sh migrate                 apply app and auth migrations
#   scripts/ci/postgres.sh prove-auth-upgrade      upgrade an existing Better Auth schema, twice
#   scripts/ci/postgres.sh prove-auth-concurrent   run two auth migrators at once
#   scripts/ci/postgres.sh prove-app-roundtrip     migrate the app schema down to 0 and back up
#
# Required environment:
#   DATABASE_URL   postgresql://... for the main CI database
set -euo pipefail

command=${1:?usage: postgres.sh migrate|prove-auth-upgrade|prove-auth-concurrent|prove-app-roundtrip}
: "${DATABASE_URL:?}"

sql() {
  psql "$1" --set ON_ERROR_STOP=1 "${@:2}"
}

# The DATABASE_URL with its database name replaced by $1.
database_url() {
  local base=${DATABASE_URL%%\?*}
  local query=${DATABASE_URL#"$base"}
  echo "${base%/*}/$1$query"
}

# Fail unless the boolean query $2 is true in database $1.
assert_true() {
  local result
  result="$(sql "$1" --tuples-only --no-align --command "$2")"
  test "$result" = "t"
}

case $command in
  migrate)
    goose -dir db/migrations postgres "$DATABASE_URL" up
    sql "$DATABASE_URL" --command "CREATE SCHEMA IF NOT EXISTS auth"
    sql "$DATABASE_URL" --file deploy/postgres/migrate-auth.sql
    sql "$DATABASE_URL" --file deploy/postgres/migrate-auth.sql
    ;;
  prove-auth-upgrade)
    sql "$DATABASE_URL" --command "CREATE DATABASE rsp_auth_upgrade"
    upgrade_url="$(database_url rsp_auth_upgrade)"
    sql "$upgrade_url" --command "CREATE SCHEMA auth"
    sql "$upgrade_url" --file apps/auth/migrations/better-auth.sql
    sql "$upgrade_url" --file deploy/postgres/migrate-auth.sql
    sql "$upgrade_url" --file deploy/postgres/migrate-auth.sql
    assert_true "$upgrade_url" \
      "SELECT to_regclass('auth.account_deletion_recovery_tokens') IS NOT NULL AND to_regclass('auth.lifecycle_outbox') IS NOT NULL AND (SELECT count(*) = 2 FROM auth.schema_migrations)"
    sql "$DATABASE_URL" --command "DROP DATABASE rsp_auth_upgrade"
    ;;
  prove-auth-concurrent)
    sql "$DATABASE_URL" --command "CREATE DATABASE rsp_auth_concurrent"
    concurrent_url="$(database_url rsp_auth_concurrent)"
    sql "$concurrent_url" --command "CREATE SCHEMA auth"
    sql "$concurrent_url" --file deploy/postgres/migrate-auth.sql &
    first_pid=$!
    sql "$concurrent_url" --file deploy/postgres/migrate-auth.sql &
    second_pid=$!
    wait "$first_pid"
    wait "$second_pid"
    assert_true "$concurrent_url" \
      "SELECT to_regclass('auth.lifecycle_outbox') IS NOT NULL AND (SELECT count(*) = 2 FROM auth.schema_migrations)"
    sql "$DATABASE_URL" --command "DROP DATABASE rsp_auth_concurrent"
    ;;
  prove-app-roundtrip)
    goose -dir db/migrations postgres "$DATABASE_URL" down-to 0
    goose -dir db/migrations postgres "$DATABASE_URL" up
    sql "$DATABASE_URL" --command \
      "SELECT to_regclass('app.audit_events'), to_regclass('migration.runs'), to_regclass('auth.users');"
    ;;
  *)
    echo "unknown command: $command" >&2
    exit 2
    ;;
esac
