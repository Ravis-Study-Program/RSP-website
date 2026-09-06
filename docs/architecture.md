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
                        +---- PostgreSQL 18

Go worker --------------'
Prometheus ---> API and auth metrics
Grafana -----> Prometheus
```

Caddy routes the single local origin and denies public metrics paths.

## Source boundaries

| Boundary    | Location                                                                | Responsibility                                                                                                |
| ----------- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Browser     | `apps/web`                                                              | Routes, accessible interaction, local timezone rendering and API queries.                                     |
| Identity    | `apps/auth`                                                             | Better Auth sessions, password/Google providers, verification, MFA, JWT/JWKS, credential lifecycle and email. |
| Contract    | `api/openapi.yaml`                                                      | Frontend API client/types and API documentation; Go handlers own server-side decoding and validation.         |
| Transport   | `backend/internal/api`                                                  | HTTP decoding/validation, authentication boundary, status codes and serialization.                            |
| Domain      | `backend/internal/{accounts,programme,practice,mockinterviews}`         | Authorization relationships and programme rules independent of HTTP.                                          |
| Persistence | `backend/internal/dal`                                                  | Native pgx queries, row mapping, and atomic data/history/audit writes.                                        |
| Jobs        | `backend/cmd/worker`, `backend/internal/worker`, `backend/internal/dal` | Scheduled LeetCode synchronization, catch-up, retry, advisory locking and PostgreSQL run state.               |
| Operations  | `deploy`, `compose.yaml`, `scripts/legacy-migration`                    | Local ingress, container topology, and one-time legacy import tooling.                                        |

Handlers own the HTTP request and response. They extract path/query/body values,
call pure authorization predicates such as `actor.IsSeasonAdmin(seasonID)`, load
records explicitly, and decide which status and error to return. Permission
helpers return booleans; loaders and mutations return values and errors. Helpers
that perform a check do not also write an HTTP response.

Domain packages own the rules used by those checks. The handler supplies an
explicit viewer ID and visibility scope to list queries; SQL applies that scope
without receiving an HTTP request or the whole actor. Mutations that span
related records use one request-scoped transaction. Database checks and locks
remain inside that transaction even when a handler also checks the same record.

HTTP handlers call the concrete PostgreSQL store. Feature packages contain
plain data types and business rules, and they do not import HTTP or database
code. PostgreSQL is the only runtime database; local development and
integration tests use disposable PostgreSQL data instead of a second storage
implementation.

The DAL is one concrete `dal.Store` with a private `pgxpool.Pool`. Its files
follow application features (`users.go`, `seasons.go`, `enrollments.go`,
`mock_interviews.go`, and so on). SQL and its row mapping live together.
List methods take named query structs so filters and cursor parameters are
visible at the call site. Multi-field mutations take named input structs and
load the current row inside the transaction. Promotion and removal are separate
operations. Response builders receive loaded data; formatting a response does
not cause a database read. Unexpected storage failures are logged and become a
generic HTTP 500, while known missing rows and conflicts retain their statuses. Handlers use domain rules and call the DAL directly;
there is no additional repository or generic CRUD layer.

The worker acquires a `dal.WorkerSession` for its advisory lock, run state, and
catalogue writes. They use the same connection until the scheduler stops.
`rspctl` delegates seed and bootstrap writes to the DAL. The one-time legacy
import tool lives under `scripts/legacy-migration`, outside the application
packages and runtime images. It uses dedicated connections for its read-only
repeatable-read source snapshot and target transaction. Goose only owns the
current application schema. Its planner separates source analysis, approved
resolutions, feature transformations, and manifest construction. Reference
resolution finishes before provenance checksums are calculated, including
snapshots that share transformed row values.

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

| Schema | Owner in code       | Contents                                                                                       |
| ------ | ------------------- | ---------------------------------------------------------------------------------------------- |
| `app`  | Go API and worker   | Programme users, roles, seasons, mentoring, practice, interviews and append-only audit events. |
| `auth` | Better Auth service | Credentials, provider accounts, sessions, verification, MFA and JWKS keys.                     |

`app.users` stores only the account ID, lifecycle state, deletion marker, and
timestamps. Profile data lives in `user_profiles`, preferences
in `user_preferences`, contact data in `user_contacts`, and MFA/version state
in `user_security`. Each table has one required row per user, so queries must
choose a data boundary instead of receiving every user field by default.

Goose versions the current `app` schema. The pinned Better Auth SQL is generated
from the exact Better Auth dependency and applied independently. Domain/history
foreign keys use `RESTRICT`; cascades are reserved for auth-owned disposable
children and pure joins. Database triggers reject mutation of append-only audit
history even if application code is bypassed. The legacy import bookkeeping
schema is created separately by the one-time import scripts when needed.

All IDs crossing the API are opaque strings. New records use UUIDv7 strings;
legacy IDs remain unchanged. Storage, comparisons, schedules and API timestamps
are UTC.

## Collections

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
