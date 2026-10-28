# Build Stage
# Pinned to the Go version go.mod requires (rather than the floating
# `alpine` tag, which silently tracks whatever Go release is newest) plus
# its digest for build reproducibility. Bumped automatically by Dependabot
# (.github/dependabot.yml, docker ecosystem).
FROM golang:1.26.10-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS builder

# Install build dependencies
RUN apk add --no-cache git make build-base

WORKDIR /app

# Copy go.mod and go.sum first to leverage layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build Arguments
ARG TAG=unknown
ARG COMMIT=unknown

# Build flspd
# -ldflags explanation:
# -w -s: strip debug information for smaller binary
# -X ...: inject version information into build variables
RUN go build -ldflags "-w -s -X github.com/flowgate-lsp/flspd/platform/build.tag=${TAG} -X github.com/flowgate-lsp/flspd/platform/build.revision=${COMMIT}" -o /bin/flspd ./cmd/flspd

# Build flspd_revenue_cli
RUN go build -ldflags "-w -s -X github.com/flowgate-lsp/flspd/platform/build.tag=${TAG} -X github.com/flowgate-lsp/flspd/platform/build.revision=${COMMIT}" -o /bin/flspd_revenue_cli ./cmd/flspd_revenue_cli

# Runtime Stage
FROM alpine:latest@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

# Install runtime dependencies
# ca-certificates: for HTTPS connections
# tzdata: for time zone support
# bash: for convenience (optional, can be removed for minimalism)
RUN apk add --no-cache ca-certificates tzdata bash

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /bin/flspd /bin/flspd
COPY --from=builder /bin/flspd_revenue_cli /bin/flspd_revenue_cli

# Expose gRPC port (default in sample.env is often random, but standardizing on 8080 or similar is good practice)
# The user can map this in docker-compose
EXPOSE 8080

# Environment variables should be provided via docker-compose or run command
ENTRYPOINT ["/bin/flspd"]
