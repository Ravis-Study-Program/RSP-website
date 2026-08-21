# Architecture

## System shape

RSP is a modular monolith with a deliberately separate identity service. The
browser sees one origin; internal service names and metrics are never part of
the public contract.

```text
Browser
  |
  v
Caddy :8080
  |-- /* ----------> React/Vite web
  |-- /api/auth/* -> Better Auth :3001
  `-- /api/v2/* ---> Go API :8080
                        |
                        +---- PostgreSQL 17.11

Go worker --------------'
Prometheus ---> API and auth metrics
Grafana -----> Prometheus
```

Caddy routes the single local origin and denies public metrics paths.

## Source boundaries

| Boundary    | Location                                        | Responsibility                                                                                                |
| ----------- | ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Browser     | `apps/web`                                      | Routes, accessible interaction, local timezone rendering and API queries.                                     |
| Identity    | `apps/auth`                                     | Better Auth sessions, password/Google providers, verification, MFA, JWT/JWKS, credential lifecycle and email. |
| Contract    | `api/openapi.yaml`                              | Public `/api/v2` operations, DTOs, Problem Details and generated-code source of truth.                        |
| Transport   | `backend/internal/httpapi`                      | HTTP decoding/validation, authentication boundary, status codes and serialization.                            |
| Domain      | `backend/internal/*` feature packages           | Authorization relationships and programme rules independent of HTTP.                                          |
| Persistence | `backend/internal/store`, `db/queries`          | Explicit pgx/sqlc persistence and transaction boundaries; no ORM or generic repository.                       |
| Jobs        | `backend/cmd/worker`, `backend/internal/worker` | Scheduled LeetCode synchronization, catch-up, retry and advisory locking.                                     |
| Operations  | `deploy`, `compose.yaml`                        | Local ingress and container topology.                                                                         |

Feature services own business rules. Transport code must not recreate role or
ownership checks, and SQL must not infer an actor from request data. Mutations
that span related records use one request-scoped transaction.

## Identity and request flow

1. Better Auth stores a rolling seven-day session in a Secure, HttpOnly,
   SameSite=Lax cookie.
2. The React process requests a five-minute access JWT from the same-origin
   auth route and keeps it only in memory.
3. The Go API validates issuer, audience, expiry, signature, JWKS key ID and
   required recent-MFA state.
4. JWT `sub` resolves through an active `app.user_auth_links` row. Email is
   mutable profile/contact data and is never a runtime identity key.
5. Domain authorization uses the resolved app user, account lifecycle, global
   roles, season role, enrollment state and mentorship relationship.

Internal auth-to-API lifecycle callbacks use an independent service token and
the container network. Neither service accepts the browser cookie as authority
for internal operations.

## PostgreSQL ownership

One PostgreSQL database contains isolated schemas:

| Schema      | Owner in code       | Contents                                                                                       |
| ----------- | ------------------- | ---------------------------------------------------------------------------------------------- |
| `app`       | Go API and worker   | Programme users, roles, seasons, mentoring, practice, interviews and append-only audit events. |
| `auth`      | Better Auth service | Credentials, provider accounts, sessions, verification, MFA and JWKS keys.                     |
| `migration` | `rsp-migrate`       | Import runs, manifests, source-row provenance, anomalies, resolutions and auto-fixes.          |

Goose versions `app` and `migration`. The pinned Better Auth SQL is generated
from the exact Better Auth dependency and applied independently. Domain/history
foreign keys use `RESTRICT`; cascades are reserved for auth-owned disposable
children and pure joins. Database triggers reject mutation of append-only audit
and import history even if application code is bypassed.

All IDs crossing the API are opaque strings. New records use UUIDv7 strings;
legacy IDs remain unchanged. Storage, comparisons, schedules and API timestamps
are UTC.

## Concurrency and collections

Every mutable DTO includes `revision`. Updates and deletes compare the supplied
revision and return `409` on stale state instead of silently overwriting.

Growing collections use server-side filtering/sorting and cursor pagination.
Cursors are versioned, HMAC-protected and bound to filter, direction and sort;
a cursor cannot be replayed under a different query. The default page size is
25 and the maximum is 100.

## Local runtime

`compose.yaml` describes one development stack. PostgreSQL data is persistent,
schema migrations are one-shot dependencies, source is bind-mounted for reload,
Mailpit captures local email, and Prometheus/Grafana provide local
observability. Production images remain defined by each application's
Dockerfile; deployment topology belongs to the deployment platform rather than
local Compose.

The first launch assumes one API and one auth replica because rate limits are
process-local. Horizontal replicas require a shared limiter before scale-out;
PostgreSQL advisory locks already prevent overlapping worker synchronization.

## Deliberate exclusions

- No Redis until a shared quota/cache requirement exists.
- No Auth0 runtime compatibility shim or `/api/v1` endpoint shim.
- No Railway-specific configuration.
- No production deployment or live cutover in this repository pass.
- No product-backlog feature is implied by the platform rewrite unless the
  approved scope explicitly names it.
