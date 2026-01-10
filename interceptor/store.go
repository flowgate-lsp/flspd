package interceptor

import (
	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/common"
)

type InterceptStore interface {
	PaymentInfo(htlcPaymentHash []byte) (string, *common.OpeningFeeParams, []byte, []byte, []byte, int64, int64, *wire.OutPoint, *string, error)
	SetFundingTx(paymentHash []byte, channelPoint *wire.OutPoint) error
	RegisterPayment(token string, params *common.OpeningFeeParams, destination, paymentHash, paymentSecret []byte, incomingAmountMsat, outgoingAmountMsat int64, tag string) error
}
