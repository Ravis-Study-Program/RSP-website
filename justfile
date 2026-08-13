set dotenv-load := true

bootstrap:
    pnpm install --frozen-lockfile
    go mod download

dev:
    docker compose --profile dev up --build

dev-native:
    docker compose up -d postgres mailpit
    pnpm --filter @rsp/auth dev & pnpm --filter @rsp/web dev & go run ./backend/cmd/api

generate:
    go generate ./...
    pnpm --filter @rsp/web generate

migrate-up:
    go run ./backend/cmd/rsp-migrate up

seed:
    go run ./backend/cmd/rspctl seed

check:
    gofmt -w backend
    go vet ./...
    pnpm check

test:
    go test ./...
    pnpm test

e2e:
    pnpm --filter @rsp/web e2e

prod-up:
    docker compose --profile prod up --build

