#!/usr/bin/env bash
# Print the head branch and commit of a pull request that may have a canary,
# as GitHub Actions step outputs (branch=..., sha=...).
#   scripts/canary/resolve-pr.sh <pr-number>
#
# Fails unless the pull request is open, from this repository, and carries the
# canary label (CANARY_LABEL, default "canary").
set -euo pipefail

pr=${1:?pull request number is required}
label=${CANARY_LABEL:-canary}

info=$(gh pr view "$pr" --repo "$GITHUB_REPOSITORY" --json headRefName,headRefOid,state,isCrossRepository,labels)
if [[ $(jq -r .isCrossRepository <<<"$info") == true || $(jq -r .state <<<"$info") != OPEN ]]; then
  echo "Pull request $pr is not an open same-repository pull request." >&2
  exit 1
fi
if ! jq -e --arg label "$label" 'any(.labels[]; .name == $label)' <<<"$info" >/dev/null; then
  echo "Add the $label label to pull request $pr to request a canary." >&2
  exit 1
fi

echo "branch=$(jq -r .headRefName <<<"$info")"
echo "sha=$(jq -r .headRefOid <<<"$info")"
