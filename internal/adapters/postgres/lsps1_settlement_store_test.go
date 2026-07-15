package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertTestOrder(t *testing.T, orderId, state string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO lsps1.orders
			(order_id, peer_id, lsp_balance_loki, client_balance_loki,
			 funding_confirms_within_blocks, required_channel_confirmations,
			 channel_expiry_blocks, token, announce_channel, created_at,
			 order_state, payment_state, payment_invoice, order_total_loki, fee_total_loki)
		 VALUES ($1, 'peer', 1000, 1000, 1, 0, 100, '', false, now(), $2, $2, 'inv', 1000, 0)
		 ON CONFLICT (order_id) DO UPDATE SET order_state = $2, payment_state = $2`,
		orderId, state)
	require.NoError(t, err)
}

// insertPaidOrder inserts a peer/created_at/funding_txid-configurable PAID
// order, for exercising the FIFO-ordering and "funded orders are excluded"
// logic in GetPaidLsps1Order(s)/GetAllPaidLsps1Orders that insertTestOrder's
// fixed fixture can't.
func insertPaidOrder(t *testing.T, orderId, peerId string, createdAt time.Time, fundingTxid *string) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`INSERT INTO lsps1.orders
			(order_id, peer_id, lsp_balance_loki, client_balance_loki,
			 funding_confirms_within_blocks, required_channel_confirmations,
			 channel_expiry_blocks, token, announce_channel, created_at,
			 order_state, payment_state, payment_invoice, funding_txid, order_total_loki, fee_total_loki)
		 VALUES ($1, $2, 1000, 1000, 1, 0, 100, '', false, $3,
			 'PAID', 'PAID', 'inv', $4, 1000, 0)
		 ON CONFLICT (order_id) DO UPDATE SET peer_id = $2, created_at = $3, funding_txid = $4`,
		orderId, peerId, createdAt, fundingTxid)
	require.NoError(t, err)
}

// TestGetLsps1OrderByPaymentInvoice_Found covers the happy path for a
// function settlement_service.go's ProcessSettledInvoice relies on to map
// an incoming lnd invoice-settled event back to the LSPS1 order it belongs
// to -- previously had zero test coverage despite being load-bearing.
func TestGetLsps1OrderByPaymentInvoice_Found(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)
	orderId := newOrderId(t)

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO lsps1.orders
			(order_id, peer_id, lsp_balance_loki, client_balance_loki,
			 funding_confirms_within_blocks, required_channel_confirmations,
			 channel_expiry_blocks, token, announce_channel, created_at,
			 order_state, payment_state, payment_invoice, order_total_loki, fee_total_loki)
		 VALUES ($1, 'peer-invoice-lookup', 1000, 1000, 1, 0, 100, '', false, now(),
			 'CREATED', 'CREATED', $2, 1000, 0)`,
		orderId, "lnbc-fixture-invoice-"+orderId)
	require.NoError(t, err)

	order, err := store.GetLsps1OrderByPaymentInvoice("lnbc-fixture-invoice-" + orderId)
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, orderId, order.OrderId)
	assert.Equal(t, "peer-invoice-lookup", order.PeerId)
}

// TestGetLsps1OrderByPaymentInvoice_NotFound documents the current
// contract: unlike PostgresInterceptStore.GetPaidLsps1Order (which
// special-cases pgx.ErrNoRows into (nil, nil)), this method has no such
// handling and returns the raw scan error instead.
func TestGetLsps1OrderByPaymentInvoice_NotFound(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)

	order, err := store.GetLsps1OrderByPaymentInvoice("no-such-invoice-exists")
	assert.Error(t, err)
	assert.Nil(t, order)
}

// TestUpdateLsps1OrderFunding_UpdatesFields is the regression test for the
// write side of the exact bug class F-8 fixed on the read side: this is
// what settlement_service.go calls right after a JIT/manual channel opens,
// to persist the funding tx so recoverStuckOrders can later tell "funded,
// just needs reconciling" apart from "never actually opened."
func TestUpdateLsps1OrderFunding_UpdatesFields(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)
	orderId := "funding-update-order"
	insertTestOrder(t, orderId, "PAID")

	err := store.UpdateLsps1OrderFunding(orderId, "abc123txid", 2)
	require.NoError(t, err)

	order, err := store.GetLsps1Order(orderId)
	require.NoError(t, err)
	require.NotNil(t, order.FundingTxid)
	assert.Equal(t, "abc123txid", *order.FundingTxid)
	require.NotNil(t, order.FundingOutpoint)
	assert.Equal(t, 2, *order.FundingOutpoint)
}

// TestUpdateLsps1OrderFunding_UnknownOrderIsNotAnError documents the
// current contract: updating a non-existent order_id affects zero rows,
// logs a warning, but does not return an error -- callers relying on this
// to signal "order not found" would be silently misled.
func TestUpdateLsps1OrderFunding_UnknownOrderIsNotAnError(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)

	err := store.UpdateLsps1OrderFunding("no-such-order-id", "txid", 0)
	assert.NoError(t, err, "zero rows affected is logged, not returned as an error")
}

// TestGetAllPaidLsps1Orders_ExcludesFundedAndOtherStates is the regression
// test for the exact query recoverStuckOrders (F-8/F-10) depends on at
// startup: only PAID orders with no funding_txid yet -- across *all*
// peers, unlike the peer-scoped GetPaidLsps1Order(s) -- are candidates for
// recovery. A PAID order that already has a funding_txid must not show up
// here (see F-8: that's exactly the case that used to risk a duplicate
// OpenChannel call).
func TestGetAllPaidLsps1Orders_ExcludesFundedAndOtherStates(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)

	fundingTxid := "already-funded-txid"
	insertPaidOrder(t, "recover-unfunded-peer-a", "peer-a", time.Now(), nil)
	insertPaidOrder(t, "recover-funded-peer-b", "peer-b", time.Now(), &fundingTxid)
	insertTestOrder(t, "recover-created-not-paid", "CREATED")
	insertTestOrder(t, "recover-completed-not-paid", "COMPLETED")

	orders, err := store.GetAllPaidLsps1Orders()
	require.NoError(t, err)

	ids := make(map[string]bool, len(orders))
	for _, o := range orders {
		ids[o.OrderId] = true
	}
	assert.True(t, ids["recover-unfunded-peer-a"], "an unfunded PAID order from any peer must be included")
	assert.False(t, ids["recover-funded-peer-b"], "a PAID order that already has a funding_txid must be excluded")
	assert.False(t, ids["recover-created-not-paid"], "a CREATED order must be excluded")
	assert.False(t, ids["recover-completed-not-paid"], "a COMPLETED order must be excluded")
}

// TestGetAllCreatedLsps1Orders_OnlyCreatedState is the regression test for
// the other half of startup recovery: orders still awaiting payment,
// checked in case the invoice settled while flspd was down.
func TestGetAllCreatedLsps1Orders_OnlyCreatedState(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)

	insertTestOrder(t, "created-recovery-a", "CREATED")
	insertTestOrder(t, "created-recovery-paid", "PAID")
	insertTestOrder(t, "created-recovery-expired", "EXPIRED")

	orders, err := store.GetAllCreatedLsps1Orders()
	require.NoError(t, err)

	ids := make(map[string]bool, len(orders))
	for _, o := range orders {
		ids[o.OrderId] = true
	}
	assert.True(t, ids["created-recovery-a"])
	assert.False(t, ids["created-recovery-paid"])
	assert.False(t, ids["created-recovery-expired"])
}

// TestGetPaidLsps1Order_OldestUnfundedForPeer covers PostgresOpeningStore's
// GetPaidLsps1Order -- unused via the ports.OpeningStore interface today
// (the only live call path is PostgresInterceptStore's separate copy in
// intercept_store.go, verified via grep before writing this), but it's
// still public interface surface with its own FIFO/exclusion logic worth
// pinning.
func TestGetPaidLsps1Order_OldestUnfundedForPeer(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)
	peer := "peer-fifo-single"

	insertPaidOrder(t, "fifo-single-older", peer, time.Now().Add(-time.Hour), nil)
	insertPaidOrder(t, "fifo-single-newer", peer, time.Now(), nil)

	order, err := store.GetPaidLsps1Order(peer)
	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, "fifo-single-older", order.OrderId, "must return the oldest unfulfilled PAID order (FIFO)")
}

// TestGetPaidLsps1Orders_AllUnfundedForPeer covers the plural variant:
// every unfunded PAID order for a peer, not just the oldest.
func TestGetPaidLsps1Orders_AllUnfundedForPeer(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)
	peer := "peer-fifo-plural"
	fundingTxid := "funded"

	insertPaidOrder(t, "fifo-plural-a", peer, time.Now().Add(-2*time.Hour), nil)
	insertPaidOrder(t, "fifo-plural-b", peer, time.Now().Add(-time.Hour), nil)
	insertPaidOrder(t, "fifo-plural-funded", peer, time.Now(), &fundingTxid)

	orders, err := store.GetPaidLsps1Orders(peer)
	require.NoError(t, err)
	require.Len(t, orders, 2, "must return both unfunded orders, excluding the funded one")
	assert.Equal(t, "fifo-plural-a", orders[0].OrderId, "must be FIFO-ordered oldest first")
	assert.Equal(t, "fifo-plural-b", orders[1].OrderId)
}

// TestUpdateLsps1OrderState_CAS_AtSQLLevel proves the WHERE order_state=$4
// clause added for F-1 actually provides atomicity at the database level,
// not just in the Go-level fake-store test in lsps/settlement_service_test.go.
// N goroutines race to transition the SAME real row out of CREATED
// concurrently; exactly one must see updated=true.
func TestUpdateLsps1OrderState_CAS_AtSQLLevel(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)
	orderId := "cas-sql-test-order"
	insertTestOrder(t, orderId, "CREATED")

	const n = 20
	var wg sync.WaitGroup
	var successCount int32
	var mu sync.Mutex
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			updated, err := store.UpdateLsps1OrderState(orderId, "PAID", "PAID", "CREATED")
			assert.NoError(t, err)
			if updated {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), successCount, "exactly one of %d concurrent CAS attempts on the same row must succeed", n)

	order, err := store.GetLsps1Order(orderId)
	require.NoError(t, err)
	assert.Equal(t, "PAID", order.OrderState)
}

// TestUpdateLsps1OrderState_NoOpWhenStateMismatches confirms the CAS
// correctly refuses to update when the order is no longer in the expected
// state — the mechanism the loser of the F-1 race relies on to safely
// back off instead of double-processing.
func TestUpdateLsps1OrderState_NoOpWhenStateMismatches(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)
	orderId := "cas-sql-mismatch-order"
	insertTestOrder(t, orderId, "PAID") // already PAID, not CREATED

	updated, err := store.UpdateLsps1OrderState(orderId, "PAID", "PAID", "CREATED")
	require.NoError(t, err)
	assert.False(t, updated, "CAS must refuse to apply when order_state no longer matches expectedCurrentState")

	order, err := store.GetLsps1Order(orderId)
	require.NoError(t, err)
	assert.Equal(t, "PAID", order.OrderState, "order state must be untouched by the failed CAS")
}

// TestGetReservedLiquidity_SumsOnlyRecentCreatedOrders exercises the
// reserved-liquidity SQL directly: only CREATED orders within the last 24h
// should count.
func TestGetReservedLiquidity_SumsOnlyRecentCreatedOrders(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO lsps1.orders
			(order_id, peer_id, lsp_balance_loki, client_balance_loki,
			 funding_confirms_within_blocks, required_channel_confirmations,
			 channel_expiry_blocks, token, announce_channel, created_at,
			 order_state, payment_state, payment_invoice, order_total_loki, fee_total_loki)
		 VALUES
		 ('reserved-recent', 'peer', 5000, 0, 1, 0, 100, '', false, now(), 'CREATED', 'CREATED', 'inv1', 5000, 0),
		 ('reserved-old', 'peer', 7000, 0, 1, 0, 100, '', false, now() - interval '48 hours', 'CREATED', 'CREATED', 'inv2', 7000, 0),
		 ('reserved-completed', 'peer', 3000, 0, 1, 0, 100, '', false, now(), 'COMPLETED', 'PAID', 'inv3', 3000, 0)
		 ON CONFLICT (order_id) DO NOTHING`)
	require.NoError(t, err)

	reserved, err := store.GetReservedLiquidity()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, reserved, uint64(5000), "the recent CREATED order must be counted")

	// Confirm the old and completed orders specifically are NOT included by
	// checking the total didn't also pick up their amounts alongside.
	assert.Less(t, reserved, uint64(5000+7000+3000), "old and non-CREATED orders must not be counted")
}

// TestExpireStaleOrders_OnlyExpiresOldCreatedOrders exercises
// ExpireStaleOrders's SQL directly.
func TestExpireStaleOrders_OnlyExpiresOldCreatedOrders(t *testing.T) {
	store := NewPostgresOpeningStore(testPool)

	_, err := testPool.Exec(context.Background(),
		`INSERT INTO lsps1.orders
			(order_id, peer_id, lsp_balance_loki, client_balance_loki,
			 funding_confirms_within_blocks, required_channel_confirmations,
			 channel_expiry_blocks, token, announce_channel, created_at,
			 order_state, payment_state, payment_invoice, order_total_loki, fee_total_loki)
		 VALUES
		 ('expire-old', 'peer', 1000, 0, 1, 0, 100, '', false, now() - interval '48 hours', 'CREATED', 'CREATED', 'inv1', 1000, 0),
		 ('expire-recent', 'peer', 1000, 0, 1, 0, 100, '', false, now(), 'CREATED', 'CREATED', 'inv2', 1000, 0)
		 ON CONFLICT (order_id) DO NOTHING`)
	require.NoError(t, err)

	_, err = store.ExpireStaleOrders()
	require.NoError(t, err)

	oldOrder, err := store.GetLsps1Order("expire-old")
	require.NoError(t, err)
	assert.Equal(t, "EXPIRED", oldOrder.OrderState)

	recentOrder, err := store.GetLsps1Order("expire-recent")
	require.NoError(t, err)
	assert.Equal(t, "CREATED", recentOrder.OrderState, "a recent order must not be expired")
}
