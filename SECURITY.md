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

## Baseline comparison pilot

The current full-tree PR gates remain strict. Daily reports and grouped updates
do not yet prevent inherited vulnerabilities from blocking unrelated PRs.

PRs run a pnpm comparison in shadow mode. The same pinned pnpm binary converts
the event's base lockfile and the PR merge lockfile into audit trees without
installing packages. A loopback capture server collects those trees; its empty
responses are capture acknowledgements, not security scan results.

The job namespaces both trees and submits their union to the public npm audit
endpoint in **one request**. It freezes that response and partitions its finding
paths back into base and PR reports. Both reports use the same advisory records,
so an advisory publication between separate scans cannot create a false regression.
This is a frozen response for this pair of trees, not a mirror of npm's full
advisory database.

The job summary and `pnpm-security-shadow` artifact distinguish new advisories,
inherited findings, removed findings, and changed exposure. The artifact retains
the combined request, frozen response, derived reports, scanner versions, and a
SHA-256 manifest with lockfile hashes. To replay a downloaded artifact offline:

```sh
node scripts/ci/pnpm-shadow.mjs --replay /path/to/artifact
```

Replay verifies the request and response hashes before comparison. The hashes
detect accidental modification; they do not authenticate an untrusted artifact.
Findings do not fail this diagnostic job; invalid snapshots, unexpected paths,
scanner failures, or registry errors do. Do not require this pilot as a branch
protection check yet.

The comparison keys findings by GHSA and package name. A new vulnerable path or
affected version flags changed exposure conservatively, not proof of greater
exploitability. Partial path removals remain inherited. A package rename cannot
reuse another package's baseline. Tests cover those cases and incomplete reports.

Before enforcement, extend snapshot-based comparison to Go and images, and require
remediation PRs to remove explicit target advisories. Review real shadow reports
first, including npm endpoint compatibility with the combined graph. Preserve
strict secret checks and full-tree gates until then; do not bypass them with
`continue-on-error`.
