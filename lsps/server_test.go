package lsps

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/flokiorg/flnd/lnrpc"
	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks
type MockLndClient struct {
	mock.Mock
}

func (m *MockLndClient) SubscribeCustomMessages(ctx context.Context, req *lnrpc.SubscribeCustomMessagesRequest) (lnrpc.Lightning_SubscribeCustomMessagesClient, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(lnrpc.Lightning_SubscribeCustomMessagesClient), args.Error(1)
}

func (m *MockLndClient) SendCustomMessage(ctx context.Context, msg *lnrpc.SendCustomMessageRequest) (*lnrpc.SendCustomMessageResponse, error) {
	args := m.Called(ctx, msg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lnrpc.SendCustomMessageResponse), args.Error(1)
}

func (m *MockLndClient) AddInvoice(ctx context.Context, req *lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lnrpc.AddInvoiceResponse), args.Error(1)
}

type MockNodesService struct {
	mock.Mock
}

func (m *MockNodesService) GetNode(token string) (*common.Node, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.Node), args.Error(1)
}

func (m *MockNodesService) GetNodes() []*common.Node {
	args := m.Called()
	return args.Get(0).([]*common.Node)
}

type MockOpeningService struct {
	mock.Mock
}

func (m *MockOpeningService) GetFeeParamsMenu(token string, privateKey *btcec.PrivateKey) ([]*common.OpeningFeeParams, error) {
	args := m.Called(token, privateKey)
	return args.Get(0).([]*common.OpeningFeeParams), args.Error(1)
}

func (m *MockOpeningService) ValidateOpeningFeeParams(params *common.OpeningFeeParams, publicKey *btcec.PublicKey) bool {
	args := m.Called(params, publicKey)
	return args.Bool(0)
}

func (m *MockOpeningService) IsCurrentChainFeeCheaper(token string, params *common.OpeningFeeParams) bool {
	args := m.Called(token, params)
	return args.Bool(0)
}

func (m *MockOpeningService) RegisterLsps2Buy(reg *common.Lsps2BuyRegistration) error {
	args := m.Called(reg)
	return args.Error(0)
}

func (m *MockOpeningService) RegisterLsps1Order(order *common.Lsps1Order) error {
	args := m.Called(order)
	return args.Error(0)
}

func (m *MockOpeningService) GetLsps1Order(orderId string) (*common.Lsps1Order, error) {
	args := m.Called(orderId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningService) RegisterLsps5Webhook(webhook *common.Lsps5Webhook) error {
	args := m.Called(webhook)
	return args.Error(0)
}

func (m *MockOpeningService) GetLsps5Webhooks(clientNodeId string) ([]*common.Lsps5Webhook, error) {
	args := m.Called(clientNodeId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*common.Lsps5Webhook), args.Error(1)
}

func (m *MockOpeningService) RemoveLsps5Webhook(clientNodeId, appName string) error {
	args := m.Called(clientNodeId, appName)
	return args.Error(0)
}

func (m *MockOpeningService) CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error) {
	args := m.Called(clientNodeId, state, excludeExpiredBefore)
	return args.Int(0), args.Error(1)
}

func (m *MockOpeningService) CountLsps5Webhooks(clientNodeId string) (int, error) {
	args := m.Called(clientNodeId)
	return args.Int(0), args.Error(1)
}

// MockLiquidityManager
type MockLiquidityManager struct {
	mock.Mock
}

func (m *MockLiquidityManager) GetAvailableOnChainLiquidity(ctx context.Context) (uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockLiquidityManager) GetReservedOnChainLiquidity(ctx context.Context) (uint64, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockLiquidityManager) CheckOnChainLiquidity(ctx context.Context, amountSat uint64) error {
	args := m.Called(ctx, amountSat)
	return args.Error(0)
}

// MockNotificationService
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) NotifyWebhookRegistered(pubkey string, webhook *common.Lsps5Webhook) bool {
	args := m.Called(pubkey, webhook)
	return args.Bool(0)
}

// Tests
func TestLsps2Buy(t *testing.T) {
	// Setup
	mockLnd := new(MockLndClient)
	mockNodes := new(MockNodesService)
	mockOpening := new(MockOpeningService)
	mockLiquidity := new(MockLiquidityManager)
	mockNotification := new(MockNotificationService)

	server := NewServer(Config{Lsps1MaxPendingOrders: 5, Lsps5MaxWebhooks: 10}, mockLnd, mockNodes, mockOpening, mockLiquidity, mockNotification)

	// Test Data
	privKey, pubKey := btcec.PrivKeyFromBytes([]byte("testprivkey1111111111111111111"))
	nodeId, _ := hex.DecodeString("033333333333333333333333333333333333333333333333333333333333333333")
	node := &common.Node{
		NodeId:     nodeId,
		PublicKey:  pubKey,
		PrivateKey: privKey,
		Tokens:     []string{"test-token"},
		NodeConfig: &config.NodeConfig{
			TimeLockDelta: 144,
		},
	}

	mockNodes.On("GetNodes").Return([]*common.Node{node})

	// Valid Params
	validUntil := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	mockOpening.On("ValidateOpeningFeeParams", mock.AnythingOfType("*common.OpeningFeeParams"), pubKey).Return(true)
	mockOpening.On("RegisterLsps2Buy", mock.AnythingOfType("*common.Lsps2BuyRegistration")).Return(nil)

	// Mock Liquidity Check (Success)
	mockLiquidity.On("CheckOnChainLiquidity", mock.Anything, mock.Anything).Return(nil)

	// Construct Request
	buyParams := Lsps2BuyParams{
		OpeningFeeParams: Lsps2OpeningFeeParams{
			MinFeeMloki:          "1000",
			Proportional:         500,
			ValidUntil:           validUntil,
			MinLifetime:          10080,
			MaxClientToSelfDelay: 2016,
			MinPaymentSizeMloki:  "1000",
			MaxPaymentSizeMloki:  "1000000",
			Promise:              "valid_promise",
		},
		PaymentSizeMloki: "100000",
	}
	paramsBytes, _ := json.Marshal(buyParams)
	req := &JsonRpcRequest{
		Method: "lsps2.buy",
		Params: paramsBytes,
		Id:     "1",
	}

	// Execute via route (bypassing listener loop for unit test)
	resp := server.route(req, []byte("peer_id"))

	// Verify
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result, ok := resp.Result.(Lsps2BuyResult)
	if !ok {
		// handle map[string]interface{} if json unmarshalling happened?
		// No, route returns types directly.
		// NOTE: route returns *JsonRpcResponse struct. Result field is interface{}.
		// Inside lsps2Buy, we assign Lsps2BuyResult struct.
		assert.IsType(t, Lsps2BuyResult{}, resp.Result)
		result = resp.Result.(Lsps2BuyResult)
	}

	assert.NotEmpty(t, result.JITChannelScid)
	assert.Equal(t, uint16(144), result.LspCltvExpiryDelta)
	assert.Equal(t, hex.EncodeToString(nodeId), result.LspNodeId)

	mockOpening.AssertExpectations(t)
}

func TestLsps2Buy_PaymentSizeValidation(t *testing.T) {
	// Setup
	mockLnd := new(MockLndClient)
	mockNodes := new(MockNodesService)
	mockOpening := new(MockOpeningService)
	mockLiquidity := new(MockLiquidityManager)
	mockNotification := new(MockNotificationService)

	server := NewServer(Config{Lsps1MaxPendingOrders: 5, Lsps5MaxWebhooks: 10}, mockLnd, mockNodes, mockOpening, mockLiquidity, mockNotification)

	// Test Data
	privKey, pubKey := btcec.PrivKeyFromBytes([]byte("testprivkey1111111111111111111"))
	node := &common.Node{
		PublicKey:  pubKey,
		PrivateKey: privKey,
		Tokens:     []string{"test-token"},
		NodeConfig: &config.NodeConfig{
			TimeLockDelta: 144,
		},
	}

	mockNodes.On("GetNodes").Return([]*common.Node{node})

	validUntil := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	mockOpening.On("ValidateOpeningFeeParams", mock.AnythingOfType("*common.OpeningFeeParams"), pubKey).Return(true)

	// Mock Liquidity Checks (Assuming they pass for validation tests unless specific failure tested)
	// Actually for validation failure BEFORE liquidity check, we don't need to mock it.
	// But let's mock it for the ones that reach it.

	// Case 1: Payment Size Too Small (Fee >= Amount) -> Reaches liquidity check first?
	// Handler sequence:
	// 1. Parse params
	// 2. Bound validation (Min/Max)
	// 3. Liquidity Check <--- This comes before computation? Yes.
	// 4. Compute Fee

	// So for Case 1, we DO need to mock liquidity check because it passes minimal bounds and reaches check.
	mockLiquidity.On("CheckOnChainLiquidity", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Case 1: Payment Size Too Small (Fee >= Amount)
	// MinFee = 1000, PaymentSize = 1000 -> Fee = 1000. Fee >= Amount (1000 >= 1000) -> Error
	buyParams1 := Lsps2BuyParams{
		OpeningFeeParams: Lsps2OpeningFeeParams{
			MinFeeMloki:          "1000",
			Proportional:         0,
			ValidUntil:           validUntil,
			MinLifetime:          10080,
			MaxClientToSelfDelay: 2016,
			MinPaymentSizeMloki:  "1000",
			MaxPaymentSizeMloki:  "1000000",
			Promise:              "valid_promise",
		},
		PaymentSizeMloki: "1000",
	}
	paramsBytes1, _ := json.Marshal(buyParams1)
	req1 := &JsonRpcRequest{Method: "lsps2.buy", Params: paramsBytes1, Id: "1"}
	resp1 := server.route(req1, []byte("peer"))
	assert.NotNil(t, resp1.Error)
	assert.Equal(t, 202, resp1.Error.Code)

	// Case 2: Payment Size Out of Bounds
	// This fails BEFORE liquidity check.
	buyParams2 := Lsps2BuyParams{
		OpeningFeeParams: Lsps2OpeningFeeParams{
			MinFeeMloki:          "0",
			Proportional:         0,
			ValidUntil:           validUntil,
			MinLifetime:          10080,
			MaxClientToSelfDelay: 2016,
			MinPaymentSizeMloki:  "1000",
			MaxPaymentSizeMloki:  "1000000",
			Promise:              "valid_promise",
		},
		PaymentSizeMloki: "500", // < MinPaymentSizeMloki
	}
	paramsBytes2, _ := json.Marshal(buyParams2)
	req2 := &JsonRpcRequest{Method: "lsps2.buy", Params: paramsBytes2, Id: "2"}
	resp2 := server.route(req2, []byte("peer"))
	assert.NotNil(t, resp2.Error)
	assert.Equal(t, 203, resp2.Error.Code)
}

func TestLsps1CreateOrder(t *testing.T) {
	// Setup
	mockLnd := new(MockLndClient)
	mockNodes := new(MockNodesService)
	mockOpening := new(MockOpeningService)
	mockLiquidity := new(MockLiquidityManager)
	mockNotification := new(MockNotificationService)

	server := NewServer(Config{Lsps1MaxPendingOrders: 5, Lsps5MaxWebhooks: 10}, mockLnd, mockNodes, mockOpening, mockLiquidity, mockNotification)

	// Test Data
	privKey, pubKey := btcec.PrivKeyFromBytes([]byte("testprivkey1111111111111111111"))
	node := &common.Node{
		PublicKey:  pubKey,
		PrivateKey: privKey,
		Tokens:     []string{"test-token"},
		NodeConfig: &config.NodeConfig{
			TimeLockDelta: 144,
		},
	}

	mockNodes.On("GetNodes").Return([]*common.Node{node})

	// Mock Fee Params Menu
	validUntil := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	fees := []*common.OpeningFeeParams{
		{
			MinFeeMloki:          1000,
			Proportional:         1000, // 1000 ppm = 0.1%
			ValidUntil:           validUntil,
			MinLifetime:          10080,
			MaxClientToSelfDelay: 2016,
			MinPaymentSizeMloki:  1000,
			MaxPaymentSizeMloki:  100000000,
			Promise:              "promise",
		},
	}
	mockOpening.On("GetFeeParamsMenu", "test-token", node.PrivateKey).Return(fees, nil)
	mockOpening.On("CountLsps1Orders", mock.Anything, "CREATED", mock.AnythingOfType("*time.Time")).Return(0, nil)

	// Mock Liquidity Check (Success)
	mockLiquidity.On("CheckOnChainLiquidity", mock.Anything, mock.Anything).Return(nil)

	// Mock AddInvoice
	mockLnd.On("AddInvoice", mock.Anything, mock.AnythingOfType("*lnrpc.Invoice")).Return(&lnrpc.AddInvoiceResponse{
		PaymentRequest: "lnbcrt1...",
	}, nil)

	// Mock Register Order
	mockOpening.On("RegisterLsps1Order", mock.AnythingOfType("*common.Lsps1Order")).Return(nil)

	// Execute
	params := Lsps1CreateOrderRequest{
		LspBalanceLoki:               "0",
		ClientBalanceLoki:            "100000",
		RequiredChannelConfirmations: 0,
		FundingConfirmsWithinBlocks:  6,
		ChannelExpiryBlocks:          2016,
		Token:                        "test-token",
		RefundOnchainAddress:         "",
		AnnounceChannel:              false,
	}
	paramsBytes, _ := json.Marshal(params)
	req := &JsonRpcRequest{Method: "lsps1.create_order", Params: paramsBytes, Id: "1"}

	resp := server.route(req, []byte("peer"))

	// Verify
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	result := resp.Result.(Lsps1CreateOrderResponse)
	assert.Equal(t, "CREATED", result.OrderState)
	assert.Equal(t, "EXPECT_PAYMENT", result.Payment.Bolt11.State)
	// Fee check: Client=100_000. Fee = Min(1) + 100_000*1000/1_000_000 = 1 + 100 = 101.
	// Wait, inputs are sat (loki). Params are mloki.
	// Fee calculation in handler:
	// capacity = 100_000
	// proportionalFee = 100_000 * 1000 / 1_000_000 = 100 loki.
	// minFeeLoki = 1000 / 1000 = 1 loki.
	// Total fee = 101.
	assert.Equal(t, "101", result.Payment.Bolt11.FeeTotalLoki)
	assert.Equal(t, "100101", result.Payment.Bolt11.OrderTotalLoki)

	mockOpening.AssertExpectations(t)
	mockLnd.AssertExpectations(t)
}
