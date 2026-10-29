package main

import (
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/flowgate-lsp/flspd/ports"
	"github.com/flowgate-lsp/flspd/services"
)

// newNodeLSPSHandlers builds the LSPS handlers that answer on one node. They
// see only that node, so its protocol list, JIT switch, LSPS1 options, fee
// menu and sales are its own, and its announcement says what it answers.
func newNodeLSPSHandlers(
	node *core.Node,
	cfg lsps.Config,
	client lsps.LndClientWrapper,
	openingService ports.OpeningService,
	liquidityManager lsps.LiquidityManager,
	notifications lsps.NotificationService,
	orderWatcher lsps.OrderWatcher,
) (*lsps.Handlers, error) {
	scope, err := services.NewNodesService([]*core.Node{node})
	if err != nil {
		return nil, err
	}
	return lsps.NewHandlers(cfg, client, scope, openingService, liquidityManager, notifications, orderWatcher), nil
}
