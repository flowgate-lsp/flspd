package postgresql

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type PostgresInterceptStore struct {
	pool *pgxpool.Pool
}

func NewPostgresInterceptStore(pool *pgxpool.Pool) *PostgresInterceptStore {
	return &PostgresInterceptStore{pool: pool}
}

func (s *PostgresInterceptStore) PaymentInfo(htlcPaymentHash []byte) (string, string, *common.OpeningFeeParams, []byte, []byte, []byte, int64, int64, *wire.OutPoint, *string, error) {
	var (
		p, tag                                   *string
		paymentHash, paymentSecret, destination  []byte
		incomingAmountMloki, outgoingAmountMloki int64
		fundingTxID                              []byte
		fundingTxOutnum                          pgtype.Int4
	)
	err := s.pool.QueryRow(context.Background(),
		`SELECT payment_hash, payment_secret, destination, incoming_amount_mloki, outgoing_amount_mloki, funding_tx_id, funding_tx_outnum, opening_fee_params, tag
			FROM payments
			WHERE payment_hash=$1 OR sha256('probing-01:' || payment_hash)=$1`,
		htlcPaymentHash).Scan(&paymentHash, &paymentSecret, &destination, &incomingAmountMloki, &outgoingAmountMloki, &fundingTxID, &fundingTxOutnum, &p, &tag)
	if err != nil {
		if err == pgx.ErrNoRows {
			err = nil
		}
		return "", "", nil, nil, nil, nil, 0, 0, nil, nil, err
	}

	var cp *wire.OutPoint
	if fundingTxID != nil {
		cp, err = lightning.NewOutPoint(fundingTxID, uint32(fundingTxOutnum.Int))
		if err != nil {
			log.Error().Msgf("invalid funding txid in database %x", fundingTxID)
		}
	}

	// opening_fee_params is nullable: a payment row with no fee params
	// attached (e.g. a plain forward, not a channel-open payment) has
	// p == nil. Returning here with zero-value token/orderId/params
	// mirrors the "not found" shape above rather than dereferencing a nil
	// extParams.
	if p == nil {
		return "", "", nil, paymentHash, paymentSecret, destination, incomingAmountMloki, outgoingAmountMloki, cp, tag, nil
	}

	var extParams *extendedParams
	err = json.Unmarshal([]byte(*p), &extParams)
	if err != nil {
		log.Error().Msgf("Failed to unmarshal OpeningFeeParams '%s': %v", *p, err)
		return "", "", nil, nil, nil, nil, 0, 0, nil, nil, err
	}
	return extParams.Token, extParams.OrderId, &extParams.Params, paymentHash, paymentSecret, destination, incomingAmountMloki, outgoingAmountMloki, cp, tag, nil
}

func (s *PostgresInterceptStore) SetFundingTx(paymentHash []byte, channelPoint *wire.OutPoint) error {
	commandTag, err := s.pool.Exec(context.Background(),
		`UPDATE payments
			SET funding_tx_id = $2, funding_tx_outnum = $3
			WHERE payment_hash=$1`,
		paymentHash, channelPoint.Hash[:], channelPoint.Index)
	log.Info().Msgf("setFundingTx(%x, %s, %d): %s err: %v", paymentHash, channelPoint.Hash.String(), channelPoint.Index, commandTag, err)
	return err
}

func (s *PostgresInterceptStore) RegisterPayment(token string, orderId string, params *common.OpeningFeeParams, destination, paymentHash, paymentSecret []byte, incomingAmountMloki, outgoingAmountMloki int64, tag string) error {
	var t *string
	if tag != "" {
		t = &tag
	}

	p := []byte{}
	if params != nil {
		var err error
		p, err = json.Marshal(extendedParams{Token: token, OrderId: orderId, Params: *params})
		if err != nil {
			log.Error().Msgf("Failed to marshal OpeningFeeParams: %v", err)
			return err
		}
	}

	commandTag, err := s.pool.Exec(context.Background(),
		`INSERT INTO
		payments (destination, payment_hash, payment_secret, incoming_amount_mloki, outgoing_amount_mloki, tag, opening_fee_params)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT DO NOTHING`,
		destination, paymentHash, paymentSecret, incomingAmountMloki, outgoingAmountMloki, t, p)
	log.Info().Msgf("registerPayment(%x, %x, %x, %v, %v, %v, %s) rows: %v err: %v",
		destination, paymentHash, paymentSecret, incomingAmountMloki, outgoingAmountMloki, tag, p, commandTag.RowsAffected(), err)
	if err != nil {
		return fmt.Errorf("registerPayment(%x, %x, %x, %v, %v, %v, %s) error: %w",
			destination, paymentHash, paymentSecret, incomingAmountMloki, outgoingAmountMloki, tag, p, err)
	}
	return nil
}

func (s *PostgresInterceptStore) GetLsps2Order(scid uint64) (*common.Lsps2BuyRegistration, error) {
	// DEBUG LOGGING
	log.Trace().Msgf("GetLsps2Order querying int64 SCID: %d (uint64: %d)", int64(scid), scid)
	row := s.pool.QueryRow(context.Background(),
		`SELECT id, lsp_id, peer_id, scid, mode, payment_size_mloki, 
			params_min_fee_mloki, params_proportional, params_valid_until, 
			params_min_lifetime, params_max_client_to_self_delay, params_promise, token
		FROM lsps2.buy_registrations WHERE scid=$1`, int64(scid))

	var reg common.Lsps2BuyRegistration
	var scidInt int64
	err := row.Scan(
		&reg.Id, &reg.LspId, &reg.PeerId, &scidInt, &reg.Mode, &reg.PaymentSizeMloki,
		&reg.ParamsMinFeeMloki, &reg.ParamsProportional, &reg.ParamsValidUntil,
		&reg.ParamsMinLifetime, &reg.ParamsMaxClientToSelfDelay, &reg.ParamsPromise, &reg.Token,
	)
	reg.Scid = uint64(scidInt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &reg, nil
}

// GetPaidLsps1Order finds the oldest PAID order for a client that hasn't been fulfilled yet
func (s *PostgresInterceptStore) GetPaidLsps1Order(clientNodeId string) (*common.Lsps1Order, error) {
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
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}
