PKG := github.com/flowgate-lsp/flspd
TAG := $(shell cat VERSION)

release-all: release-flspd release-cli

release-flspd:
	go get $(PKG)/cmd/flspd
	CGO_ENABLED=0 go build -v -trimpath -o flspd -ldflags "-s -w -X $(PKG)/build.tag=$(TAG)" ./cmd/flspd

release-cli:
	go get $(PKG)/cmd/flspd_revenue_cli
	CGO_ENABLED=0 go build -v -trimpath -o flspd_revenue_cli -ldflags="-s -w -X $(PKG)/build.tag=$(TAG)" ./cmd/flspd_revenue_cli

clean:
	rm -f flspd
	rm -f flspd_revenue_cli
	rm -rf dist

# CI/CD Verification Targets
	@echo "Building for linux/amd64..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -trimpath -o dist/flspd-linux-amd64 -ldflags "-s -w -X $(PKG)/build.tag=$(TAG)" ./cmd/flspd
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -trimpath -o dist/flspd_revenue_cli-linux-amd64 -ldflags "-s -w -X $(PKG)/build.tag=$(TAG)" ./cmd/flspd_revenue_cli
	@echo "Building for linux/arm64..."
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -v -trimpath -o dist/flspd-linux-arm64 -ldflags "-s -w -X $(PKG)/build.tag=$(TAG)" ./cmd/flspd
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -v -trimpath -o dist/flspd_revenue_cli-linux-arm64 -ldflags "-s -w -X $(PKG)/build.tag=$(TAG)" ./cmd/flspd_revenue_cli

docker-verify:
	docker build . -t flspd:local-test

# Internal helper for setup
_rundev_setup:
	mkdir -p data/postgres
	@if [ ! -f data/nodes.yaml ]; then cp nodes.sample.yaml data/nodes.yaml; fi
	@if [ ! -f .env ]; then cp compose.env.sample .env; fi

rundev: _rundev_setup
	docker compose -f docker-compose.dev.yml up

rundev-build: _rundev_setup
	docker compose -f docker-compose.dev.yml up --build

stopdev:
	docker compose -f docker-compose.dev.yml down
