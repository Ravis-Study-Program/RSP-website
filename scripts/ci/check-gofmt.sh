#!/usr/bin/env bash
# Fail when any Go file under backend/ is not gofmt-formatted.
#   scripts/ci/check-gofmt.sh
set -euo pipefail

unformatted="$(gofmt -l backend)"
if [[ -n "$unformatted" ]]; then
  printf 'Go files need gofmt:\n%s\n' "$unformatted" >&2
  exit 1
fi
