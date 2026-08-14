#!/bin/sh
set -eu

fail() {
  printf 'production environment error: %s\n' "$1" >&2
  exit 1
}

require() {
  name="$1"
  eval "value=\${$name:-}"
  [ -n "$value" ] || fail "$name is required"
}

require PUBLIC_ORIGIN
case "$PUBLIC_ORIGIN" in
  https://*) ;;
  *) fail "PUBLIC_ORIGIN must use https" ;;
esac
case "$PUBLIC_ORIGIN" in
  */) fail "PUBLIC_ORIGIN must not have a trailing slash" ;;
esac
origin_authority="${PUBLIC_ORIGIN#https://}"
case "$origin_authority" in
  ""|*/*|*\?*|*\#*) fail "PUBLIC_ORIGIN must be an origin without a path, query, or fragment" ;;
esac

require AUTH_ISSUER
[ "$AUTH_ISSUER" = "$PUBLIC_ORIGIN" ] || fail "AUTH_ISSUER must equal PUBLIC_ORIGIN"
require AUTH_AUDIENCE
require AUTH_JWKS_URL
[ "$AUTH_JWKS_URL" = "http://auth:3001/api/auth/jwks" ] || fail "AUTH_JWKS_URL must use the internal auth JWKS endpoint"
require AUTH_TRUSTED_ORIGINS
[ "$AUTH_TRUSTED_ORIGINS" = "$PUBLIC_ORIGIN" ] || fail "AUTH_TRUSTED_ORIGINS must contain only PUBLIC_ORIGIN"
[ "${AUTH_TRUST_PROXY_HEADERS:-}" = "true" ] || fail "AUTH_TRUST_PROXY_HEADERS must be true"
[ "${AUTH_IP_ADDRESS_HEADER:-}" = "x-real-ip" ] || fail "AUTH_IP_ADDRESS_HEADER must be x-real-ip"
[ "${DEV_AUTH_BYPASS:-false}" = "false" ] || fail "DEV_AUTH_BYPASS must be false"
[ "${VITE_USE_DEMO_DATA:-false}" = "false" ] || fail "VITE_USE_DEMO_DATA must be false"

for name in POSTGRES_PASSWORD MIGRATION_DB_PASSWORD APP_DB_PASSWORD AUTH_DB_PASSWORD BETTER_AUTH_SECRET IDENTITY_SERVICE_TOKEN CURSOR_SECRET GOOGLE_CLIENT_ID GOOGLE_CLIENT_SECRET EMAIL_FROM AWS_REGION GRAFANA_ADMIN_PASSWORD; do
  require "$name"
done

[ "${#POSTGRES_PASSWORD}" -ge 20 ] || fail "POSTGRES_PASSWORD must contain at least 20 characters"
[ "${#MIGRATION_DB_PASSWORD}" -ge 20 ] || fail "MIGRATION_DB_PASSWORD must contain at least 20 characters"
[ "${#APP_DB_PASSWORD}" -ge 20 ] || fail "APP_DB_PASSWORD must contain at least 20 characters"
[ "${#AUTH_DB_PASSWORD}" -ge 20 ] || fail "AUTH_DB_PASSWORD must contain at least 20 characters"
[ "${#BETTER_AUTH_SECRET}" -ge 32 ] || fail "BETTER_AUTH_SECRET must contain at least 32 characters"
[ "${#IDENTITY_SERVICE_TOKEN}" -ge 32 ] || fail "IDENTITY_SERVICE_TOKEN must contain at least 32 characters"
[ "${#CURSOR_SECRET}" -ge 32 ] || fail "CURSOR_SECRET must contain at least 32 characters"
[ "${#GRAFANA_ADMIN_PASSWORD}" -ge 20 ] || fail "GRAFANA_ADMIN_PASSWORD must contain at least 20 characters"
[ "${EMAIL_PROVIDER:-}" = "ses" ] || fail "EMAIL_PROVIDER must be ses"
[ "${AUTH_COOKIE_SECURE:-}" = "true" ] || fail "AUTH_COOKIE_SECURE must be true"
case "$EMAIL_FROM" in
  *rsp.local*) fail "EMAIL_FROM must use an authorized production sender" ;;
esac

secret_names="POSTGRES_PASSWORD MIGRATION_DB_PASSWORD APP_DB_PASSWORD AUTH_DB_PASSWORD BETTER_AUTH_SECRET IDENTITY_SERVICE_TOKEN CURSOR_SECRET GOOGLE_CLIENT_SECRET GRAFANA_ADMIN_PASSWORD"
seen_names=""
for name in $secret_names; do
  eval "value=\${$name}"
  for previous_name in $seen_names; do
    eval "previous_value=\${$previous_name}"
    [ "$value" != "$previous_value" ] || fail "$name must not reuse $previous_name"
  done
  seen_names="$seen_names $name"
done

case "$BETTER_AUTH_SECRET:$IDENTITY_SERVICE_TOKEN:$CURSOR_SECRET:$POSTGRES_PASSWORD:$MIGRATION_DB_PASSWORD:$APP_DB_PASSWORD:$AUTH_DB_PASSWORD:$GRAFANA_ADMIN_PASSWORD" in
  *local*|*change-me*|*password*) fail "local placeholder credentials cannot be used in production" ;;
esac

printf 'production environment validation passed\n'
