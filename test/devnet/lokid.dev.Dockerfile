# syntax=docker/dockerfile:1.4
#
# Dev build of lokid (go-flokicoin's own full node daemon) for the
# devnet's private regtest chain. `gflc` is supplied as an additional
# build context pointing at the sibling flokiorg/go-flokicoin checkout
# (see docker-compose.yml), so this builds from whatever's on disk there
# instead of a pinned/production release — never the production image,
# mirroring the sibling-checkout convention flnd/dev.Dockerfile already
# uses in this repo's own docker-compose.dev.yml.
FROM golang:1.26.8-alpine AS dev

RUN apk add --no-cache git gcc musl-dev

WORKDIR /go/src/github.com/flokiorg/go-flokicoin

ENV GOFLAGS=-mod=mod

COPY --from=gflc go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY --from=gflc . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -o /bin/lokid . && \
    go build -o /bin/lokid-cli ./cmd/lokid-cli

ENTRYPOINT ["lokid"]
