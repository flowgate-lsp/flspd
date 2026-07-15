package core

import (
	ecies "github.com/ecies/go/v2"
	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/lightning"
	"golang.org/x/sync/singleflight"
)

type Node struct {
	NodeId []byte
	Client lightning.Client
	// NodeConfig is held behind an atomic ref (not a raw pointer) so
	// concurrent readers (e.g. the interceptor's HTLC hot path) and a
	// concurrent writer (the dashboard's config-update handler) can't
	// race on the struct's fields — see config.NodeConfigRef and C-3.
	NodeConfig          *config.NodeConfigRef
	PrivateKey          *btcec.PrivateKey
	PublicKey           *btcec.PublicKey
	EciesPrivateKey     *ecies.PrivateKey
	EciesPublicKey      *ecies.PublicKey
	OpenChannelReqGroup singleflight.Group
	Tokens              []string
}
