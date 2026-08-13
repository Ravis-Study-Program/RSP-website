# RSP auth service

This service owns browser authentication under `/api/auth/*`. Better Auth keeps
its tables in PostgreSQL's `auth` schema; the Go API maps the JWT `sub` through
an active `app.user_auth_links` row before authorising any application request.

Supported sign-in methods are verified email/password and Google. Matching
emails are never linked implicitly. Connecting Google requires an authenticated,
fresh session and uses Better Auth's explicit `linkSocial` flow.

Sessions are database backed, held in a `Secure`, `HttpOnly`, `SameSite=Lax`
cookie, and roll for seven days. The `/api/auth/token` endpoint returns a
five-minute JWT for the Go API; the browser must keep it in memory only. JWKS
keys rotate every 30 days with a 30-day verification grace period.

TOTP and single-use backup codes are provided by Better Auth. Every access JWT
contains the time of the most recent second-factor completion. The Go API must
require a recent value for Coordinator, Director, and System Admin actions.
OAuth sessions intentionally start without recent MFA and must complete the
application step-up flow before privileged actions.

## Commands

```sh
pnpm check
pnpm test
pnpm build
pnpm generate-schema
pnpm migrate
```

Copy `.env.example` to the environment managed by Compose or the deployment
platform. Never commit a populated `.env` file. Production uses ambient AWS
credentials for SES; static AWS credentials are not application variables.

Internal lifecycle endpoints require `Authorization: Bearer
$IDENTITY_SERVICE_TOKEN`. They revoke sessions when the Go service suspends an
account, accepts a deletion request, cancels deletion, or completes
pseudonymisation. These endpoints are not exposed by Nginx.
