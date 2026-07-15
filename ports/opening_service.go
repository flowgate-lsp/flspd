package ports

import (
	"time"

	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flowgate-lsp/flspd/core"
)

type OpeningService interface {
	GetFeeParamsMenu(token string, privateKey *btcec.PrivateKey) ([]*core.OpeningFeeParams, error)
	ValidateOpeningFeeParams(params *core.OpeningFeeParams, publicKey *btcec.PublicKey) bool
	IsCurrentChainFeeCheaper(token string, params *core.OpeningFeeParams) bool
	RegisterLsps2Buy(reg *core.Lsps2BuyRegistration) error
	RegisterLsps1Order(order *core.Lsps1Order) error
	GetLsps1Order(orderId string) (*core.Lsps1Order, error)
	RegisterLsps5Webhook(webhook *core.Lsps5Webhook) error
	GetLsps5Webhooks(clientNodeId string) ([]*core.Lsps5Webhook, error)
	RemoveLsps5Webhook(clientNodeId, appName string) error
	CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error)
	CountLsps5Webhooks(clientNodeId string) (int, error)
}
