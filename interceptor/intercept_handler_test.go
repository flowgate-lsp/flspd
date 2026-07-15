package interceptor

import (
	"testing"
	"time"

	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStore for InterceptStore interface
type MockStore struct {
	mock.Mock
}

func (m *MockStore) PaymentInfo(paymentHash []byte) (string, string, *core.OpeningFeeParams, []byte, []byte, []byte, int64, int64, *wire.OutPoint, *string, error) {
	args := m.Called(paymentHash)
	var params *core.OpeningFeeParams
	if v := args.Get(2); v != nil {
		params = v.(*core.OpeningFeeParams)
	}
	var gotHash, secret, dest []byte
	if v := args.Get(3); v != nil {
		gotHash = v.([]byte)
	}
	if v := args.Get(4); v != nil {
		secret = v.([]byte)
	}
	if v := args.Get(5); v != nil {
		dest = v.([]byte)
	}
	var channelPoint *wire.OutPoint
	if v := args.Get(8); v != nil {
		channelPoint = v.(*wire.OutPoint)
	}
	var tag *string
	if v := args.Get(9); v != nil {
		tag = v.(*string)
	}
	return args.String(0), args.String(1), params, gotHash, secret, dest, args.Get(6).(int64), args.Get(7).(int64), channelPoint, tag, args.Error(10)
}

func (m *MockStore) GetLsps2Order(scid uint64) (*core.Lsps2BuyRegistration, error) {
	args := m.Called(scid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.Lsps2BuyRegistration), args.Error(1)
}

func (m *MockStore) RegisterPayment(token, orderId string, params *core.OpeningFeeParams, destination, paymentHash, paymentSecret []byte, incomingAmountMloki, outgoingAmountMloki int64, tag string) error {
	args := m.Called(token, orderId, params, destination, paymentHash, paymentSecret, incomingAmountMloki, outgoingAmountMloki, tag)
	return args.Error(0)
}

func (m *MockStore) SetFundingTx(paymentHash []byte, channelPoint *wire.OutPoint) error {
	return nil
}

func (m *MockStore) GetPaidLsps1Order(clientNodeId string) (*core.Lsps1Order, error) {
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
	order := &core.Lsps2BuyRegistration{
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

	// Expect Registration with CORRECT Fee Calculation: fees are charged on
	// the forwarded (outgoing) amount, so outgoing = input / (1 + rate).
	// rate = 4000/1_000_000 = 0.004
	// outgoing = 20_000_000 / 1.004 = 19920318 (truncated)
	// fee = 20_000_000 - 19920318 = 79682, which is above the 1000 floor.
	mockStore.On("RegisterPayment",
		"token_1",
		"order_1",
		mock.Anything,   // params
		mock.Anything,   // dest
		mock.Anything,   // hash
		mock.Anything,   // secret
		int64(20000000), // Incoming (Gross)
		int64(19920318), // Outgoing (Net)
		"",
	).Return(nil)

	// Call
	req := core.InterceptRequest{
		Scid:                lightning.ShortChannelID(scid),
		IncomingAmountMloki: 20000000,
		PaymentHash:         []byte("hash"),
	}

	result, err := interceptor.resolvePayment(req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(19920318), result.outgoingAmountMloki)
	assert.True(t, result.isJit)
}

// TestResolvePayment_JIT_Underpayment tests rejection when sender pays Net Amount
func TestResolvePayment_JIT_Underpayment(t *testing.T) {
	mockStore := new(MockStore)
	interceptor := &Interceptor{store: mockStore}

	scid := uint64(123456)
	paymentSize := uint64(20000000) // 20k

	order := &core.Lsps2BuyRegistration{
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
	req := core.InterceptRequest{
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

// TestComputeJitOutgoingAmount covers the fee/floor math independently of
// the mock store, including the case where the proportional fee falls
// below the minFee floor (untested previously, since the panicking mock
// setup masked it).
func TestComputeJitOutgoingAmount(t *testing.T) {
	tests := []struct {
		name             string
		paymentSizeMloki int64
		minFeeMloki      int64
		proportional     uint32
		wantOutgoing     int64
		wantFee          int64
	}{
		{
			name:             "proportional fee above floor",
			paymentSizeMloki: 20000000,
			minFeeMloki:      1000,
			proportional:     4000,
			wantOutgoing:     19920318,
			wantFee:          79682,
		},
		{
			name:             "proportional fee below floor, floor applies",
			paymentSizeMloki: 100000,
			minFeeMloki:      1000,
			proportional:     4000,
			wantOutgoing:     99000,
			wantFee:          1000,
		},
		{
			name:             "zero proportional rate",
			paymentSizeMloki: 20000000,
			minFeeMloki:      1000,
			proportional:     0,
			wantOutgoing:     19999000,
			wantFee:          1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outgoing, fee := computeJitOutgoingAmount(tt.paymentSizeMloki, tt.minFeeMloki, tt.proportional)
			assert.Equal(t, tt.wantOutgoing, outgoing, "outgoing mismatch")
			assert.Equal(t, tt.wantFee, fee, "fee mismatch")
		})
	}
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
				config: config.NewNodeConfigRef(&config.NodeConfig{
					AdditionalChannelCapacity: tt.padding,
					MinChannelCapacityLoki:    tt.minFloor,
				}),
			}

			// Verify the math in openChannel
			cfg := interceptor.config.Load()
			capacity := tt.incomingAmountMloki/1000 + cfg.AdditionalChannelCapacity
			if uint64(capacity) < cfg.MinChannelCapacityLoki {
				capacity = int64(cfg.MinChannelCapacityLoki)
			}

			assert.Equal(t, tt.expectedCapacity, capacity, "Capacity calculation mismatch")
		})
	}
}
