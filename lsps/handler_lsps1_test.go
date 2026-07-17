package lsps

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/flokiorg/flnd/lnrpc/invoicesrpc"
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestTableHandleLsps1CreateOrder tests create order scenarios using a table-driven approach
func TestTableHandleLsps1CreateOrder(t *testing.T) {
	// Define table scenarios
	tests := []struct {
		name                 string
		req                  Lsps1CreateOrderRequest
		peerId               string
		mockSetup            func(*MockOpeningStore, *MockLightningClient, *MockNodesService, *MockLiquidityManager, *MockNotificationService)
		config               lspsConfig
		expectedError        int    // 0 means no error expected, otherwise JSON-RPC error code
		expectedOrderState   string // Expected state in response (if success)
		expectedErrorMessage string // Partial match for error message
	}{
		{
			name: "Borderline Capacity - Small order accepted with padding",
			req: Lsps1CreateOrderRequest{
				ClientBalanceLoki:            "100",
				LspBalanceLoki:               "0",
				FundingConfirmsWithinBlocks:  1,
				RequiredChannelConfirmations: 0,
				ChannelExpiryBlocks:          201600,
				Token:                        "",
				AnnounceChannel:              false,
			},
			peerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f",
			mockSetup: func(store *MockOpeningStore, lnd *MockLightningClient, nodes *MockNodesService, liquidity *MockLiquidityManager, notif *MockNotificationService) {
				store.On("CountLsps1Orders", mock.Anything, string(Lsps1OrderStateCreated), mock.Anything).Return(0, nil)
				lnd.On("AddHoldInvoice", mock.Anything, mock.MatchedBy(func(req *invoicesrpc.AddHoldInvoiceRequest) bool {
					return req.Value >= 100 // Client balance + fee
				})).Return(&invoicesrpc.AddHoldInvoiceResp{PaymentRequest: "lnbc1..."}, nil)
				store.On("RegisterLsps1Order", mock.MatchedBy(func(order *core.Lsps1Order) bool {
					return order.ClientBalanceLoki == 100 && order.LspBalanceLoki == 0 &&
						order.OrderState == string(Lsps1OrderStateCreated)
				})).Return(nil)
				store.On("GetFeeParamsMenu", mock.Anything, mock.Anything).Return([]*core.OpeningFeeParams{}, nil)

				minClient := uint64(0)
				maxClient := uint64(100_000_000)
				minLsp := uint64(0)
				maxLsp := uint64(100_000_000)
				minChan := uint64(20000)
				maxChan := uint64(100_000_000)
				minReqConf := uint16(0)
				minFundingConf := uint16(1)
				supportsZeroReserve := true
				maxExpiry := uint32(201600)
				orderExpiry := uint32(86400)

				nodes.On("GetNodes").Return([]*core.Node{{NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
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
					AdditionalChannelCapacity:            21000, // 100 + 21000 > 20000
				})}}, nil)
				nodes.On("GetMinChannelCapacity").Return(uint64(20000))
				liquidity.On("CheckOnChainLiquidity", mock.Anything, mock.Anything).Return(nil)
				notif.On("NotifyOrderStateChanged", mock.Anything, mock.Anything, "CREATED", "", "").Return(true, nil)
			},
			config:             lspsConfig{Lsps1MaxPendingOrders: 5},
			expectedError:      0,
			expectedOrderState: string(Lsps1OrderStateCreated),
		},
		{
			name: "Success - Happy Path",
			req: Lsps1CreateOrderRequest{
				ClientBalanceLoki:            "1000000",
				LspBalanceLoki:               "5000000",
				FundingConfirmsWithinBlocks:  1,
				RequiredChannelConfirmations: 0,
				ChannelExpiryBlocks:          201600,
				Token:                        "",
				AnnounceChannel:              false,
			},
			peerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f",
			mockSetup: func(store *MockOpeningStore, lnd *MockLightningClient, nodes *MockNodesService, liquidity *MockLiquidityManager, notif *MockNotificationService) {
				var capturedHash []byte
				store.On("CountLsps1Orders", mock.Anything, string(Lsps1OrderStateCreated), mock.Anything).Return(0, nil)
				lnd.On("AddHoldInvoice", mock.Anything, mock.MatchedBy(func(req *invoicesrpc.AddHoldInvoiceRequest) bool {
					capturedHash = req.Hash
					return req.Value >= 1000000 && len(req.Hash) == 32 // At least client balance + fee, real sha256 hash
				})).Return(&invoicesrpc.AddHoldInvoiceResp{PaymentRequest: "lnbc1..."}, nil)
				store.On("RegisterLsps1Order", mock.MatchedBy(func(order *core.Lsps1Order) bool {
					// The order's stored preimage must actually back the hash
					// handed to AddHoldInvoice -- otherwise the invoice would
					// be unsettleable.
					if order.PaymentPreimage == nil {
						return false
					}
					preimage, err := hex.DecodeString(*order.PaymentPreimage)
					if err != nil || len(preimage) != 32 {
						return false
					}
					hash := sha256.Sum256(preimage)
					return bytes.Equal(hash[:], capturedHash) &&
						order.ClientBalanceLoki == 1000000 && order.LspBalanceLoki == 5000000 &&
						order.OrderState == string(Lsps1OrderStateCreated) &&
						order.PaymentState == string(Lsps1PaymentStateExpectPayment)
				})).Return(nil)
				store.On("GetFeeParamsMenu", mock.Anything, mock.Anything).Return([]*core.OpeningFeeParams{}, nil)

				minClient := uint64(0)
				maxClient := uint64(100_000_000)
				minLsp := uint64(0)
				maxLsp := uint64(100_000_000)
				minChan := uint64(20000)
				maxChan := uint64(100_000_000)
				minReqConf := uint16(0)
				minFundingConf := uint16(1)
				supportsZeroReserve := true
				maxExpiry := uint32(201600)
				orderExpiry := uint32(86400)

				nodes.On("GetNodes").Return([]*core.Node{{NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
					FeeRate:                              0.000001,
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
				})}}, nil)
				nodes.On("GetMinChannelCapacity").Return(uint64(20000))
				liquidity.On("CheckOnChainLiquidity", mock.Anything, mock.Anything).Return(nil)
				notif.On("NotifyOrderStateChanged", mock.Anything, mock.Anything, "CREATED", "", "").Return(true, nil)
			},
			config:             lspsConfig{Lsps1MaxPendingOrders: 5},
			expectedError:      0,
			expectedOrderState: string(Lsps1OrderStateCreated),
		},
		{
			name: "Failure - Zero Balances",
			req: Lsps1CreateOrderRequest{
				ClientBalanceLoki: "0",
				LspBalanceLoki:    "0",
			},
			peerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f",
			mockSetup: func(store *MockOpeningStore, lnd *MockLightningClient, nodes *MockNodesService, liquidity *MockLiquidityManager, notif *MockNotificationService) {
				// GetNodes is now called early for config values
				minClient := uint64(0)
				maxClient := uint64(100_000_000)
				minLsp := uint64(0)
				maxLsp := uint64(100_000_000)
				minReqConf := uint16(0)
				minFundingConf := uint16(1)
				supportsZeroReserve := true
				maxExpiry := uint32(201600)
				orderExpiry := uint32(86400)
				minChan := uint64(20000)
				maxChan := uint64(100_000_000)
				nodes.On("GetNodes").Return([]*core.Node{{NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
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
				})}}, nil).Maybe()
			},
			config:               lspsConfig{Lsps1MaxPendingOrders: 5},
			expectedError:        -32602,
			expectedErrorMessage: "Invalid params",
		},
		{
			name: "Failure - Client Balance Too High",
			req: Lsps1CreateOrderRequest{
				ClientBalanceLoki: "200000000", // > 100M limit
				LspBalanceLoki:    "0",
			},
			peerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f",
			mockSetup: func(store *MockOpeningStore, lnd *MockLightningClient, nodes *MockNodesService, liquidity *MockLiquidityManager, notif *MockNotificationService) {
				// GetNodes is now called early for config values
				minClient := uint64(0)
				maxClient := uint64(100_000_000)
				minLsp := uint64(0)
				maxLsp := uint64(100_000_000)
				minReqConf := uint16(0)
				minFundingConf := uint16(1)
				supportsZeroReserve := true
				maxExpiry := uint32(201600)
				orderExpiry := uint32(86400)
				minChan := uint64(20000)
				maxChan := uint64(100_000_000)
				nodes.On("GetNodes").Return([]*core.Node{{NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
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
				})}}, nil).Maybe()
			},
			config:               lspsConfig{Lsps1MaxPendingOrders: 5},
			expectedError:        100,                            // Application specific error
			expectedErrorMessage: "Client balance out of bounds", // Partial match
		},
		{
			name: "Failure - Too Many Pending Orders",
			req: Lsps1CreateOrderRequest{
				ClientBalanceLoki: "1000000",
			},
			peerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f",
			mockSetup: func(store *MockOpeningStore, lnd *MockLightningClient, nodes *MockNodesService, liquidity *MockLiquidityManager, notif *MockNotificationService) {
				store.On("GetFeeParamsMenu", mock.Anything, mock.Anything).Return([]*core.OpeningFeeParams{}, nil)
				store.On("CountLsps1Orders", mock.Anything, string(Lsps1OrderStateCreated), mock.Anything).Return(5, nil) // Already 5
				store.On("GetFeeParamsMenu", mock.Anything, mock.Anything).Return([]*core.OpeningFeeParams{}, nil)
				store.On("CountLsps1Orders", mock.Anything, string(Lsps1OrderStateCreated), mock.Anything).Return(5, nil) // Already 5
				minClient := uint64(0)
				maxClient := uint64(100_000_000)
				minLsp := uint64(0)
				maxLsp := uint64(100_000_000)
				minReqConf := uint16(0)
				minFundingConf := uint16(1)
				supportsZeroReserve := true
				maxExpiry := uint32(201600)
				orderExpiry := uint32(86400)
				minChan := uint64(20000)
				maxChan := uint64(100_000_000)

				nodes.On("GetNodes").Return([]*core.Node{{NodeConfig: config.NewNodeConfigRef(&config.NodeConfig{
					FeeRate:                              0.000001,
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
				})}}, nil)
				nodes.On("GetMinChannelCapacity").Return(uint64(20000))
				liquidity.On("CheckOnChainLiquidity", mock.Anything, mock.Anything).Return(nil)
			},
			config:               lspsConfig{Lsps1MaxPendingOrders: 5},
			expectedError:        -32603,
			expectedErrorMessage: "Too many pending orders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(MockOpeningStore)
			mockLnd := new(MockLightningClient)
			mockNodes := new(MockNodesService)
			mockLiquidity := new(MockLiquidityManager)
			mockNotif := new(MockNotificationService)

			tt.mockSetup(mockStore, mockLnd, mockNodes, mockLiquidity, mockNotif)

			server := &Handlers{
				config:              Config{Lsps1MaxPendingOrders: tt.config.Lsps1MaxPendingOrders},
				lndClient:           mockLnd,
				nodesService:        mockNodes,
				openingService:      mockStore,
				liquidityManager:    mockLiquidity,
				notificationService: mockNotif,
			}

			// Marshal request
			paramsBytes, _ := json.Marshal(tt.req)
			req := &JsonRpcRequest{
				Id:     json.RawMessage(`"test-id"`),
				Method: "lsps1.create_order",
				Params: paramsBytes,
			}

			peerBytes, _ := hex.DecodeString(tt.peerId)
			resp := server.handleLsps1CreateOrder(req, peerBytes)

			if tt.expectedError != 0 {
				assert.NotNil(t, resp.Error, "Expected error but got nil")
				if resp.Error != nil {
					assert.Equal(t, tt.expectedError, resp.Error.Code)
					if tt.expectedErrorMessage != "" {
						assert.Contains(t, resp.Error.Message, tt.expectedErrorMessage)
					}
				}
			} else {
				assert.Nil(t, resp.Error, "Expected success but got error: %v", resp.Error)
				if resp.Result != nil {
					// Check result structure
					// We need to marshal/unmarshal result to check fields effectively purely from 'resp.Result' interface{}
					// But our handler returns *JsonRpcResponse with Result interface{} which holds the struct directly usually
					// Wait, NewJsonRpcResponse takes interface{}, so resp.Result holds the struct.
					res, ok := resp.Result.(Lsps1CreateOrderResponse)
					assert.True(t, ok, "Result should be Lsps1CreateOrderResponse")
					assert.Equal(t, tt.expectedOrderState, res.OrderState)
				}
			}

			mockStore.AssertExpectations(t)
			mockLnd.AssertExpectations(t)
			mockNodes.AssertExpectations(t)
			mockLiquidity.AssertExpectations(t)
		})
	}
}

// Helpers/Mocks specifically for these tests if not common
type lspsConfig struct {
	Lsps1MaxPendingOrders int
}
