package postgresql

import (
	"context"

	"github.com/flowgate-lsp/flspd/common"
	"github.com/rs/zerolog/log"
)

// GetLsps1OrderByPaymentInvoice retrieves an LSPS1 order by its payment invoice string
func (s *PostgresOpeningStore) GetLsps1OrderByPaymentInvoice(paymentInvoice string) (*common.Lsps1Order, error) {
	row := s.pool.QueryRow(context.Background(),
		`SELECT order_id, peer_id, lsp_balance_loki, client_balance_loki, 
			funding_confirms_within_blocks, required_channel_confirmations, 
			channel_expiry_blocks, token, announce_channel, created_at, 
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			COALESCE(refund_onchain_address, ''), COALESCE(order_total_loki, 0), COALESCE(fee_total_loki, 0)
		FROM lsps1.orders WHERE payment_invoice=$1`, paymentInvoice)

	var order common.Lsps1Order
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

// UpdateLsps1OrderState updates the order_state and payment_state of an LSPS1 order
func (s *PostgresOpeningStore) UpdateLsps1OrderState(orderId string, newState string, newPaymentState string) error {
	result, err := s.pool.Exec(context.Background(),
		`UPDATE lsps1.orders 
		 SET order_state = $2, payment_state = $3
		 WHERE order_id = $1`,
		orderId, newState, newPaymentState)

	if err != nil {
		log.Error().Err(err).Msgf("UpdateLsps1OrderState: failed to update order %s", orderId)
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		log.Warn().Msgf("UpdateLsps1OrderState: no rows updated for order %s", orderId)
	}

	return nil
}

// GetPaidLsps1Order finds the oldest PAID order for a client that hasn't been fulfilled yet
func (s *PostgresOpeningStore) GetPaidLsps1Order(clientNodeId string) (*common.Lsps1Order, error) {
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

	var order common.Lsps1Order
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
