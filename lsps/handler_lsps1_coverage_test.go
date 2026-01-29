package lsps

import (
	"encoding/json"
	"testing"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"

	// "github.com/flowgate-lsp/flspd/lsps/mocks" // Removed
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleLsps1CreateOrder_Matrix(t *testing.T) {
	// Setup constants from LSPS1 defaults/limits for testing
	const (
		MinClient      = uint64(0)
		MaxClient      = uint64(100_000_000)
		MaxLsp         = uint64(100_000_000)
		MinChan        = uint64(20000) // Assumed default from nodesService mock
		ValidLsp       = uint64(50000)
		ValidClient    = uint64(0)
		OverflowLsp    = MaxLsp + 1
		OverflowClient = MaxClient + 1
	)

	tests := []struct {
		name          string
		clientBalance string
		lspBalance    string
		mockSetup     func(*MockLiquidityManager, *MockNodesService, *MockOpeningService, *MockLndClient, *MockNotificationService)
		expectCode    int
		expectMsg     string
	}{
		{
			name:          "Reject: Both Balances Zero",
			clientBalance: "0",
			lspBalance:    "0",
			expectCode:    -32602,
			expectMsg:     "Invalid params: either client_balance or lsp_balance must be non-zero",
		},
		{
			name:          "Reject: Client Balance Overflow",
			clientBalance: "100000001", // MaxClient + 1
			lspBalance:    "0",
			expectCode:    100,
			expectMsg:     "Client balance out of bounds",
		},
		{
			name:          "Reject: LSP Balance Overflow",
			clientBalance: "0",
			lspBalance:    "100000001", // MaxLsp + 1
			expectCode:    100,
			expectMsg:     "LSP balance out of bounds",
		},
		{
			name:          "Reject: Channel Capacity Too Small",
			clientBalance: "0",
			lspBalance:    "10000", // < MinChan (20000)
			mockSetup: func(lm *MockLiquidityManager, ns *MockNodesService, os *MockOpeningService, lc *MockLndClient, notif *MockNotificationService) {
				lm.On("CheckOnChainLiquidity", mock.Anything, uint64(10000)).Return(nil)
				ns.On("GetMinChannelCapacity").Return(MinChan)
			},
			expectCode: 100,
			expectMsg:  "Channel capacity too small",
		},
		{
			name:          "Success: Valid Inbound Order",
			clientBalance: "0",
			lspBalance:    "50000",
			mockSetup: func(lm *MockLiquidityManager, ns *MockNodesService, os *MockOpeningService, lc *MockLndClient, notif *MockNotificationService) {
				lm.On("CheckOnChainLiquidity", mock.Anything, uint64(50000)).Return(nil)
				ns.On("GetMinChannelCapacity").Return(MinChan)
				ns.On("GetNodes").Return([]*common.Node{{NodeConfig: &config.NodeConfig{}}})
				os.On("GetFeeParamsMenu", mock.Anything, mock.Anything).Return([]*common.OpeningFeeParams{{
					MinFeeMloki: 1000, Proportional: 100,
				}}, nil)
				os.On("CountLsps1Orders", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
				lc.On("AddInvoice", mock.Anything, mock.Anything).Return(&lnrpc.AddInvoiceResponse{PaymentRequest: "lnbc..."}, nil)
				os.On("RegisterLsps1Order", mock.Anything).Return(nil)
				notif.On("NotifyOrderStateChanged", mock.Anything, mock.Anything, "CREATED", "", "").Return(true, nil)
			},
			expectCode: 0, // Success (no error field in response, but test checks result != nil)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock dependencies
			mockLM := new(MockLiquidityManager)
			mockNS := new(MockNodesService)
			mockOS := new(MockOpeningService)
			mockLC := new(MockLndClient)
			mockNotif := new(MockNotificationService)

			// Default mocks if not customized
			minClient := MinClient
			maxClient := MaxClient
			minLsp := uint64(0)
			maxLsp := MaxLsp
			minChan := MinChan
			maxChan := uint64(100_000_000)
			minReqConf := uint16(0)
			minFundingConf := uint16(1)
			supportsZeroReserve := true
			maxExpiry := uint32(201600)
			orderExpiry := uint32(86400)

			mockNS.On("GetNodes").Return([]*common.Node{{NodeConfig: &config.NodeConfig{
				Lsps1MinRequiredChannelConfirmations: &minReqConf,
				Lsps1MinFundingConfirmsWithinBlocks:  &minFundingConf,
				Lsps1SupportsZeroChannelReserve:      &supportsZeroReserve,
				Lsps1MaxChannelExpiryBlocks:          &maxExpiry,
				Lsps1OrderExpirySeconds:              &orderExpiry,
				Lsps1MinInitialClientBalanceLoki:     &minClient,
				Lsps1MaxInitialClientBalanceLoki:     &maxClient,
				Lsps1MinInitialLspBalanceLoki:        &minLsp,
				Lsps1MaxInitialLspBalanceLoki:        &maxLsp,
				Lsps1MinChannelBalanceLoki:           &minChan,
				Lsps1MaxChannelBalanceLoki:           &maxChan,
			}}}).Maybe()

			if tt.mockSetup != nil {
				tt.mockSetup(mockLM, mockNS, mockOS, mockLC, mockNotif)
			} else {
				// Default permissive mocks for validations that proceed far enough to need them
				mockLM.On("CheckOnChainLiquidity", mock.Anything, mock.Anything).Return(nil).Maybe()
				mockNS.On("GetMinChannelCapacity").Return(MinChan).Maybe()
			}

			// Server setup
			cfg := Config{Lsps1MaxPendingOrders: 10}
			server := NewServer(cfg, mockLC, mockNS, mockOS, mockLM, mockNotif)

			// Request construction
			reqData := map[string]interface{}{
				"client_balance_loki":            tt.clientBalance,
				"lsp_balance_loki":               tt.lspBalance,
				"required_channel_confirmations": 0,
				"funding_confirms_within_blocks": 1,
				"channel_expiry_blocks":          144,
				"token":                          "",
				"refund_onchain_address":         "",
				"announce_channel":               false,
			}
			paramsBytes, _ := json.Marshal(reqData)
			req := &JsonRpcRequest{
				Id:      1,
				Method:  "lsps1.create_order",
				Params:  paramsBytes,
				JsonRpc: "2.0",
			}
			peer := []byte("peer_id")

			// Execution
			resp := server.handleLsps1CreateOrder(req, peer)

			// Verification
			if tt.expectCode != 0 {
				assert.NotNil(t, resp.Error)
				assert.Equal(t, tt.expectCode, resp.Error.Code)
				assert.Contains(t, resp.Error.Message, tt.expectMsg)
			} else {
				assert.Nil(t, resp.Error)
				assert.NotNil(t, resp.Result)
			}
		})
	}
}
