# Known defects, fixes and launch constraints

Valid legacy behavior is recorded separately in
[`semantic-parity.md`](semantic-parity.md). Items below must not be reintroduced
merely to make a v2 workflow resemble the old implementation.

## Legacy defects intentionally corrected

These behaviors are not parity requirements:

- Request-body user IDs allowed actor spoofing in legacy attempt, enrollment
  and mock operations. Version 2 derives the actor from a validated token.
- Mock-interview ownership trusted caller-provided identities. Creation fixes
  interviewer to the actor and corrections are separately authorized/audited.
- Rich text could reach `dangerouslySetInnerHTML` without a server allowlist.
  Version 2 sanitizes supported Tiptap HTML on write and migration.
- Some cache keys omitted filters and malformed cursors fell back to page one.
  Version 2 binds signed cursors to filters/sort and rejects invalid cursors.
- Graduate state was a stale flag maintained by a daily job. Alumni now derives
  only from completed student enrollment.
- Dummy-data and manual scrape production endpoints are replaced by an explicit
  seed CLI and an audited, advisory-locked worker/admin operation.

Regression tests must remain for actor spoofing/IDOR, stored XSS, private-field
serialization, malformed/replayed cursors and kicked-only alumni access.

## Accepted initial architecture limits

- Account/auth request quotas are in-process and launch assumes one API/auth
  replica. A shared limiter is required before horizontal scale-out.
- PostgreSQL is the only durable runtime dependency; there is intentionally no
  Redis/cache cluster yet.
- Production Auth0 identity import and live data cutover cannot be completed
  without tenant/database access, verified backups and a maintenance window.
  Synthetic fixture support does not remove this deployment prerequisite.
- TLS terminates outside the portable Compose profile. Direct public exposure of
  its plain HTTP port is unsupported.

## Instrumentation constraint

The checked-in Grafana dashboard covers API latency/error, auth failure, DB
pool, worker, recommendation and migration signals. A missing series is an
exporter/scrape fault, not a measured zero. Every new metric must use bounded
labels and must never contain member IDs, emails, slugs, request IDs or raw URL
values.

## Not product scope

Unchecked items in `product-backlog.md` remain future decisions. The rewrite
does not implicitly deliver applications, job tracking, hosted programme
content, announcements, video ingestion or company-directory workflows.
