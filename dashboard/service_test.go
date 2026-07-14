package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
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
	"gopkg.in/yaml.v3"
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

func (m *MockOpeningStore) UpdateLsps1OrderState(orderId string, newState string, newPaymentState string, expectedCurrentState string) (bool, error) {
	args := m.Called(orderId, newState, newPaymentState, expectedCurrentState)
	return args.Bool(0), args.Error(1)
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
		NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
			Name:       "Test Node",
			NodePubkey: "02abc...",
		}),
	}

	mockNodes.On("GetNodes").Return([]*common.Node{testNode})
	mockStore.On("GetReservedLiquidity").Return(uint64(10000), nil)

	s := NewDashboardService(mockNodes, mockOpeningSvc, mockStore, mockHistory, "pubkey_test", "/tmp/nodes.test.yaml")

	t.Run("get_summary", func(t *testing.T) {
		// Scoped with .Once() so these don't leak into later subtests that
		// register their own, more specific expectations for the same
		// mocked methods (list_peers, list_orders below).
		mockLnd.On("WalletBalance", mock.Anything).Return(&lightning.WalletBalanceResult{
			ConfirmedBalance: 1000000,
		}, nil).Once()
		// Matches the specific (summaryListLimit, 0) args handleGetSummary
		// and handleListPeers both call with — not scoped with .Once()
		// since handleListPeers (next subtest) calls it too. Distinct from
		// list_orders' own (defaultListLimit, 0) expectation below, since
		// summaryListLimit != defaultListLimit.
		mockStore.On("ListLsps1Orders", 1000, 0).Return([]*common.Lsps1Order{}, nil)
		mockLnd.On("GetBlockHeight").Return(uint32(100), nil).Once()
		mockLnd.On("ListPeers").Return([]*lightning.Peer{}, nil).Once()

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
		// order1's PeerId ("") doesn't match any peer from ListPeers, so
		// handleListOrders falls back to GetNodeInfo to enrich it.
		mockLnd.On("GetNodeInfo", "").Return(nil, fmt.Errorf("not found"))

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

// TestHandleGetConfig_StripsSecrets is the regression test for C-1:
// get_config used to return the full NodeConfig, including
// flspdPrivateKey, tokens (client auth tokens), and the flnd block
// (macaroon + TLS cert), to any authenticated dashboard client.
func TestHandleGetConfig_StripsSecrets(t *testing.T) {
	mockNodes := new(MockNodesService)
	node := &common.Node{
		NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
			Name:            "Secret Node",
			NodePubkey:      "02abc...",
			FlspdPrivateKey: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			Tokens:          []string{"super-secret-client-token"},
			Lnd: &config.LndConfig{
				Address:  "flnd:10005",
				Macaroon: "0102030405",
				Cert:     "-----BEGIN CERTIFICATE-----",
			},
		}),
	}
	mockNodes.On("GetNodes").Return([]*common.Node{node})

	s := NewDashboardService(mockNodes, nil, nil, nil, "pubkey_test", "")

	req := &lsps.JsonRpcRequest{Id: json.RawMessage(`1`), Method: "get_config"}
	resp := s.handleGetConfig(req)
	assert.Nil(t, resp.Error)

	// Round-trip through JSON the same way a real RPC response would, so
	// this test catches the bug even if a future change returns the
	// secrets under a different Go field but the same JSON shape.
	raw, err := json.Marshal(resp.Result)
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "deadbeefdeadbeef", "flspdPrivateKey leaked into the response")
	assert.NotContains(t, string(raw), "super-secret-client-token", "tokens leaked into the response")
	assert.NotContains(t, string(raw), "0102030405", "macaroon leaked into the response")
	assert.NotContains(t, string(raw), "BEGIN CERTIFICATE", "TLS cert leaked into the response")

	// And the safe fields should still be present — this isn't just a
	// bug fix, it still needs to be useful.
	assert.Contains(t, string(raw), "Secret Node")
	assert.Contains(t, string(raw), "02abc")
}

// TestHandleUpdateConfig_PreservesOtherNodes is the regression test for
// C-2: update_config used to marshal only the single edited node back to
// nodes.yaml, silently dropping every other configured node on a
// multi-node deployment.
func TestHandleUpdateConfig_PreservesOtherNodes(t *testing.T) {
	mockNodes := new(MockNodesService)
	nodeA := &common.Node{
		NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
			Name:                   "Node A",
			NodePubkey:             "02aaa...",
			TargetConf:             6,
			ChannelMinimumFeeMloki: 1000,
		}),
	}
	nodeB := &common.Node{
		NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
			Name:                   "Node B",
			NodePubkey:             "02bbb...",
			TargetConf:             6,
			ChannelMinimumFeeMloki: 1000,
		}),
	}
	mockNodes.On("GetNodes").Return([]*common.Node{nodeA, nodeB})

	configPath := "/tmp/nodes-c2-test.yaml"
	defer os.Remove(configPath)

	s := NewDashboardService(mockNodes, nil, nil, nil, "pubkey_test", configPath)

	updateReq := struct {
		Config config.NodeConfig `json:"config"`
	}{
		Config: config.NodeConfig{
			Name:                   "Node A Renamed",
			NodePubkey:             "02aaa...",
			TargetConf:             6,
			ChannelMinimumFeeMloki: 1000,
		},
	}
	body, err := json.Marshal(updateReq)
	assert.NoError(t, err)

	req := &lsps.JsonRpcRequest{Id: json.RawMessage(`1`), Method: "update_config", Params: body}
	resp := s.handleUpdateConfig(req)
	assert.Nil(t, resp.Error)

	data, err := os.ReadFile(configPath)
	assert.NoError(t, err)

	var persisted []*config.NodeConfig
	assert.NoError(t, yaml.Unmarshal(data, &persisted))
	assert.Len(t, persisted, 2, "both nodes should still be present in the persisted file")
	assert.Equal(t, "Node A Renamed", persisted[0].Name)
	assert.Equal(t, "Node B", persisted[1].Name, "the second node must survive editing the first")
}

// TestNodeConfig_ConcurrentReadWriteNoRace is the regression test for C-3:
// config used to be a raw *config.NodeConfig mutated in place
// (*nodes[0].NodeConfig = newValue) with no synchronization against
// concurrent readers on the HTLC hot path. Run with -race: this must pass
// clean, not just avoid a panic.
func TestNodeConfig_ConcurrentReadWriteNoRace(t *testing.T) {
	ref := config.NewNodeConfigRef(&config.NodeConfig{TimeLockDelta: 144})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Simulate the interceptor's hot path: many concurrent readers taking
	// a snapshot and reading fields off it.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					cfg := ref.Load()
					_ = cfg.TimeLockDelta
					_ = cfg.AdditionalChannelCapacity
				}
			}
		}()
	}

	// Simulate the dashboard: one writer swapping in a whole new config
	// repeatedly.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			ref.Store(&config.NodeConfig{TimeLockDelta: uint32(i)})
		}
	}()

	// Let the readers run a little past the writer finishing, then stop.
	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}
