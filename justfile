set dotenv-load := true
set shell := ["bash", "-euo", "pipefail", "-c"]

# Install pinned JavaScript and Go dependencies.
bootstrap:
    pnpm install --frozen-lockfile
    go mod download

# Complete local stack with source reload.
dev:
    docker compose up --build -d

# Regenerate checked-in Go and TypeScript clients from api/openapi.yaml.
generate:
    go generate ./...
    pnpm --filter @rsp/web generate

# Apply app and pinned Better Auth schema migrations to local PostgreSQL.
migrate-up:
    docker compose run --rm --build migrate-app
    docker compose run --rm migrate-auth

# Seed deterministic non-production data.
seed:
    APP_ENV=development DATABASE_URL="postgresql://rsp_app:${APP_DB_PASSWORD:-rsp-local-app-database-password}@127.0.0.1:${POSTGRES_PORT:-5432}/${POSTGRES_DB:-rsp}?sslmode=disable&options=-c%20search_path%3Dapp" go run ./backend/cmd/rspctl seed

# Create local email/password accounts and a complete development fixture.
fake-data:
    APP_ENV=development RSP_FAKE_DATA_ORIGIN="${PUBLIC_ORIGIN:-http://localhost:8080}" RSP_FAKE_DATA_MAILPIT_URL="http://127.0.0.1:${MAILPIT_UI_PORT:-8025}" DATABASE_URL="postgresql://rsp_app:${APP_DB_PASSWORD:-rsp-local-app-database-password}@127.0.0.1:${POSTGRES_PORT:-5432}/${POSTGRES_DB:-rsp}?sslmode=disable&options=-c%20search_path%3Dapp" node scripts/generate-fake-data.mjs

# Generate the current TOTP code from a fake account's printed secret.
fake-totp secret:
    node scripts/generate-totp.mjs "{{secret}}"

# Non-mutating formatting, static-analysis, type, and Compose checks.
check:
    #!/usr/bin/env bash
    set -euo pipefail
    unformatted="$(gofmt -l backend)"
    if [[ -n "$unformatted" ]]; then
      printf 'Go files need gofmt:\n%s\n' "$unformatted" >&2
      exit 1
    fi
    go vet ./...
    pnpm check
    docker compose config --quiet

# Go and TypeScript unit/contract tests.
test:
    go test ./...
    pnpm test

# Browser workflows and automated accessibility checks.
e2e:
    pnpm --filter @rsp/web e2e

# Full browser acceptance through Caddy, Better Auth, Go, and PostgreSQL.
# Uses its own project, ports, and volumes and always tears them down.
e2e-real:
    #!/usr/bin/env bash
    set -euo pipefail
    project="rsp-real-e2e"
    export COMPOSE_PROJECT_NAME="$project"
    output_env="$(mktemp)"
    export REAL_E2E_HTTP_PORT="${RSP_E2E_PORT:-18080}"
    export POSTGRES_PORT="${RSP_E2E_POSTGRES_PORT:-15432}"
    export MAILPIT_SMTP_PORT="${RSP_E2E_MAILPIT_SMTP_PORT:-11025}"
    export MAILPIT_UI_PORT="${RSP_E2E_MAILPIT_UI_PORT:-18025}"
    export PROMETHEUS_PORT="${RSP_E2E_PROMETHEUS_PORT:-19090}"
    export GRAFANA_PORT="${RSP_E2E_GRAFANA_PORT:-13000}"
    export PUBLIC_ORIGIN="http://127.0.0.1:${REAL_E2E_HTTP_PORT}"
    export AUTH_ISSUER="$PUBLIC_ORIGIN"
    export AUTH_TRUSTED_ORIGINS="$PUBLIC_ORIGIN"
    export POSTGRES_PASSWORD="rsp-real-e2e-postgres-password"
    export MIGRATION_DB_PASSWORD="rsp-real-e2e-migration-password"
    export APP_DB_PASSWORD="rsp-real-e2e-app-database-password"
    export AUTH_DB_PASSWORD="rsp-real-e2e-auth-database-password"
    export BETTER_AUTH_SECRET="rsp-real-e2e-better-auth-secret-000000"
    export IDENTITY_SERVICE_TOKEN="rsp-real-e2e-identity-service-token-000000"
    export CURSOR_SECRET="rsp-real-e2e-cursor-secret-0000000000000"
    export RSP_E2E_BASE_URL="$PUBLIC_ORIGIN"
    export RSP_E2E_MAILPIT_URL="http://127.0.0.1:${MAILPIT_UI_PORT}"
    export APP_ENV=test
    export RSP_E2E_ALLOW_MUTATION=true
    export RSP_E2E_STUDENT_EMAIL="e2e.student@example.test"
    export RSP_E2E_STUDENT_PASSWORD="RspRealE2eOnly-Student-2026"
    export RSP_E2E_COORDINATOR_EMAIL="e2e.coordinator@example.test"
    export RSP_E2E_COORDINATOR_PASSWORD="RspRealE2eOnly-Coordinator-2026"
    export RSP_E2E_DIRECTOR_EMAIL="e2e.director@example.test"
    export RSP_E2E_DIRECTOR_PASSWORD="RspRealE2eOnly-Director-2026"
    compose=(docker compose -p "$project" -f compose.yaml -f deploy/compose.real-e2e.yaml)
    cleanup() {
      status=$?
      trap - EXIT
      if [[ $status -ne 0 ]]; then
        app_tables_ready="$("${compose[@]}" exec -T postgres psql --username rsp --dbname rsp --tuples-only --no-align --command "SELECT to_regclass('app.users') IS NOT NULL AND to_regclass('app.user_auth_links') IS NOT NULL AND to_regclass('app.identity_event_receipts') IS NOT NULL" 2>/dev/null || true)"
        if [[ "$app_tables_ready" == "t" ]]; then
          "${compose[@]}" exec -T postgres psql --username rsp --dbname rsp --command \
            "SELECT (SELECT count(*) FROM app.users) AS app_users, (SELECT count(*) FROM app.user_auth_links) AS auth_links, (SELECT count(*) FROM app.identity_event_receipts) AS identity_receipts;" || true
        fi
        auth_outbox_ready="$("${compose[@]}" exec -T postgres psql --username rsp --dbname rsp --tuples-only --no-align --command "SELECT to_regclass('auth.lifecycle_outbox') IS NOT NULL" 2>/dev/null || true)"
        if [[ "$auth_outbox_ready" == "t" ]]; then
          "${compose[@]}" exec -T postgres psql --username rsp --dbname rsp --command \
            "SELECT payload->>'type' AS event_type, payload->>'authUserId' AS auth_user_id, attempt_count, last_error FROM auth.lifecycle_outbox WHERE delivered_at IS NULL ORDER BY created_at;" || true
        fi
        "${compose[@]}" logs --tail=500 || true
      fi
      "${compose[@]}" down --volumes --remove-orphans || true
      rm -f "$output_env"
      exit "$status"
    }
    trap cleanup EXIT INT TERM
    "${compose[@]}" up --build --detach
    curl --fail --show-error --retry 30 --retry-delay 2 --retry-all-errors "$PUBLIC_ORIGIN/health/live" >/dev/null
    curl --fail --show-error --retry 30 --retry-delay 2 --retry-all-errors "$PUBLIC_ORIGIN/api/v2/health/ready" >/dev/null
    "${compose[@]}" stop auth
    "${compose[@]}" run --rm --no-deps \
      -e AUTH_LIVE_TEST=true \
      -e AUTH_LIVE_MAILPIT_URL=http://mailpit:8025 \
      -e BETTER_AUTH_URL=http://127.0.0.1:3001 \
      -e AUTH_TRUSTED_ORIGINS=http://127.0.0.1:3001 \
      -e IDENTITY_SERVICE_URL=http://127.0.0.1:4000 \
      auth pnpm --filter @rsp/auth exec vitest run test/live-auth-handler.test.ts
    "${compose[@]}" start auth
    auth_ready=false
    for _ in {1..30}; do
      if "${compose[@]}" exec -T auth node -e "fetch('http://127.0.0.1:3001/health/ready').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"; then
        auth_ready=true
        break
      fi
      sleep 2
    done
    [[ "$auth_ready" == "true" ]]
    RSP_E2E_ALLOW_DATABASE_WRITE=true RSP_E2E_OUTPUT_ENV="$output_env" \
      node deploy/scripts/provision-real-e2e.mjs
    set -a
    source "$output_env"
    set +a
    pnpm --filter @rsp/web e2e:real

# Useful lifecycle helpers.
dev-down:
    docker compose down

logs:
    docker compose logs --follow --tail=200
