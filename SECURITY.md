# Security remediation

Do not disclose vulnerabilities or credentials in public issues. Use the
repository's private vulnerability reporting channel or contact a maintainer
privately. The automated tracker contains dependency advisories only; never paste
secrets or private exploit details into it.

## Daily triage

CI scans `main` daily at 06:00 UTC and supports a manual run. Only security and
container and scanner-contract jobs run on scheduled/manual invocations. The workflow updates one
tracking issue with curated image vulnerability fields, job status, and links to
full logs/artifacts. Raw Go and pnpm reports remain in artifacts. The tracker never
copies free-form scanner output, secret findings, or source context into issues.
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
that every ecosystem or breaking fix can share one PR. Routine updates group by
manager. Runtime updates share a separate group: custom managers cover workflow
Go/Node/pnpm versions and Dockerfile Corepack commands alongside native Docker
and npm manager coverage. Review each group for consistent runtime versions.
Other versions embedded in shell commands still need manual updates.

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

The package manager is pnpm throughout. “npm” here names the registry and its
audit protocol; no npm CLI command installs or audits this project.

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

### Go comparison

The Go pilot inventories both module graphs, including replacements, plus the
standard library and toolchain. It freezes the Go database module index and
downloads all advisory records for that union. Each record must match the index's
modification time; a change during capture fails the job rather than mixes data.
Both scans use that local database, one govulncheck version, and the same local
Go toolchain on Linux/amd64. Read-only module mode prevents dependency metadata
updates. The scan covers `./...` with the default build tags, like the strict gate.

The `go-security-shadow` artifact retains the database, raw JSON streams, module
inventories, input hashes, and a hash manifest. Unexpected database requests fail
if the frozen database lacks the requested records. Registry/build/scan errors
fail the diagnostic job; vulnerability findings remain advisory in shadow mode.
Replay verifies the retained file hashes and compares results offline:

```sh
node scripts/ci/go-security-shadow.mjs --replay /path/to/artifact
```

The comparison keys Go findings by advisory and module, then tracks affected
versions, packages, and symbols. It distinguishes module/package exposure from
reachable vulnerable symbols. Caller traces and source positions do not create
false regressions after harmless code moves. Go advisories do not share pnpm's
severity scale; the report does not invent one.

### Image comparison

On PRs, a separate pilot runner builds six PR images and the base commit's
images under separate tags. The duplicate PR build isolates the required image
gate from pilot disk pressure and timeouts. It records immutable image IDs and requires
Linux/amd64. Both sets use Trivy 0.70.0 and one freshly downloaded database.
Every scan disables database updates and online dependency lookups. The job
checks the database hashes before and after the scans. Java analysis requires a
separate database; this pilot does not download one and must not gain Java images
without a matching snapshot extension.

The pilot retains all severities and unfixed OS/library vulnerabilities. A
severity downgrade or loss of an available fix must not look like remediation.
The strict image gate still blocks only fixable HIGH/CRITICAL findings.
The comparison keeps image, package, advisory, and library-file
identities separate. OS target labels omit mutable image tags, so retagging cannot
create a regression. Changed versions, severity, fix versions, paths, or distro
versions flag changed exposure. Missing image sets, unsupported/EOL OS reports,
missing package inventories, scanner errors, or database changes fail the pilot.

The `image-security-shadow` artifact retains raw reports, image IDs, database
metadata and fingerprint, and a report hash manifest for seven days. The database
binary stays on the runner; it is too large to upload on every PR. Replay checks
the retained report/metadata hashes and image identities, not the database binary:

```sh
node scripts/ci/image-security-shadow.mjs --replay /path/to/artifact
```

This replays the comparison, not the image scans. A full rescan would also need
the original images and database, which this artifact does not preserve.

The separate **Image baseline comparison (shadow)** job fails when the pilot or
artifact upload fails. Its runner and time budget are independent of the required
container gate.
Do not require that diagnostic job in branch protection yet. The existing Trivy
steps remain strict and do not use `continue-on-error`.

Floating Docker base tags and OS package repositories can change between builds.
The frozen vulnerability database removes advisory drift, not build-input drift.
Review shadow results before enforcement; reproducible image inputs need a
separate decision. Preserve strict secret checks throughout.

## Remediation targets

Apply the `security-remediation` label to a remediation PR. Renovate applies it
to vulnerability PRs, but a maintainer must supply the intended targets; the bot
does not infer them from scanner output. Create this label during setup if it
does not exist. An ordinary PR may opt in with the target block alone.

Add exactly one block to the PR body. List only the applicable ecosystems:

````markdown
```security-remediation
{
  "pnpm": [{ "id": "GHSA-aaaa-bbbb-cccc", "name": "nodemailer" }],
  "go": [{ "id": "GO-2026-1234", "name": "golang.org/x/net" }],
  "images": [{ "image": "api", "id": "CVE-2026-1234", "name": "libssl3" }]
}
```
````

Replace the examples with actual IDs and package names. Go names identify modules,
not import paths or symbols. Image names are `api`, `auth`, `worker`, `web`,
`migrations`, or `ops`. Wildcards, duplicates, unknown fields, and empty declarations
fail validation. A labelled PR without a block also fails.

The **Explicit remediation targets** job reads current PR metadata and checks that
its base/head refs still match the scan run. It downloads only the relevant
artifacts from that same run and invokes their integrity-checked replay. Every
target must exist on the base and disappear from all matching PR findings.
Partial path removal, a lower image severity, or an unfixed finding still counts
as present. A Go reachability-only mitigation does not satisfy full module
remediation; document mitigations through the exception process instead.

The `security-remediation` artifact records the checked intent, refs, and results.
Missing snapshots, hash failures, stale refs, targets absent from the base, and
unresolved targets fail the job. Unrelated inherited findings do not satisfy or
invalidate a declared target; the existing full-tree gates still apply separately.

Body edits and label changes trigger a fresh CI run, including new snapshots and
the target check. This deliberately favors simple, complete verification over
artifact reuse. Wait for the latest metadata-triggered run before merge.
Successful security uploads replace earlier attempts to avoid artifact name
conflicts. The check also requires successful upstream jobs for each declared
ecosystem; a failed refresh cannot fall back to an older artifact. Verify the
metadata event lifecycle on GitHub before making this check a required gate.
No current security or secret gate has been relaxed.

## Review evidence before enforcement

The **Pinned scanner contracts** job runs the pinned govulncheck on a small Go
program and Trivy on an Alpine image, then passes their real output through the
same parsers as the pilots. It also checks the loopback audit capture with the
pinned pnpm binary. Run these integration tests locally with the pinned tools
on `PATH` and `SECURITY_CONTRACT_TESTS=true`; ordinary unit runs skip them.

A maintainer must review real GitHub runs before a gate change. Local fixtures
and contract checks do not substitute for this evidence. Record run URLs, base/head
refs, the reviewer, and the decision in the security tracker. Verify at least:

- An ordinary PR: unchanged dependencies yield inherited findings, not false new ones.
- A grouped security PR: explicit targets resolve, and all existing regression tests pass.
- A partial fix: an affected path, image, or Go module remains and the target check fails.
- A scan or build failure: the diagnostic reports incomplete coverage, not clean results.
- Image build times, disk use, and floating-input drift remain acceptable.

Then confirm that metadata events refresh target statuses, decide how reproducible image
inputs work, and choose which diagnostic checks become required. Only after that review
should we replace blanket PR blocking with a tested baseline-aware policy.
