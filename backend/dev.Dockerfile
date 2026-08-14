# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.6
ARG ALPINE_VERSION=3.24
ARG AIR_VERSION=v1.63.0

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION}

ARG AIR_VERSION

RUN apk add --no-cache ca-certificates git tzdata \
    && addgroup -S -g 10001 rsp \
    && adduser -S -D -u 10001 -G rsp rsp \
    && install -d -o rsp -g rsp /workspace /go/pkg/mod /home/rsp/.cache/go-build \
    && GOBIN=/usr/local/bin go install "github.com/air-verse/air@${AIR_VERSION}" \
    && chown -R rsp:rsp /go/pkg/mod /home/rsp/.cache/go-build

WORKDIR /workspace
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend ./backend
COPY api ./api
COPY db ./db
COPY deploy/dev ./deploy/dev
RUN chown -R rsp:rsp /workspace

USER 10001:10001
ENV GOCACHE=/home/rsp/.cache/go-build
CMD ["air", "-c", "deploy/dev/air-api.toml"]
