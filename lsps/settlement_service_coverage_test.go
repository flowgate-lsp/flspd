package lsps

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/flokiorg/go-flokicoin/chaincfg/chainhash"
	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMatrixProcessSettledInvoice tests various settlement scenarios
func TestMatrixProcessSettledInvoice(t *testing.T) {
	// Matrix dimensions:
	// 1. Invoice Type: Valid LSPS1, Non-LSPS1 (unknown)
	// 2. Order State: CREATED, PAID (already), FAILED
	// 3. Peer Connection: Connected, Disconnected, Error
	// 4. Fee Estimation: Success, Failure (Fallback)
	// 5. Channel Open: Success, Failure

	tests := []struct {
		name                 string
		invoice              string
		order                *common.Lsps1Order // nil if unknown invoice
		peerConnected        bool
		connError            error
		feeError             error
		openChannelError     error
		expectStateUpdate    bool   // Expect update to PAID
		expectChannelAttempt bool   // Expect attempt to open channel
		expectCompletion     bool   // Expect update to COMPLETED
		expectedEndState     string // expected state in DB call if completed
	}{
		{
			name:                 "Happy Path - Standard Success",
			invoice:              "lnbc1...valid",
			order:                &common.Lsps1Order{OrderId: "o1", PeerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f", OrderState: string(Lsps1OrderStateCreated), ClientBalanceLoki: 1000},
			peerConnected:        true,
			expectStateUpdate:    true,
			expectChannelAttempt: true,
			expectCompletion:     true,
			expectedEndState:     string(Lsps1OrderStateCompleted),
		},
		{
			name:                 "Ignored - Non-LSPS1 Invoice",
			invoice:              "lnbc1...unknown",
			order:                nil,
			expectStateUpdate:    false,
			expectChannelAttempt: false,
		},
		{
			name:                 "Ignored - Already Paid Order",
			invoice:              "lnbc1...paid",
			order:                &common.Lsps1Order{OrderId: "o2", PeerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f", OrderState: string(Lsps1OrderStatePaid)},
			expectStateUpdate:    false,
			expectChannelAttempt: false,
		},
		{
			name:                 "Deferred - Peer Disconnected",
			invoice:              "lnbc1...disco",
			order:                &common.Lsps1Order{OrderId: "o3", PeerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f", OrderState: string(Lsps1OrderStateCreated), ClientBalanceLoki: 1000},
			peerConnected:        false,
			expectStateUpdate:    true,
			expectChannelAttempt: true, // It attempts, but returns early
			expectCompletion:     false,
		},
		{
			name:                 "Fallback - Fee Estimation Fails",
			invoice:              "lnbc1...nofee",
			order:                &common.Lsps1Order{OrderId: "o4", PeerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f", OrderState: string(Lsps1OrderStateCreated), ClientBalanceLoki: 1000},
			peerConnected:        true,
			feeError:             assert.AnError,
			expectStateUpdate:    true,
			expectChannelAttempt: true,
			expectCompletion:     true,
			expectedEndState:     string(Lsps1OrderStateCompleted),
		},
		{
			name:                 "Retry/Fail - Channel Open Fails",
			invoice:              "lnbc1...openfail",
			order:                &common.Lsps1Order{OrderId: "o5", PeerId: "02677063d89856f4d04be859e9a4d3393165b6f3cd78641bc3c4ef9ca2fde37f", OrderState: string(Lsps1OrderStateCreated), ClientBalanceLoki: 1000},
			peerConnected:        true,
			openChannelError:     assert.AnError,
			expectStateUpdate:    true,
			expectChannelAttempt: true,
			expectCompletion:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(MockOpeningStore)
			mockClient := new(MockLightningClient)
			mockHistory := new(MockHistoryStore)
			mockFee := new(MockFeeEstimator)

			// Setup Invoice Lookup
			if tt.order == nil {
				mockStore.On("GetLsps1OrderByPaymentInvoice", tt.invoice).Return(nil, assert.AnError) // Simulate not found
			} else {
				mockStore.On("GetLsps1OrderByPaymentInvoice", tt.invoice).Return(tt.order, nil)
			}

			// Setup State Update to PAID
			if tt.expectStateUpdate {
				mockStore.On("UpdateLsps1OrderState", tt.order.OrderId, string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid), string(Lsps1OrderStateCreated)).Return(true, nil)
			}

			// Setup Channel Opening Logic
			if tt.expectChannelAttempt && tt.order != nil && tt.order.ClientBalanceLoki > 0 {
				peerIdBytes, _ := hex.DecodeString(tt.order.PeerId)
				// Connectivity
				mockClient.On("IsConnected", peerIdBytes).Return(tt.peerConnected, tt.connError)

				if tt.peerConnected {
					// Fee
					if tt.feeError != nil {
						mockFee.On("EstimateFeeRate", mock.Anything, mock.Anything).Return(nil, tt.feeError)
					} else {
						f := 1.0
						mockFee.On("EstimateFeeRate", mock.Anything, mock.Anything).Return(&chain.FeeEstimation{LokiPerVByte: &f}, nil)
					}

					// Open Channel
					if tt.openChannelError != nil {
						mockClient.On("OpenChannel", mock.Anything).Return(nil, tt.openChannelError)
						mockStore.On("UpdateLsps1OrderState", tt.order.OrderId, string(Lsps1OrderStateFailed), string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid)).Return(true, nil)
					} else {
						chanPoint := &wire.OutPoint{Hash: chainhash.Hash{}, Index: 0}
						mockClient.On("OpenChannel", mock.Anything).Return(chanPoint, nil)

						// Completion Updates
						if tt.expectCompletion {
							mockStore.On("UpdateLsps1OrderFunding", tt.order.OrderId, mock.Anything, mock.Anything).Return(nil)
							mockStore.On("UpdateLsps1OrderState", tt.order.OrderId, tt.expectedEndState, string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid)).Return(true, nil)
						}
					}
				}
			}

			svc := NewLsps1SettlementService(mockClient, mockStore, mockHistory, config.NewNodeConfigRef(&config.NodeConfig{}), mockFee, chain.FeeStrategyEconomy, nil)

			svc.processSettledInvoice(&lightning.InvoiceUpdate{
				PaymentRequest: tt.invoice,
				State:          lightning.InvoiceStateSettled,
				AmountMloki:    1000000,
			})

			// Wait for goroutines
			time.Sleep(50 * time.Millisecond)

			mockStore.AssertExpectations(t)
			mockClient.AssertExpectations(t)
		})
	}
}
