#!/usr/bin/env bash
# Create or update the single canary comment on a pull request.
#   scripts/canary/comment.sh ready   <pr-number> <url> <sha>
#   scripts/canary/comment.sh deleted <pr-number> <reason>
set -euo pipefail

command=${1:?usage: comment.sh ready|deleted <pr-number> ...}
pr=${2:?pull request number is required}
marker='<!-- rsp-canary -->'

case $command in
  ready)
    url=${3:?canary url is required}
    sha=${4:?commit sha is required}
    body="${marker}
**Canary ready:** ${url}

- Fake data only. Sign up with any email address, then open the verification email at [captured emails](${url}/mailpit/).
- Use **Switch roles** in the header to become a Student, Mentor, Coordinator, Director or System Admin. Privileged roles activate after you set up two-factor authentication in Settings.
- Deployed \`${sha::7}\`. Pushes redeploy it. To start over with fresh data, run the **Canary** workflow with this PR number and **reset** ticked.
- Remove the \`canary\` label to delete this canary early."
    ;;
  deleted)
    reason=${3:?reason is required}
    body="${marker}
Canary deleted with its database because ${reason}."
    ;;
  *)
    echo "unknown command: $command" >&2
    exit 2
    ;;
esac

comment_id=$(gh api --paginate "repos/${GITHUB_REPOSITORY}/issues/${pr}/comments" \
  --jq ".[] | select(.user.login == \"github-actions[bot]\" and (.body | contains(\"${marker}\"))) | .id" | head -n 1)

if [[ -n $comment_id ]]; then
  gh api --method PATCH "repos/${GITHUB_REPOSITORY}/issues/comments/${comment_id}" -f body="$body" >/dev/null
else
  gh api --method POST "repos/${GITHUB_REPOSITORY}/issues/${pr}/comments" -f body="$body" >/dev/null
fi
