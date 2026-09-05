# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.6
ARG ALPINE_VERSION=3.24

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS dependencies
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

FROM dependencies AS service-build
ARG BINARY=api
COPY backend ./backend
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/rsp-service \
      "./backend/cmd/${BINARY}"

FROM dependencies AS goose-build
ARG GOOSE_VERSION=v3.27.3
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download "github.com/pressly/goose/v3@${GOOSE_VERSION}" \
    && cp -R "/go/pkg/mod/github.com/pressly/goose/v3@${GOOSE_VERSION}" /goose \
    && chmod -R u+w /goose
WORKDIR /goose
# This image only runs PostgreSQL migrations; omit the other database drivers.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go get golang.org/x/crypto@v0.55.0 \
    && CGO_ENABLED=0 go build \
      -tags='no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' \
      -trimpath \
      -ldflags="-s -w -X main.version=${GOOSE_VERSION}" \
      -o /out/goose \
      ./cmd/goose

FROM dependencies AS ops-build
COPY backend ./backend
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/rspctl \
      ./backend/cmd/rspctl \
    && CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/rsp-migrate \
      ./backend/cmd/rsp-migrate

FROM alpine:${ALPINE_VERSION} AS runtime
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 rsp \
    && adduser -S -D -H -u 10001 -G rsp rsp
COPY --from=service-build /out/rsp-service /usr/local/bin/rsp-service
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/rsp-service"]

FROM alpine:${ALPINE_VERSION} AS migrations
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 rsp \
    && adduser -S -D -H -u 10001 -G rsp rsp
COPY --from=goose-build /out/goose /usr/local/bin/goose
COPY --chown=10001:10001 db/migrations /migrations
ENV GOOSE_DRIVER=postgres \
    GOOSE_MIGRATION_DIR=/migrations
USER 10001:10001
ENTRYPOINT ["goose"]
CMD ["up"]

FROM alpine:${ALPINE_VERSION} AS ops
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 rsp \
    && adduser -S -D -H -u 10001 -G rsp rsp \
    && mkdir /work \
    && chown 10001:10001 /work
COPY --from=ops-build /out/rspctl /usr/local/bin/rspctl
COPY --from=ops-build /out/rsp-migrate /usr/local/bin/rsp-migrate
WORKDIR /work
USER 10001:10001
CMD ["rspctl"]
