package postgres

import (
	"context"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/rs/zerolog/log"
)

// GetLsps1OrderByPaymentInvoice retrieves an LSPS1 order by its payment invoice string
func (s *PostgresOpeningStore) GetLsps1OrderByPaymentInvoice(paymentInvoice string) (*core.Lsps1Order, error) {
	row := s.pool.QueryRow(context.Background(),
		`SELECT order_id, peer_id, lsp_balance_loki, client_balance_loki, 
			funding_confirms_within_blocks, required_channel_confirmations, 
			channel_expiry_blocks, token, announce_channel, created_at, 
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			COALESCE(refund_onchain_address, ''), COALESCE(order_total_loki, 0), COALESCE(fee_total_loki, 0)
		FROM lsps1.orders WHERE payment_invoice=$1`, paymentInvoice)

	var order core.Lsps1Order
	err := row.Scan(
		&order.OrderId, &order.PeerId, &order.LspBalanceLoki, &order.ClientBalanceLoki,
		&order.FundingConfirmsWithinBlocks, &order.RequiredChannelConfirmations,
		&order.ChannelExpiryBlocks, &order.Token, &order.AnnounceChannel, &order.CreatedAt,
		&order.OrderState, &order.PaymentState, &order.PaymentInvoice, &order.FundingTxid, &order.FundingOutpoint,
		&order.RefundOnchainAddress, &order.OrderTotalLoki, &order.FeeTotalLoki,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// UpdateLsps1OrderState performs a compare-and-set update of order_state
// and payment_state: the WHERE clause requires order_state to still equal
// expectedCurrentState, so two concurrent writers racing to transition the
// same order (e.g. a live settlement event vs. a startup recovery sweep,
// see F-1) can't both succeed. The loser gets updated=false and must not
// proceed with any side effect that assumes it won the transition.
func (s *PostgresOpeningStore) UpdateLsps1OrderState(orderId string, newState string, newPaymentState string, expectedCurrentState string) (bool, error) {
	result, err := s.pool.Exec(context.Background(),
		`UPDATE lsps1.orders
		 SET order_state = $2, payment_state = $3
		 WHERE order_id = $1 AND order_state = $4`,
		orderId, newState, newPaymentState, expectedCurrentState)

	if err != nil {
		log.Error().Err(err).Msgf("UpdateLsps1OrderState: failed to update order %s", orderId)
		return false, err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		log.Warn().Msgf("UpdateLsps1OrderState: no rows updated for order %s (expected current state %s did not match, or order not found)", orderId, expectedCurrentState)
		return false, nil
	}

	return true, nil
}

// GetPaidLsps1Order finds the oldest PAID order for a client that hasn't been fulfilled yet
func (s *PostgresOpeningStore) GetPaidLsps1Order(clientNodeId string) (*core.Lsps1Order, error) {
	// We want orders that are PAID but don't have a channel (FundingTxid is null/empty)
	// Ordered by created_at ASC to process oldest first (FIFO)
	row := s.pool.QueryRow(context.Background(),
		`SELECT order_id, peer_id, lsp_balance_loki, client_balance_loki, 
			funding_confirms_within_blocks, required_channel_confirmations, 
			channel_expiry_blocks, token, announce_channel, created_at, 
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			COALESCE(refund_onchain_address, ''), COALESCE(order_total_loki, 0), COALESCE(fee_total_loki, 0)
		 FROM lsps1.orders 
		 WHERE peer_id=$1 AND order_state='PAID' AND (funding_txid IS NULL OR funding_txid = '')
		 ORDER BY created_at ASC LIMIT 1`, clientNodeId)

	var order core.Lsps1Order
	err := row.Scan(
		&order.OrderId, &order.PeerId, &order.LspBalanceLoki, &order.ClientBalanceLoki,
		&order.FundingConfirmsWithinBlocks, &order.RequiredChannelConfirmations,
		&order.ChannelExpiryBlocks, &order.Token, &order.AnnounceChannel, &order.CreatedAt,
		&order.OrderState, &order.PaymentState, &order.PaymentInvoice, &order.FundingTxid, &order.FundingOutpoint,
		&order.RefundOnchainAddress, &order.OrderTotalLoki, &order.FeeTotalLoki,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// GetPaidLsps1Orders finds all PAID orders for a client that haven't been fulfilled yet,
// ordered by created_at ASC (FIFO).
func (s *PostgresOpeningStore) GetPaidLsps1Orders(clientNodeId string) ([]*core.Lsps1Order, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT order_id, peer_id, lsp_balance_loki, client_balance_loki, 
			funding_confirms_within_blocks, required_channel_confirmations, 
			channel_expiry_blocks, token, announce_channel, created_at, 
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			COALESCE(refund_onchain_address, ''), COALESCE(order_total_loki, 0), COALESCE(fee_total_loki, 0)
		 FROM lsps1.orders 
		 WHERE peer_id=$1 AND order_state='PAID' AND (funding_txid IS NULL OR funding_txid = '')
		 ORDER BY created_at ASC`, clientNodeId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*core.Lsps1Order
	for rows.Next() {
		var order core.Lsps1Order
		err := rows.Scan(
			&order.OrderId, &order.PeerId, &order.LspBalanceLoki, &order.ClientBalanceLoki,
			&order.FundingConfirmsWithinBlocks, &order.RequiredChannelConfirmations,
			&order.ChannelExpiryBlocks, &order.Token, &order.AnnounceChannel, &order.CreatedAt,
			&order.OrderState, &order.PaymentState, &order.PaymentInvoice, &order.FundingTxid, &order.FundingOutpoint,
			&order.RefundOnchainAddress, &order.OrderTotalLoki, &order.FeeTotalLoki,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}
	return orders, rows.Err()
}

// GetAllPaidLsps1Orders finds all PAID orders across all peers that haven't been fulfilled yet.
// Used for startup recovery to retry orders that were stuck when flspd was down.
func (s *PostgresOpeningStore) GetAllPaidLsps1Orders() ([]*core.Lsps1Order, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT order_id, peer_id, lsp_balance_loki, client_balance_loki, 
			funding_confirms_within_blocks, required_channel_confirmations, 
			channel_expiry_blocks, token, announce_channel, created_at, 
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			COALESCE(refund_onchain_address, ''), COALESCE(order_total_loki, 0), COALESCE(fee_total_loki, 0)
		 FROM lsps1.orders 
		 WHERE order_state='PAID' AND (funding_txid IS NULL OR funding_txid = '')
		 ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*core.Lsps1Order
	for rows.Next() {
		var order core.Lsps1Order
		err := rows.Scan(
			&order.OrderId, &order.PeerId, &order.LspBalanceLoki, &order.ClientBalanceLoki,
			&order.FundingConfirmsWithinBlocks, &order.RequiredChannelConfirmations,
			&order.ChannelExpiryBlocks, &order.Token, &order.AnnounceChannel, &order.CreatedAt,
			&order.OrderState, &order.PaymentState, &order.PaymentInvoice, &order.FundingTxid, &order.FundingOutpoint,
			&order.RefundOnchainAddress, &order.OrderTotalLoki, &order.FeeTotalLoki,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}
	return orders, rows.Err()
}

// GetAllCreatedLsps1Orders returns all orders still in CREATED state that have not expired.
// Used during startup to check whether any invoice was settled in LND while flspd was down.
func (s *PostgresOpeningStore) GetAllCreatedLsps1Orders() ([]*core.Lsps1Order, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT order_id, peer_id, lsp_balance_loki, client_balance_loki,
			funding_confirms_within_blocks, required_channel_confirmations,
			channel_expiry_blocks, token, announce_channel, created_at,
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			COALESCE(refund_onchain_address, ''), COALESCE(order_total_loki, 0), COALESCE(fee_total_loki, 0)
		 FROM lsps1.orders
		 WHERE order_state='CREATED'
		 ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*core.Lsps1Order
	for rows.Next() {
		var order core.Lsps1Order
		err := rows.Scan(
			&order.OrderId, &order.PeerId, &order.LspBalanceLoki, &order.ClientBalanceLoki,
			&order.FundingConfirmsWithinBlocks, &order.RequiredChannelConfirmations,
			&order.ChannelExpiryBlocks, &order.Token, &order.AnnounceChannel, &order.CreatedAt,
			&order.OrderState, &order.PaymentState, &order.PaymentInvoice, &order.FundingTxid, &order.FundingOutpoint,
			&order.RefundOnchainAddress, &order.OrderTotalLoki, &order.FeeTotalLoki,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}
	return orders, rows.Err()
}

// UpdateLsps1OrderFunding updates the funding_txid and funding_outpoint of an LSPS1 order
func (s *PostgresOpeningStore) UpdateLsps1OrderFunding(orderId string, fundingTxid string, fundingOutpoint int) error {
	result, err := s.pool.Exec(context.Background(),
		`UPDATE lsps1.orders 
		 SET funding_txid = $2, funding_outpoint = $3
		 WHERE order_id = $1`,
		orderId, fundingTxid, fundingOutpoint)

	if err != nil {
		log.Error().Err(err).Msgf("UpdateLsps1OrderFunding: failed to update order %s", orderId)
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		log.Warn().Msgf("UpdateLsps1OrderFunding: no rows updated for order %s", orderId)
	}

	return nil
}
