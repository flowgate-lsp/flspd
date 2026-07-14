package lsps

import (
	"context"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flokiorg/go-flokicoin/chaincfg/chainhash"
	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLightningClient for testing
type MockLightningClient struct {
	mock.Mock
}

func (m *MockLightningClient) GetInfo() (*lightning.GetInfoResult, error) {
	args := m.Called()
	return args.Get(0).(*lightning.GetInfoResult), args.Error(1)
}

func (m *MockLightningClient) GetBlockHeight() (uint32, error) {
	args := m.Called()
	return args.Get(0).(uint32), args.Error(1)
}

func (m *MockLightningClient) ListPeers() ([]*lightning.Peer, error) {
	args := m.Called()
	return args.Get(0).([]*lightning.Peer), args.Error(1)
}

func (m *MockLightningClient) IsConnected(destination []byte) (bool, error) {

	args := m.Called(destination)
	return args.Bool(0), args.Error(1)
}

func (m *MockLightningClient) OpenChannel(req *lightning.OpenChannelRequest) (*wire.OutPoint, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*wire.OutPoint), args.Error(1)
}

func (m *MockLightningClient) GetChannel(peerID []byte, channelPoint wire.OutPoint) (*lightning.GetChannelResult, error) {
	args := m.Called(peerID, channelPoint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lightning.GetChannelResult), args.Error(1)
}

func (m *MockLightningClient) GetPeerId(scid *lightning.ShortChannelID) ([]byte, error) {
	args := m.Called(scid)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockLightningClient) GetClosedChannels(nodeID string, channelPoints map[string]uint64) (map[string]uint64, error) {
	args := m.Called(nodeID, channelPoints)
	return args.Get(0).(map[string]uint64), args.Error(1)
}

func (m *MockLightningClient) WaitOnline(peerID []byte, deadline time.Time) error {
	args := m.Called(peerID, deadline)
	return args.Error(0)
}

func (m *MockLightningClient) WaitChannelActive(peerID []byte, deadline time.Time) error {
	args := m.Called(peerID, deadline)
	return args.Error(0)
}

func (m *MockLightningClient) ListChannels() ([]*lightning.Channel, error) {
	args := m.Called()
	return args.Get(0).([]*lightning.Channel), args.Error(1)
}

func (m *MockLightningClient) SubscribeInvoices() (<-chan *lightning.InvoiceUpdate, <-chan error, error) {
	args := m.Called()
	return args.Get(0).(<-chan *lightning.InvoiceUpdate), args.Get(1).(<-chan error), args.Error(2)
}

func (m *MockLightningClient) SendCustomMessage(ctx context.Context, msg *lnrpc.SendCustomMessageRequest) (*lnrpc.SendCustomMessageResponse, error) {
	args := m.Called(ctx, msg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lnrpc.SendCustomMessageResponse), args.Error(1)
}

func (m *MockLightningClient) SubscribeCustomMessages(ctx context.Context, req *lnrpc.SubscribeCustomMessagesRequest) (lnrpc.Lightning_SubscribeCustomMessagesClient, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(lnrpc.Lightning_SubscribeCustomMessagesClient), args.Error(1)
}

func (m *MockLightningClient) GetNodeInfo(pubkey string) (*lightning.NodeInfo, error) {
	args := m.Called(pubkey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lightning.NodeInfo), args.Error(1)
}

func (m *MockLightningClient) LookupInvoiceState(ctx context.Context, paymentRequest string) (lightning.InvoiceState, error) {
	args := m.Called(ctx, paymentRequest)
	return args.Get(0).(lightning.InvoiceState), args.Error(1)
}

// MockOpeningStore for testing
type MockOpeningStore struct {
	mock.Mock
}

func (m *MockOpeningStore) GetFeeParamsSettings(token string) ([]*common.OpeningFeeParamsSetting, error) {
	args := m.Called(token)
	return args.Get(0).([]*common.OpeningFeeParamsSetting), args.Error(1)
}

func (m *MockOpeningStore) SetFeeParams(token string, validity time.Duration, params common.OpeningFeeParams) error {
	args := m.Called(token, validity, params)
	return args.Error(0)
}

func (m *MockOpeningStore) RegisterLsps2Buy(reg *common.Lsps2BuyRegistration) error {
	args := m.Called(reg)
	return args.Error(0)
}

func (m *MockOpeningStore) GetLsps2BuyRegistration(scid uint64) (*common.Lsps2BuyRegistration, error) {
	args := m.Called(scid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.Lsps2BuyRegistration), args.Error(1)
}

func (m *MockOpeningStore) RegisterLsps1Order(order *common.Lsps1Order) error {
	args := m.Called(order)
	return args.Error(0)
}

func (m *MockOpeningStore) GetLsps1Order(orderId string) (*common.Lsps1Order, error) {
	args := m.Called(orderId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) GetLsps1OrderByPaymentInvoice(paymentInvoice string) (*common.Lsps1Order, error) {
	args := m.Called(paymentInvoice)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) UpdateLsps1OrderState(orderId string, newState string, newPaymentState string, expectedCurrentState string) (bool, error) {
	args := m.Called(orderId, newState, newPaymentState, expectedCurrentState)
	return args.Bool(0), args.Error(1)
}

func (m *MockOpeningStore) RegisterLsps5Webhook(webhook *common.Lsps5Webhook) error {
	args := m.Called(webhook)
	return args.Error(0)
}

func (m *MockOpeningStore) GetLsps5Webhooks(clientNodeId string) ([]*common.Lsps5Webhook, error) {
	args := m.Called(clientNodeId)
	return args.Get(0).([]*common.Lsps5Webhook), args.Error(1)
}

func (m *MockOpeningStore) RemoveLsps5Webhook(clientNodeId, appName string) error {
	args := m.Called(clientNodeId, appName)
	return args.Error(0)
}

func (m *MockOpeningStore) CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error) {
	args := m.Called(clientNodeId, state, excludeExpiredBefore)
	return args.Int(0), args.Error(1)
}

func (m *MockOpeningStore) CountLsps5Webhooks(clientNodeId string) (int, error) {
	args := m.Called(clientNodeId)
	return args.Int(0), args.Error(1)
}

func (m *MockOpeningStore) GetPaidLsps1Order(clientNodeId string) (*common.Lsps1Order, error) {
	args := m.Called(clientNodeId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) GetPaidLsps1Orders(clientNodeId string) ([]*common.Lsps1Order, error) {
	args := m.Called(clientNodeId)
	return args.Get(0).([]*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) GetAllPaidLsps1Orders() ([]*common.Lsps1Order, error) {
	args := m.Called()
	return args.Get(0).([]*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) ListLsps1Orders(limit, offset int) ([]*common.Lsps1Order, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) GetAllCreatedLsps1Orders() ([]*common.Lsps1Order, error) {
	args := m.Called()
	return args.Get(0).([]*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) UpdateLsps1OrderFunding(orderId string, fundingTxid string, fundingOutpoint int) error {
	args := m.Called(orderId, fundingTxid, fundingOutpoint)
	return args.Error(0)
}


func (m *MockOpeningStore) GetFeeParamsMenu(token string, privateKey *btcec.PrivateKey) ([]*common.OpeningFeeParams, error) {
	args := m.Called(token, privateKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*common.OpeningFeeParams), args.Error(1)
}

func (m *MockOpeningStore) ValidateOpeningFeeParams(params *common.OpeningFeeParams, publicKey *btcec.PublicKey) bool {
	args := m.Called(params, publicKey)
	return args.Bool(0)
}

func (m *MockOpeningStore) IsCurrentChainFeeCheaper(token string, params *common.OpeningFeeParams) bool {
	args := m.Called(token, params)
	return args.Bool(0)
}

func (m *MockOpeningStore) GetReservedLiquidity() (uint64, error) {
	args := m.Called()
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockOpeningStore) ExpireStaleOrders() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockLightningClient) WalletBalance(ctx context.Context) (*lightning.WalletBalanceResult, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lightning.WalletBalanceResult), args.Error(1)
}

func (m *MockLightningClient) QueryRoutes(destination []byte, amountMloki uint64, outgoingChanId uint64) ([]*lightning.Route, error) {
	args := m.Called(destination, amountMloki)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*lightning.Route), args.Error(1)
}

// MockHistoryStore for testing
type MockHistoryStore struct {
	mock.Mock
}

func (m *MockHistoryStore) UpdateChannels(ctx context.Context, updates []*history.ChannelUpdate) error {
	args := m.Called(ctx, updates)
	return args.Error(0)
}

func (m *MockHistoryStore) InsertForwards(ctx context.Context, forwards []*history.Forward, nodeId []byte) error {
	args := m.Called(ctx, forwards, nodeId)
	return args.Error(0)
}

func (m *MockHistoryStore) UpdateForwards(ctx context.Context, forwards []*history.Forward, nodeId []byte) error {
	args := m.Called(ctx, forwards, nodeId)
	return args.Error(0)
}

func (m *MockHistoryStore) FetchLndForwardOffset(ctx context.Context, nodeId []byte) (*time.Time, error) {
	args := m.Called(ctx, nodeId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockHistoryStore) AddOpenChannelHtlc(ctx context.Context, htlc *history.OpenChannelHtlc) error {
	args := m.Called(ctx, htlc)
	return args.Error(0)
}

// MockFeeEstimator for testing
type MockFeeEstimator struct {
	mock.Mock
}

func (m *MockFeeEstimator) EstimateFeeRate(ctx context.Context, strategy chain.FeeStrategy) (*chain.FeeEstimation, error) {
	args := m.Called(ctx, strategy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*chain.FeeEstimation), args.Error(1)
}

// TestProcessSettledInvoice_Success tests happy path invoice settlement
func TestProcessSettledInvoice_Success(t *testing.T) {
	mockClient := new(MockLightningClient)
	mockStore := new(MockOpeningStore)

	invoice := "lnbc1test..."
	order := &common.Lsps1Order{
		OrderId:           "test-order-123",
		PeerId:            "deadbeef",
		OrderState:        "CREATED",
		PaymentState:      "CREATED",
		ClientBalanceLoki: 1000000,
	}

	mockStore.On("GetLsps1OrderByPaymentInvoice", invoice).Return(order, nil)
	mockStore.On("UpdateLsps1OrderState", "test-order-123", "PAID", "PAID", "CREATED").Return(true, nil)

	// Mock connectivity check to return false, so we stop after PAID notification
	// "deadbeef" decoded is [222 173 190 239]
	peerIdBytes, _ := hex.DecodeString("deadbeef")
	mockClient.On("IsConnected", peerIdBytes).Return(false, nil)

	historyStore := new(MockHistoryStore)
	feeEstimator := new(MockFeeEstimator)
	mockNotification := new(MockNotificationService)
	conf := &config.NodeConfig{}

	svc := NewLsps1SettlementService(mockClient, mockStore, historyStore, conf, feeEstimator, chain.FeeStrategyEconomy, mockNotification)

	// Expect notification
	mockNotification.On("NotifyOrderStateChanged", "deadbeef", "test-order-123", "PAID", "", "").Return(true, nil)

	update := &lightning.InvoiceUpdate{
		State:          lightning.InvoiceStateSettled,
		PaymentRequest: invoice,
		AmountMloki:    1000000,
	}

	svc.processSettledInvoice(update)

	// Wait for goroutines (notification is async)
	time.Sleep(50 * time.Millisecond)

	mockStore.AssertExpectations(t)
	mockNotification.AssertExpectations(t)
}

func (m *MockLightningClient) AddInvoice(ctx context.Context, invoice *lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error) {
	args := m.Called(ctx, invoice)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lnrpc.AddInvoiceResponse), args.Error(1)
}

// TestProcessSettledInvoice_OrderNotFound tests non-LSPS1 invoice
func TestProcessSettledInvoice_OrderNotFound(t *testing.T) {
	mockClient := new(MockLightningClient)
	mockStore := new(MockOpeningStore)

	invoice := "lnbc1test..."

	mockStore.On("GetLsps1OrderByPaymentInvoice", invoice).Return(nil, assert.AnError)

	historyStore := new(MockHistoryStore)
	feeEstimator := new(MockFeeEstimator)
	conf := &config.NodeConfig{}

	svc := NewLsps1SettlementService(mockClient, mockStore, historyStore, conf, feeEstimator, chain.FeeStrategyEconomy, nil)

	update := &lightning.InvoiceUpdate{
		State:          lightning.InvoiceStateSettled,
		PaymentRequest: invoice,
	}

	svc.processSettledInvoice(update)

	// Should not call UpdateLsps1OrderState
	mockStore.AssertNotCalled(t, "UpdateLsps1OrderState", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestProcessSettledInvoice_TriggerChannelOpen tests that channel opening is triggered
func TestProcessSettledInvoice_TriggerChannelOpen(t *testing.T) {
	mockClient := new(MockLightningClient)
	mockStore := new(MockOpeningStore)
	historyStore := new(MockHistoryStore)
	feeEstimator := new(MockFeeEstimator)
	conf := &config.NodeConfig{
		FeeRate:       0.000001,
		BaseFeeMloki:  1000,
		TimeLockDelta: 40,
	}

	invoice := "lnbc1test..."
	order := &common.Lsps1Order{
		OrderId:           "test-order-123",
		PeerId:            "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f", // Valid hex for peer ID
		OrderState:        "CREATED",
		PaymentState:      "CREATED",
		ClientBalanceLoki: 1000000,
		LspBalanceLoki:    5000000,
		AnnounceChannel:   false,
	}

	peerId, _ := hex.DecodeString(order.PeerId)

	mockStore.On("GetLsps1OrderByPaymentInvoice", invoice).Return(order, nil)
	mockStore.On("UpdateLsps1OrderState", "test-order-123", "PAID", "PAID", "CREATED").Return(true, nil)

	// Connectivity check
	mockClient.On("IsConnected", peerId).Return(true, nil)

	// Fee estimation
	feeEst := &chain.FeeEstimation{
		LokiPerVByte: func() *float64 { f := 1.0; return &f }(),
	}
	feeEstimator.On("EstimateFeeRate", mock.Anything, chain.FeeStrategyEconomy).Return(feeEst, nil)

	// Channel opening
	chanPoint := &wire.OutPoint{
		Hash: func() chainhash.Hash {
			h, _ := chainhash.NewHashFromStr("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
			return *h
		}(),
		Index: 0,
	}
	mockClient.On("OpenChannel", mock.MatchedBy(func(req *lightning.OpenChannelRequest) bool {
		return req.CapacityLoki == 6000000 && req.PushLoki == 1000000
	})).Return(chanPoint, nil)

	// Store updates
	mockStore.On("UpdateLsps1OrderFunding", "test-order-123", chanPoint.Hash.String(), 0).Return(nil)
	mockStore.On("UpdateLsps1OrderState", "test-order-123", "COMPLETED", "PAID", "PAID").Return(true, nil)

	mockNotification := new(MockNotificationService)

	// Expect notifications
	// 1. PAID
	mockNotification.On("NotifyOrderStateChanged", order.PeerId, "test-order-123", "PAID", "", "").Return(true, nil)
	// 2. COMPLETED
	mockNotification.On("NotifyOrderStateChanged", order.PeerId, "test-order-123", "COMPLETED", chanPoint.String(), "").Return(true, nil)

	svc := NewLsps1SettlementService(mockClient, mockStore, historyStore, conf, feeEstimator, chain.FeeStrategyEconomy, mockNotification)

	update := &lightning.InvoiceUpdate{
		State:          lightning.InvoiceStateSettled,
		PaymentRequest: invoice,
		AmountMloki:    1000000,
	}

	svc.processSettledInvoice(update)

	// Wait for goroutine to finish (simple sleep for now as it's a unit test)
	time.Sleep(100 * time.Millisecond)

	mockStore.AssertExpectations(t)
	mockClient.AssertExpectations(t)
	mockNotification.AssertExpectations(t)
}

// TestProcessSettledInvoice_AlreadyPaid tests duplicate settlement
func TestProcessSettledInvoice_AlreadyPaid(t *testing.T) {
	mockClient := new(MockLightningClient)
	mockStore := new(MockOpeningStore)

	invoice := "lnbc1test..."
	order := &common.Lsps1Order{
		OrderId:      "test-order-123",
		OrderState:   "PAID", // Already paid
		PaymentState: "PAID",
	}

	mockStore.On("GetLsps1OrderByPaymentInvoice", invoice).Return(order, nil)

	historyStore := new(MockHistoryStore)
	feeEstimator := new(MockFeeEstimator)
	conf := &config.NodeConfig{}

	svc := NewLsps1SettlementService(mockClient, mockStore, historyStore, conf, feeEstimator, chain.FeeStrategyEconomy, nil)

	update := &lightning.InvoiceUpdate{
		State:          lightning.InvoiceStateSettled,
		PaymentRequest: invoice,
	}

	svc.processSettledInvoice(update)

	// Should not call UpdateLsps1OrderState for already paid order
	mockStore.AssertNotCalled(t, "UpdateLsps1OrderState", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestGracefulShutdown tests context cancellation
func TestGracefulShutdown(t *testing.T) {
	mockClient := new(MockLightningClient)
	mockStore := new(MockOpeningStore)

	invoiceChan := make(chan *lightning.InvoiceUpdate, 1)
	errChan := make(chan error, 1)

	mockClient.On("SubscribeInvoices").Return((<-chan *lightning.InvoiceUpdate)(invoiceChan), (<-chan error)(errChan), nil)
	mockStore.On("GetAllPaidLsps1Orders").Return([]*common.Lsps1Order{}, nil)
	mockStore.On("GetAllCreatedLsps1Orders").Return([]*common.Lsps1Order{}, nil)

	historyStore := new(MockHistoryStore)
	feeEstimator := new(MockFeeEstimator)
	conf := &config.NodeConfig{}

	svc := NewLsps1SettlementService(mockClient, mockStore, historyStore, conf, feeEstimator, chain.FeeStrategyEconomy, nil)

	ctx, cancel := context.WithCancel(context.Background())
	err := svc.Start(ctx)

	assert.NoError(t, err)

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Cancel and stop
	cancel()
	svc.Stop()

	// Give it a moment to shutdown
	time.Sleep(100 * time.Millisecond)

	mockClient.AssertExpectations(t)
}

// TestLsps1SettlementService_StartupRecovery tests that PAID but unfulfilled orders are processed on startup
func TestLsps1SettlementService_StartupRecovery(t *testing.T) {
	mockClient := new(MockLightningClient)
	mockStore := new(MockOpeningStore)
	historyStore := new(MockHistoryStore)
	feeEstimator := new(MockFeeEstimator)
	mockNotification := new(MockNotificationService)
	conf := &config.NodeConfig{}

	peerPubkey := "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f"
	peerIdBytes, _ := hex.DecodeString(peerPubkey)

	staleOrder := &common.Lsps1Order{
		OrderId:           "stale-order-001",
		PeerId:            peerPubkey,
		OrderState:        "PAID",
		PaymentState:      "PAID",
		ClientBalanceLoki: 1000000,
		LspBalanceLoki:    5000000,
	}

	// 1. Return the stale order during recovery check; no CREATED orders with settled invoices
	mockStore.On("GetAllPaidLsps1Orders").Return([]*common.Lsps1Order{staleOrder}, nil)
	mockStore.On("GetAllCreatedLsps1Orders").Return([]*common.Lsps1Order{}, nil)

	// 2. Mock the channel opening process
	mockClient.On("IsConnected", peerIdBytes).Return(true, nil)
	feeEst := &chain.FeeEstimation{LokiPerVByte: func() *float64 { f := 1.0; return &f }()}
	feeEstimator.On("EstimateFeeRate", mock.Anything, chain.FeeStrategyEconomy).Return(feeEst, nil)

	chanPoint := &wire.OutPoint{
		Hash:  chainhash.Hash{0xde, 0xad, 0xbe, 0xef},
		Index: 0,
	}
	mockClient.On("OpenChannel", mock.Anything).Return(chanPoint, nil)
	mockStore.On("UpdateLsps1OrderFunding", "stale-order-001", chanPoint.Hash.String(), 0).Return(nil)
	mockStore.On("UpdateLsps1OrderState", "stale-order-001", "COMPLETED", "PAID", "PAID").Return(true, nil)
	mockNotification.On("NotifyOrderStateChanged", peerPubkey, "stale-order-001", "COMPLETED", chanPoint.String(), "").Return(true, nil)

	invoiceChan := make(chan *lightning.InvoiceUpdate, 1)
	errChan := make(chan error, 1)

	mockClient.On("SubscribeInvoices").Return((<-chan *lightning.InvoiceUpdate)(invoiceChan), (<-chan error)(errChan), nil)

	svc := NewLsps1SettlementService(mockClient, mockStore, historyStore, conf, feeEstimator, chain.FeeStrategyEconomy, mockNotification)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := svc.Start(ctx)
	assert.NoError(t, err)

	// Wait for startup recovery goroutine
	time.Sleep(200 * time.Millisecond)

	svc.Stop()

	mockStore.AssertCalled(t, "GetAllPaidLsps1Orders")
	mockStore.AssertCalled(t, "UpdateLsps1OrderState", "stale-order-001", "COMPLETED", "PAID", "PAID")
	mockNotification.AssertExpectations(t)
}

// raceTestStore wraps MockOpeningStore but implements UpdateLsps1OrderState
// with real, mutex-protected compare-and-set semantics instead of a canned
// mock response, so two goroutines racing for the same order genuinely
// only let one through — proving the F-1 fix holds at the service layer,
// not just that a mock was configured a certain way.
type raceTestStore struct {
	*MockOpeningStore
	mu    sync.Mutex
	state map[string]string // orderId -> current order_state
}

func newRaceTestStore(orderId, initialState string) *raceTestStore {
	return &raceTestStore{
		MockOpeningStore: new(MockOpeningStore),
		state:            map[string]string{orderId: initialState},
	}
}

func (s *raceTestStore) UpdateLsps1OrderState(orderId, newState, newPaymentState, expectedCurrentState string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state[orderId] != expectedCurrentState {
		return false, nil
	}
	s.state[orderId] = newState
	return true, nil
}

// TestF1_ConcurrentSettlementAndRecoveryOpenExactlyOneChannel reproduces
// the F-1 race: a live SubscribeInvoices settlement event and the startup
// recovery sweep can both observe the same order as CREATED and race to
// mark it PAID. Before the fix, both paths would call openChannelForOrder,
// double-opening a channel. After the fix, only the CAS winner proceeds.
func TestF1_ConcurrentSettlementAndRecoveryOpenExactlyOneChannel(t *testing.T) {
	const orderId = "race-order-1"
	const invoice = "lnbc1race..."
	const peerId = "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f"

	mockStore := newRaceTestStore(orderId, "CREATED")
	// Both racing paths look up the SAME order (with OrderState still
	// CREATED in the returned struct, matching what a real DB read would
	// return before either writer has moved it) — this is the stale read
	// that made the race possible pre-fix.
	order := &common.Lsps1Order{
		OrderId:           orderId,
		PeerId:            peerId,
		OrderState:        "CREATED",
		PaymentState:      "CREATED",
		ClientBalanceLoki: 1000000,
		LspBalanceLoki:    5000000,
	}
	mockStore.On("GetLsps1OrderByPaymentInvoice", invoice).Return(order, nil)

	peerIdBytes, _ := hex.DecodeString(peerId)
	mockClient := new(MockLightningClient)
	mockClient.On("IsConnected", peerIdBytes).Return(true, nil)

	feeEstimator := new(MockFeeEstimator)
	f := 1.0
	feeEstimator.On("EstimateFeeRate", mock.Anything, mock.Anything).Return(&chain.FeeEstimation{LokiPerVByte: &f}, nil)

	chanPoint := &wire.OutPoint{Hash: chainhash.Hash{0xf1}, Index: 0}
	mockClient.On("OpenChannel", mock.Anything).Return(chanPoint, nil)
	mockStore.On("UpdateLsps1OrderFunding", orderId, mock.Anything, mock.Anything).Return(nil)

	historyStore := new(MockHistoryStore)
	conf := &config.NodeConfig{FeeRate: 0.000001, BaseFeeMloki: 1000, TimeLockDelta: 40}

	svc := NewLsps1SettlementService(mockClient, mockStore, historyStore, conf, feeEstimator, chain.FeeStrategyEconomy, nil)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		// The live settlement-event path.
		svc.processSettledInvoice(&lightning.InvoiceUpdate{
			State:          lightning.InvoiceStateSettled,
			PaymentRequest: invoice,
			AmountMloki:    1000000,
		})
	}()
	go func() {
		defer wg.Done()
		// The startup-recovery path, racing for the same order.
		svc.processSettledOrder(order)
	}()
	wg.Wait()

	// Give the winner's async openChannelForOrder goroutine time to finish.
	time.Sleep(100 * time.Millisecond)

	mockClient.AssertNumberOfCalls(t, "OpenChannel", 1)

	mockStore.mu.Lock()
	finalState := mockStore.state[orderId]
	mockStore.mu.Unlock()
	assert.Equal(t, "COMPLETED", finalState, "order should end up COMPLETED via exactly one winner, not stuck or double-processed")
}

