#!/usr/bin/env bash
# Plan Auth0 identities and exercise the legacy migration against its fixtures:
# dry-run, apply, verify, and prove a repeated apply is refused unchanged.
#   scripts/ci/legacy-migration-fixture.sh
set -euo pipefail

testdata=scripts/legacy-migration/migration/testdata
migration_dir="$(mktemp -d)"
trap 'rm -rf "$migration_dir"' EXIT

migrate() {
  go run ./scripts/legacy-migration/cmd "$@"
}

migrate auth0 plan \
  --auth0-users "$testdata/auth0_users.json" \
  --app-candidates "$testdata/auth0_app_candidates.json" \
  --resolutions "$testdata/auth0_resolutions.json" \
  --plan "$migration_dir/auth0-plan.json"
jq -e '
  .version == 1 and
  (.checksum | length == 64) and
  .reconciliation.auth0UserCount == 4 and
  .reconciliation.providerIdentityCount == 4 and
  .reconciliation.explicitResolutionCount == 2 and
  .reconciliation.statusCounts.matched == 1 and
  .reconciliation.statusCounts.requires_resolution == 2 and
  .reconciliation.statusCounts.password_reset_required == 1 and
  .reconciliation.statusCounts.imported == 0 and
  .reconciliation.unresolvedAuth0UserIds == ["auth0|ambiguous", "auth0|unverified"] and
  ([.reconciliation.providerAccounts[] |
    [.provider, .providerAccountId] | join("\u0000")] |
    length == (unique | length)) and
  ([.items[] | select(.auth0UserId == "auth0|ambiguous")] | length == 1) and
  all(.items[] | select(.auth0UserId == "auth0|ambiguous");
    .status == "requires_resolution" and
    (.resolvedAppUserId // "") == "" and
    (.candidateAppUserIds | length) == 2) and
  all(.items[] | select(.auth0UserId == "auth0|unverified");
    .status == "requires_resolution" and
    (.resolvedAppUserId // "") == "" and
    ((.candidateAppUserIds // []) | length) == 0)
' "$migration_dir/auth0-plan.json"

migrate legacy dry-run \
  --source-fixture "$testdata/valid_snapshot.json" \
  --manifest "$migration_dir/manifest.json"
migrate legacy apply \
  --source-fixture "$testdata/valid_snapshot.json" \
  --target-state "$migration_dir/target-state.json" \
  --manifest "$migration_dir/manifest.json"
migrate legacy verify \
  --target-state "$migration_dir/target-state.json" \
  --manifest "$migration_dir/manifest.json" \
  > "$migration_dir/verification.json"
jq -e '
  .foreignKeyErrors == 0 and
  (.runId | length > 0) and
  (.manifestChecksum | length == 64) and
  (.verifiedAt | length > 0) and
  (.counts | length == 17) and
  (.checksums | length == 17)
' "$migration_dir/verification.json"
jq -e '
  .version == 1 and
  (.sourceSchemaFingerprint | length == 64) and
  (.sourceSnapshotChecksum | length == 64) and
  (.efHistory | length > 0) and
  (.tables | length == 17) and
  (.mappings | length > 0) and
  (.histograms | length > 0) and
  (.anomalies | length == 0) and
  (.autoFixes | length > 0) and
  (.checksum | length == 64)
' "$migration_dir/manifest.json"

cp "$migration_dir/target-state.json" "$migration_dir/before-repeat.json"
if migrate legacy apply \
  --source-fixture "$testdata/valid_snapshot.json" \
  --target-state "$migration_dir/target-state.json" \
  --manifest "$migration_dir/manifest.json" \
  > "$migration_dir/repeat.log" 2>&1; then
  echo "Repeated migration unexpectedly succeeded" >&2
  exit 1
fi
grep -F 'migration run has already been applied' "$migration_dir/repeat.log"
cmp "$migration_dir/before-repeat.json" "$migration_dir/target-state.json"
