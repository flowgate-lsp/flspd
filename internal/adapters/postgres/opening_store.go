package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type extendedParams struct {
	Token   string                `json:"token"`
	OrderId string                `json:"order_id,omitempty"`
	Params  core.OpeningFeeParams `json:"fees_params"`
}

type PostgresOpeningStore struct {
	pool *pgxpool.Pool
}

func NewPostgresOpeningStore(pool *pgxpool.Pool) *PostgresOpeningStore {
	return &PostgresOpeningStore{pool: pool}
}

func (s *PostgresOpeningStore) GetFeeParamsSettings(token string) ([]*core.OpeningFeeParamsSetting, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT validity, params FROM new_channel_params WHERE (token=$1 OR token IS NULL) AND validity>0`, token)
	if err != nil {
		log.Error().Msgf("GetFeeParamsSettings(%v) error: %v", token, err)
		return nil, err
	}
	defer rows.Close()

	var settings []*core.OpeningFeeParamsSetting
	for rows.Next() {
		var validity int64
		var param string
		err = rows.Scan(&validity, &param)
		if err != nil {
			return nil, err
		}

		var params *core.OpeningFeeParams
		err := json.Unmarshal([]byte(param), &params)
		if err != nil {
			log.Error().Msgf("Failed to unmarshal fee param '%v': %v", param, err)
			return nil, err
		}

		duration := time.Second * time.Duration(validity)
		settings = append(settings, &core.OpeningFeeParamsSetting{
			Validity: duration,
			Params:   params,
		})
	}

	return settings, nil
}

func (s *PostgresOpeningStore) SetFeeParams(token string, validity time.Duration, params core.OpeningFeeParams) error {
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return err
	}
	validitySec := int64(validity.Seconds())
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM new_channel_params WHERE token=$1 AND validity=$2`, token, validitySec)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `INSERT INTO new_channel_params (token, validity, params) VALUES ($1, $2, $3)`, token, validitySec, paramsBytes)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PostgresOpeningStore) RegisterLsps2Buy(reg *core.Lsps2BuyRegistration) error {
	// DEBUG LOGGING
	log.Trace().Msgf("RegisterLsps2Buy inserting int64 SCID: %d (uint64: %d)", int64(reg.Scid), reg.Scid)
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO lsps2.buy_registrations (
			id, lsp_id, peer_id, scid, mode, payment_size_mloki, 
			params_min_fee_mloki, params_proportional, params_valid_until, 
			params_min_lifetime, params_max_client_to_self_delay, params_promise, token
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		reg.Id, reg.LspId, reg.PeerId, int64(reg.Scid), reg.Mode, reg.PaymentSizeMloki,
		reg.ParamsMinFeeMloki, reg.ParamsProportional, reg.ParamsValidUntil,
		reg.ParamsMinLifetime, reg.ParamsMaxClientToSelfDelay, reg.ParamsPromise, reg.Token)
	if err != nil {
		log.Error().Msgf("RegisterLsps2Buy error: %v", err)
		return err
	}
	return nil
}

func (s *PostgresOpeningStore) GetLsps2BuyRegistration(scid uint64) (*core.Lsps2BuyRegistration, error) {
	row := s.pool.QueryRow(context.Background(),
		`SELECT id, lsp_id, peer_id, scid, mode, payment_size_mloki, 
			params_min_fee_mloki, params_proportional, params_valid_until, 
			params_min_lifetime, params_max_client_to_self_delay, params_promise, token
		FROM lsps2.buy_registrations WHERE scid=$1`, int64(scid))

	var reg core.Lsps2BuyRegistration
	var scidInt int64
	err := row.Scan(
		&reg.Id, &reg.LspId, &reg.PeerId, &scidInt, &reg.Mode, &reg.PaymentSizeMloki,
		&reg.ParamsMinFeeMloki, &reg.ParamsProportional, &reg.ParamsValidUntil,
		&reg.ParamsMinLifetime, &reg.ParamsMaxClientToSelfDelay, &reg.ParamsPromise, &reg.Token,
	)
	if err != nil {
		return nil, err
	}
	reg.Scid = uint64(scidInt)
	return &reg, nil
}

func (s *PostgresOpeningStore) RegisterLsps1Order(order *core.Lsps1Order) error {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO lsps1.orders (
			order_id, peer_id, lsp_balance_loki, client_balance_loki, 
			funding_confirms_within_blocks, required_channel_confirmations, 
			channel_expiry_blocks, token, announce_channel, created_at, 
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			refund_onchain_address, order_total_loki, fee_total_loki
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		order.OrderId, order.PeerId, order.LspBalanceLoki, order.ClientBalanceLoki,
		order.FundingConfirmsWithinBlocks, order.RequiredChannelConfirmations,
		order.ChannelExpiryBlocks, order.Token, order.AnnounceChannel, order.CreatedAt,
		order.OrderState, order.PaymentState, order.PaymentInvoice, order.FundingTxid, order.FundingOutpoint,
		order.RefundOnchainAddress, order.OrderTotalLoki, order.FeeTotalLoki)
	if err != nil {
		log.Error().Msgf("RegisterLsps1Order error: %v", err)
		return err
	}
	return nil
}

func (s *PostgresOpeningStore) GetLsps1Order(orderId string) (*core.Lsps1Order, error) {
	row := s.pool.QueryRow(context.Background(),
		`SELECT order_id, peer_id, lsp_balance_loki, client_balance_loki, 
			funding_confirms_within_blocks, required_channel_confirmations, 
			channel_expiry_blocks, token, announce_channel, created_at, 
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			COALESCE(refund_onchain_address, ''), COALESCE(order_total_loki, 0), COALESCE(fee_total_loki, 0)
		FROM lsps1.orders WHERE order_id=$1`, orderId)

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

func (s *PostgresOpeningStore) ListLsps1Orders(limit, offset int) ([]*core.Lsps1Order, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT order_id, peer_id, lsp_balance_loki, client_balance_loki, 
			funding_confirms_within_blocks, required_channel_confirmations, 
			channel_expiry_blocks, token, announce_channel, created_at, 
			order_state, payment_state, payment_invoice, funding_txid, funding_outpoint,
			COALESCE(refund_onchain_address, ''), COALESCE(order_total_loki, 0), COALESCE(fee_total_loki, 0)
		FROM lsps1.orders 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*core.Lsps1Order
	for rows.Next() {
		var o core.Lsps1Order
		err := rows.Scan(
			&o.OrderId, &o.PeerId, &o.LspBalanceLoki, &o.ClientBalanceLoki,
			&o.FundingConfirmsWithinBlocks, &o.RequiredChannelConfirmations,
			&o.ChannelExpiryBlocks, &o.Token, &o.AnnounceChannel, &o.CreatedAt,
			&o.OrderState, &o.PaymentState, &o.PaymentInvoice, &o.FundingTxid, &o.FundingOutpoint,
			&o.RefundOnchainAddress, &o.OrderTotalLoki, &o.FeeTotalLoki,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &o)
	}
	return orders, nil
}

func (s *PostgresOpeningStore) RegisterLsps5Webhook(webhook *core.Lsps5Webhook) error {
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO lsps5.webhooks (client_node_id, app_name, url, transport, created_at) 
		VALUES ($1, $2, $3, $4, $5) 
		ON CONFLICT (client_node_id, app_name) DO UPDATE SET url=$3, transport=$4, created_at=$5`,
		webhook.ClientNodeId, webhook.AppName, webhook.Url, webhook.Transport, webhook.CreatedAt)
	if err != nil {
		log.Error().Msgf("RegisterLsps5Webhook error: %v", err)
		return err
	}
	return nil
}

func (s *PostgresOpeningStore) GetLsps5Webhooks(clientNodeId string) ([]*core.Lsps5Webhook, error) {
	rows, err := s.pool.Query(context.Background(),
		`SELECT client_node_id, app_name, url, transport, created_at FROM lsps5.webhooks WHERE client_node_id=$1`,
		clientNodeId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []*core.Lsps5Webhook
	for rows.Next() {
		var w core.Lsps5Webhook
		err := rows.Scan(&w.ClientNodeId, &w.AppName, &w.Url, &w.Transport, &w.CreatedAt)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, &w)
	}
	return webhooks, nil
}

func (s *PostgresOpeningStore) RemoveLsps5Webhook(clientNodeId, appName string) error {
	_, err := s.pool.Exec(context.Background(),
		`DELETE FROM lsps5.webhooks WHERE client_node_id=$1 AND app_name=$2`,
		clientNodeId, appName)
	return err
}

func (s *PostgresOpeningStore) CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error) {
	var count int
	var err error

	if excludeExpiredBefore != nil {
		err = s.pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM lsps1.orders WHERE peer_id=$1 AND order_state=$2 AND created_at >= $3`,
			clientNodeId, state, *excludeExpiredBefore).Scan(&count)
	} else {
		err = s.pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM lsps1.orders WHERE peer_id=$1 AND order_state=$2`,
			clientNodeId, state).Scan(&count)
	}

	return count, err
}

func (s *PostgresOpeningStore) CountLsps5Webhooks(clientNodeId string) (int, error) {
	var count int
	err := s.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM lsps5.webhooks WHERE client_node_id=$1`,
		clientNodeId).Scan(&count)
	return count, err
}

func (s *PostgresOpeningStore) GetReservedLiquidity() (uint64, error) {
	var reserved uint64
	// Only count CREATED orders from the last 24 hours as reserved
	// Older unpaid orders are considered expired/abandoned
	err := s.pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(lsp_balance_loki), 0) FROM lsps1.orders 
		 WHERE order_state = 'CREATED' 
		 AND created_at > NOW() - INTERVAL '24 hours'`).Scan(&reserved)
	if err != nil {
		return 0, err
	}
	return reserved, nil
}

// ExpireStaleOrders marks CREATED orders older than 24 hours as EXPIRED
func (s *PostgresOpeningStore) ExpireStaleOrders() (int64, error) {
	result, err := s.pool.Exec(context.Background(),
		`UPDATE lsps1.orders 
		 SET order_state = 'EXPIRED' 
		 WHERE order_state = 'CREATED' 
		 AND created_at <= NOW() - INTERVAL '24 hours'`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
