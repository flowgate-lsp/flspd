package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	btcec "github.com/flokiorg/go-flokicoin/crypto"
	"github.com/flokiorg/go-flokicoin/crypto/ecdsa"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/platform/logging"
	"github.com/flowgate-lsp/flspd/ports"
	"github.com/rs/zerolog/log"
)

type openingService struct {
	store        ports.OpeningStore
	nodesService ports.NodesService
}

func NewOpeningService(
	store ports.OpeningStore,
	nodesService ports.NodesService,
) ports.OpeningService {
	return &openingService{
		store:        store,
		nodesService: nodesService,
	}
}

func (s *openingService) GetFeeParamsMenu(token string, privateKey *btcec.PrivateKey) ([]*core.OpeningFeeParams, error) {
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionRequest)).
		Str("token", token).
		Msg("GetFeeParamsMenu: Fetching fee params")
	var menu []*core.OpeningFeeParams
	settings, err := s.store.GetFeeParamsSettings(token)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentOpeningService)).
			Str("action", string(logging.LogActionFailure)).
			Msg("GetFeeParamsMenu: Failed to fetch fee params settings")
		return nil, fmt.Errorf("failed to get opening_fee_params")
	}

	if len(settings) == 0 {
		log.Debug().
			Str("component", string(logging.LogComponentOpeningService)).
			Str("token", token).
			Msg("GetFeeParamsMenu: No fee params settings found in DB")
	}

	for _, setting := range settings {
		maxPaymentSize := setting.Params.MaxPaymentSizeMloki
		if maxPaymentSize == 0 {
			maxPaymentSize = 10_000_000_000 // 10k Loki
		}

		validUntil := time.Now().UTC().Add(setting.Validity)
		params := &core.OpeningFeeParams{
			MinFeeMloki:          setting.Params.MinFeeMloki,
			Proportional:         setting.Params.Proportional,
			ValidUntil:           validUntil.Format(common.TimeFormat),
			MinLifetime:          setting.Params.MinLifetime,
			MaxClientToSelfDelay: setting.Params.MaxClientToSelfDelay,
			MinPaymentSizeMloki:  setting.Params.MinPaymentSizeMloki,
			MaxPaymentSizeMloki:  maxPaymentSize,
		}

		promise, err := createPromise(privateKey, params)
		if err != nil {
			log.Error().
				Err(err).
				Str("component", string(logging.LogComponentOpeningService)).
				Msg("GetFeeParamsMenu: Failed to create promise")
			return nil, err
		}

		params.Promise = *promise
		menu = append(menu, params)
	}

	sort.Slice(menu, func(i, j int) bool {
		if menu[i].MinFeeMloki == menu[j].MinFeeMloki {
			return menu[i].Proportional < menu[j].Proportional
		}

		return menu[i].MinFeeMloki < menu[j].MinFeeMloki
	})
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionResponse)).
		Int("items_count", len(menu)).
		Msg("GetFeeParamsMenu: Returning fee params menu")
	return menu, nil
}

func (s *openingService) ValidateOpeningFeeParams(params *core.OpeningFeeParams, publicKey *btcec.PublicKey) bool {
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionValidate)).
		Msg("ValidateOpeningFeeParams: Validating params")
	if params == nil {
		return false
	}

	err := verifyPromise(publicKey, params)
	if err != nil {
		return false
	}

	t, err := time.Parse(common.TimeFormat, params.ValidUntil)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentOpeningService)).
			Str("action", string(logging.LogActionFailure)).
			Str("valid_until", params.ValidUntil).
			Msg("ValidateOpeningFeeParams: Invalid time format")
		return false
	}

	if time.Now().UTC().After(t) {
		log.Debug().
			Str("component", string(logging.LogComponentOpeningService)).
			Str("action", string(logging.LogActionValidate)).
			Time("valid_until", t).
			Msg("ValidateOpeningFeeParams: Promise expired")
		return false
	}

	return true
}

func (s *openingService) IsCurrentChainFeeCheaper(token string, params *core.OpeningFeeParams) bool {
	settings, err := s.store.GetFeeParamsSettings(token)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentOpeningService)).
			Msg("IsCurrentChainFeeCheaper: Failed to get fee params settings")
		return false
	}

	for _, setting := range settings {
		if setting.Params.MinFeeMloki <= params.MinFeeMloki {
			return true
		}
	}

	return false
}

func createPromise(lspPrivateKey *btcec.PrivateKey, params *core.OpeningFeeParams) (*string, error) {
	hash, err := paramsHash(params)
	if err != nil {
		return nil, err
	}
	// Sign the hash with the private key of the LSP id.
	sig := ecdsa.SignCompact(lspPrivateKey, hash[:], true)

	promise := hex.EncodeToString(sig)
	return &promise, nil
}

func paramsHash(params *core.OpeningFeeParams) ([]byte, error) {
	// First hash all the values in the params in a fixed order.
	items := []interface{}{
		params.MinFeeMloki,
		params.Proportional,
		params.ValidUntil,
		params.MinLifetime,
		params.MaxClientToSelfDelay,
		params.MinPaymentSizeMloki,
		params.MaxPaymentSizeMloki,
	}
	blob, err := json.Marshal(items)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentOpeningService)).
			Msg("paramsHash: Failed to marshal params")
		return nil, err
	}
	hash := sha256.Sum256(blob)
	return hash[:], nil
}

func verifyPromise(lspPublicKey *btcec.PublicKey, params *core.OpeningFeeParams) error {
	hash, err := paramsHash(params)
	if err != nil {
		return err
	}
	sig, err := hex.DecodeString(params.Promise)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentOpeningService)).
			Msg("verifyPromise: Failed to decode promise signature")
		return err
	}
	pub, _, err := ecdsa.RecoverCompact(sig, hash)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentOpeningService)).
			Msg("verifyPromise: Failed to recover key from signature")
		return err
	}
	if !lspPublicKey.IsEqual(pub) {
		log.Error().
			Str("component", string(logging.LogComponentOpeningService)).
			Msg("verifyPromise: Signature not signed by us")
		return fmt.Errorf("invalid promise")
	}
	return nil
}

func (s *openingService) RegisterLsps2Buy(reg *core.Lsps2BuyRegistration) error {
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionRequest)).
		Uint64("scid", reg.Scid).
		Str("peer_id", reg.PeerId).
		Msg("RegisterLsps2Buy: Registering buy")
	return s.store.RegisterLsps2Buy(reg)
}

func (s *openingService) RegisterLsps1Order(order *core.Lsps1Order) error {
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionRequest)).
		Str("order_id", order.OrderId).
		Str("peer_id", order.PeerId).
		Msg("RegisterLsps1Order: Registering order")
	return s.store.RegisterLsps1Order(order)
}

func (s *openingService) GetLsps1Order(orderId string) (*core.Lsps1Order, error) {
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionRequest)).
		Str("order_id", orderId).
		Msg("GetLsps1Order: Fetching order")
	return s.store.GetLsps1Order(orderId)
}

func (s *openingService) RegisterLsps5Webhook(webhook *core.Lsps5Webhook) error {
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionRequest)).
		Str("app_name", webhook.AppName).
		Str("url", webhook.Url).
		Msg("RegisterLsps5Webhook: Registering webhook")
	return s.store.RegisterLsps5Webhook(webhook)
}

func (s *openingService) GetLsps5Webhooks(clientNodeId string) ([]*core.Lsps5Webhook, error) {
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionRequest)).
		Str("client_node_id", clientNodeId).
		Msg("GetLsps5Webhooks: Fetching webhooks")
	return s.store.GetLsps5Webhooks(clientNodeId)
}

func (s *openingService) RemoveLsps5Webhook(clientNodeId, appName string) error {
	log.Trace().
		Str("component", string(logging.LogComponentOpeningService)).
		Str("action", string(logging.LogActionRequest)).
		Str("client_node_id", clientNodeId).
		Str("app_name", appName).
		Msg("RemoveLsps5Webhook: Removing webhook")
	return s.store.RemoveLsps5Webhook(clientNodeId, appName)
}

func (s *openingService) CountLsps1Orders(clientNodeId string, state string, excludeExpiredBefore *time.Time) (int, error) {
	return s.store.CountLsps1Orders(clientNodeId, state, excludeExpiredBefore)
}

func (s *openingService) CountLsps5Webhooks(clientNodeId string) (int, error) {
	return s.store.CountLsps5Webhooks(clientNodeId)
}
