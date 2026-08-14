# Operations runbook

## Local topology

Copy `.env.example` to `.env`, then run `just dev`. The development profile
builds and starts the complete stack:

| Component        | Container route     | Host access                       |
| ---------------- | ------------------- | --------------------------------- |
| Nginx            | `:8080`             | `http://localhost:8080`           |
| React/Vite       | `web:5173`          | Through Nginx only                |
| Go API           | `api:8080`          | Through Nginx only                |
| Better Auth      | `auth:3001`         | Through Nginx only                |
| PostgreSQL 17.11 | `postgres:5432`     | `127.0.0.1:5432` for native tools |
| Mailpit SMTP/UI  | `mailpit:1025/8025` | `127.0.0.1:1025/8025`             |
| Prometheus       | `prometheus:9090`   | `127.0.0.1:9090`                  |
| Grafana          | `grafana:3000`      | `127.0.0.1:3000`                  |

Go/TypeScript source is bind-mounted into development images. App and Better
Auth migrations finish successfully before dependent processes start. Stop the
profile with `just dev-down`; named volumes persist.

Use `just dev-native` when debugging native processes. It leaves PostgreSQL and
Mailpit in containers, sets schema-specific database URLs per process, and
points Vite's proxy at localhost.

## Health and readiness

| Probe                      | Meaning                                      |
| -------------------------- | -------------------------------------------- |
| Nginx `/health/live`       | Edge process can serve HTTP.                 |
| API `/api/v2/health/live`  | API process is alive; no dependency promise. |
| API `/api/v2/health/ready` | Required API dependencies respond.           |
| Auth `/health/live`        | Auth process is alive.                       |
| Auth `/health/ready`       | Auth can query its PostgreSQL schema.        |

Liveness should trigger process restart only. Readiness should remove a service
from traffic without a restart storm. During an incident, test internal probes
from the Compose network before blaming Nginx.

```sh
curl --fail --show-error http://localhost:8080/health/live
curl --fail --show-error http://localhost:8080/api/v2/health/ready
docker compose --profile dev ps
docker compose --profile dev logs --tail=200 api-dev auth-dev postgres
```

Logs are JSON for API/auth application events and carry request IDs. Search by
request ID across Nginx, auth and API. Never paste session cookies, JWTs,
password-reset links, provider tokens or raw request bodies into a ticket.

## Metrics and dashboards

Prometheus scrapes API `/api/v2/metrics` and auth `/metrics` directly over the
container network. Nginx returns `404` for public metrics paths. Metric labels
must be bounded categories such as status class, operation, result or job;
member IDs, emails, slugs, request IDs and raw URLs are prohibited labels.

Grafana automatically provisions the `RSP service overview` dashboard with API
traffic/latency/error signals, auth failures, DB pool, worker runs,
recommendation outcomes and migration status. A missing series indicates a
scrape/exporter problem and is not equivalent to a measured zero.

Initial alert candidates:

- API/auth readiness unavailable for two minutes.
- 5xx ratio above 2% for ten minutes with a minimum request volume.
- Authentication origin/rate-limit rejection spike.
- PostgreSQL pool saturation or connection acquisition delay.
- LeetCode worker has no successful run after Sunday 05:00 UTC.
- Migration status is applying/failed outside an approved window.

## Worker schedule

LeetCode synchronization is due Sunday 03:00 UTC. After downtime the worker
catches up once, uses a dedicated PostgreSQL advisory lock, applies bounded
timeouts/retries, and records partial failure rather than silently succeeding.
An audited privileged API action may request the same job. Do not run an ad-hoc
second scraper; use the shared operation so locking and audit apply.
`LEETCODE_SYNC_URL` defaults to the official HTTPS GraphQL endpoint and may be
overridden only with a compatible approved endpoint, such as an isolated test
stub.

## Production packaging smoke

`just prod-up` validates required production settings before building. It
rejects HTTP public origins, local placeholders, short secrets, insecure cookies,
non-SES mail, an unauthorized local sender, absent Google credentials, reused
service/database secrets, development auth bypass, demo data, or a trusted-origin
list wider than the one public application origin. Expected secret sources are
the deployment platform secret store or workload identity—not `.env` committed
to Git.

Production Compose assumes TLS terminates at a trusted host-local edge and
forwards to Nginx port 8080, which is bound to loopback. PostgreSQL, Prometheus
and Grafana host ports are also loopback-only. `AUTH_ISSUER` must exactly equal
the HTTPS `PUBLIC_ORIGIN`,
`AUTH_TRUSTED_ORIGINS` contains only that origin, and `AUTH_JWKS_URL` is the
internal `http://auth:3001/api/auth/jwks` service address. Before a smoke run:

```sh
sh deploy/scripts/require-production-env.sh
docker compose --profile prod config --quiet
docker compose --profile prod build
docker compose --profile prod up -d
curl --fail --show-error --retry 12 --retry-delay 5 http://localhost:8080/health/live
curl --fail --show-error --retry 12 --retry-delay 5 http://localhost:8080/api/v2/health/ready
docker compose --profile prod ps
```

Inspect logs and stop with `just prod-down`. This proves packaging/topology; it
does not authorize public deployment or data cutover.

### External image vulnerability exception

The production images built by this repository still fail CI on any fixed
HIGH or CRITICAL finding; they have no allowlist. The following vendor-image
residue is a separate, time-bounded upstream exception, verified for
`linux/amd64` with Trivy 0.70.0 and the vulnerability database updated
2026-08-14 01:10 UTC:

| Image                       | Fixed HIGH/CRITICAL residue                                                                                                   | Upstream boundary                                                                                              |
| --------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `postgres:17.11-alpine3.24` | 16 HIGH and 1 CRITICAL, all in `/usr/local/bin/gosu` built with Go 1.24.6; Alpine and PostgreSQL packages have zero findings  | Latest PostgreSQL 17 patch image; the remaining scanner results are confined to the upstream entrypoint helper |
| `prom/prometheus:v3.13.2`   | 4 HIGH occurrences: CVE-2026-39821 and CVE-2026-46600 in both `prometheus` and `promtool` (Go 1.26.5; fixed in 1.26.6)        | Latest stable release; 3.14 is pre-release only                                                                |
| `axllent/mailpit:v1.30.7`   | 2 HIGH: CVE-2026-39821 and CVE-2026-46600 in the Mailpit binary (Go 1.26.5; fixed in 1.26.6)                                  | Latest stable release                                                                                          |
| `grafana/grafana:13.1.3`    | 21 HIGH and 0 CRITICAL: 4 in the main image target, 5 in the bundled Elasticsearch plugin and 12 in the bundled Zipkin plugin | Latest stable release; selected over Grafana 12.4.8, which retains a fixed kin-openapi CRITICAL finding        |

Re-scan every vendor image when its tag changes and remove each exception as
soon as an official fixed image exists; never copy an exception to a new tag.
This table does not permit suppressing or weakening the custom-image Trivy
gate.

CI additionally applies `deploy/compose.auth-smoke.yaml` only after validating
the real production environment and scanning the unmodified production images.
That disposable overlay publishes the built auth runtime on loopback and swaps
SES for Mailpit so `deploy/scripts/auth-http-smoke.sh` can exercise password
policy, unverified rejection, a database-verified sign-in, cookie-backed
sessions, five-minute JWT/JWKS claims, Go API token validation, password-reset
revocation, and sign-out. The script deliberately refuses to run unless
`AUTH_SMOKE_ALLOW_DATABASE_WRITE=true`; never point it at a persistent or
production database.

Before live traffic, configure the host-local TLS terminator to discard any
client-supplied forwarding headers, then set `X-Real-IP` to the verified client
address. Production Nginx trusts that bridge peer for anonymous quotas and
deliberately forwards `X-Forwarded-Proto: https`; rebinding its clear-text port
to a public interface would violate the cookie/origin trust model and make IP
quotas spoofable.

## Schema changes

1. Add a new forward Goose migration; never edit an already-applied production
   migration.
2. Test app migration up/down/up against PostgreSQL 17.11.
3. Regenerate sqlc/OpenAPI artifacts as applicable and run `just check` and
   `just test`.
4. Back up before production apply. Prefer backward-compatible expand/contract
   changes so old and new binaries can overlap safely.
5. Better Auth schema changes come only from its pinned generator/dependency and
   are reviewed separately from app SQL.

## Credential rotation

- Rotate `BETTER_AUTH_SECRET` only with a session invalidation plan.
- Rotate JWKS keys through Better Auth, retaining the previous public key long
  enough for already-issued five-minute JWTs.
- Rotate `IDENTITY_SERVICE_TOKEN` on API and auth together; temporarily overlap
  only if implementation explicitly supports two keys.
- Database credential rotation should create/test the new role or password,
  update services, verify pools reconnect, then revoke the old credential.
- Google/SES compromise requires provider-side revocation, application secret
  update, session revocation and incident review.

## Common failures

| Symptom                  | Check                                                                  |
| ------------------------ | ---------------------------------------------------------------------- |
| API never becomes ready  | PostgreSQL health, `migrate-app` exit code and app-schema URL.         |
| Auth never becomes ready | `migrate-auth`, auth-schema URL, generated migration drift.            |
| Browser gets 502         | Target service health and Docker network alias (`api`, `auth`, `web`). |
| Sign-in loops            | public origin, forwarded protocol, Secure cookie and trusted origins.  |
| JWT rejected             | issuer/audience, clock, JWKS URL/key rotation and active auth link.    |
| Mail absent locally      | Mailpit UI, SMTP host `mailpit:1025`, verification rate limit.         |
| Dashboard shows no data  | Prometheus target status, exporter response and query time range.      |

Use `backup-restore.md` before data repair and `incident-response.md` whenever
availability, integrity or confidentiality may be affected.
