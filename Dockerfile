# Build Stage
FROM golang:alpine AS builder

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

# Build lspd (now flspd)
# -ldflags explanation:
# -w -s: strip debug information for smaller binary
# -X ...: inject version information into build variables
RUN go build -ldflags "-w -s -X github.com/flowgate-lsp/flspd/build.tag=${TAG} -X github.com/flowgate-lsp/flspd/build.revision=${COMMIT}" -o /bin/flspd ./cmd/flspd

# Build lspd_revenue_cli
RUN go build -ldflags "-w -s -X github.com/flowgate-lsp/flspd/build.tag=${TAG} -X github.com/flowgate-lsp/flspd/build.revision=${COMMIT}" -o /bin/lspd_revenue_cli ./cmd/lspd_revenue_cli

# Runtime Stage
FROM alpine:latest

# Install runtime dependencies
# ca-certificates: for HTTPS connections
# tzdata: for time zone support
# bash: for convenience (optional, can be removed for minimalism)
RUN apk add --no-cache ca-certificates tzdata bash

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /bin/flspd /bin/flspd
COPY --from=builder /bin/lspd_revenue_cli /bin/lspd_revenue_cli

# Expose gRPC port (default in sample.env is often random, but standardizing on 8080 or similar is good practice)
# The user can map this in docker-compose
EXPOSE 8080

# Environment variables should be provided via docker-compose or run command
ENTRYPOINT ["/bin/flspd"]
