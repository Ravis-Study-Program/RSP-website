# syntax=docker/dockerfile:1.7
FROM node:24.6.0-alpine3.22 AS base

WORKDIR /workspace
RUN corepack enable && corepack prepare pnpm@10.15.0 --activate

COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile --filter @rsp/web...

FROM base AS development

COPY apps/web apps/web
COPY api/openapi.yaml api/openapi.yaml
USER node
EXPOSE 5173
CMD ["pnpm", "--filter", "@rsp/web", "dev"]

FROM base AS build

COPY apps/web apps/web
COPY api/openapi.yaml api/openapi.yaml
RUN pnpm --filter @rsp/web build

FROM nginxinc/nginx-unprivileged:1.29.1-alpine AS runtime

COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build --chown=101:101 /workspace/apps/web/dist /usr/share/nginx/html

USER 101
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/nginx-health || exit 1
