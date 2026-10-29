set shell := ["bash", "-c"]

VERSION := `cat VERSION`
PKG := "github.com/flowgate-lsp/flspd"
COMPOSE := "docker compose -f deploy/docker-compose.dev.yml --project-directory ."

# Default recipe: show help
default:
    @just --list

### --- 🏗️ Build --- ###

# Build binaries and/or the frontend.
# Usage: just build [backend|cli|frontend|dist|all]  (default: all)
[group('build')]
build target="all":
    #!/usr/bin/env bash
    set -euo pipefail
    backend() {
        go get {{PKG}}/cmd/flspd
        CGO_ENABLED=0 go build -v -trimpath -o flspd -ldflags "-s -w -X {{PKG}}/platform/build.tag={{VERSION}}" ./cmd/flspd
    }
    cli() {
        go get {{PKG}}/cmd/flspd_revenue_cli
        CGO_ENABLED=0 go build -v -trimpath -o flspd_revenue_cli -ldflags="-s -w -X {{PKG}}/platform/build.tag={{VERSION}}" ./cmd/flspd_revenue_cli
    }
    frontend() { (cd web && npm run build); }
    dist() {
        mkdir -p dist
        echo "Building for linux/amd64..."
        GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -trimpath -o dist/flspd-linux-amd64 -ldflags "-s -w -X {{PKG}}/platform/build.tag={{VERSION}}" ./cmd/flspd
        echo "Building for linux/arm64..."
        GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -v -trimpath -o dist/flspd-linux-arm64 -ldflags "-s -w -X {{PKG}}/platform/build.tag={{VERSION}}" ./cmd/flspd
    }
    case "{{target}}" in
        backend)  backend ;;
        cli)      cli ;;
        frontend) frontend ;;
        dist)     dist ;;
        all)      backend; cli; frontend ;;
        *) echo "unknown build target: {{target}} (want: backend|cli|frontend|dist|all)" >&2; exit 1 ;;
    esac

# Build the image lokihub's regtest stack runs as its test LSP (LP-13; see
# deploy/regtest/README.md). Needs an nmilat checkout carrying niplsp until
# go.mod pins one (BACKLOG T8-5).
# Usage: just image-regtest /path/to/nmilat [image tag]  (default flspd:regtest)
[group('build')]
image-regtest nmilat tag="flspd:regtest":
    docker build -f deploy/Dockerfile.regtest \
        --build-context nmilat={{nmilat}} \
        --build-arg COMMIT=$(git rev-parse --short HEAD) \
        -t {{tag}} .

# Deploy the frontend to Cloudflare Workers
[group('build')]
deploy-frontend: (build "frontend")
    cd web && npx wrangler deploy

### --- 🚀 Development --- ###

# Initialize the dev environment (backend data/config and/or frontend deps).
# Usage: just setup [backend|frontend|all]  (default: all)
[group('dev')]
setup target="all":
    #!/usr/bin/env bash
    set -euo pipefail
    backend() {
        mkdir -p data/postgres data/flnd
        [ -f deploy/nodes.yaml ] || cp deploy/nodes.dev.sample.yaml deploy/nodes.yaml
        [ -f .env ] || cp deploy/compose.env.sample .env
        [ -f data/flnd/flnd.conf ] || cp data/flnd/flnd.conf.sample data/flnd/flnd.conf
        "{{just_executable()}}" _relay-config
    }
    frontend() { (cd web && npm install); }
    case "{{target}}" in
        backend)  backend ;;
        frontend) frontend ;;
        all)      backend; frontend ;;
        *) echo "unknown setup target: {{target}} (want: backend|frontend|all)" >&2; exit 1 ;;
    esac

# Write the dev relay's data/relay/relay.yaml from its sample, with a fresh
# relay key, when it is missing. An older docker-compose.dev.yml mounted it
# as a file, so Docker may have made an empty directory in its place before
# setup ran: that directory is removed first (T8-6, R4-F6).
[private]
_relay-config:
    #!/usr/bin/env bash
    set -euo pipefail
    f=data/relay/relay.yaml
    if [ -d "$f" ]; then
        rmdir "$f" 2>/dev/null || { echo "$f is a directory Docker created before the file existed; remove it (it may need sudo) and run this again" >&2; exit 1; }
    fi
    if [ ! -f "$f" ]; then
        mkdir -p data/relay
        sed "s/__RELAY_PRIVKEY__/$(openssl rand -hex 32)/" deploy/relay.dev.sample.yaml > "$f"
    fi

# Start the frontend in development mode but connected to a production/remote backend
[group('dev')]
frontend-dev-prod url:
    cd web && VITE_PROXY_URL={{url}} npm run dev

# Manage the local docker compose stack: mainnet-backed flnd (real wallet,
# real funds -- see docker-compose.dev.yml) + flspd (hot-reloading via Air)
# + postgres + frontend.
#   up       start the stack or a specific service (ensures the shared
#            "flowgate-integration" network other product stacks join for
#            integration testing, and the dev relay's config, exist first)
#   down     stop the stack
#   restart  restart the stack or a specific service
#   ps       show service status
#   logs     follow logs for the stack or a specific service
#   sh       exec into a service (default cmd: /bin/sh)
#   network  create the shared "flowgate-integration" network (idempotent)
# Usage: just docker <up|down|restart|ps|logs|sh|network> [service] [cmd]
[group('dev')]
docker action *args:
    #!/usr/bin/env bash
    set -euo pipefail
    set -- {{args}}
    ensure_network() {
        docker network inspect flowgate-integration >/dev/null 2>&1 || docker network create flowgate-integration
    }
    case "{{action}}" in
        network) ensure_network ;;
        up)      ensure_network; "{{just_executable()}}" _relay-config; {{COMPOSE}} up "$@" -d ;;
        down)    {{COMPOSE}} down ;;
        restart) {{COMPOSE}} restart "$@" ;;
        ps)      {{COMPOSE}} ps ;;
        logs)    {{COMPOSE}} logs -f "$@" ;;
        sh)
            service="${1:?usage: just docker sh <service> [cmd]}"
            cmd="${2:-/bin/sh}"
            {{COMPOSE}} exec "$service" "$cmd"
            ;;
        *) echo "unknown docker action: {{action}} (want: up|down|restart|ps|logs|sh|network)" >&2; exit 1 ;;
    esac

### --- 🧹 Operations --- ###

# Clean build artifacts (binaries, dist output, air's tmp/ build dir, compiled
# test binaries), and optionally ("all") also stop the stack and clear
# docker-managed caches. Deliberately NEVER touches data/ -- data/flnd is a
# live mainnet flnd wallet holding real funds and data/postgres its
# associated state; resetting either must be a deliberate manual
# `rm -rf data/<subdir>`, never a side effect of a clean command.
# Usage: just clean [all]
[group('ops')]
clean target="":
    #!/usr/bin/env bash
    set -euo pipefail
    rm -f flspd flspd_revenue_cli
    rm -rf dist web/dist tmp
    find . -name "*.test" -type f -delete
    if [ "{{target}}" = "all" ]; then
        {{COMPOSE}} down -v --remove-orphans
    elif [ -n "{{target}}" ]; then
        echo "unknown clean target: {{target}} (want: all)" >&2
        exit 1
    fi

### --- ✅ Testing & Linting --- ###

# Note: "all" runs backend+frontend only -- lsps is required separately by
# .agents/skills/flspd-lsps-protocol, not part of the routine suite.
# Run tests. Usage: just test [backend|frontend|lsps|all]  (default: all)
[group('test')]
test target="all":
    #!/usr/bin/env bash
    set -euo pipefail
    backend() { go test -v -race ./...; }
    frontend() { (cd web && npm run test); }
    lsps() { go test -v ./lsps/... -count=1; }
    case "{{target}}" in
        backend)  backend ;;
        frontend) frontend ;;
        lsps)     lsps ;;
        all)      backend; frontend ;;
        *) echo "unknown test target: {{target}} (want: backend|frontend|lsps|all)" >&2; exit 1 ;;
    esac

# Run Go linter
[group('test')]
lint:
    golangci-lint run ./...

# Format Go code
[group('test')]
fmt:
    go fmt ./...

# Vet Go code
[group('test')]
vet:
    go vet ./...
