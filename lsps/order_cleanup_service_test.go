package lsps

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// runCount wires a thread-safe call counter into a MockOpeningStore's
// ExpireStaleOrders expectation via .Run(), rather than reading mock.Calls
// directly from a test goroutine -- mock.Mock's own Calls slice is only
// safe to append to under its internal lock (which Called() does), not to
// read concurrently from elsewhere, and OrderCleanupService's loop runs in
// its own goroutine here.
func runCount(store *MockOpeningStore, err error) *int32 {
	var count int32
	store.On("ExpireStaleOrders").Run(func(mock.Arguments) {
		atomic.AddInt32(&count, 1)
	}).Return(int64(0), err)
	return &count
}

// TestOrderCleanupService_RunsImmediatelyOnStart covers NewOrderCleanupService
// + Start's "run once before the first tick" behavior, previously entirely
// untested. Uses a long interval so only the immediate run can plausibly
// fire within the test's wait window, not a ticker tick.
func TestOrderCleanupService_RunsImmediatelyOnStart(t *testing.T) {
	store := new(MockOpeningStore)
	count := runCount(store, nil)

	svc := NewOrderCleanupService(store, time.Hour)
	go svc.Start()
	defer svc.Stop()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(count) >= 1
	}, time.Second, 10*time.Millisecond, "Start must run cleanup once immediately, not only wait for the first tick")
}

// TestOrderCleanupService_RunsOnEachTick covers the ticker-driven loop
// itself, using a short interval so multiple real ticks fire within a fast
// test rather than only exercising the immediate-on-start call.
func TestOrderCleanupService_RunsOnEachTick(t *testing.T) {
	store := new(MockOpeningStore)
	count := runCount(store, nil)

	svc := NewOrderCleanupService(store, 15*time.Millisecond)
	go svc.Start()
	defer svc.Stop()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(count) >= 3
	}, time.Second, 10*time.Millisecond, "the ticker must drive repeated cleanup runs, not just the initial one")
}

// TestOrderCleanupService_StopReturnsPromptly proves Stop's close(stopCh)
// + <-doneCh handshake actually unblocks Start's loop rather than hanging
// -- a regression in that handshake (e.g. Start not selecting on stopCh)
// would hang this test until it times out, not silently pass.
func TestOrderCleanupService_StopReturnsPromptly(t *testing.T) {
	store := new(MockOpeningStore)
	runCount(store, nil)

	svc := NewOrderCleanupService(store, time.Hour)
	go svc.Start()

	// Give Start a moment to reach its select loop before stopping it.
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return promptly")
	}
}

// TestOrderCleanupService_ExpireErrorDoesNotStopTheLoop covers the error
// branch: a failed ExpireStaleOrders call must be logged and skipped, not
// crash or halt future ticks.
func TestOrderCleanupService_ExpireErrorDoesNotStopTheLoop(t *testing.T) {
	store := new(MockOpeningStore)
	count := runCount(store, assert.AnError)

	svc := NewOrderCleanupService(store, 15*time.Millisecond)
	go svc.Start()
	defer svc.Stop()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(count) >= 2
	}, time.Second, 10*time.Millisecond, "an ExpireStaleOrders error on one tick must not stop later ticks from running")
}
