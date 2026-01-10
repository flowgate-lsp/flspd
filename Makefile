PKG := github.com/flowgate-lsp/flspd
TAG := $(shell git describe --tags --dirty)

release-all: release-flspd release-cli

release-flspd:
	go get $(PKG)/cmd/flspd
	CGO_ENABLED=0 go build -v -trimpath -o flspd -ldflags "-s -w -X $(PKG)/build.tag=$(TAG)" $(PKG)/cmd/flspd

release-cli:
	go get $(PKG)/cmd/flspd_revenue_cli
	CGO_ENABLED=0 go build -v -trimpath -o flspd_revenue_cli -ldflags="-s -w -X $(PKG)/build.tag=$(TAG)" $(PKG)/cmd/flspd_revenue_cli

clean:
	rm -f flspd
	rm -f flspd_revenue_cli
