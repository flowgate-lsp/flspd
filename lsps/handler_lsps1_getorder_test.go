package lsps

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newGetOrderTestServer builds a *Handlers with a NodeConfig fixture that
// satisfies getLsps1ConfigValues (isOrderExpired/handleLsps1GetOrder both
// call it), with orderExpirySeconds configurable per test.
func newGetOrderTestServer(t *testing.T, mockOS *MockOpeningService, orderExpirySeconds uint32) *Handlers {
	t.Helper()
	mockNS := new(MockNodesService)
	minReqConf := uint16(0)
	minFundingConf := uint16(1)
	supportsZeroReserve := true
	maxExpiry := uint32(201600)
	minClient, maxClient := uint64(0), uint64(100_000_000)
	minLsp, maxLsp := uint64(0), uint64(100_000_000)
	minChan, maxChan := uint64(20000), uint64(100_000_000)

	mockNS.On("GetNodes").Return([]*core.Node{{NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
		Lsps1MinRequiredChannelConfirmations: &minReqConf,
		Lsps1MinFundingConfirmsWithinBlocks:  &minFundingConf,
		Lsps1SupportsZeroChannelReserve:      &supportsZeroReserve,
		Lsps1MaxChannelExpiryBlocks:          &maxExpiry,
		Lsps1OrderExpirySeconds:              &orderExpirySeconds,
		Lsps1MinInitialClientBalanceLoki:     &minClient,
		Lsps1MaxInitialClientBalanceLoki:     &maxClient,
		Lsps1MinInitialLspBalanceLoki:        &minLsp,
		Lsps1MaxInitialLspBalanceLoki:        &maxLsp,
		Lsps1MinChannelBalanceLoki:           &minChan,
		Lsps1MaxChannelBalanceLoki:           &maxChan,
	})}}).Maybe()

	return NewHandlers(Config{}, new(MockLndClient), mockNS, mockOS, new(MockLiquidityManager), new(MockNotificationService))
}

func TestHandleLsps1GetInfo(t *testing.T) {
	mockOS := new(MockOpeningService)
	mockOS.On("GetFeeParamsMenu", "token-a", mock.Anything).Return([]*core.OpeningFeeParams{
		{MinFeeMloki: 1000, Proportional: 100, ValidUntil: "2099-01-01T00:00:00.000Z"},
	}, nil)
	server := newGetOrderTestServer(t, mockOS, 3600)
	// newGetOrderTestServer's NodesService fixture has no Tokens, so
	// override with one that does, to exercise the fee-params-gathering
	// loop rather than just the (already-covered-elsewhere) empty case.
	mockNS := new(MockNodesService)
	mockNS.On("GetNodes").Return([]*core.Node{{
		Tokens: []string{"token-a"},
		NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
			Lsps1MinRequiredChannelConfirmations: uint16Ptr(1),
			Lsps1MinFundingConfirmsWithinBlocks:  uint16Ptr(2),
			Lsps1SupportsZeroChannelReserve:      boolPtr(true),
			Lsps1MaxChannelExpiryBlocks:          uint32Ptr(201600),
			Lsps1OrderExpirySeconds:              uint32Ptr(3600),
			Lsps1MinInitialClientBalanceLoki:     uint64Ptr(0),
			Lsps1MaxInitialClientBalanceLoki:     uint64Ptr(100_000_000),
			Lsps1MinInitialLspBalanceLoki:        uint64Ptr(0),
			Lsps1MaxInitialLspBalanceLoki:        uint64Ptr(100_000_000),
			Lsps1MinChannelBalanceLoki:           uint64Ptr(20000),
			Lsps1MaxChannelBalanceLoki:           uint64Ptr(100_000_000),
		}),
	}})
	server.nodesService = mockNS

	req := &JsonRpcRequest{Id: json.RawMessage(`"id"`)}
	resp := server.handleLsps1GetInfo(req)

	require.Nil(t, resp.Error)
	result, ok := resp.Result.(Lsps1GetInfoResponse)
	require.True(t, ok)
	assert.Equal(t, []string{"lsps1.create_order", "lsps1.get_order"}, result.Options)
	require.Len(t, result.OpeningFeeParams, 1)
	assert.Equal(t, "1000", result.OpeningFeeParams[0].MinFeeMloki)
	assert.EqualValues(t, 1, result.MinRequiredChannelConfirmations)
	assert.True(t, result.SupportsZeroChannelReserve)
	assert.Equal(t, "20000", result.MinChannelBalanceLoki)
}

func uint16Ptr(v uint16) *uint16 { return &v }
func uint32Ptr(v uint32) *uint32 { return &v }
func uint64Ptr(v uint64) *uint64 { return &v }
func boolPtr(v bool) *bool       { return &v }

func TestIsOrderExpired(t *testing.T) {
	server := newGetOrderTestServer(t, new(MockOpeningService), 3600) // 1 hour

	assert.False(t, server.isOrderExpired(time.Now()), "a just-created order must not be expired")
	assert.True(t, server.isOrderExpired(time.Now().Add(-2*time.Hour)), "an order older than the expiry window must be expired")
}

func TestHandleLsps1GetOrder_InvalidParams(t *testing.T) {
	server := newGetOrderTestServer(t, new(MockOpeningService), 3600)
	req := &JsonRpcRequest{Id: json.RawMessage(`"id"`), Params: json.RawMessage(`not-json`)}

	resp := server.handleLsps1GetOrder(req)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
}

func TestHandleLsps1GetOrder_NotFound(t *testing.T) {
	mockOS := new(MockOpeningService)
	mockOS.On("GetLsps1Order", "missing-order").Return(nil, assert.AnError)
	server := newGetOrderTestServer(t, mockOS, 3600)

	req := &JsonRpcRequest{Id: json.RawMessage(`"id"`), Params: json.RawMessage(`{"order_id":"missing-order"}`)}
	resp := server.handleLsps1GetOrder(req)

	require.NotNil(t, resp.Error)
	assert.Equal(t, 101, resp.Error.Code)
}

func TestHandleLsps1GetOrder_ActiveCreatedOrder(t *testing.T) {
	mockOS := new(MockOpeningService)
	order := &core.Lsps1Order{
		OrderId:      "order-active",
		CreatedAt:    time.Now(),
		OrderState:   string(Lsps1OrderStateCreated),
		PaymentState: "EXPECT_PAYMENT",
		Token:        "tok",
	}
	mockOS.On("GetLsps1Order", "order-active").Return(order, nil)
	server := newGetOrderTestServer(t, mockOS, 3600) // 1 hour, well in the future

	req := &JsonRpcRequest{Id: json.RawMessage(`"id"`), Params: json.RawMessage(`{"order_id":"order-active"}`)}
	resp := server.handleLsps1GetOrder(req)

	require.Nil(t, resp.Error)
	result, ok := resp.Result.(Lsps1CreateOrderResponse)
	require.True(t, ok)
	assert.Equal(t, "order-active", result.OrderId)
	assert.Equal(t, string(Lsps1OrderStateCreated), result.OrderState, "a non-expired CREATED order must stay CREATED")
	assert.Nil(t, result.Channel, "an order with no funding_txid must report a nil channel")
}

// TestHandleLsps1GetOrder_ExpiredCreatedOrder_StateOverridden covers the
// override path: a CREATED order past its expiry window must be reported
// as EXPIRED even though that's not what's stored, since expiry is
// computed on read rather than via a background sweep touching every row.
func TestHandleLsps1GetOrder_ExpiredCreatedOrder_StateOverridden(t *testing.T) {
	mockOS := new(MockOpeningService)
	order := &core.Lsps1Order{
		OrderId:      "order-expired",
		CreatedAt:    time.Now().Add(-2 * time.Hour),
		OrderState:   string(Lsps1OrderStateCreated),
		PaymentState: "EXPECT_PAYMENT",
	}
	mockOS.On("GetLsps1Order", "order-expired").Return(order, nil)
	server := newGetOrderTestServer(t, mockOS, 3600) // 1 hour expiry, order is 2h old

	req := &JsonRpcRequest{Id: json.RawMessage(`"id"`), Params: json.RawMessage(`{"order_id":"order-expired"}`)}
	resp := server.handleLsps1GetOrder(req)

	require.Nil(t, resp.Error)
	result, ok := resp.Result.(Lsps1CreateOrderResponse)
	require.True(t, ok)
	assert.Equal(t, string(Lsps1OrderStateExpired), result.OrderState)
	assert.Equal(t, string(Lsps1PaymentStateExpired), result.Payment.Bolt11.State)
}

// TestHandleLsps1GetOrder_PaidOrder_MapsToCreatedForSpecCompliance covers
// the internal-PAID-state-to-spec-CREATED-state mapping: LSPS1 only
// defines CREATED/COMPLETED/FAILED, so the internal PAID bookkeeping
// state (paid but not yet channel-funded) reports back as CREATED.
func TestHandleLsps1GetOrder_PaidOrder_MapsToCreatedForSpecCompliance(t *testing.T) {
	mockOS := new(MockOpeningService)
	order := &core.Lsps1Order{
		OrderId:      "order-paid",
		CreatedAt:    time.Now(),
		OrderState:   string(Lsps1OrderStatePaid),
		PaymentState: "PAID",
	}
	mockOS.On("GetLsps1Order", "order-paid").Return(order, nil)
	server := newGetOrderTestServer(t, mockOS, 3600)

	req := &JsonRpcRequest{Id: json.RawMessage(`"id"`), Params: json.RawMessage(`{"order_id":"order-paid"}`)}
	resp := server.handleLsps1GetOrder(req)

	require.Nil(t, resp.Error)
	result, ok := resp.Result.(Lsps1CreateOrderResponse)
	require.True(t, ok)
	assert.Equal(t, string(Lsps1OrderStateCreated), result.OrderState, "internal PAID state must map to spec-compliant CREATED")
}

// TestHandleLsps1GetOrder_FundedOrder_ReportsChannel covers the
// channel-info-populated branch: once a funding_txid is recorded, the
// response's channel field must be populated, not nil.
func TestHandleLsps1GetOrder_FundedOrder_ReportsChannel(t *testing.T) {
	mockOS := new(MockOpeningService)
	txid := "abc123"
	vout := 1
	order := &core.Lsps1Order{
		OrderId:         "order-funded",
		CreatedAt:       time.Now(),
		OrderState:      string(Lsps1OrderStateCompleted),
		PaymentState:    "PAID",
		FundingTxid:     &txid,
		FundingOutpoint: &vout,
	}
	mockOS.On("GetLsps1Order", "order-funded").Return(order, nil)
	server := newGetOrderTestServer(t, mockOS, 3600)

	req := &JsonRpcRequest{Id: json.RawMessage(`"id"`), Params: json.RawMessage(`{"order_id":"order-funded"}`)}
	resp := server.handleLsps1GetOrder(req)

	require.Nil(t, resp.Error)
	result, ok := resp.Result.(Lsps1CreateOrderResponse)
	require.True(t, ok)
	require.NotNil(t, result.Channel)
	channelMap, ok := result.Channel.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "abc123:1", channelMap["funding_outpoint"])
}
