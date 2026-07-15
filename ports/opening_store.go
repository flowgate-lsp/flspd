package ports

import (
	"time"

	"github.com/flowgate-lsp/flspd/core"
)

type OpeningStore interface {
	GetFeeParamsSettings(token string) ([]*core.OpeningFeeParamsSetting, error)
	SetFeeParams(token string, validity time.Duration, params core.OpeningFeeParams) error
	RegisterLsps2Buy(reg *core.Lsps2BuyRegistration) error
	GetLsps2BuyRegistration(scid uint64) (*core.Lsps2BuyRegistration, error)
	RegisterLsps1Order(order *core.Lsps1Order) error
	GetLsps1Order(orderId string) (*core.Lsps1Order, error)
	GetLsps1OrderByPaymentInvoice(paymentInvoice string) (*core.Lsps1Order, error)
	GetPaidLsps1Order(clientNodeId string) (*core.Lsps1Order, error)
	GetPaidLsps1Orders(clientNodeId string) ([]*core.Lsps1Order, error)
	GetAllPaidLsps1Orders() ([]*core.Lsps1Order, error)
	GetAllCreatedLsps1Orders() ([]*core.Lsps1Order, error)
	ListLsps1Orders(limit, offset int) ([]*core.Lsps1Order, error)
	// UpdateLsps1OrderState performs a compare-and-set: it only applies
	// newState/newPaymentState if the order's current order_state still
	// equals expectedCurrentState. Returns updated=false (with a nil
	// error) if another writer already moved the order out of
	// expectedCurrentState — callers MUST check this and not proceed with
	// side effects (like opening a channel) on a failed CAS, since it
	// means a concurrent writer already handled this transition.
	UpdateLsps1OrderState(orderId string, newState string, newPaymentState string, expectedCurrentState string) (updated bool, err error)
	UpdateLsps1OrderFunding(orderId string, fundingTxid string, fundingOutpoint int) error
	RegisterLsps5Webhook(webhook *core.Lsps5Webhook) error
	GetLsps5Webhooks(clientNodeId string) ([]*core.Lsps5Webhook, error)
	RemoveLsps5Webhook(clientNodeId, appName string) error
	CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error)
	CountLsps5Webhooks(clientNodeId string) (int, error)
	GetReservedLiquidity() (uint64, error)
	ExpireStaleOrders() (int64, error)
}
