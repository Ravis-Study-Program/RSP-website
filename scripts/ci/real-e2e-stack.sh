#!/usr/bin/env bash
# Drive the isolated Docker Compose stack used by the real-stack browser tests.
#
#   scripts/ci/real-e2e-stack.sh up               build, start and wait for health
#   scripts/ci/real-e2e-stack.sh auth-live-tests  run the live Better Auth tests against it
#   scripts/ci/real-e2e-stack.sh diagnostics      print database state and logs after a failure
#   scripts/ci/real-e2e-stack.sh down             stop it and delete its volumes
#
# Uses the stack settings from the CI job environment (COMPOSE_PROJECT_NAME,
# RSP_E2E_BASE_URL, POSTGRES_USER, POSTGRES_DB, ...).
set -euo pipefail

command=${1:?usage: real-e2e-stack.sh up|auth-live-tests|diagnostics|down}

compose() {
  docker compose -f compose.yaml -f deploy/compose.real-e2e.yaml "$@"
}

# Run a query in the stack's database; extra psql flags come first.
stack_sql() {
  compose exec -T postgres psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" "$@"
}

case $command in
  up)
    : "${RSP_E2E_BASE_URL:?}"
    compose up --build --detach
    curl --fail --show-error --retry 30 --retry-delay 2 --retry-all-errors \
      "$RSP_E2E_BASE_URL/health/live" > /dev/null
    curl --fail --show-error --retry 30 --retry-delay 2 --retry-all-errors \
      "$RSP_E2E_BASE_URL/api/v2/health/ready" > /dev/null
    compose ps
    ;;
  auth-live-tests)
    # The tests start their own auth server on the auth container's port.
    compose stop auth
    compose run --rm --no-deps \
      -e AUTH_LIVE_TEST=true \
      -e AUTH_LIVE_MAILPIT_URL=http://mailpit:8025 \
      -e BETTER_AUTH_URL=http://127.0.0.1:3001 \
      -e AUTH_TRUSTED_ORIGINS=http://127.0.0.1:3001 \
      -e IDENTITY_SERVICE_URL=http://127.0.0.1:4000 \
      auth pnpm --filter @rsp/auth exec vitest run test/live-auth-handler.test.ts
    compose start auth
    for _ in {1..30}; do
      if compose exec -T auth node -e \
        "fetch('http://127.0.0.1:3001/health/ready').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"; then
        exit 0
      fi
      sleep 2
    done
    echo "auth did not become ready after the live tests" >&2
    exit 1
    ;;
  diagnostics)
    app_tables_ready="$(stack_sql --tuples-only --no-align --command \
      "SELECT to_regclass('app.users') IS NOT NULL AND to_regclass('app.user_auth_links') IS NOT NULL AND to_regclass('app.identity_event_receipts') IS NOT NULL" 2>/dev/null || true)"
    if [[ "$app_tables_ready" == "t" ]]; then
      stack_sql --command \
        "SELECT (SELECT count(*) FROM app.users) AS app_users, (SELECT count(*) FROM app.user_auth_links) AS auth_links, (SELECT count(*) FROM app.identity_event_receipts) AS identity_receipts;" || true
    fi
    auth_outbox_ready="$(stack_sql --tuples-only --no-align --command \
      "SELECT to_regclass('auth.lifecycle_outbox') IS NOT NULL" 2>/dev/null || true)"
    if [[ "$auth_outbox_ready" == "t" ]]; then
      stack_sql --command \
        "SELECT payload->>'type' AS event_type, payload->>'authUserId' AS auth_user_id, attempt_count, last_error FROM auth.lifecycle_outbox WHERE delivered_at IS NULL ORDER BY created_at;" || true
    fi
    compose logs --tail=500
    ;;
  down)
    compose down --volumes --remove-orphans
    ;;
  *)
    echo "unknown command: $command" >&2
    exit 2
    ;;
esac
