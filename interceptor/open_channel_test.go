package interceptor

import (
	"testing"

	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestOutPoint() *wire.OutPoint {
	var h wire.OutPoint
	return &h
}

func baseTestConfig() *config.NodeConfig {
	return &config.NodeConfig{
		MinChannelCapacityLoki:    20_000,
		AdditionalChannelCapacity: 1_000,
		TargetConf:                6,
		FeeRate:                   0.001,
		BaseFeeMloki:              1000,
		TimeLockDelta:             144,
	}
}

// newTestInterceptor wires up an Interceptor with mocked lightning.Client /
// chain.FeeEstimator so openChannel's real logic (not a copy of its math)
// runs against controllable dependencies.
func newTestInterceptor(cfg *config.NodeConfig, client *MockLightningClient, fee *fakeFeeEstimator, store *MockStore) *Interceptor {
	if fee == nil {
		fee = &fakeFeeEstimator{estimation: fixedFeeEstimation()}
	}
	if store == nil {
		store = new(MockStore)
	}
	return &Interceptor{
		config:       config.NewNodeConfigRef(cfg),
		client:       client,
		feeEstimator: fee,
		feeStrategy:  chain.FeeStrategyHalfHour,
		store:        store,
	}
}

func TestOpenChannel_Success_NonJitUsesConfiguredRoutingFee(t *testing.T) {
	cfg := baseTestConfig()
	client := new(MockLightningClient)
	expectedOutpoint := newTestOutPoint()

	client.On("OpenChannel", mock.MatchedBy(func(req *lightning.OpenChannelRequest) bool {
		// 20_000_000 mloki / 1000 = 20000 loki + 1000 loki padding = 21000 loki
		return req.CapacityLoki == 21_000 &&
			req.ZeroConf == false &&
			req.RoutingPolicy.Ppm == uint32(cfg.FeeRate*1_000_000) &&
			req.RoutingPolicy.BaseMloki == cfg.BaseFeeMloki
	})).Return(expectedOutpoint, nil)

	i := newTestInterceptor(cfg, client, nil, nil)

	outpoint, err := i.openChannel([]byte{1}, []byte{2}, 20_000_000, nil, false)
	require.NoError(t, err)
	assert.Equal(t, expectedOutpoint, outpoint)
	client.AssertExpectations(t)
}

func TestOpenChannel_JitUsesZeroRoutingFeeAndZeroConf(t *testing.T) {
	cfg := baseTestConfig()
	cfg.JitEnabled = true
	client := new(MockLightningClient)
	expectedOutpoint := newTestOutPoint()

	client.On("OpenChannel", mock.MatchedBy(func(req *lightning.OpenChannelRequest) bool {
		return req.ZeroConf == true &&
			req.RoutingPolicy.Ppm == 0 &&
			req.RoutingPolicy.BaseMloki == 0 &&
			req.MinConfs != nil && *req.MinConfs == 0
	})).Return(expectedOutpoint, nil)

	i := newTestInterceptor(cfg, client, nil, nil)

	_, err := i.openChannel([]byte{1}, []byte{2}, 20_000_000, nil, true)
	require.NoError(t, err)
	client.AssertExpectations(t)
}

func TestOpenChannel_RejectsCapacityBelowNetworkFloor(t *testing.T) {
	cfg := baseTestConfig()
	cfg.MinChannelCapacityLoki = 1_000_000 // far above what this tiny payment would produce
	client := new(MockLightningClient)
	i := newTestInterceptor(cfg, client, nil, nil)

	_, err := i.openChannel([]byte{1}, []byte{2}, 1_000, nil, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "smaller than minimum")
	client.AssertNotCalled(t, "OpenChannel", mock.Anything)
}

func TestOpenChannel_Jit_RejectsCapacityBelowBusinessFloor(t *testing.T) {
	cfg := baseTestConfig()
	cfg.JitEnabled = true
	businessFloor := uint64(500_000)
	cfg.JitMinChannelBalanceLoki = &businessFloor
	client := new(MockLightningClient)
	i := newTestInterceptor(cfg, client, nil, nil)

	_, err := i.openChannel([]byte{1}, []byte{2}, 20_000_000, nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "business minimum")
	client.AssertNotCalled(t, "OpenChannel", mock.Anything)
}

func TestOpenChannel_Jit_RejectsPaymentAboveMaxSize(t *testing.T) {
	cfg := baseTestConfig()
	cfg.JitEnabled = true
	cfg.JitMaxPaymentSizeMloki = 10_000_000
	client := new(MockLightningClient)
	i := newTestInterceptor(cfg, client, nil, nil)

	_, err := i.openChannel([]byte{1}, []byte{2}, 20_000_000, nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
	client.AssertNotCalled(t, "OpenChannel", mock.Anything)
}

func TestOpenChannel_ClientErrorPropagates(t *testing.T) {
	cfg := baseTestConfig()
	client := new(MockLightningClient)
	client.On("OpenChannel", mock.Anything).Return(nil, assert.AnError)
	i := newTestInterceptor(cfg, client, nil, nil)

	_, err := i.openChannel([]byte{1}, []byte{2}, 20_000_000, nil, false)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestOpenChannel_FeeEstimatorErrorFallsBackToTargetConf(t *testing.T) {
	cfg := baseTestConfig()
	client := new(MockLightningClient)
	expectedOutpoint := newTestOutPoint()

	client.On("OpenChannel", mock.MatchedBy(func(req *lightning.OpenChannelRequest) bool {
		return req.TargetConf != nil && *req.TargetConf == cfg.TargetConf
	})).Return(expectedOutpoint, nil)

	failingFee := &fakeFeeEstimator{err: assert.AnError}
	i := newTestInterceptor(cfg, client, failingFee, nil)

	_, err := i.openChannel([]byte{1}, []byte{2}, 20_000_000, nil, false)
	require.NoError(t, err, "a fee-estimator failure must fall back to configured TargetConf, not abort the open")
	client.AssertExpectations(t)
}

func TestOpenChannel_CallsSetFundingTxOnSuccess(t *testing.T) {
	cfg := baseTestConfig()
	client := new(MockLightningClient)
	expectedOutpoint := newTestOutPoint()
	client.On("OpenChannel", mock.Anything).Return(expectedOutpoint, nil)

	store := new(MockStore)
	// MockStore.SetFundingTx is a plain stub (always nil), so this mainly
	// documents that a successful open reaches the funding-tx persistence
	// step at all, without erroring out before it.
	i := newTestInterceptor(cfg, client, nil, store)

	outpoint, err := i.openChannel([]byte{1}, []byte{2}, 20_000_000, nil, false)
	require.NoError(t, err)
	assert.Equal(t, expectedOutpoint, outpoint)
}

func TestNewInterceptHandler_DecodesNodePubkeyIntoNodeid(t *testing.T) {
	cfg := config.NewNodeConfigRef(&config.NodeConfig{NodePubkey: "deadbeef"})
	store := new(MockStore)

	i := NewInterceptHandler(nil, cfg, store, nil, nil, nil, chain.FeeStrategyHalfHour, nil, nil)

	require.NotNil(t, i)
	assert.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, i.nodeid)
}
