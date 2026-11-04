# Security remediation

Do not disclose vulnerabilities or credentials in public issues. Use the
repository's private vulnerability reporting channel or contact a maintainer
privately. The automated tracker contains dependency advisories only; never paste
secrets or private exploit details into it.

## Daily triage

CI scans `main` daily at 06:00 UTC and supports a manual run. Only security and
container jobs run on scheduled/manual invocations. The workflow updates one
tracking issue with scanner output, job status, and links to full logs/artifacts.
A failed job means findings **or a scan error**, never a clean bill of health.
Review the logs to distinguish them. A failed build or dependency install also
means incomplete coverage.

The maintainer who triages the report assigns an owner and adds the remediation
PR, fixed version (or “no fix available”), and target date to an issue comment.
Triage critical findings within one day and high findings within three days.
Target fixes within seven days for critical findings and fourteen for high
findings. Confirmed exploitation requires immediate incident response; see
`docs/incident-response.md`. These targets start at discovery, not fix publication.
If no fix exists, document mitigations and reassess at least weekly.

Exceptions require a named maintainer's approval, rationale, compensating controls,
and an expiry date no more than 30 days away. Do not add permanent blanket ignores.

## Activate Renovate

1. Install/authorize the Renovate GitHub App for this repository.
2. Enable GitHub's dependency graph and Dependabot alerts. Grant Renovate read
   access to those alerts. Disable duplicate Dependabot update PRs if configured.
3. Confirm that the app reads `renovate.json`, then inspect its first security PR.
   Verify that multiple fixable alerts share the security group, that the pnpm
   lockfile updates, and that normal CI runs on every bot commit.
4. Check Go, transitive pnpm, and image findings against the daily report. GitHub
   alerts do not cover everything scanners detect. Create manual fixes for gaps.

Security updates bypass the weekly routine-update schedule. Renovate proposes the
lowest fixed version and never auto-merges. Grouping is a target, not a promise
that every ecosystem or breaking fix can share one PR. Runtime versions embedded
in workflow environment variables and shell commands may need manual updates;
the enabled managers do not cover every version string.

## Verify and merge

Require the existing quality, unit, integration, browser, real-stack, and image
checks on the latest PR commit. Keep security fixes separate from routine upgrades.
Use the opt-in canary for frontend/auth review when needed; see `docs/canary.md`.
A maintainer reviews and merges each remediation PR. Agent checks and auto-merge
are not part of this initial rollout.

## Next phase: baseline-aware gates

The current full-tree PR gates remain strict. Daily reports and grouped updates
do not yet prevent inherited vulnerabilities from blocking unrelated PRs.

Before changing that policy, implement a base-versus-head comparison with tests
for unchanged findings, new advisories, worsened exposure, removed findings,
renamed packages, and scanner failures. Use matching scanner versions and advisory
data. Compare advisory/package identities and exposure, not finding counts.
Require remediation PRs to remove their targeted findings. Preserve strict secret
checks. Enable the new gate only after a shadow run proves its behavior against
real reports; do not weaken gates with `continue-on-error` in the meantime.
