# Legacy semantic parity baseline

This baseline freezes valid product behavior imported from the legacy checkout
at commit `8391ba51f4d56226c4ac28afd9748e83183f9016`, whose tree is
`13d4a47b2164cb3fc898ffe16383a748157fd2c6`. It is a product-level oracle, not
a promise to retain `/api/v1`, C# DTO shapes, status-code mistakes, destructive
administrative CRUD, or security defects.

The machine-readable cases are in
[`golden/legacy-workflows.json`](golden/legacy-workflows.json). They cover the
legacy member and administrator workflow families for profiles, seasons and
weeks, enrollments and mentorships, practice and recommendations, mock
interviews, and administration.

## Executable acceptance gate

Run the focused gate with:

```sh
go test ./backend/internal/contract \
  -run '^TestLegacyWorkflowSemanticParityEvidence$' -count=1
```

The test in
[`semantic_parity_test.go`](../backend/internal/contract/semantic_parity_test.go)
strictly parses the fixture and fails when:

- the recorded source repository, commit, or tree differs from the imported
  provenance;
- workflow IDs are duplicated, a required field is empty, or a route uses a
  method other than `GET`, `POST`, `PATCH`, or `DELETE`;
- a fixture route does not resolve to an operation in `api/openapi.yaml` after
  removing its query string, stripping `/api/v2`, and normalizing template
  parameter names;
- an evidence path is absolute, escapes the repository (including through a
  symlink), is not a test source, or its named test anchor no longer exists;
- any zero-based expectation index is invalid or has no named executable
  evidence; or
- an intentional replacement route for account administration, season
  close/reopen, week mutation, enrollment mutation/removal, or mentorship
  mutation is omitted.

Each `evidence` item maps one or more zero-based `expectations` to a
repository-relative test `path` and a real named-test `anchor`. The gate checks
the integrity of that map; the Go, Vitest, and Playwright suites execute the
referenced tests at their appropriate test layers. An OpenAPI operation alone
is evidence only for a contract claim such as presence and revision gating, not
for an untested data or browser outcome.

The quick JSON shape check is:

```sh
jq -e '
  . as $fixture |
  .version == 2 and
  (.source.commit | test("^[0-9a-f]{40}$")) and
  (.source.tree | test("^[0-9a-f]{40}$")) and
  ([.workflows[].id] | length == (unique | length)) and
  ([.workflows[].family] | unique | sort ==
    ["admin_crud", "attempts_recommendation", "enrollment_mentorship",
     "mock_interviews", "seasons_weeks", "user_profile"]) and
  (.workflows | length == 13) and
  all(.workflows[];
    (.operations | length) > 0 and
    (.expect | length) > 0 and
    (.v2 | length) > 0 and
    (.evidence | length) > 0 and
    (([range(0; (.expect | length))] -
      ([.evidence[].expectations[]] | unique)) | length == 0)) and
  (.residualCoverage == [])
' docs/golden/legacy-workflows.json
```

## Intentional replacements

Parity preserves the valid outcome, not every legacy command shape:

- Legacy System Admin user creation is replaced by Better Auth
  self-registration and the authenticated identity-lifecycle bridge. Account
  removal is the user's recoverable deletion lifecycle followed by
  pseudonymization. System Admins list accounts and use audited account-state
  and global-role controls; they do not create or hard-delete user rows.
- Legacy season deletion is replaced by revision- and reason-gated
  `POST /api/v2/seasons/{id}/close` and `/reopen`. Historical records remain,
  and reopen restores only enrollments completed by that close.
- Legacy enrollment deletion is replaced by revision- and reason-gated
  `POST /api/v2/seasons/{id}/members/{memberId}/remove`, which changes lifecycle
  state instead of erasing programme history.
- Week update/delete, enrollment update, and mentorship update/delete are the
  landed member-resource routes:
  `PATCH`/`DELETE /api/v2/seasons/{id}/weeks/{weekId}`,
  `PATCH /api/v2/seasons/{id}/members/{memberId}`, and
  `PATCH`/`DELETE /api/v2/seasons/{id}/mentorships/{mentorshipId}`.

The following legacy behaviors remain defects rather than golden behavior:

- caller-supplied actor IDs and cross-user or cross-season IDOR paths;
- unsanitized rich text and private-field over-serialization;
- silent last-write-wins updates and malformed-cursor fallback;
- a mutable graduate flag and dummy-data or manual-scrape production
  endpoints; and
- silent identity linking based on matching email.

Their corrected behavior and regressions are tracked in
[`known-defects.md`](known-defects.md). Approved auth, lifecycle, MFA,
close/reopen, and recommendation behavior can extend the legacy baseline;
unchecked backlog ideas cannot.

## Coverage status

Every fixture expectation has named executable evidence. There are no known
parity observations without a directly asserting test. A future unmapped
observation is a release blocker and must not be pointed at a nearby
authorization, schema, or UI test.

## Evidence chain

- [`source-provenance.md`](source-provenance.md) records the source commit, tree,
  dirty paths, and SHA-256 checksums without modifying the old checkout.
- The import-time checksums identify both uncommitted mock-interview files
  without retaining obsolete Mantine source in this repository.
- [`api-migration.md`](api-migration.md) maps legacy concerns to the v2
  contract.
- The synthetic 18-table migration fixture verifies IDs, counts, checksums,
  sanitization changes, subtype rows, and relationships independently of this
  semantic evidence map.
