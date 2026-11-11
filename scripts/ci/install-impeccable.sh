#!/usr/bin/env bash
# Download the pinned Impeccable engine. The public release URL often
# answers 504, which the skill launcher treats as a hard failure.
#   scripts/ci/install-impeccable.sh
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
version=$(tr -d '[:space:]' < "$root/.agents/skills/impeccable/scripts/VERSION")
base="${IMPECCABLE_DOWNLOAD_BASE:-https://github.com/pbakaus/impeccable/releases/download}"
url="$base/engine-v$version/impeccable-linux-x64"
dest="${RUNNER_TEMP:-/tmp}/impeccable-$version"

fetch() {
  local target=$1 source=$2
  local attempt
  for attempt in 1 2 3 4 5; do
    if curl -fL --retry 3 --retry-all-errors --retry-delay 2 -o "$target" "$source"; then
      return 0
    fi
    echo "download attempt $attempt failed: $source" >&2
    sleep $((attempt * 2))
  done
  echo "could not download $source" >&2
  return 1
}

fetch "$dest" "$url"
fetch "$dest.sha256" "$url.sha256"

expected=$(awk 'NR==1 { print $1 }' "$dest.sha256")
actual=$(sha256sum "$dest" | awk '{ print $1 }')
if [[ -z "$expected" || "$actual" != "$expected" ]]; then
  echo "checksum mismatch for $url" >&2
  exit 1
fi
chmod +x "$dest"

if [[ -n "${GITHUB_ENV:-}" ]]; then
  echo "IMPECCABLE_BIN=$dest" >> "$GITHUB_ENV"
fi
echo "installed impeccable $version at $dest"
