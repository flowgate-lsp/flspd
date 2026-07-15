package lsps

import (
	"context"
	"testing"

	"github.com/flokiorg/go-flokicoin/chaincfg/chainhash"
	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// newRecoveryTestService builds a Lsps1SettlementService with mocked
// dependencies, wired directly (not via Start) so recoverStuckOrders/
// recoverSettledCreatedOrders can be called and asserted on in isolation.
// feeEstimator defaults to a fixed estimation if the caller doesn't need to
// control it.
func newRecoveryTestService(client *MockLightningClient, store *MockOpeningStore, feeEstimator *MockFeeEstimator) *Lsps1SettlementService {
	if feeEstimator == nil {
		feeEstimator = new(MockFeeEstimator)
		feeEst := &chain.FeeEstimation{LokiPerVByte: func() *float64 { f := 1.0; return &f }()}
		feeEstimator.On("EstimateFeeRate", mock.Anything, mock.Anything).Return(feeEst, nil)
	}
	historyStore := new(MockHistoryStore)
	conf := &config.NodeConfig{}
	svc := NewLsps1SettlementService(client, store, historyStore, config.NewNodeConfigRef(conf), feeEstimator, chain.FeeStrategyEconomy, nil)
	svc.ctx = context.Background()
	return svc
}

func createdOrder(orderId, invoice string) *core.Lsps1Order {
	return &core.Lsps1Order{
		OrderId:        orderId,
		PeerId:         "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f",
		OrderState:     "CREATED",
		PaymentState:   "CREATED",
		PaymentInvoice: invoice,
		OrderTotalLoki: 1000,
	}
}

func TestRecoverSettledCreatedOrders_SettledInvoiceProcessesOrder(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	order := createdOrder("order-settled", "inv-settled")

	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{order}, nil)
	client.On("LookupInvoiceState", mock.Anything, "inv-settled").Return(lightning.InvoiceStateSettled, nil)
	client.On("IsConnected", mock.Anything).Return(true, nil)

	chanPoint := &wire.OutPoint{Hash: chainhash.Hash{0x01}, Index: 0}
	client.On("OpenChannel", mock.Anything).Return(chanPoint, nil)
	store.On("UpdateLsps1OrderFunding", "order-settled", chanPoint.Hash.String(), 0).Return(nil)
	store.On("UpdateLsps1OrderState", "order-settled", "PAID", "PAID", "CREATED").Return(true, nil)
	store.On("UpdateLsps1OrderState", "order-settled", "COMPLETED", "PAID", "PAID").Return(true, nil)

	svc := newRecoveryTestService(client, store, nil)

	svc.recoverSettledCreatedOrders()

	store.AssertCalled(t, "UpdateLsps1OrderState", "order-settled", "PAID", "PAID", "CREATED")
	client.AssertCalled(t, "OpenChannel", mock.Anything)
}

func TestRecoverSettledCreatedOrders_CanceledInvoiceIsNotProcessed(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	order := createdOrder("order-canceled", "inv-canceled")

	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{order}, nil)
	client.On("LookupInvoiceState", mock.Anything, "inv-canceled").Return(lightning.InvoiceStateCanceled, nil)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverSettledCreatedOrders()

	store.AssertNotCalled(t, "UpdateLsps1OrderState", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	client.AssertNotCalled(t, "OpenChannel", mock.Anything)
}

func TestRecoverSettledCreatedOrders_OpenInvoiceIsNotProcessed(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	order := createdOrder("order-open", "inv-open")

	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{order}, nil)
	client.On("LookupInvoiceState", mock.Anything, "inv-open").Return(lightning.InvoiceStateOpen, nil)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverSettledCreatedOrders()

	store.AssertNotCalled(t, "UpdateLsps1OrderState", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	client.AssertNotCalled(t, "OpenChannel", mock.Anything)
}

func TestRecoverSettledCreatedOrders_EmptyPaymentInvoiceSkipsLndLookup(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	order := createdOrder("order-no-invoice", "") // never got as far as generating an invoice

	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{order}, nil)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverSettledCreatedOrders()

	client.AssertNotCalled(t, "LookupInvoiceState", mock.Anything, mock.Anything)
}

func TestRecoverSettledCreatedOrders_LookupErrorSkipsOrderButContinuesOthers(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	broken := createdOrder("order-broken", "inv-broken")
	healthy := createdOrder("order-healthy", "inv-healthy")

	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{broken, healthy}, nil)
	client.On("LookupInvoiceState", mock.Anything, "inv-broken").Return(lightning.InvoiceStateOpen, assert.AnError)
	client.On("LookupInvoiceState", mock.Anything, "inv-healthy").Return(lightning.InvoiceStateOpen, nil)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverSettledCreatedOrders()

	client.AssertCalled(t, "LookupInvoiceState", mock.Anything, "inv-healthy")
}

func TestRecoverSettledCreatedOrders_EmptyListSkipsLndEntirely(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{}, nil)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverSettledCreatedOrders()

	client.AssertNotCalled(t, "LookupInvoiceState", mock.Anything, mock.Anything)
}

func TestRecoverStuckOrders_PaidStoreErrorSkipsCreatedOrderCheck(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	store.On("GetAllPaidLsps1Orders").Return([]*core.Lsps1Order{}, assert.AnError)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverStuckOrders()

	store.AssertNotCalled(t, "GetAllCreatedLsps1Orders")
}

func TestRecoverStuckOrders_NoStuckOrdersStillChecksCreatedOrders(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	store.On("GetAllPaidLsps1Orders").Return([]*core.Lsps1Order{}, nil)
	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{}, nil)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverStuckOrders()

	store.AssertCalled(t, "GetAllCreatedLsps1Orders")
}

func fundedTxid() string {
	return "0102030405060708010203040506070801020304050607080102030405060" + "7"
}

// paidOrderWithFunding builds a PAID order that already has funding details
// recorded — the state F-8 describes: OpenChannel and
// UpdateLsps1OrderFunding both succeeded on some earlier run, but the
// COMPLETED state write itself failed (or never ran), leaving the order
// stuck in PAID despite the channel already existing on-chain.
func paidOrderWithFunding(orderId string) *core.Lsps1Order {
	txid := fundedTxid()
	outpoint := 0
	return &core.Lsps1Order{
		OrderId:         orderId,
		PeerId:          "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f",
		OrderState:      "PAID",
		PaymentState:    "PAID",
		OrderTotalLoki:  1000,
		FundingTxid:     &txid,
		FundingOutpoint: &outpoint,
	}
}

// TestRecoverStuckOrders_FundedOrderReconcilesToCompletedWithoutReopening is
// the regression test for F-8: a PAID order whose funding was already
// successfully recorded (simulating a DB write failure specifically on the
// COMPLETED transition, post-open) must be reconciled to COMPLETED by
// verifying the channel is still active in LND — not by calling
// OpenChannel again, which would risk opening a second channel for the
// same order.
func TestRecoverStuckOrders_FundedOrderReconcilesToCompletedWithoutReopening(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	order := paidOrderWithFunding("order-funded")

	store.On("GetAllPaidLsps1Orders").Return([]*core.Lsps1Order{order}, nil)
	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{}, nil)
	client.On("GetChannel", mock.Anything, mock.Anything).Return(&lightning.GetChannelResult{}, nil)
	store.On("UpdateLsps1OrderState", "order-funded", "COMPLETED", "PAID", "PAID").Return(true, nil)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverStuckOrders()

	client.AssertNotCalled(t, "OpenChannel", mock.Anything)
	store.AssertCalled(t, "UpdateLsps1OrderState", "order-funded", "COMPLETED", "PAID", "PAID")
}

// TestRecoverStuckOrders_FundedOrderWithMissingChannelIsLeftForManualReview
// covers the other side: if the recorded funding doesn't correspond to an
// active LND channel (e.g. force-closed before recovery ran), recovery
// must not guess by retrying OpenChannel — that's exactly the double-open
// risk this fix exists to avoid. The order is left in PAID.
func TestRecoverStuckOrders_FundedOrderWithMissingChannelIsLeftForManualReview(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	order := paidOrderWithFunding("order-funded-missing")

	store.On("GetAllPaidLsps1Orders").Return([]*core.Lsps1Order{order}, nil)
	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{}, nil)
	client.On("GetChannel", mock.Anything, mock.Anything).Return(nil, assert.AnError)

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverStuckOrders()

	client.AssertNotCalled(t, "OpenChannel", mock.Anything)
	store.AssertNotCalled(t, "UpdateLsps1OrderState", "order-funded-missing", "COMPLETED", "PAID", "PAID")
}

// TestRecoverStuckOrders_UnfundedOrderStillRetriesOpenChannel confirms the
// F-8 fix is scoped correctly: a stuck PAID order with NO recorded funding
// (the channel-open itself may never have run) must still go through the
// existing openChannelForOrder retry path, unchanged.
func TestRecoverStuckOrders_UnfundedOrderStillRetriesOpenChannel(t *testing.T) {
	client := new(MockLightningClient)
	store := new(MockOpeningStore)
	order := &core.Lsps1Order{
		OrderId:      "order-unfunded",
		PeerId:       "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f",
		OrderState:   "PAID",
		PaymentState: "PAID",
	}

	store.On("GetAllPaidLsps1Orders").Return([]*core.Lsps1Order{order}, nil)
	store.On("GetAllCreatedLsps1Orders").Return([]*core.Lsps1Order{}, nil)
	client.On("IsConnected", mock.Anything).Return(false, nil) // deferred, not a failure

	svc := newRecoveryTestService(client, store, nil)
	svc.recoverStuckOrders()

	client.AssertCalled(t, "IsConnected", mock.Anything)
	client.AssertNotCalled(t, "GetChannel", mock.Anything, mock.Anything)
}
