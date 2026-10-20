#!/usr/bin/env bash
# Build and check the production container images.
#
#   scripts/ci/containers.sh build             build every image, tagged rsp-<name>:ci
#   scripts/ci/containers.sh check-web-bundle  fail if the web image ships demo fixtures
set -euo pipefail

command=${1:?usage: containers.sh build|check-web-bundle}

case $command in
  build)
    docker build --file backend/Dockerfile --target runtime --build-arg BINARY=api --tag rsp-api:ci .
    docker build --file backend/Dockerfile --target runtime --build-arg BINARY=worker --tag rsp-worker:ci .
    docker build --file backend/Dockerfile --target migrations --tag rsp-migrations:ci .
    docker build --file backend/Dockerfile --target ops --tag rsp-ops:ci .
    docker build --file apps/auth/Dockerfile --target runtime --tag rsp-auth:ci .
    docker build --file apps/web/Dockerfile --target runtime --tag rsp-web:ci .
    docker run --rm --entrypoint sh rsp-ops:ci -c \
      'command -v rspctl >/dev/null && command -v rsp-migrate >/dev/null'
    ;;
  check-web-bundle)
    if docker run --rm --entrypoint sh rsp-web:ci -c \
      "grep -R -E 'rsp-demo-|example\\.test|Amelia Chen' /srv"; then
      echo "Production web image contains demo-only fixture data" >&2
      exit 1
    fi
    ;;
  *)
    echo "unknown command: $command" >&2
    exit 2
    ;;
esac
