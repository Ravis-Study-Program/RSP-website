#!/usr/bin/env bash
# Build and check the production container images.
#
#   scripts/ci/containers.sh build             build every image, tagged rsp-<name>:ci
#   scripts/ci/containers.sh check-web-bundle  fail if the web image ships demo fixtures
# Set IMAGE_TAG to build/check a separate image set (default: ci).
set -euo pipefail

command=${1:?usage: containers.sh build|check-web-bundle}
tag=${IMAGE_TAG:-ci}

case $command in
  build)
    docker build --file backend/Dockerfile --target runtime --build-arg BINARY=api --tag "rsp-api:$tag" .
    docker build --file backend/Dockerfile --target runtime --build-arg BINARY=worker --tag "rsp-worker:$tag" .
    docker build --file backend/Dockerfile --target migrations --tag "rsp-migrations:$tag" .
    docker build --file backend/Dockerfile --target ops --tag "rsp-ops:$tag" .
    docker build --file apps/auth/Dockerfile --target runtime --tag "rsp-auth:$tag" .
    docker build --file apps/web/Dockerfile --target runtime --tag "rsp-web:$tag" .
    docker run --rm --entrypoint sh "rsp-ops:$tag" -c \
      'command -v rspctl >/dev/null && command -v rsp-migrate >/dev/null'
    ;;
  check-web-bundle)
    if docker run --rm --entrypoint sh "rsp-web:$tag" -c \
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
