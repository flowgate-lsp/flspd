package lsps

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/flowgate-lsp/flspd/common"
	"github.com/rs/zerolog/log"
)

type Lsps2GetInfoLegacyParams struct {
	Token string `json:"token"`
}

type Lsps2OpeningFeeParams struct {
	MinFeeMloki          string `json:"min_fee_mloki"`
	Proportional         uint32 `json:"proportional"`
	ValidUntil           string `json:"valid_until"`
	MinLifetime          uint32 `json:"min_lifetime"`
	MaxClientToSelfDelay uint32 `json:"max_client_to_self_delay"`
	MinPaymentSizeMloki  string `json:"min_payment_size_mloki"`
	MaxPaymentSizeMloki  string `json:"max_payment_size_mloki"`
	Promise              string `json:"promise"`
}

type Lsps2BuyParams struct {
	OpeningFeeParams Lsps2OpeningFeeParams `json:"opening_fee_params"`
	PaymentSizeMloki string                `json:"payment_size_mloki,omitempty"`
}

type Lsps2BuyResult struct {
	JITChannelScid     string `json:"jit_channel_scid"`
	LspCltvExpiryDelta uint16 `json:"lsp_cltv_expiry_delta"`
	ClientTrustsLsp    bool   `json:"client_trusts_lsp"`
	LspNodeId          string `json:"lsp_node_id"`
}

func (s *Server) handleLsps2GetVersions(req *JsonRpcRequest) *JsonRpcResponse {
	log.Trace().
		Str("component", string(common.LogComponentLSPS2)).
		Str("action", string(common.LogActionRequest)).
		Msg("LSPS2: Received get_versions request")
	result := map[string][]int{
		"versions": {1},
	}
	log.Trace().
		Str("component", string(common.LogComponentLSPS2)).
		Str("action", string(common.LogActionResponse)).
		Msgf("LSPS2: get_versions result: %+v", result)
	return NewJsonRpcResponse(req.Id, result)
}

func (s *Server) lsps2GetInfo(req *JsonRpcRequest) *JsonRpcResponse {
	var params Lsps2GetInfoLegacyParams
	// params might be empty or null
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &params)
	}
	log.Trace().
		Str("component", string(common.LogComponentLSPS2)).
		Str("action", string(common.LogActionRequest)).
		Interface("params", params).
		Msg("LSPS2: Received get_info request")

	nodes := s.nodesService.GetNodes()
	var menu []*Lsps2OpeningFeeParams

	// Gather fee params from ALL configured nodes (policies)
	// If a specific token was requested, we could filter, but LSPS2 get_info implies
	// returning the available options. FLSPD associates options with tokens.
	// We will return all valid options from all nodes.
	for _, node := range nodes {
		// If the request provided a token, and it doesn't match this node, skip?
		// The client might provide a token to "login" or identify partially.
		// If provided token matches one of the node's tokens, we prioritize it?
		// For now, let's include all.
		// Note: node.Tokens is []string.
		if params.Token != "" {
			found := false
			for _, t := range node.Tokens {
				if t == params.Token {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Use the first token of the node to fetch fee params?
		// OpeningService.GetFeeParamsMenu takes a token.
		// We should iterate over node.Tokens?
		for _, token := range node.Tokens {
			fees, err := s.openingService.GetFeeParamsMenu(token, node.PrivateKey)
			if err != nil {
				log.Warn().
					Err(err).
					Str("component", string(common.LogComponentLSPS2)).
					Str("action", string(common.LogActionFailure)).
					Str("token", token).
					Msg("LSPS2: Failed to get fee params for token")
				continue
			}

			for _, fee := range fees {
				item := &Lsps2OpeningFeeParams{
					MinFeeMloki:          fmt.Sprintf("%d", fee.MinFeeMloki),
					Proportional:         fee.Proportional,
					ValidUntil:           fee.ValidUntil,
					MinLifetime:          fee.MinLifetime,
					MaxClientToSelfDelay: fee.MaxClientToSelfDelay,
					MinPaymentSizeMloki:  fmt.Sprintf("%d", fee.MinPaymentSizeMloki),
					MaxPaymentSizeMloki:  fmt.Sprintf("%d", fee.MaxPaymentSizeMloki),
					Promise:              fee.Promise,
				}
				menu = append(menu, item)
			}
		}
	}

	log.Trace().
		Str("component", string(common.LogComponentLSPS2)).
		Str("action", string(common.LogActionResponse)).
		Msgf("LSPS2: get_info success, menu items=%d", len(menu))
	return NewJsonRpcResponse(req.Id, map[string]interface{}{
		"opening_fee_params_menu": menu,
	})
}

func (s *Server) lsps2Buy(req *JsonRpcRequest, peer []byte) *JsonRpcResponse {
	log.Trace().Msgf("LSPS2: lsps2Buy raw params: %s", string(req.Params))
	var params Lsps2BuyParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		log.Trace().
			Err(err).
			Str("component", string(common.LogComponentLSPS2)).
			Str("action", string(common.LogActionValidate)).
			Msg("LSPS2: buy invalid params")
		return NewJsonRpcErrorResponse(req.Id, -32602, "Invalid params", nil)
	}
	log.Trace().
		Str("component", string(common.LogComponentLSPS2)).
		Str("action", string(common.LogActionRequest)).
		Interface("params", params).
		Msg("LSPS2: Received buy request")

	// Validate params
	minFee, _ := strconv.ParseUint(params.OpeningFeeParams.MinFeeMloki, 10, 64)
	commonParams := &common.OpeningFeeParams{
		MinFeeMloki:          minFee,
		Proportional:         params.OpeningFeeParams.Proportional,
		ValidUntil:           params.OpeningFeeParams.ValidUntil,
		MinLifetime:          params.OpeningFeeParams.MinLifetime,
		MaxClientToSelfDelay: params.OpeningFeeParams.MaxClientToSelfDelay,
		// These fields are not in common.OpeningFeeParams, but needed for paramsHash if we updated it?
		// Wait, we updated common.OpeningFeeParams to include them. We need to parse them from request params.
		MinPaymentSizeMloki: parseUint64(params.OpeningFeeParams.MinPaymentSizeMloki),
		MaxPaymentSizeMloki: parseUint64(params.OpeningFeeParams.MaxPaymentSizeMloki),
		Promise:             params.OpeningFeeParams.Promise,
	}

	nodes := s.nodesService.GetNodes()
	var validNode *common.Node
	var validToken string

Loop:
	for _, node := range nodes {
		if s.openingService.ValidateOpeningFeeParams(commonParams, node.PublicKey) {
			validNode = node
			if len(node.Tokens) > 0 {
				validToken = node.Tokens[0]
			}
			break Loop
		}
	}

	if validNode == nil {
		return NewJsonRpcErrorResponse(req.Id, 100, "Invalid promise or expired params", nil)
	}

	scid, err := generateRandomScid()
	if err != nil {
		return NewJsonRpcErrorResponse(req.Id, -32603, "Internal error", nil)
	}

	var paymentSize *uint64
	if params.PaymentSizeMloki != "" {
		size, _ := strconv.ParseUint(params.PaymentSizeMloki, 10, 64)
		paymentSize = &size

		// Validate against min/max
		if *paymentSize < commonParams.MinPaymentSizeMloki || *paymentSize > commonParams.MaxPaymentSizeMloki {
			return NewJsonRpcErrorResponse(req.Id, 203, "Payment size out of bounds", nil)
		}

		// Validation: Check On-Chain Liquidity
		// We approximate needed liquidity as the payment size (converted to sats).
		// Note: PaymentSizeMloki is milliloki, we need sats (loki). 1 Loki = 1000 mLoki.
		neededSat := *paymentSize / 1000
		if err := s.liquidityManager.CheckOnChainLiquidity(context.Background(), neededSat); err != nil {
			log.Warn().
				Err(err).
				Str("component", string(common.LogComponentLSPS2)).
				Str("action", string(common.LogActionReject)).
				Msg("LSPS2: Buy rejected - insufficient on-chain liquidity")
			return NewJsonRpcErrorResponse(req.Id, 203, "Payment size too large: insufficient liquidity", nil)
		}

		// Compute Opening Fee
		openingFee, err := computeOpeningFee(*paymentSize, commonParams.MinFeeMloki, uint64(commonParams.Proportional))
		if err != nil {
			return NewJsonRpcErrorResponse(req.Id, 203, "Fee computation overflow", nil)
		}

		if openingFee >= *paymentSize {
			return NewJsonRpcErrorResponse(req.Id, 202, "Payment size too small (fee >= amount)", nil)
		}
	}

	regID, _ := generateUUID()

	err = s.openingService.RegisterLsps2Buy(&common.Lsps2BuyRegistration{
		Id:                         regID,
		LspId:                      hex.EncodeToString(validNode.PublicKey.SerializeCompressed()),
		PeerId:                     hex.EncodeToString(peer),
		Scid:                       scid,
		Mode:                       0,
		PaymentSizeMloki:           paymentSize,
		ParamsMinFeeMloki:          commonParams.MinFeeMloki,
		ParamsProportional:         commonParams.Proportional,
		ParamsValidUntil:           commonParams.ValidUntil,
		ParamsMinLifetime:          commonParams.MinLifetime,
		ParamsMaxClientToSelfDelay: commonParams.MaxClientToSelfDelay,
		ParamsPromise:              commonParams.Promise,
		Token:                      validToken,
	})

	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentLSPS2)).
			Str("action", string(common.LogActionFailure)).
			Msg("LSPS2: Failed to register buy in database")
		return NewJsonRpcErrorResponse(req.Id, -32603, "Internal DB error", nil)
	}

	result := Lsps2BuyResult{
		JITChannelScid:     formatScid(scid),
		LspCltvExpiryDelta: uint16(validNode.NodeConfig.TimeLockDelta),
		ClientTrustsLsp:    false,
		LspNodeId:          hex.EncodeToString(validNode.NodeId),
	}
	log.Trace().Msgf("LSPS2: lsps2Buy result: %+v", result)

	return NewJsonRpcResponse(req.Id, result)
}
