FROM node:24.19.0-alpine3.24 AS base
WORKDIR /workspace
RUN corepack enable && corepack prepare pnpm@10.15.0 --activate
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/auth/package.json apps/auth/package.json

FROM base AS dependencies
RUN pnpm install --frozen-lockfile --filter @rsp/auth...

FROM dependencies AS development
ENV NODE_ENV=development
COPY --chown=node:node apps/auth apps/auth
USER node
EXPOSE 3001
CMD ["pnpm", "--filter", "@rsp/auth", "dev"]

FROM dependencies AS build
COPY apps/auth apps/auth
RUN pnpm --filter @rsp/auth build

FROM build AS production-dependencies
RUN pnpm --filter @rsp/auth --prod --no-optional deploy /out/auth

FROM alpine:3.24 AS runtime
ENV NODE_ENV=production
RUN apk add --no-cache ca-certificates libstdc++ \
    && addgroup -S -g 10001 rsp \
    && adduser -S -D -H -u 10001 -G rsp rsp
COPY --from=base /usr/local/bin/node /usr/local/bin/node
WORKDIR /workspace
COPY --from=production-dependencies --chown=10001:10001 /out/auth ./apps/auth
USER 10001:10001
EXPOSE 3001
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:3001/health/live').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"
CMD ["node", "apps/auth/dist/server.js"]
