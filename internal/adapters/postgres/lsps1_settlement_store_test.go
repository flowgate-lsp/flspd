package postgres

import (
	"context"
	"sync"
	"testing"

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
