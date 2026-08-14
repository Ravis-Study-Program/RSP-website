# Incident response

This runbook covers availability, data integrity, authentication and privacy
events. Safety and preservation of evidence take precedence over a fast but
unreviewed database edit.

## Severity

| Severity | Examples                                                                                                      | Initial response                                                                  |
| -------- | ------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| SEV-1    | Confirmed credential/PII exposure, destructive corruption, total outage during a critical programme operation | Page incident lead/security/data owner immediately; freeze risky writes.          |
| SEV-2    | Major role cannot work, widespread auth failure, worker repeatedly corrupting/duplicating data                | Assign incident lead and responder; mitigate within the current operating period. |
| SEV-3    | Degraded non-critical workflow, isolated incorrect result with safe workaround                                | Track owner, evidence and bounded repair.                                         |

## First 15 minutes

1. Name incident lead, scribe and technical responders; record UTC start time.
2. State observed impact and evidence separately from hypotheses.
3. Preserve request IDs, deployment revision, service status, bounded logs,
   migration/worker/audit state and relevant provider status.
4. Stop the specific source of harm. Prefer removing traffic, pausing a job or
   revoking a credential over broad deletion/restart loops.
5. Decide communication cadence and who can authorize rollback, restore or
   provider revocation.

Useful read-only checks:

```sh
docker compose --profile prod ps
docker compose --profile prod logs --since=30m --tail=500 nginx api auth worker postgres
curl --fail --show-error http://localhost:8080/health/live
curl --fail --show-error http://localhost:8080/api/v2/health/ready
```

Redact tokens, cookies, provider payloads, emails and rich-text contents before
sharing logs. A request ID is safe; a bearer token is not.

## Scenario playbooks

### Authentication or account takeover

- Disable affected sign-in provider/action if compromise is ongoing.
- Revoke affected sessions; for key/secret compromise, revoke broadly and
  rotate Better Auth/JWKS/provider credentials in a coordinated order.
- Check explicit provider-link audit events, MFA changes, resets, account state
  and privileged-role grants.
- Do not merge/unlink identities based only on an email assertion.
- Preserve provider audit/export evidence and follow breach-notification policy.

### Authorization or private-field exposure

- Remove the affected API operation/route from traffic or deploy the smallest
  verified permission fix.
- Determine actors, resource/season relationships and fields serialized; avoid
  querying more PII than needed.
- Test every permission-matrix state and cross-season/IDOR negative case before
  reopening.
- Treat logs/caches containing private responses as affected copies.

### Database integrity or failed migration

- Freeze writes and preserve the failed target. Do not hand-edit append-only
  audit/import rows.
- Compare Goose version, migration run, source manifest checksum, counts and
  constraints.
- Use `rsp-migrate legacy rollback --run-id ...` only for that import run; use a
  verified backup for broader damage.
- Restore to isolation first and have a second operator verify the target before
  destructive repair or traffic switch.

### Worker/external service failure

- Confirm advisory-lock ownership and last successful UTC run.
- Stop repeated harmful runs; retain the partial-failure report.
- Do not start parallel manual scrapers. Retry through the audited worker/admin
  operation after dependency health returns.
- Reconcile inserted/updated/failed counts and external rate limits.

### Resource exhaustion or traffic spike

- Identify Nginx/auth rejection rates, API error/latency, DB pool and container
  limits.
- Tighten anonymous edge limits or shed non-critical work. Do not raise DB pool
  sizes blindly when PostgreSQL is already saturated.
- The initial limiter is process-local; do not add replicas expecting a global
  quota until a shared limiter exists.

## Resolution and follow-up

Before resolution, verify member workflows, permission negatives, health,
worker/migration state and monitoring. Record the exact mitigation and any
temporary control with an owner/expiry.

Within the retrospective, document timeline, impact, root cause, contributing
conditions, detection gaps, what limited impact and concrete follow-ups. Store
redacted evidence according to retention policy. Never alter audit history to
make the incident timeline look cleaner.
