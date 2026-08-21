# syntax=docker/dockerfile:1.7
FROM node:24.19.0-alpine3.24 AS base

WORKDIR /workspace
RUN corepack enable && corepack prepare pnpm@10.15.0 --activate

COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile --filter @rsp/web...

FROM base AS development

COPY --chown=node:node apps/web apps/web
COPY --chown=node:node api/openapi.yaml api/openapi.yaml
RUN chown -R node:node /workspace/apps/web
USER node
EXPOSE 5173
CMD ["pnpm", "--filter", "@rsp/web", "dev"]

FROM base AS build

COPY apps/web apps/web
COPY api/openapi.yaml api/openapi.yaml
RUN pnpm --filter @rsp/web build

FROM golang:1.26.6-alpine3.24 AS caddy-build

ARG CADDY_VERSION=2.11.4
ARG XCADDY_VERSION=v0.4.5

RUN apk add --no-cache git
RUN go install github.com/caddyserver/xcaddy/cmd/xcaddy@${XCADDY_VERSION}
RUN xcaddy build v${CADDY_VERSION} \
      --output /out/caddy \
      --replace golang.org/x/net=golang.org/x/net@v0.56.0 \
      --replace golang.org/x/text=golang.org/x/text@v0.39.0 \
      --replace google.golang.org/grpc=google.golang.org/grpc@v1.82.1

FROM alpine:3.24 AS runtime

RUN apk add --no-cache ca-certificates mailcap \
    && addgroup -S -g 10001 rsp \
    && adduser -S -D -H -u 10001 -G rsp rsp \
    && mkdir -p /config/caddy /data/caddy /srv \
    && chown -R 10001:10001 /config /data /srv

COPY --from=caddy-build --chown=10001:10001 /out/caddy /usr/bin/caddy
COPY --chown=10001:10001 apps/web/Caddyfile /etc/caddy/Caddyfile
COPY --from=build --chown=10001:10001 /workspace/apps/web/dist /srv

ENV XDG_CONFIG_HOME=/config \
    XDG_DATA_HOME=/data
USER 10001:10001
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health/live || exit 1
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
