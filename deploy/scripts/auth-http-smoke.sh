#!/bin/sh
set -eu

fail() {
  printf 'auth HTTP smoke failed: %s\n' "$1" >&2
  exit 1
}

[ "${AUTH_SMOKE_ALLOW_DATABASE_WRITE:-}" = "true" ] ||
  fail "set AUTH_SMOKE_ALLOW_DATABASE_WRITE=true for the disposable smoke database"

for command in curl docker jq openssl sed; do
  command -v "$command" >/dev/null 2>&1 || fail "$command is required"
done

base_url="${AUTH_SMOKE_URL:-http://127.0.0.1:3001}"
api_url="${AUTH_SMOKE_API_URL:-http://127.0.0.1:4000}"
mailpit_url="${AUTH_SMOKE_MAILPIT_URL:-http://127.0.0.1:8025}"
expected_issuer="${AUTH_SMOKE_EXPECTED_ISSUER:-$base_url}"
database_name="${POSTGRES_DB:-rsp}"
database_user="${POSTGRES_USER:-rsp}"
run_suffix="${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-0}-$$"
email="auth-smoke-${run_suffix}@example.test"
password='CiAuthSmokePassword-2026'
new_password='CiAuthSmokePassword-2026-Reset'
temporary_directory="$(mktemp -d)"
trap 'rm -rf "$temporary_directory"' EXIT HUP INT TERM

post_json() {
  endpoint="$1"
  payload="$2"
  response_file="$3"
  cookie_file="${4:-}"
  if [ -n "$cookie_file" ]; then
    curl --silent --show-error --output "$response_file" --write-out '%{http_code}' \
      --cookie "$cookie_file" --cookie-jar "$cookie_file" \
      --header 'content-type: application/json' --header "origin: $base_url" \
      --request POST --data "$payload" "$base_url$endpoint"
  else
    curl --silent --show-error --output "$response_file" --write-out '%{http_code}' \
      --header 'content-type: application/json' --header "origin: $base_url" \
      --request POST --data "$payload" "$base_url$endpoint"
  fi
}

session_count() {
  docker compose exec -T postgres psql \
    --username "$database_user" --dbname "$database_name" --no-align --tuples-only \
    --command "SELECT count(*) FROM auth.sessions s JOIN auth.users u ON u.id=s.user_id WHERE u.email='$email';" \
    | tr -d '[:space:]'
}

decode_jwt_part() {
  encoded="$1"
  remainder=$((${#encoded} % 4))
  case "$remainder" in
    0) padded="$encoded" ;;
    2) padded="${encoded}==" ;;
    3) padded="${encoded}=" ;;
    *) fail "JWT contains an invalid base64url segment" ;;
  esac
  printf '%s' "$padded" | tr '_-' '/+' | openssl base64 -d -A
}

curl --fail --silent --show-error --retry 24 --retry-delay 5 --retry-all-errors \
  "$base_url/health/ready" >/dev/null
curl --fail --silent --show-error --retry 24 --retry-delay 5 --retry-all-errors \
  "$mailpit_url/api/v1/info" >/dev/null

weak_payload="$(jq -cn --arg email "$email" \
  '{name:"Auth Smoke",email:$email,password:"short"}')"
weak_status="$(post_json /api/auth/sign-up/email "$weak_payload" \
  "$temporary_directory/weak.json")"
[ "$weak_status" = "400" ] || fail "short password returned HTTP $weak_status, expected 400"
jq -e '.code == "PASSWORD_TOO_SHORT"' "$temporary_directory/weak.json" >/dev/null ||
  fail "short-password response did not carry PASSWORD_TOO_SHORT"

signup_payload="$(jq -cn --arg email "$email" --arg password "$password" \
  '{name:"Auth Smoke",email:$email,password:$password}')"
signup_status="$(post_json /api/auth/sign-up/email "$signup_payload" \
  "$temporary_directory/signup.json")"
[ "$signup_status" = "200" ] || fail "valid sign-up returned HTTP $signup_status, expected 200"
jq -e --arg email "$email" \
  '.token == null and .user.email == $email and .user.emailVerified == false' \
  "$temporary_directory/signup.json" >/dev/null || fail "valid sign-up response was unexpected"

signin_payload="$(jq -cn --arg email "$email" --arg password "$password" \
  '{email:$email,password:$password}')"
unverified_status="$(post_json /api/auth/sign-in/email "$signin_payload" \
  "$temporary_directory/unverified.json")"
[ "$unverified_status" = "403" ] ||
  fail "unverified sign-in returned HTTP $unverified_status, expected 403"
jq -e '.code == "EMAIL_NOT_VERIFIED"' "$temporary_directory/unverified.json" >/dev/null ||
  fail "unverified sign-in did not carry EMAIL_NOT_VERIFIED"
[ "$(session_count)" = "0" ] || fail "unverified sign-in created a database session"

verification_messages="$temporary_directory/verification-messages.json"
curl --fail --silent --show-error --retry 12 --retry-delay 1 \
  "$mailpit_url/api/v1/messages" > "$verification_messages"
verification_message_id="$(jq -er --arg email "$email" \
  '[.messages[] | select(.Subject == "Verify your RSP email") | select(any(.To[]; .Address == $email))][0].ID' \
  "$verification_messages")" || fail "Mailpit did not capture the verification email"
curl --fail --silent --show-error "$mailpit_url/api/v1/message/$verification_message_id" \
  > "$temporary_directory/verification-message.json"
verification_url="$(jq -er '.Text' "$temporary_directory/verification-message.json" | tr -d '\r' \
  | sed -n 's/^Verify email: //p' | sed -n '1p')"
[ -n "$verification_url" ] || fail "verification URL was absent from the captured email"
verification_status="$(curl --silent --show-error --output "$temporary_directory/verification.json" \
  --write-out '%{http_code}' "$verification_url")"
case "$verification_status" in 200|302) ;; *) fail "verification callback returned HTTP $verification_status" ;; esac

verified_rows="$(docker compose exec -T postgres psql \
  --username "$database_user" --dbname "$database_name" --no-align --tuples-only \
  --command "SELECT count(*) FROM auth.users WHERE email='$email' AND email_verified;" \
  | tr -d '[:space:]')"
[ "$verified_rows" = "1" ] || fail "verification callback did not verify exactly one database user"
verified_events="$(docker compose exec -T postgres psql \
  --username "$database_user" --dbname "$database_name" --no-align --tuples-only \
  --command "SELECT count(*) FROM auth.lifecycle_outbox WHERE payload->>'type'='email_verified' AND payload->>'authUserId'=(SELECT id FROM auth.users WHERE email='$email');" \
  | tr -d '[:space:]')"
[ "$verified_events" = "1" ] || fail "verification callback did not enqueue the email_verified lifecycle event"

cookie_file="$temporary_directory/cookies.txt"
signin_status="$(post_json /api/auth/sign-in/email "$signin_payload" \
  "$temporary_directory/signin.json" "$cookie_file")"
[ "$signin_status" = "200" ] || fail "verified sign-in returned HTTP $signin_status, expected 200"
jq -e --arg email "$email" '.user.email == $email and .user.emailVerified == true' \
  "$temporary_directory/signin.json" >/dev/null || fail "verified sign-in response was unexpected"
[ "$(session_count)" = "1" ] || fail "verified sign-in did not create exactly one database session"
grep -q 'rsp-auth.session_token' "$cookie_file" || fail "sign-in did not set the session cookie"

token_status="$(curl --silent --show-error --output "$temporary_directory/token.json" \
  --write-out '%{http_code}' --cookie "$cookie_file" "$base_url/api/auth/token")"
[ "$token_status" = "200" ] || fail "token exchange returned HTTP $token_status, expected 200"
access_token="$(jq -er '.token' "$temporary_directory/token.json")" || fail "token response was missing JWT"
token_header="$(decode_jwt_part "$(printf '%s' "$access_token" | cut -d. -f1)")"
token_claims="$(decode_jwt_part "$(printf '%s' "$access_token" | cut -d. -f2)")"
printf '%s' "$token_header" > "$temporary_directory/token-header.json"
printf '%s' "$token_claims" > "$temporary_directory/token-claims.json"

jwks_status="$(curl --silent --show-error --output "$temporary_directory/jwks.json" \
  --write-out '%{http_code}' "$base_url/api/auth/jwks")"
[ "$jwks_status" = "200" ] || fail "JWKS returned HTTP $jwks_status, expected 200"
jq -e --slurpfile header "$temporary_directory/token-header.json" \
  '.keys | any(.kid == $header[0].kid and .alg == $header[0].alg and .kty == "OKP")' \
  "$temporary_directory/jwks.json" >/dev/null || fail "JWT key ID/algorithm was absent from JWKS"
jq -e --arg issuer "$expected_issuer" '
  .iss == $issuer and
  ((.aud == "rsp-api") or ((.aud | type) == "array" and (.aud | index("rsp-api") != null))) and
  .sub == .authUserId and
  .emailVerified == true and
  .accountState == "active" and
  .securityVersion == 1 and
  .mfaVerified == false and
  (.sessionId | type) == "string" and
  (.exp - .iat) == 300 and
  (has("email") | not)
' "$temporary_directory/token-claims.json" >/dev/null || fail "JWT claims did not match the five-minute access-token contract"

me_status="$(curl --silent --show-error --output "$temporary_directory/me.json" \
  --write-out '%{http_code}' --header "authorization: Bearer $access_token" \
  "$api_url/api/v2/me")"
[ "$me_status" = "200" ] || fail "Go API rejected the issued JWT with HTTP $me_status"
auth_subject="$(jq -er '.sub' "$temporary_directory/token-claims.json")"
mapped_user_id="$(printf '%s\n' \
  "SELECT user_id FROM app.user_auth_links WHERE auth_subject=:'auth_subject' AND active;" \
  | docker compose exec -T postgres psql \
      --username "$database_user" --dbname "$database_name" --no-align --tuples-only \
      --set auth_subject="$auth_subject" \
  | tr -d '[:space:]')"
[ -n "$mapped_user_id" ] || fail "Go API did not persist an active auth-subject link"
jq -e --arg app_user_id "$mapped_user_id" \
  '.id == $app_user_id and .emailVerified == true' "$temporary_directory/me.json" >/dev/null ||
  fail "Go API identity did not resolve through user_auth_links"
[ "$mapped_user_id" != "$auth_subject" ] || fail "Go runtime identity incorrectly reused the auth subject"

signed_out_cookie="$temporary_directory/signed-out-cookie.txt"
cp "$cookie_file" "$signed_out_cookie"
signout_status="$(post_json /api/auth/sign-out '{}' "$temporary_directory/signout.json" "$cookie_file")"
[ "$signout_status" = "200" ] || fail "sign-out returned HTTP $signout_status, expected 200"
jq -e '.success == true' "$temporary_directory/signout.json" >/dev/null || fail "sign-out response was unexpected"
[ "$(session_count)" = "0" ] || fail "sign-out did not delete the database session"
revoked_status="$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
  --cookie "$signed_out_cookie" "$base_url/api/auth/token")"
case "$revoked_status" in 401|403) ;; *) fail "signed-out cookie returned HTTP $revoked_status" ;; esac

reset_cookie="$temporary_directory/reset-session-cookie.txt"
signin_status="$(post_json /api/auth/sign-in/email "$signin_payload" \
  "$temporary_directory/signin-before-reset.json" "$reset_cookie")"
[ "$signin_status" = "200" ] || fail "pre-reset sign-in returned HTTP $signin_status"
[ "$(session_count)" = "1" ] || fail "pre-reset sign-in did not create a session"

reset_request_payload="$(jq -cn --arg email "$email" '{email:$email}')"
reset_request_status="$(post_json /api/auth/request-password-reset "$reset_request_payload" \
  "$temporary_directory/reset-request.json")"
[ "$reset_request_status" = "200" ] ||
  fail "password-reset request returned HTTP $reset_request_status, expected 200"
jq -e '.status == true' "$temporary_directory/reset-request.json" >/dev/null ||
  fail "password-reset request response was unexpected"

message_list="$temporary_directory/messages.json"
curl --fail --silent --show-error --retry 12 --retry-delay 1 \
  "$mailpit_url/api/v1/messages" > "$message_list"
message_id="$(jq -er '[.messages[] | select(.Subject == "Reset your RSP password")][0].ID' \
  "$message_list")" || fail "Mailpit did not capture the password-reset email"
curl --fail --silent --show-error "$mailpit_url/api/v1/message/$message_id" \
  > "$temporary_directory/reset-message.json"
reset_token="$(jq -er '.Text' "$temporary_directory/reset-message.json" | tr -d '\r' \
  | sed -n 's#.*reset-password/\([^?[:space:]]*\).*#\1#p' | sed -n '1p')"
[ -n "$reset_token" ] || fail "password-reset token was absent from the captured email"

reset_payload="$(jq -cn --arg token "$reset_token" --arg password "$new_password" \
  '{token:$token,newPassword:$password}')"
reset_status="$(post_json /api/auth/reset-password "$reset_payload" \
  "$temporary_directory/reset.json")"
[ "$reset_status" = "200" ] || fail "password reset returned HTTP $reset_status, expected 200"
jq -e '.status == true' "$temporary_directory/reset.json" >/dev/null || fail "password reset response was unexpected"
[ "$(session_count)" = "0" ] || fail "password reset did not revoke existing sessions"
reset_revoked_status="$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
  --cookie "$reset_cookie" "$base_url/api/auth/token")"
case "$reset_revoked_status" in 401|403) ;; *) fail "pre-reset cookie returned HTTP $reset_revoked_status" ;; esac

old_password_status="$(post_json /api/auth/sign-in/email "$signin_payload" \
  "$temporary_directory/old-password.json")"
[ "$old_password_status" = "401" ] ||
  fail "old password returned HTTP $old_password_status after reset, expected 401"
new_signin_payload="$(jq -cn --arg email "$email" --arg password "$new_password" \
  '{email:$email,password:$password}')"
new_cookie="$temporary_directory/new-cookie.txt"
new_signin_status="$(post_json /api/auth/sign-in/email "$new_signin_payload" \
  "$temporary_directory/new-signin.json" "$new_cookie")"
[ "$new_signin_status" = "200" ] || fail "new password sign-in returned HTTP $new_signin_status"
new_token_status="$(curl --silent --show-error --output "$temporary_directory/new-token.json" \
  --write-out '%{http_code}' --cookie "$new_cookie" "$base_url/api/auth/token")"
[ "$new_token_status" = "200" ] || fail "new session could not obtain a token"
new_claims="$(decode_jwt_part "$(jq -er '.token' "$temporary_directory/new-token.json" | cut -d. -f2)")"
printf '%s' "$new_claims" | jq -e '.securityVersion == 2 and (.exp - .iat) == 300' >/dev/null ||
  fail "post-reset token did not carry the incremented security version"

post_json /api/auth/sign-out '{}' "$temporary_directory/final-signout.json" "$new_cookie" >/dev/null
[ "$(session_count)" = "0" ] || fail "final sign-out left a database session"

printf 'auth HTTP/PostgreSQL smoke passed: password policy, verification, session, JWT/JWKS, API validation, reset and revocation\n'
