# Pull request canaries

Any pull request from a branch in this repository can request its own
pre-production stack at `https://pr-<number>.<CANARY_DOMAIN>`. Canaries are
opt-in: nothing is deployed until someone adds the `canary` label. It runs the production images
with a canary overlay ([compose.canary.yaml](../compose.canary.yaml)) and has its
own PostgreSQL volume seeded with fake data. The stack is deleted, with its
database, when the label is removed or the pull request closes.

## Using a canary

1. Add the `canary` label to the pull request. Open the link in the
   **Canary ready** comment that follows. The first deploy takes several
   minutes; later pushes redeploy the same stack while the label stays on.
2. Sign up with any email address. Nothing is sent: open **Captured emails**
   (bottom-right marker, or `/mailpit/`) and follow the verification link.
3. Choose **Switch roles** in the header. Pick any season role in the seeded
   _Development Season_ (Student, Mentor, Coordinator, or Removed to test a
   kicked/withdrawn member) and either global role.
   - A new Student is mentored by _Dev Mentor_; a new Mentor or Coordinator
     takes over _Dev Student_ as their mentee.
   - Director, System Admin and Coordinator stay `pending_mfa` until you set up
     two-factor authentication in Settings, exactly as in production.
4. To start over, run the **Canary** workflow manually with the pull request
   number and **reset** ticked. It deletes the stack and database and redeploys.
5. Remove the label when you are done testing to free the server early.

Google sign-in is disabled on canaries because OAuth redirect URIs cannot be
wildcards. Use email and password.

## What differs from production

| Setting                 | Production       | Canary                                  |
| ----------------------- | ---------------- | --------------------------------------- |
| `APP_ENV`               | `production`     | `canary`                                |
| `/api/v2/canary/access` | 404              | Self-service role changes (audited)     |
| Email                   | SES              | Mailpit, served at `/mailpit/`          |
| Google sign-in          | Required         | Disabled                                |
| Data                    | Real             | `rspctl seed` fake data on every deploy |
| Secrets                 | Platform secrets | Random per canary, generated on create  |

Guards that keep canary behaviour out of production:

- The API and auth service refuse to start with `APP_ENV=canary` unless the
  public origin has a `canary` DNS label, so it cannot run on the production
  domain. The auth service also refuses SES in a canary.
- Canary routes are registered only when `APP_ENV=canary`; production returns 404.
- The canary bar is compiled in only when the web image is built with
  `VITE_RSP_CANARY=true`, which only the overlay sets.
- Canary stacks use their own Compose projects and volumes. Never point a
  canary at a copy of production data: anyone using it can make themselves a
  System Admin and read every captured email.

## One-time setup

1. **DNS.** Point a wildcard record such as `*.canary.example.org` at the
   Dokploy server. On Cloudflare, set it to **DNS only**: the free proxied
   certificate covers only one subdomain level, so a proxied
   `pr-1.canary.example.org` fails with `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`.
   Traefik issues each canary's Let's Encrypt certificate itself.
2. **Dokploy.** Create a project and environment for canaries, and connect
   the GitHub provider for this repository. Create an API key under
   Settings > Profile > API/CLI.
3. **GitHub.** Under Settings > Secrets and variables > Actions, add:

   | Name                            | Kind     | Value                                                 |
   | ------------------------------- | -------- | ----------------------------------------------------- |
   | `DOKPLOY_URL`                   | Variable | `https://dokploy.example.org`                         |
   | `DOKPLOY_CANARY_ENVIRONMENT_ID` | Variable | The canary environment's id                           |
   | `DOKPLOY_GITHUB_ID`             | Variable | The GitHub provider id                                |
   | `CANARY_DOMAIN`                 | Variable | `canary.example.org`                                  |
   | `DOKPLOY_API_KEY`               | Secret   | The Dokploy API key                                   |
   | `CANARY_MAILPIT_AUTH`           | Secret   | Optional `user:password` for the captured-email inbox |

4. **Label.** Create a `canary` label in the repository (Issues > Labels).
   Anyone who can label pull requests can request a canary.
5. **Capacity.** Each canary runs PostgreSQL, the API, worker, auth, web,
   Caddy and Mailpit (roughly 1 GB of RAM at rest) and builds images on the
   server. Size the server for the number of canaries you expect to run at
   once, not the number of open pull requests.

## Running the overlay locally

```sh
docker network create dokploy-network  # once; the production stack expects it
docker compose --env-file canary.env \
  -f compose.prod.yaml -f compose.canary.yaml up -d --build
```

`canary.env` needs the variables [scripts/canary/dokploy.sh](../scripts/canary/dokploy.sh)
generates, with a `PUBLIC_ORIGIN` such as `https://pr-1.canary.example.org`.
