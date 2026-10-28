package ports

import "github.com/flowgate-lsp/flspd/core"

type NodesService interface {
	GetNode(token string) (*core.Node, error)
	GetNodes() []*core.Node
	GetMinChannelCapacity() uint64
}
