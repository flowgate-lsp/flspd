package interceptor

import (
	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/core"
)

type InterceptStore interface {
	PaymentInfo(htlcPaymentHash []byte) (string, string, *core.OpeningFeeParams, []byte, []byte, []byte, int64, int64, *wire.OutPoint, *string, error)
	SetFundingTx(paymentHash []byte, channelPoint *wire.OutPoint) error
	RegisterPayment(token string, orderId string, params *core.OpeningFeeParams, destination, paymentHash, paymentSecret []byte, incomingAmountMloki, outgoingAmountMloki int64, tag string) error
	GetLsps2Order(scid uint64) (*core.Lsps2BuyRegistration, error)
	GetPaidLsps1Order(clientNodeId string) (*core.Lsps1Order, error)
}
