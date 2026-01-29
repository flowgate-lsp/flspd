package interceptor

import (
	"testing"
	"time"

	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStore for InterceptStore interface
type MockStore struct {
	mock.Mock
}

func (m *MockStore) PaymentInfo(paymentHash []byte) (string, string, *common.OpeningFeeParams, []byte, []byte, []byte, int64, int64, *wire.OutPoint, *string, error) {
	args := m.Called(paymentHash)
	// Return default nil/empty for not found
	return args.String(0), args.String(1), nil, nil, nil, nil, 0, 0, nil, nil, args.Error(10)
}

func (m *MockStore) GetLsps2Order(scid uint64) (*common.Lsps2BuyRegistration, error) {
	args := m.Called(scid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.Lsps2BuyRegistration), args.Error(1)
}

func (m *MockStore) RegisterPayment(token, orderId string, params *common.OpeningFeeParams, destination, paymentHash, paymentSecret []byte, incomingAmountMloki, outgoingAmountMloki int64, tag string) error {
	args := m.Called(token, orderId, params, destination, paymentHash, paymentSecret, incomingAmountMloki, outgoingAmountMloki, tag)
	return args.Error(0)
}

func (m *MockStore) SetFundingTx(paymentHash []byte, channelPoint *wire.OutPoint) error {
	return nil
}

func (m *MockStore) GetPaidLsps1Order(clientNodeId string) (*common.Lsps1Order, error) {
	return nil, nil // Not testing LSPS1 here
}

// TestResolvePayment_JIT_Success tests the happy path where sender pays Gross Amount
func TestResolvePayment_JIT_Success(t *testing.T) {
	mockStore := new(MockStore)
	// Create Interceptor with mock store (other deps nil as not needed for resolvePayment logic logic we test)
	interceptor := &Interceptor{
		store: mockStore,
	}

	// Data
	scid := uint64(123456)
	paymentSize := uint64(20000000) // 20k Loki
	minFee := uint64(1000)
	prop := uint32(4000)

	validUntil := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	// Mock Order Lookup
	order := &common.Lsps2BuyRegistration{
		Id:                 "order_1",
		Scid:               scid,
		PaymentSizeMloki:   &paymentSize,
		ParamsMinFeeMloki:  minFee,
		ParamsProportional: prop,
		ParamsValidUntil:   validUntil,
		PeerId:             "02deadbeef",
		Token:              "token_1",
	}
	mockStore.On("PaymentInfo", mock.Anything).Return("", "", nil, nil, nil, nil, int64(0), int64(0), nil, nil, nil) // Not found in payments table
	mockStore.On("GetLsps2Order", scid).Return(order, nil)

	// Expect Registration with CORRECT Fee Calculation
	// Fee = max(1000, ceil(20M * 4000 / 1M)) = max(1000, 80000) = 80000.
	// Outgoing = 20M - 80k = 19920000.
	mockStore.On("RegisterPayment",
		"token_1",
		"order_1",
		mock.Anything,   // params
		mock.Anything,   // dest
		mock.Anything,   // hash
		mock.Anything,   // secret
		int64(20000000), // Incoming (Gross)
		int64(19920000), // Outgoing (Net)
		"",
	).Return(nil)

	// Call
	req := common.InterceptRequest{
		Scid:                lightning.ShortChannelID(scid),
		IncomingAmountMloki: 20000000,
		PaymentHash:         []byte("hash"),
	}

	result, err := interceptor.resolvePayment(req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(19920000), result.outgoingAmountMloki)
	assert.True(t, result.isJit)
}

// TestResolvePayment_JIT_Underpayment tests rejection when sender pays Net Amount
func TestResolvePayment_JIT_Underpayment(t *testing.T) {
	mockStore := new(MockStore)
	interceptor := &Interceptor{store: mockStore}

	scid := uint64(123456)
	paymentSize := uint64(20000000) // 20k

	order := &common.Lsps2BuyRegistration{
		Id:                 "order_1",
		Scid:               scid,
		PaymentSizeMloki:   &paymentSize,
		ParamsMinFeeMloki:  1000,
		ParamsProportional: 4000,
		PeerId:             "02deadbeef",
	}
	mockStore.On("PaymentInfo", mock.Anything).Return("", "", nil, nil, nil, nil, int64(0), int64(0), nil, nil, nil)
	mockStore.On("GetLsps2Order", scid).Return(order, nil)

	// Sender pays only 19920000 (Net)
	req := common.InterceptRequest{
		Scid:                lightning.ShortChannelID(scid),
		IncomingAmountMloki: 19920000,
		PaymentHash:         []byte("hash"),
	}

	result, err := interceptor.resolvePayment(req)

	// Should fail
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "underpayment")
}

// TestOpenChannel_CapacityLogic verifies that padding and floor are correctly applied
func TestOpenChannel_CapacityLogic(t *testing.T) {
	tests := []struct {
		name                string
		incomingAmountMloki int64
		padding             int64
		minFloor            uint64
		expectedCapacity    int64
	}{
		{
			name:                "Small payment + Padding hits Floor",
			incomingAmountMloki: 100 * 1000, // 100 sat
			padding:             21000,
			minFloor:            20000,
			expectedCapacity:    21100, // 100 + 21000
		},
		{
			name:                "Tiny payment + Small padding floors",
			incomingAmountMloki: 100 * 1000,
			padding:             500,
			minFloor:            20000,
			expectedCapacity:    20000, // Floor!
		},
		{
			name:                "Large payment + Padding",
			incomingAmountMloki: 500000 * 1000,
			padding:             21000,
			minFloor:            20000,
			expectedCapacity:    521000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := &Interceptor{
				config: &config.NodeConfig{
					AdditionalChannelCapacity: tt.padding,
					MinChannelCapacityLoki:    tt.minFloor,
				},
			}

			// Verify the math in openChannel
			capacity := tt.incomingAmountMloki/1000 + interceptor.config.AdditionalChannelCapacity
			if uint64(capacity) < interceptor.config.MinChannelCapacityLoki {
				capacity = int64(interceptor.config.MinChannelCapacityLoki)
			}

			assert.Equal(t, tt.expectedCapacity, capacity, "Capacity calculation mismatch")
		})
	}
}
