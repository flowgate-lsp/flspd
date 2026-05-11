package dashboard

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/flokiorg/flnd/lnrpc"
	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/lsps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock definitions
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

func (m *MockNodesService) GetMinChannelCapacity() uint64 {
	args := m.Called()
	return args.Get(0).(uint64)
}

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
	return m.Called(reg).Error(0)
}

func (m *MockOpeningStore) GetLsps2BuyRegistration(scid uint64) (*common.Lsps2BuyRegistration, error) {
	args := m.Called(scid)
	return args.Get(0).(*common.Lsps2BuyRegistration), args.Error(1)
}

func (m *MockOpeningStore) RegisterLsps1Order(order *common.Lsps1Order) error {
	return m.Called(order).Error(0)
}

func (m *MockOpeningStore) GetLsps1Order(orderId string) (*common.Lsps1Order, error) {
	args := m.Called(orderId)
	return args.Get(0).(*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) GetLsps1OrderByPaymentInvoice(paymentInvoice string) (*common.Lsps1Order, error) {
	args := m.Called(paymentInvoice)
	return args.Get(0).(*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) GetPaidLsps1Order(clientNodeId string) (*common.Lsps1Order, error) {
	args := m.Called(clientNodeId)
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

func (m *MockOpeningStore) UpdateLsps1OrderState(orderId string, newState string, newPaymentState string) error {
	return m.Called(orderId, newState, newPaymentState).Error(0)
}

func (m *MockOpeningStore) UpdateLsps1OrderFunding(orderId string, fundingTxid string, fundingOutpoint int) error {
	return m.Called(orderId, fundingTxid, fundingOutpoint).Error(0)
}

func (m *MockOpeningStore) RegisterLsps5Webhook(webhook *common.Lsps5Webhook) error {
	return m.Called(webhook).Error(0)
}

func (m *MockOpeningStore) GetLsps5Webhooks(clientNodeId string) ([]*common.Lsps5Webhook, error) {
	args := m.Called(clientNodeId)
	return args.Get(0).([]*common.Lsps5Webhook), args.Error(1)
}

func (m *MockOpeningStore) RemoveLsps5Webhook(clientNodeId, appName string) error {
	return m.Called(clientNodeId, appName).Error(0)
}

func (m *MockOpeningStore) CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error) {
	args := m.Called(clientNodeId, state, excludeExpiredBefore)
	return args.Int(0), args.Error(1)
}

func (m *MockOpeningStore) CountLsps5Webhooks(clientNodeId string) (int, error) {
	args := m.Called(clientNodeId)
	return args.Int(0), args.Error(1)
}

func (m *MockOpeningStore) GetReservedLiquidity() (uint64, error) {
	args := m.Called()
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockOpeningStore) ExpireStaleOrders() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockOpeningStore) ListLsps1Orders(limit, offset int) ([]*common.Lsps1Order, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningStore) GetAllCreatedLsps1Orders() ([]*common.Lsps1Order, error) {
	args := m.Called()
	return args.Get(0).([]*common.Lsps1Order), args.Error(1)
}

type MockOpeningService struct {
	mock.Mock
}

func (m *MockOpeningService) GetFeeParamsMenu(token string, privateKey *btcec.PrivateKey) ([]*common.OpeningFeeParams, error) {
	args := m.Called(token, privateKey)
	return args.Get(0).([]*common.OpeningFeeParams), args.Error(1)
}

func (m *MockOpeningService) ValidateOpeningFeeParams(params *common.OpeningFeeParams, publicKey *btcec.PublicKey) bool {
	return m.Called(params, publicKey).Bool(0)
}

func (m *MockOpeningService) IsCurrentChainFeeCheaper(token string, params *common.OpeningFeeParams) bool {
	return m.Called(token, params).Bool(0)
}

func (m *MockOpeningService) RegisterLsps2Buy(reg *common.Lsps2BuyRegistration) error {
	return m.Called(reg).Error(0)
}

func (m *MockOpeningService) RegisterLsps1Order(order *common.Lsps1Order) error {
	return m.Called(order).Error(0)
}

func (m *MockOpeningService) GetLsps1Order(orderId string) (*common.Lsps1Order, error) {
	args := m.Called(orderId)
	return args.Get(0).(*common.Lsps1Order), args.Error(1)
}

func (m *MockOpeningService) RegisterLsps5Webhook(webhook *common.Lsps5Webhook) error {
	return m.Called(webhook).Error(0)
}

func (m *MockOpeningService) GetLsps5Webhooks(clientNodeId string) ([]*common.Lsps5Webhook, error) {
	args := m.Called(clientNodeId)
	return args.Get(0).([]*common.Lsps5Webhook), args.Error(1)
}

func (m *MockOpeningService) RemoveLsps5Webhook(clientNodeId, appName string) error {
	return m.Called(clientNodeId, appName).Error(0)
}

func (m *MockOpeningService) CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error) {
	return m.Called(clientNodeId, state, excludeExpiredBefore).Int(0), m.Called(clientNodeId, state, excludeExpiredBefore).Error(1)
}

func (m *MockOpeningService) CountLsps5Webhooks(clientNodeId string) (int, error) {
	return m.Called(clientNodeId).Int(0), m.Called(clientNodeId).Error(1)
}

type MockHistoryStore struct {
	mock.Mock
}

func (m *MockHistoryStore) UpdateChannels(ctx context.Context, updates []*history.ChannelUpdate) error {
	return m.Called(ctx, updates).Error(0)
}

func (m *MockHistoryStore) InsertForwards(ctx context.Context, forwards []*history.Forward, nodeId []byte) error {
	return m.Called(ctx, forwards, nodeId).Error(0)
}

func (m *MockHistoryStore) UpdateForwards(ctx context.Context, forwards []*history.Forward, nodeId []byte) error {
	return m.Called(ctx, forwards, nodeId).Error(0)
}

func (m *MockHistoryStore) FetchLndForwardOffset(ctx context.Context, nodeId []byte) (*time.Time, error) {
	args := m.Called(ctx, nodeId)
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockHistoryStore) AddOpenChannelHtlc(ctx context.Context, htlc *history.OpenChannelHtlc) error {
	return m.Called(ctx, htlc).Error(0)
}

type MockLightningClient struct {
	mock.Mock
}

func (m *MockLightningClient) GetInfo() (*lightning.GetInfoResult, error) {
	args := m.Called()
	return args.Get(0).(*lightning.GetInfoResult), args.Error(1)
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
	return args.Get(0).(*wire.OutPoint), args.Error(1)
}

func (m *MockLightningClient) GetChannel(peerID []byte, channelPoint wire.OutPoint) (*lightning.GetChannelResult, error) {
	args := m.Called(peerID, channelPoint)
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
	return m.Called(peerID, deadline).Error(0)
}

func (m *MockLightningClient) WaitChannelActive(peerID []byte, deadline time.Time) error {
	return m.Called(peerID, deadline).Error(0)
}

func (m *MockLightningClient) ListChannels() ([]*lightning.Channel, error) {
	args := m.Called()
	return args.Get(0).([]*lightning.Channel), args.Error(1)
}

func (m *MockLightningClient) SubscribeInvoices() (<-chan *lightning.InvoiceUpdate, <-chan error, error) {
	args := m.Called()
	return args.Get(0).(<-chan *lightning.InvoiceUpdate), args.Get(1).(<-chan error), args.Error(2)
}

func (m *MockLightningClient) WalletBalance(ctx context.Context) (*lightning.WalletBalanceResult, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lightning.WalletBalanceResult), args.Error(1)
}

func (m *MockLightningClient) GetBlockHeight() (uint32, error) {
	args := m.Called()
	return args.Get(0).(uint32), args.Error(1)
}

func (m *MockLightningClient) QueryRoutes(destination []byte, amountMloki uint64, outgoingChanId uint64) ([]*lightning.Route, error) {
	args := m.Called(destination, amountMloki, outgoingChanId)
	return args.Get(0).([]*lightning.Route), args.Error(1)
}

func (m *MockLightningClient) GetNodeInfo(pubkey string) (*lightning.NodeInfo, error) {
	args := m.Called(pubkey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lightning.NodeInfo), args.Error(1)
}

func (m *MockLightningClient) SubscribePeerOnline(ctx context.Context) (<-chan string, error) {
	args := m.Called(ctx)
	return args.Get(0).(<-chan string), args.Error(1)
}

func (m *MockLightningClient) AddInvoice(ctx context.Context, invoice *lnrpc.Invoice) (*lnrpc.AddInvoiceResponse, error) {
	args := m.Called(ctx, invoice)
	return args.Get(0).(*lnrpc.AddInvoiceResponse), args.Error(1)
}

func (m *MockLightningClient) LookupInvoiceState(ctx context.Context, paymentRequest string) (lightning.InvoiceState, error) {
	args := m.Called(ctx, paymentRequest)
	return args.Get(0).(lightning.InvoiceState), args.Error(1)
}

func TestDashboardService_RPC(t *testing.T) {
	mockNodes := new(MockNodesService)
	mockOpeningSvc := new(MockOpeningService)
	mockStore := new(MockOpeningStore)
	mockHistory := new(MockHistoryStore)
	mockLnd := new(MockLightningClient)

	// Setup a test node
	testNode := &common.Node{
		NodeId: []byte("testnode"),
		Client: mockLnd,
		NodeConfig: &config.NodeConfig{
			Name:       "Test Node",
			NodePubkey: "02abc...",
		},
	}

	mockNodes.On("GetNodes").Return([]*common.Node{testNode})
	mockStore.On("GetReservedLiquidity").Return(uint64(10000), nil)

	s := NewDashboardService(mockNodes, mockOpeningSvc, mockStore, mockHistory, "pubkey_test", "/tmp/nodes.test.yaml")

	t.Run("get_summary", func(t *testing.T) {
		req := &lsps.JsonRpcRequest{
			Id:     json.RawMessage(`1`),
			Method: "get_summary",
		}
		resp := s.handleGetSummary(req)
		assert.Nil(t, resp.Error)
		
		result := resp.Result.(SummaryResponse)
		assert.Equal(t, "Test Node", result.NodeName)
		assert.Equal(t, uint64(10000), result.LiquidityBlocked)
	})

	t.Run("list_peers", func(t *testing.T) {
		mockLnd.On("ListPeers").Return([]*lightning.Peer{
			{Pubkey: "peer1", Active: true},
		}, nil)

		req := &lsps.JsonRpcRequest{
			Id:     json.RawMessage(`2`),
			Method: "list_peers",
		}
		resp := s.handleListPeers(req)
		assert.Nil(t, resp.Error)
		
		peers := resp.Result.([]interface{})
		assert.Equal(t, 1, len(peers))
	})

	t.Run("list_orders", func(t *testing.T) {
		mockStore.On("ListLsps1Orders", 100, 0).Return([]*common.Lsps1Order{
			{OrderId: "order1", OrderState: "CREATED"},
		}, nil)

		req := &lsps.JsonRpcRequest{
			Id:     json.RawMessage(`3`),
			Method: "list_orders",
		}
		resp := s.handleListOrders(req)
		assert.Nil(t, resp.Error)
		
		orders := resp.Result.([]*common.Lsps1Order)
		assert.Equal(t, 1, len(orders))
	})
	
	t.Run("get_config", func(t *testing.T) {
		// Mock YAML file for config
		configPath := "/tmp/nodes.test.yaml"
		err := os.WriteFile(configPath, []byte("nodes:\n  - name: Test Node\n"), 0644)
		assert.NoError(t, err)
		defer os.Remove(configPath)
		
		req := &lsps.JsonRpcRequest{
			Id:     json.RawMessage(`4`),
			Method: "get_config",
		}
		resp := s.handleGetConfig(req)
		assert.Nil(t, resp.Error)
	})
}
