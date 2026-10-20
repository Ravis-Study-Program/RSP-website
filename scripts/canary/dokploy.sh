#!/usr/bin/env bash
# Create, update or delete one pull request's canary stack on Dokploy.
#
#   scripts/canary/dokploy.sh deploy  <pr-number> <branch>
#   scripts/canary/dokploy.sh destroy <pr-number>
#
# Required environment:
#   DOKPLOY_URL                    e.g. https://dokploy.example.org
#   DOKPLOY_API_KEY                Dokploy API key (Settings > Profile > API/CLI)
#   DOKPLOY_ENVIRONMENT_ID         Dokploy environment that holds the canaries
#   DOKPLOY_GITHUB_ID              Dokploy GitHub provider id for this repository
#   CANARY_DOMAIN                  wildcard parent, e.g. canary.example.org
#   GITHUB_REPOSITORY              owner/name (set by GitHub Actions)
# Optional:
#   CANARY_MAILPIT_AUTH            "user:password" protecting the captured inbox
#
# Each canary gets fresh random secrets on creation. Redeploys keep them,
# because the PostgreSQL volume was initialised with the original passwords.
set -euo pipefail
# Fail inside $(...) too, so a failed API call cannot yield an empty id.
shopt -s inherit_errexit 2>/dev/null || true

command=${1:?usage: dokploy.sh deploy|destroy <pr-number> [branch]}
pr=${2:?pull request number is required}
[[ $pr =~ ^[0-9]+$ ]] || { echo "invalid pull request number: $pr" >&2; exit 2; }

: "${DOKPLOY_URL:?}" "${DOKPLOY_API_KEY:?}" "${CANARY_DOMAIN:?}"
name="rsp-pr-${pr}"
host="pr-${pr}.${CANARY_DOMAIN}"
origin="https://${host}"

api() {
  local method=$1 route=$2 body=${3:-} response
  local args=(--fail-with-body --silent --show-error --max-time 60
    -X "$method" -H "x-api-key: ${DOKPLOY_API_KEY}" -H 'accept: application/json')
  if [[ -n $body ]]; then
    args+=(-H 'content-type: application/json' --data "$body")
  fi
  if ! response=$(curl "${args[@]}" "${DOKPLOY_URL%/}/api/${route}"); then
    echo "Dokploy ${route} failed: ${response:-no response body}" >&2
    return 1
  fi
  printf '%s' "$response"
}

# A wrong id makes compose.create return a bare 404, so check it first and
# list the environments the API key can see.
require_environment() {
  local projects
  projects=$(api GET project.all)
  if ! jq -e --arg id "$DOKPLOY_ENVIRONMENT_ID" \
    'any(.[].environments[]?; .environmentId == $id)' <<<"$projects" >/dev/null; then
    echo "DOKPLOY_CANARY_ENVIRONMENT_ID ${DOKPLOY_ENVIRONMENT_ID} is not an environment this API key can see." >&2
    echo "Available environments (project / environment: id):" >&2
    jq -r '.[] | .name as $project | .environments[]? | "  \($project) / \(.name): \(.environmentId)"' <<<"$projects" >&2
    exit 1
  fi
}

# project.all nests compose services under projects (and environments on
# newer Dokploy), so search the whole tree for the canary by name.
find_compose_id() {
  api GET project.all | jq -r --arg name "$name" \
    '[.. | objects | select(has("composeId") and .name == $name) | .composeId] | first // empty'
}

compose_status() {
  api GET "compose.one?composeId=$1" | jq -r '.composeStatus // "unknown"'
}

secret() { openssl rand -hex 32; }

canary_env() {
  cat <<EOF
POSTGRES_DB=rsp
POSTGRES_USER=rsp
POSTGRES_PASSWORD=$(secret)
MIGRATION_DB_PASSWORD=$(secret)
APP_DB_PASSWORD=$(secret)
AUTH_DB_PASSWORD=$(secret)
PUBLIC_ORIGIN=${origin}
AUTH_ISSUER=${origin}
AUTH_AUDIENCE=rsp-api
AUTH_TRUSTED_ORIGINS=${origin}
CURSOR_SECRET=$(secret)
BETTER_AUTH_SECRET=$(secret)
IDENTITY_SERVICE_TOKEN=$(secret)
LEETCODE_SYNC_URL=https://leetcode.com/graphql/
EMAIL_PROVIDER=smtp
EMAIL_FROM=no-reply@canary.invalid
AWS_REGION=ap-southeast-2
CANARY_MAILPIT_AUTH=${CANARY_MAILPIT_AUTH:-}
EOF
}

create_compose() {
  local branch=$1 compose_id owner repository
  : "${DOKPLOY_ENVIRONMENT_ID:?}" "${DOKPLOY_GITHUB_ID:?}" "${GITHUB_REPOSITORY:?}"
  owner=${GITHUB_REPOSITORY%%/*}
  repository=${GITHUB_REPOSITORY#*/}
  require_environment

  compose_id=$(api POST compose.create "$(jq -n --arg name "$name" --arg env "$DOKPLOY_ENVIRONMENT_ID" \
    '{name: $name, appName: $name, environmentId: $env, composeType: "docker-compose",
      description: "Canary for pull request; deleted when the pull request closes."}')" | jq -r '.composeId')
  [[ -n $compose_id && $compose_id != null ]] || { echo "compose.create returned no composeId" >&2; exit 1; }

  # Dokploy prefixes the custom command with "docker". It layers the canary
  # overlay over the production stack under this canary's own project name.
  api POST compose.update "$(jq -n \
    --arg id "$compose_id" --arg github "$DOKPLOY_GITHUB_ID" \
    --arg owner "$owner" --arg repository "$repository" --arg branch "$branch" \
    --arg command "compose -p ${name} -f compose.prod.yaml -f compose.canary.yaml up -d --build --remove-orphans" \
    --arg env "$(canary_env)" \
    '{composeId: $id, sourceType: "github", githubId: $github, owner: $owner,
      repository: $repository, branch: $branch, composePath: "compose.prod.yaml",
      command: $command, env: $env, createEnvFile: true, autoDeploy: false}')" >/dev/null

  api POST domain.create "$(jq -n --arg id "$compose_id" --arg host "$host" \
    '{composeId: $id, host: $host, serviceName: "caddy", port: 8080,
      domainType: "compose", https: true, certificateType: "letsencrypt", path: "/"}')" >/dev/null

  echo "$compose_id"
}

wait_until_ready() {
  local compose_id=$1 deadline=$((SECONDS + 1500)) status
  # Give Dokploy's queue a moment to pick the deployment up.
  sleep 15
  while ((SECONDS < deadline)); do
    status=$(compose_status "$compose_id")
    case $status in
      error) echo "Dokploy reported a failed deployment for ${name}" >&2; return 1 ;;
      done) break ;;
    esac
    sleep 10
  done
  while ((SECONDS < deadline)); do
    if curl --fail --silent --max-time 10 "${origin}/api/v2/health/ready" >/dev/null; then
      return 0
    fi
    sleep 10
  done
  echo "timed out waiting for ${origin}" >&2
  return 1
}

case $command in
  deploy)
    branch=${3:?branch is required for deploy}
    compose_id=$(find_compose_id)
    if [[ -z $compose_id ]]; then
      compose_id=$(create_compose "$branch")
      echo "Created ${name} (${compose_id})"
    fi
    api POST compose.deploy "$(jq -n --arg id "$compose_id" --arg title "PR #${pr} ${GITHUB_SHA:-}" \
      '{composeId: $id, title: $title}')" >/dev/null
    wait_until_ready "$compose_id"
    echo "url=${origin}"
    ;;
  destroy)
    compose_id=$(find_compose_id)
    if [[ -z $compose_id ]]; then
      echo "No canary named ${name}; nothing to delete."
      exit 0
    fi
    api POST compose.delete "$(jq -n --arg id "$compose_id" '{composeId: $id, deleteVolumes: true}')" >/dev/null
    echo "Deleted ${name} and its volumes."
    ;;
  *)
    echo "unknown command: $command" >&2
    exit 2
    ;;
esac
