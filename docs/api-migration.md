# API v1 to v2 migration

The React application is the only supported API consumer. Version 2 therefore
replaces the legacy surface directly; there is no server-side `/api/v1` shim.
Old browser routes have client redirects, but integrations must migrate to the
OpenAPI contract in `api/openapi.yaml`.

## Resource mapping

Legacy action-shaped routes map to resource-oriented families:

| Legacy concern           | Version 2 family                                              |
| ------------------------ | ------------------------------------------------------------- |
| Current user and profile | `/api/v2/me`, `/api/v2/users`, `/api/v2/users/{id}`           |
| Programme and weeks      | `/api/v2/seasons`, `/api/v2/seasons/{id}/weeks`               |
| Enrollment users         | `/api/v2/seasons/{id}/members`                                |
| Mentor relationships     | `/api/v2/seasons/{id}/mentorships`                            |
| Graduate background task | Derived from completed student enrollment; no endpoint or job |
| LeetCode catalogue       | `/api/v2/leetcode-problems`                                   |
| User problem attempts    | `/api/v2/problem-attempts`                                    |
| Suggested problem        | `/api/v2/recommendations/current` and `/dismiss`              |
| Mock CRUD/review         | `/api/v2/mock-interviews` and round review subresources       |
| Manual scrape            | `/api/v2/admin/leetcode/sync` plus the worker schedule        |

## Contract changes

- JSON is camel case and enums are strings. `behavioural` uses Australian
  spelling throughout.
- Actor/user IDs are removed from mutation bodies. The validated JWT and active
  `user_auth_links` row identify the caller.
- Creation returns `201`; successful deletion returns `204`; stale revisions
  and invariant conflicts return `409`.
- Rate limits return `429`, `Retry-After`, and a Problem Details body.
- All mutable resources carry a positive `revision` used for optimistic
  concurrency.
- All timestamps are RFC3339 UTC. The frontend, not the API, renders local time.
- Private profile, mentoring and interview fields are omitted unless the
  relationship-based permission permits them. A denied serializer must not
  emit the field as `null` merely to reveal its existence.

## Errors

Errors use `application/problem+json`:

```json
{
  "type": "https://rsp.example/problems/validation-failed",
  "title": "Validation failed",
  "status": 400,
  "detail": "One or more fields are invalid.",
  "instance": "/api/v2/problem-attempts",
  "code": "validation_failed",
  "requestId": "request-id",
  "errors": [{ "field": "confidence", "message": "Must be between 1 and 5." }]
}
```

Clients may branch on stable `code`; titles and details remain human-facing.
Support should ask for `requestId` and must not ask a member to copy a JWT.

## Pagination

Collection requests accept `limit` (25 by default, 100 maximum), `cursor`,
`direction`, `sort`, and resource filters. Responses use:

```json
{
  "items": [],
  "pageInfo": {
    "nextCursor": null,
    "previousCursor": null,
    "hasMore": false
  },
  "totalCount": 0
}
```

Cursors are opaque. Consumers must not decode, edit, persist indefinitely or
reuse them after changing filters/sort.

## Browser route replacements

The SPA redirects old overview, users, LeetCode, mentors and query-string
profile URLs to canonical routes. New links should use `/dashboard`, `/people`,
`/practice`, `/seasons/:slug/mentees`, and `/people/:slug`; redirects are a
transition aid rather than stable navigation contracts.

## Generated-code gate

After changing OpenAPI:

```sh
just generate
just check
git diff -- backend/internal/generated apps/web/src/api/generated
```

Generated changes are reviewed with the contract change. CI regenerates both
sides and rejects drift.
