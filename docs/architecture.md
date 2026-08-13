# Architecture

Nginx provides one browser origin. It serves the React application, proxies
`/api/auth/*` to the Better Auth service and `/api/v2/*` to the Go API. The API
and worker own the PostgreSQL `app` schema; Better Auth owns `auth`; import
manifests live in `migration`. Domain code is organized by feature behind
explicit services and request-scoped transactions.

OpenAPI is the public contract. Browser callers never supply an actor ID: the
API resolves the validated JWT subject through `app.user_auth_links`.

