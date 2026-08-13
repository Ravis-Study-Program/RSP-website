FROM node:24.6.0-alpine AS dependencies
WORKDIR /workspace
RUN corepack enable && corepack prepare pnpm@10.15.0 --activate
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/auth/package.json apps/auth/package.json
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile

FROM dependencies AS development
ENV NODE_ENV=development
COPY apps/auth apps/auth
USER node
EXPOSE 3001
CMD ["pnpm", "--filter", "@rsp/auth", "dev"]

FROM dependencies AS build
COPY apps/auth apps/auth
RUN pnpm --filter @rsp/auth build

FROM dependencies AS production-dependencies
RUN pnpm prune --prod

FROM node:24.6.0-alpine AS runtime
ENV NODE_ENV=production
WORKDIR /workspace
COPY --from=production-dependencies --chown=node:node /workspace/node_modules ./node_modules
COPY --from=production-dependencies --chown=node:node /workspace/apps/auth/node_modules ./apps/auth/node_modules
COPY --from=production-dependencies --chown=node:node /workspace/apps/auth/package.json ./apps/auth/package.json
COPY --from=build --chown=node:node /workspace/apps/auth/dist ./apps/auth/dist
USER node
EXPOSE 3001
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:3001/health/live').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"
CMD ["node", "apps/auth/dist/server.js"]
