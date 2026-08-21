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

FROM caddy:2.10.2-alpine AS runtime

COPY apps/web/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /workspace/apps/web/dist /srv

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health/live || exit 1
