PKG := github.com/flowgate-lsp/flspd
TAG := $(shell git describe --tags --dirty)

release-all: release-lspd release-cli

release-lspd:
	go get $(PKG)/cmd/lspd
	CGO_ENABLED=0 go build -v -trimpath -o lspd -ldflags "-s -w -X $(PKG)/build.tag=$(TAG)" $(PKG)/cmd/lspd

release-cli:
	go get $(PKG)/cmd/lspd_revenue_cli
	CGO_ENABLED=0 go build -v -trimpath -o lspd_revenue_cli -ldflags="-s -w -X $(PKG)/build.tag=$(TAG)" $(PKG)/cmd/lspd_revenue_cli

clean:
	rm -f lspd
	rm -f lspd_revenue_cli
