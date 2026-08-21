# Local operations

Copy `.env.example` to `.env`, then run `just dev`. Compose starts the complete
development stack:

| Component   | Container route     | Host access                       |
| ----------- | ------------------- | --------------------------------- |
| Caddy       | `caddy:8080`        | `http://localhost:8080`           |
| React/Vite  | `web:5173`          | Through Caddy only                |
| Go API      | `api:8080`          | Through Caddy only                |
| Better Auth | `auth:3001`         | Through Caddy only                |
| PostgreSQL  | `postgres:5432`     | `127.0.0.1:5432` for native tools |
| Mailpit     | `mailpit:1025/8025` | `127.0.0.1:1025/8025`             |
| Prometheus  | `prometheus:9090`   | `http://localhost:9090`            |
| Grafana     | `grafana:3000`      | `http://localhost:3000`            |

App and Better Auth migrations run before dependent services start. Go and
TypeScript source directories are bind-mounted, so API, worker, auth, and web
processes reload after edits.

## Common commands

```sh
just dev          # build and start in the background
just logs         # follow all service logs
just migrate-up   # rerun both migration sets
just dev-down     # stop the stack; keep database and mail volumes
docker compose ps
```

To reset all local state, run `docker compose down --volumes`.

Prometheus automatically scrapes the API and auth metrics endpoints. Grafana
starts with the Prometheus data source and RSP dashboard provisioned; its local
credentials come from `GRAFANA_ADMIN_USER` and `GRAFANA_ADMIN_PASSWORD`.

## Health checks

```sh
curl --fail http://localhost:8080/health/live
curl --fail http://localhost:8080/api/v2/health/ready
docker compose logs --tail=200 api auth postgres
```

| Symptom                  | Check                                                     |
| ------------------------ | --------------------------------------------------------- |
| API never becomes ready  | PostgreSQL health and `migrate-app` output.                |
| Auth never becomes ready | `migrate-auth` output and auth database credentials.       |
| Browser gets 502         | `docker compose ps` and the target service's logs.         |
| Sign-in loops            | public origin, cookie settings, and trusted origins.       |
| Mail is absent           | Mailpit at <http://localhost:8025> and the `auth` logs.    |

Logs are JSON for API and auth events and carry request IDs. Never paste session
cookies, JWTs, password-reset links, provider tokens, or request bodies into an
issue.
