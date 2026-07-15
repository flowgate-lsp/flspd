set shell := ["bash", "-c"]

VERSION := `cat VERSION`
PKG := "github.com/flowgate-lsp/flspd"
COMPOSE := "docker compose -f docker-compose.dev.yml"

# Default recipe: show help
default:
    @just --list

### --- 🏗️ Build --- ###

# Build all binaries and the frontend
build: build-backend build-cli build-frontend

# Build the flspd daemon
build-backend:
    go get {{PKG}}/cmd/flspd
    CGO_ENABLED=0 go build -v -trimpath -o flspd -ldflags "-s -w -X {{PKG}}/platform/build.tag={{VERSION}}" ./cmd/flspd

# Build the revenue CLI
build-cli:
    go get {{PKG}}/cmd/flspd_revenue_cli
    CGO_ENABLED=0 go build -v -trimpath -o flspd_revenue_cli -ldflags="-s -w -X {{PKG}}/platform/build.tag={{VERSION}}" ./cmd/flspd_revenue_cli

# Build the frontend production bundle
build-frontend:
    cd frontend && npm run build

# Deploy the frontend to Cloudflare Workers
deploy-frontend: build-frontend
    cd frontend && npx wrangler deploy

# Build for various platforms (CI/CD)
dist:
    mkdir -p dist
    @echo "Building for linux/amd64..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -trimpath -o dist/flspd-linux-amd64 -ldflags "-s -w -X {{PKG}}/platform/build.tag={{VERSION}}" ./cmd/flspd
    @echo "Building for linux/arm64..."
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -v -trimpath -o dist/flspd-linux-arm64 -ldflags "-s -w -X {{PKG}}/platform/build.tag={{VERSION}}" ./cmd/flspd

### --- 🚀 Development --- ###

# Initialize development environment
setup: setup-backend setup-frontend

# Setup backend data and config
setup-backend:
    mkdir -p data/postgres data/flnd
    [ -f data/nodes.yaml ] || cp configs/examples/nodes.sample.yaml data/nodes.yaml
    [ -f .env ] || cp configs/examples/compose.env.sample .env
    [ -f data/flnd/flnd.conf ] || cp data/flnd/flnd.conf.sample data/flnd/flnd.conf

# Install frontend dependencies
setup-frontend:
    cd frontend && npm install

# Start the frontend in development mode but connected to a production/remote backend
frontend-dev-prod url:
    cd frontend && VITE_PROXY_URL={{url}} npm run dev

# Start the stack or a specific service
up service="":
    {{COMPOSE}} up {{service}} -d

# Stop the stack
down:
    {{COMPOSE}} down

# Restart the stack or a specific service
restart service="":
    {{COMPOSE}} restart {{service}}

# View service status
ps:
    {{COMPOSE}} ps

# View logs for the stack or a specific service
logs service="":
    {{COMPOSE}} logs -f {{service}}

# Execute a command in a service (default: sh)
sh service cmd="/bin/sh":
    {{COMPOSE}} exec {{service}} {{cmd}}

### --- 🧹 Operations --- ###

# Clean build artifacts (binaries, dist output, air's tmp/ build dir, compiled test binaries)
clean:
    rm -f flspd flspd_revenue_cli
    rm -rf dist frontend/dist tmp
    find . -name "*.test" -type f -delete

# Deep clean: build artifacts, docker volumes, AND local dev state (data/ —
# nodes.yaml, flnd data dir, postgres data dir). Not folded into plain
# `clean`: data/ can hold live devnet config/wallet state a developer would
# not expect a routine "clean build artifacts" command to touch.
clean-all: clean
    rm -rf data
    {{COMPOSE}} down -v --remove-orphans

### --- ✅ Testing & Linting --- ###

# Run all tests (Go and Frontend)
test: test-backend test-frontend

# Run backend tests with race detector
test-backend:
    go test -v -race ./...

# Run LSPS protocol specific tests (required by .agents/skills/flspd)
test-lsps:
    go test -v ./lsps/... -count=1

# Run frontend tests
test-frontend:
    cd frontend && npm run test

# Run Go linter (required by .agents/skills/golang-pro)
lint:
    golangci-lint run ./...

# Format Go code
fmt:
    go fmt ./...

# Vet Go code
vet:
    go vet ./...
