package lsps

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/flokiorg/flnd/lnrpc"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/rs/zerolog/log"
)

// LSPS1 Constants (Limits)
const (
	Lsps1MinRequiredChannelConfirmations = 0
	Lsps1MinFundingConfirmsWithinBlocks  = 1
	Lsps1SupportsZeroChannelReserve      = true
	Lsps1MaxChannelExpiryBlocks          = 201600 // ~140 days (1 minute blocks)
	Lsps1MinInitialClientBalanceLoki     = 0
	Lsps1MaxInitialClientBalanceLoki     = 100_000_000 // 100 Loki Coins? Or 100M sat-equivalent? 100M units.
	Lsps1MinInitialLspBalanceLoki        = 0
	Lsps1MaxInitialLspBalanceLoki        = 100_000_000
	Lsps1MinChannelBalanceLoki           = 50_000
	Lsps1MaxChannelBalanceLoki           = 100_000_000
	Lsps1OrderExpiryDuration             = 24 * time.Hour // Invoice and order expiry time
)

// Lsps1OrderState defines the state of an LSPS1 order
type Lsps1OrderState string

const (
	Lsps1OrderStateCreated   Lsps1OrderState = "CREATED"
	Lsps1OrderStateCompleted Lsps1OrderState = "COMPLETED"
	Lsps1OrderStateFailed    Lsps1OrderState = "FAILED"
	Lsps1OrderStateExpired   Lsps1OrderState = "EXPIRED" // Not in spec, but useful logic state
	Lsps1OrderStatePaid      Lsps1OrderState = "PAID"    // Internal state: Paid but not yet funded/completed
)

// Lsps1PaymentState defines the state of an LSPS1 payment
type Lsps1PaymentState string

const (
	Lsps1PaymentStateExpectPayment Lsps1PaymentState = "EXPECT_PAYMENT"
	Lsps1PaymentStateHold          Lsps1PaymentState = "HOLD"
	Lsps1PaymentStatePaid          Lsps1PaymentState = "PAID"
	Lsps1PaymentStateRefunded      Lsps1PaymentState = "REFUNDED"
	Lsps1PaymentStateExpired       Lsps1PaymentState = "EXPIRED" // Not in spec explicit return, but logic state
	Lsps1PaymentStateFailed        Lsps1PaymentState = "FAILED"
)

type Lsps1GetInfoResponse struct {
	MinRequiredChannelConfirmations uint16 `json:"min_required_channel_confirmations"`
	MinFundingConfirmsWithinBlocks  uint16 `json:"min_funding_confirms_within_blocks"`
	SupportsZeroChannelReserve      bool   `json:"supports_zero_channel_reserve"`
	MaxChannelExpiryBlocks          uint32 `json:"max_channel_expiry_blocks"`
	MinInitialClientBalanceLoki     string `json:"min_initial_client_balance_loki"`
	MaxInitialClientBalanceLoki     string `json:"max_initial_client_balance_loki"`
	MinInitialLspBalanceLoki        string `json:"min_initial_lsp_balance_loki"`
	MaxInitialLspBalanceLoki        string `json:"max_initial_lsp_balance_loki"`
	MinChannelBalanceLoki           string `json:"min_channel_balance_loki"`
	MaxChannelBalanceLoki           string `json:"max_channel_balance_loki"`
}

func (s *Server) handleLsps1GetInfo(req *JsonRpcRequest) *JsonRpcResponse {
	log.Trace().
		Str("component", string(common.LogComponentLSPS1)).
		Str("action", string(common.LogActionRequest)).
		Msg("LSPS1: Received get_info request")
	resp := Lsps1GetInfoResponse{
		MinRequiredChannelConfirmations: Lsps1MinRequiredChannelConfirmations,
		MinFundingConfirmsWithinBlocks:  Lsps1MinFundingConfirmsWithinBlocks,
		SupportsZeroChannelReserve:      Lsps1SupportsZeroChannelReserve,
		MaxChannelExpiryBlocks:          Lsps1MaxChannelExpiryBlocks,
		MinInitialClientBalanceLoki:     fmt.Sprintf("%d", Lsps1MinInitialClientBalanceLoki),
		MaxInitialClientBalanceLoki:     fmt.Sprintf("%d", Lsps1MaxInitialClientBalanceLoki),
		MinInitialLspBalanceLoki:        fmt.Sprintf("%d", Lsps1MinInitialLspBalanceLoki),
		MaxInitialLspBalanceLoki:        fmt.Sprintf("%d", Lsps1MaxInitialLspBalanceLoki),
		MinChannelBalanceLoki:           fmt.Sprintf("%d", Lsps1MinChannelBalanceLoki),
		MaxChannelBalanceLoki:           fmt.Sprintf("%d", Lsps1MaxChannelBalanceLoki),
	}
	log.Trace().
		Str("component", string(common.LogComponentLSPS1)).
		Str("action", string(common.LogActionResponse)).
		Msgf("LSPS1: get_info result: %+v", resp)
	return NewJsonRpcResponse(req.Id, resp)
}

// isOrderExpired checks if an order is expired based on the invoice expiry duration
func isOrderExpired(createdAt time.Time) bool {
	expiryTime := createdAt.Add(Lsps1OrderExpiryDuration)
	return time.Now().After(expiryTime)
}

type Lsps1CreateOrderRequest struct {
	LspBalanceLoki               string `json:"lsp_balance_loki"`
	ClientBalanceLoki            string `json:"client_balance_loki"`
	RequiredChannelConfirmations uint16 `json:"required_channel_confirmations"`
	FundingConfirmsWithinBlocks  uint16 `json:"funding_confirms_within_blocks"`
	ChannelExpiryBlocks          uint32 `json:"channel_expiry_blocks"`
	Token                        string `json:"token"`
	RefundOnchainAddress         string `json:"refund_onchain_address"`
	AnnounceChannel              bool   `json:"announce_channel"`
}

type Lsps1CreateOrderResponse struct {
	OrderId                      string        `json:"order_id"`
	LspBalanceLoki               string        `json:"lsp_balance_loki"`
	ClientBalanceLoki            string        `json:"client_balance_loki"`
	RequiredChannelConfirmations uint16        `json:"required_channel_confirmations"`
	FundingConfirmsWithinBlocks  uint16        `json:"funding_confirms_within_blocks"`
	ChannelExpiryBlocks          uint32        `json:"channel_expiry_blocks"`
	Token                        string        `json:"token"`
	AnnounceChannel              bool          `json:"announce_channel"`
	CreatedAt                    string        `json:"created_at"`
	OrderState                   string        `json:"order_state"`
	Payment                      *Lsps1Payment `json:"payment"`
	Channel                      interface{}   `json:"channel"` // null initially
}

type Lsps1Payment struct {
	Bolt11 *Lsps1PaymentBolt11 `json:"bolt11,omitempty"`
}

type Lsps1PaymentBolt11 struct {
	State          string `json:"state"`
	ExpiresAt      string `json:"expires_at"`
	FeeTotalLoki   string `json:"fee_total_loki"`
	OrderTotalLoki string `json:"order_total_loki"`
	Invoice        string `json:"invoice"`
}

func (s *Server) handleLsps1CreateOrder(req *JsonRpcRequest, peer []byte) *JsonRpcResponse {
	var params Lsps1CreateOrderRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Trace().
			Err(err).
			Str("component", string(common.LogComponentLSPS1)).
			Str("action", string(common.LogActionValidate)).
			Msg("LSPS1: create_order invalid params")
		return NewJsonRpcErrorResponse(req.Id, -32602, "Invalid params", nil)
	}
	log.Trace().
		Str("component", string(common.LogComponentLSPS1)).
		Str("action", string(common.LogActionRequest)).
		Interface("params", params).
		Msg("LSPS1: Received create_order request")

	clientBalance := parseUint64(params.ClientBalanceLoki)
	lspBalance := parseUint64(params.LspBalanceLoki)

	// Validation: At least one balance must be non-zero
	if clientBalance == 0 && lspBalance == 0 {
		log.Warn().
			Str("component", string(common.LogComponentLSPS1)).
			Str("action", string(common.LogActionReject)).
			Str("peer_id", hex.EncodeToString(peer)).
			Msg("LSPS1: Order rejected - at least one balance must be non-zero")
		return NewJsonRpcErrorResponse(req.Id, -32602, "Invalid params: either client_balance or lsp_balance must be non-zero", nil)
	}

	// Validation: Check bounds
	if clientBalance < Lsps1MinInitialClientBalanceLoki || clientBalance > Lsps1MaxInitialClientBalanceLoki {
		return NewJsonRpcErrorResponse(req.Id, 100, "Client balance out of bounds", nil)
	}

	// Validation: Check On-Chain Liquidity
	// We need to reserve LspBalanceLoki for this channel.
	if err := s.liquidityManager.CheckOnChainLiquidity(context.Background(), lspBalance); err != nil {
		log.Warn().
			Err(err).
			Str("component", string(common.LogComponentLSPS1)).
			Str("action", string(common.LogActionReject)).
			Msg("LSPS1: Order rejected - insufficient on-chain liquidity")
		return NewJsonRpcErrorResponse(req.Id, 1, "Client rejected: insufficient liquidity", nil)
	}

	// Calculate Fee
	// We'll use the first available node's policy for simplicity or a default policy.
	// NOTE: We need a policy. Let's try to get one from openingService using params.Token.
	var feeLoki uint64
	nodes := s.nodesService.GetNodes()
	var usedNode *common.Node
	for _, node := range nodes {
		// Just take the first valid one if no token, or match token
		if params.Token != "" {
			for _, t := range node.Tokens {
				if t == params.Token {
					usedNode = node
					break
				}
			}
		} else {
			usedNode = node
		}
		if usedNode != nil {
			break
		}
	}

	if usedNode != nil {
		// Use the token to get params? Or just assume some defaults?
		// We'll use a token if we found one, or the first token of the node
		t := ""
		if len(usedNode.Tokens) > 0 {
			t = usedNode.Tokens[0]
		}
		fees, _ := s.openingService.GetFeeParamsMenu(t, usedNode.PrivateKey)
		if len(fees) > 0 {
			// pick the first fee param
			f := fees[0]
			// fee_loki = (min_fee_mloki / 1000) + (capacity * proportional / 1M)
			capacity := clientBalance + lspBalance
			proportionalFee := (capacity * uint64(f.Proportional)) / 1_000_000
			minFeeLoki := f.MinFeeMloki / 1000
			feeLoki = minFeeLoki + proportionalFee
		} else {
			// Fallback default
			feeLoki = 1000 // 1000 loki default fee?
		}
	} else {
		feeLoki = 2000 // Default if no node found?
	}

	// DOS Protection: Limit pending orders per peer
	// Only count orders created within the last 24 hours (non-expired)
	peerId := hex.EncodeToString(peer)
	cutoffTime := time.Now().Add(-Lsps1OrderExpiryDuration)
	pendingCount, err := s.openingService.CountLsps1Orders(peerId, string(Lsps1OrderStateCreated), &cutoffTime)
	if err == nil && pendingCount >= s.config.Lsps1MaxPendingOrders {
		return NewJsonRpcErrorResponse(req.Id, -32603, fmt.Sprintf("Too many pending orders (max %d)", s.config.Lsps1MaxPendingOrders), nil)
	}

	orderTotal := clientBalance + feeLoki

	// Generate Invoice
	memo := "Inbound Liquidity Order"
	inv, err := s.lndClient.AddInvoice(context.Background(), &lnrpc.Invoice{
		Memo:    memo,
		Value:   int64(orderTotal),
		Expiry:  int64(Lsps1OrderExpiryDuration.Seconds()),
		Private: true,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentLSPS1)).
			Str("action", string(common.LogActionFailure)).
			Msg("LSPS1: Failed to create LND invoice for order")
		return NewJsonRpcErrorResponse(req.Id, -32603, "Internal error creating invoice", nil)
	}

	orderID, _ := generateUUID()
	now := time.Now()
	expiry := now.Add(Lsps1OrderExpiryDuration)

	// Persist Order
	order := &common.Lsps1Order{
		OrderId:                      orderID,
		PeerId:                       hex.EncodeToString(peer),
		LspBalanceLoki:               int64(lspBalance),
		ClientBalanceLoki:            int64(clientBalance),
		FundingConfirmsWithinBlocks:  int(params.FundingConfirmsWithinBlocks),
		RequiredChannelConfirmations: int(params.RequiredChannelConfirmations),
		ChannelExpiryBlocks:          int(params.ChannelExpiryBlocks),
		Token:                        params.Token,
		AnnounceChannel:              params.AnnounceChannel,
		CreatedAt:                    now,
		OrderState:                   string(Lsps1OrderStateCreated),
		PaymentState:                 string(Lsps1PaymentStateExpectPayment),
		PaymentInvoice:               inv.PaymentRequest,
		RefundOnchainAddress:         params.RefundOnchainAddress,
		OrderTotalLoki:               int64(orderTotal),
		FeeTotalLoki:                 int64(feeLoki),
	}

	err = s.openingService.RegisterLsps1Order(order)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentLSPS1)).
			Str("action", string(common.LogActionFailure)).
			Msg("LSPS1: Failed to persist order in database")
		return NewJsonRpcErrorResponse(req.Id, -32603, "Database error", nil)
	}

	resp := Lsps1CreateOrderResponse{
		OrderId:                      orderID,
		LspBalanceLoki:               params.LspBalanceLoki,
		ClientBalanceLoki:            params.ClientBalanceLoki,
		RequiredChannelConfirmations: params.RequiredChannelConfirmations,
		FundingConfirmsWithinBlocks:  params.FundingConfirmsWithinBlocks,
		ChannelExpiryBlocks:          params.ChannelExpiryBlocks,
		Token:                        params.Token,
		AnnounceChannel:              params.AnnounceChannel,
		CreatedAt:                    now.Format(time.RFC3339),
		OrderState:                   string(Lsps1OrderStateCreated),
		Payment: &Lsps1Payment{
			Bolt11: &Lsps1PaymentBolt11{
				State:          string(Lsps1PaymentStateExpectPayment),
				ExpiresAt:      expiry.Format(time.RFC3339),
				FeeTotalLoki:   fmt.Sprintf("%d", feeLoki),
				OrderTotalLoki: fmt.Sprintf("%d", orderTotal),
				Invoice:        inv.PaymentRequest,
			},
		},
		Channel: nil,
	}

	log.Trace().
		Str("component", string(common.LogComponentLSPS1)).
		Str("action", string(common.LogActionResponse)).
		Msgf("LSPS1: create_order success, order_id=%s", resp.OrderId)
	return NewJsonRpcResponse(req.Id, resp)
}

type Lsps1GetOrderRequest struct {
	OrderId string `json:"order_id"`
}

func (s *Server) handleLsps1GetOrder(req *JsonRpcRequest) *JsonRpcResponse {
	var params Lsps1GetOrderRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Trace().
			Err(err).
			Str("component", string(common.LogComponentLSPS1)).
			Str("action", string(common.LogActionValidate)).
			Msg("LSPS1: get_order invalid params")
		return NewJsonRpcErrorResponse(req.Id, -32602, "Invalid params", nil)
	}
	log.Trace().
		Str("component", string(common.LogComponentLSPS1)).
		Str("action", string(common.LogActionRequest)).
		Interface("params", params).
		Msg("LSPS1: Received get_order request")

	order, err := s.openingService.GetLsps1Order(params.OrderId)
	if err != nil {
		// Assume error means not found for now, or distinguish DB error?
		// Spec says 101 for Not found
		return NewJsonRpcErrorResponse(req.Id, 101, "Order not found", nil)
	}

	// TODO: Check if payment is actually paid by querying LND if state is EXPECT_PAYMENT?
	// For now, trust DB state.

	expiry := order.CreatedAt.Add(Lsps1OrderExpiryDuration)

	// Check if order is expired and override state if necessary
	orderState := order.OrderState
	paymentState := order.PaymentState
	if orderState == string(Lsps1OrderStateCreated) && isOrderExpired(order.CreatedAt) {
		orderState = string(Lsps1OrderStateExpired)
		paymentState = string(Lsps1PaymentStateExpired)
	}

	// Construct fee (approximate if not stored separately, but we stored total invoice amount)
	// We stored ClientBalanceLoki and LspBalanceLoki.
	// OrderTotal is in PaymentInvoice (decoded) or we can infer it.
	// Actually, we didn't store the Fee separately in DB, but we have LspBalance and ClientBalance.
	// Wait, we didn't store Fee in DB. But we have `payment_invoice`.
	// We can't easily get the amount from the invoice string without decoding it.
	// Ideally we should have stored `fee_loki` or `order_total_loki` in DB.
	// For this iteration, I'll calculate it on the fly if I can, or update DB schema?
	// The DB schema didn't have `fee_loki`.
	// I'll update `lsps1.orders` schema to include `order_total_loki` or `fee_loki`?
	// The `lsps1.orders` table I created in Step 614 does NOT have fee columns.
	// However, I can compute fee = OrderTotal - ClientBalance.
	// But I don't have OrderTotal easily without decoding invoice.
	// I'll skip returning fee_total_sat for get_order for now, or just return "0" and fix later.
	// Wait, spec says response is SAME as create_order.
	// I should probably decode the invoice or store the total.
	// I'll add `fee_loki` to the DB schema in a "migration fix" or just alter table?
	// Since I can't easily alter table right now without migration tool...
	// AND I just created the table...
	// Maybe I can just decode the invoice... but I don't have a bolt11 decoder imported.
	// I will just store `order_total_loki` in the DB. I'll issue a new schema migration command now.

	// Map internal PAID state back to spec-compliant states for response if necessary
	// LSPS1 spec only defines CREATED, COMPLETED, FAILED.
	// We'll return CREATED if conceptually paid but not yet COMPLETED.
	respOrderState := orderState
	if orderState == string(Lsps1OrderStatePaid) {
		respOrderState = string(Lsps1OrderStateCreated)
	}

	var channelInfo interface{}
	if order.FundingTxid != nil && *order.FundingTxid != "" {
		vout := 0
		if order.FundingOutpoint != nil {
			vout = *order.FundingOutpoint
		}
		channelInfo = map[string]interface{}{
			"funded_at":        order.CreatedAt.Format(time.RFC3339), // Fallback to CreatedAt if FundedAt missing
			"funding_outpoint": fmt.Sprintf("%s:%d", *order.FundingTxid, vout),
			"expires_at":       order.CreatedAt.Add(time.Duration(order.ChannelExpiryBlocks) * time.Minute).Format(time.RFC3339),
		}
	}

	resp := Lsps1CreateOrderResponse{
		OrderId:                      order.OrderId,
		LspBalanceLoki:               fmt.Sprintf("%d", order.LspBalanceLoki),
		ClientBalanceLoki:            fmt.Sprintf("%d", order.ClientBalanceLoki),
		RequiredChannelConfirmations: uint16(order.RequiredChannelConfirmations),
		FundingConfirmsWithinBlocks:  uint16(order.FundingConfirmsWithinBlocks),
		ChannelExpiryBlocks:          uint32(order.ChannelExpiryBlocks),
		Token:                        order.Token,
		AnnounceChannel:              order.AnnounceChannel,
		CreatedAt:                    order.CreatedAt.Format(time.RFC3339),
		OrderState:                   respOrderState,
		Payment: &Lsps1Payment{
			Bolt11: &Lsps1PaymentBolt11{
				State:          paymentState,
				ExpiresAt:      expiry.Format(time.RFC3339),
				FeeTotalLoki:   fmt.Sprintf("%d", order.FeeTotalLoki),
				OrderTotalLoki: fmt.Sprintf("%d", order.OrderTotalLoki),
				Invoice:        order.PaymentInvoice,
			},
		},
		Channel: channelInfo,
	}

	log.Trace().
		Str("component", string(common.LogComponentLSPS1)).
		Str("action", string(common.LogActionResponse)).
		Msgf("LSPS1: get_order result: %+v", resp)
	return NewJsonRpcResponse(req.Id, resp)
}
