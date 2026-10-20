#!/usr/bin/env bash
# Create or update the single canary comment on a pull request.
#   scripts/canary/comment.sh <pr-number> <body containing <!-- rsp-canary -->>
set -euo pipefail

pr=${1:?pull request number is required}
body=${2:?comment body is required}
marker='<!-- rsp-canary -->'

comment_id=$(gh api --paginate "repos/${GITHUB_REPOSITORY}/issues/${pr}/comments" \
  --jq ".[] | select(.user.login == \"github-actions[bot]\" and (.body | contains(\"${marker}\"))) | .id" | head -n 1)

if [[ -n $comment_id ]]; then
  gh api --method PATCH "repos/${GITHUB_REPOSITORY}/issues/comments/${comment_id}" -f body="$body" >/dev/null
else
  gh api --method POST "repos/${GITHUB_REPOSITORY}/issues/${pr}/comments" -f body="$body" >/dev/null
fi
