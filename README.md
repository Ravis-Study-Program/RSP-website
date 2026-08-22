# RSP Website

RSP Website is the second-generation platform for Ravi's Study Program. It is
a React application backed by a Go modular monolith, a separate Better Auth
service, and PostgreSQL 18. Caddy presents the browser with one origin.

The repository intentionally contains no license, production credentials,
database exports, or legacy C# runtime.

## Repository map

```text
apps/web/       React, Vite, Base UI, TanStack Query and Table
apps/auth/      Better Auth TypeScript service
backend/        Go API, worker, administration CLI and migration CLI
api/            OpenAPI 3 contract used for generated Go and TypeScript code
db/             Goose migrations and sqlc queries
deploy/         Caddy, observability, and local-runtime configuration
docs/           Architecture, migration, security and operations runbooks
compose.yaml    Local development stack
```

## Prerequisites

- Docker Desktop or another Compose v2 compatible engine
- `just`
- Go 1.26.6 for native development
- Node 24.19.0 and pnpm 10.15.0 for native development

The language versions are also recorded in `.tool-versions`. All production
containers are multi-stage, run as non-root where the upstream image permits,
and are available for common amd64 and arm64 Docker hosts.

## Start locally

```sh
cp .env.example .env
just bootstrap
just dev
```

Open <http://localhost:8080>. Mailpit is available at
<http://localhost:8025>, Prometheus at <http://localhost:9090>, and Grafana at
<http://localhost:3000>. PostgreSQL and observability ports bind to loopback;
Caddy is the browser application origin.

## Standard commands

| Command           | Purpose                                                                                                   |
| ----------------- | --------------------------------------------------------------------------------------------------------- |
| `just bootstrap`  | Install pinned Go and pnpm dependencies.                                                                  |
| `just dev`        | Start the application, PostgreSQL, Mailpit, Prometheus, and Grafana.                                      |
| `just generate`   | Regenerate transport and browser clients from OpenAPI.                                                    |
| `just migrate-up` | Apply app Goose migrations and the pinned Better Auth schema.                                             |
| `just seed`       | Load deterministic local-only fixture data.                                                               |
| `just fake-data`  | Create local email/password Student, Mentor, Coordinator and Site Admin accounts with fixture data.       |
| `just fake-totp`  | Generate the current TOTP code from a fake account secret.                                                |
| `just check`      | Run non-mutating formatting, static, TypeScript and Compose checks.                                       |
| `just test`       | Run Go and TypeScript tests.                                                                              |
| `just e2e`        | Run desktop/mobile Playwright and axe checks.                                                             |
| `just e2e-real`   | Rebuild an isolated test stack and run authenticated Student, Coordinator and Director browser mutations. |

## Contracts and safety

- The browser calls `/api/auth/*` and `/api/v2/*` through Caddy. It never sends
  an actor ID; identity comes from a short-lived access JWT.
- Public requests to metrics paths receive `404`.
- The API owns PostgreSQL schema `app`, Better Auth owns `auth`, and legacy
  import provenance lives in `migration`.
- Application and audit history use UTC. The UI renders the member's saved IANA
  timezone.
- `.env.example` contains local placeholders only. Production validation rejects
  them; deployment secrets must come from the platform secret store.

Start with [architecture](docs/architecture.md), then use the
[operations runbook](docs/operations.md) for health, logs and observability.
Legacy behavior is frozen in the [semantic parity baseline](docs/semantic-parity.md).
Data and identity cutovers are deliberately separate runbooks:
[data migration](docs/data-migration.md) and
[auth cutover](docs/auth-cutover.md).
