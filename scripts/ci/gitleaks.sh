#!/usr/bin/env bash
# Download a checksum-pinned gitleaks CLI and scan the full git history.
# The gitleaks GitHub Action requires a paid licence for organisation
# repositories, so CI runs the MIT-licensed CLI directly.
#   scripts/ci/gitleaks.sh
#
# Required environment:
#   GITLEAKS_VERSION   e.g. 8.30.1
#   GITLEAKS_SHA256    sha256 of the linux_x64 release archive
# Optional:
#   RUNNER_TEMP        download directory (set by GitHub Actions)
set -euo pipefail

: "${GITLEAKS_VERSION:?}" "${GITLEAKS_SHA256:?}"
dir=${RUNNER_TEMP:-$(mktemp -d)}
archive="gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz"
url="https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/${archive}"

# GitHub's release host often returns 504. Retry that before failing the job.
downloaded=0
for attempt in 1 2 3 4 5; do
  if curl --fail --silent --show-error --location --retry 3 --retry-all-errors --retry-delay 2 \
    --output "$dir/$archive" "$url"; then
    downloaded=1
    break
  fi
  echo "download attempt $attempt failed: $url" >&2
  sleep $((attempt * 2))
done
if [[ "$downloaded" != 1 ]]; then
  echo "could not download $url" >&2
  exit 22
fi
echo "${GITLEAKS_SHA256}  $dir/$archive" | sha256sum --check --strict
tar --extract --gzip --file "$dir/$archive" --directory "$dir" gitleaks
"$dir/gitleaks" git --config .gitleaks.toml --redact --no-banner --exit-code 1 .
