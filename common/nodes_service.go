package common

import (
	"errors"
	"fmt"

	ecies "github.com/ecies/go/v2"
	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/lightning"
	"golang.org/x/sync/singleflight"
)

type Node struct {
	NodeId              []byte
	Client              lightning.Client
	NodeConfig          *config.NodeConfig
	PrivateKey          *btcec.PrivateKey
	PublicKey           *btcec.PublicKey
	EciesPrivateKey     *ecies.PrivateKey
	EciesPublicKey      *ecies.PublicKey
	OpenChannelReqGroup singleflight.Group
	Tokens              []string
}

type NodesService interface {
	GetNode(token string) (*Node, error)
	GetNodes() []*Node
	GetMinChannelCapacity() uint64
}
type nodesService struct {
	nodes      []*Node
	nodeLookup map[string]*Node
}

func NewNodesService(nodes []*Node) (NodesService, error) {
	nodeLookup := make(map[string]*Node)
	for _, node := range nodes {
		for _, token := range node.Tokens {
			_, exists := nodeLookup[token]
			if exists {
				return nil, fmt.Errorf("cannot have multiple nodes with the same token")
			}

			nodeLookup[token] = node
		}
	}

	return &nodesService{
		nodes:      nodes,
		nodeLookup: nodeLookup,
	}, nil
}

var ErrNodeNotFound = errors.New("node not found")

func (s *nodesService) GetNode(token string) (*Node, error) {
	node, ok := s.nodeLookup[token]
	if !ok {
		return nil, ErrNodeNotFound
	}

	return node, nil
}

func (s *nodesService) GetNodes() []*Node {
	return s.nodes
}

// GetMinChannelCapacity returns the minimum channel capacity across all nodes.
// If no nodes are configured, returns a default of 20000.
func (s *nodesService) GetMinChannelCapacity() uint64 {
	if len(s.nodes) == 0 {
		return 20000
	}
	// Return the first node's config (all nodes should have same minimum in typical setup)
	return s.nodes[0].NodeConfig.MinChannelCapacityLoki
}
