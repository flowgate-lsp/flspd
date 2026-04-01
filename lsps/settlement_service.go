package lsps

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/rs/zerolog/log"
)

// Lsps1SettlementService monitors LND invoice settlements and processes LSPS1 orders
type Lsps1SettlementService struct {
	client              lightning.Client
	openingStore        common.OpeningStore
	historyStore        history.Store
	config              *config.NodeConfig
	feeEstimator        chain.FeeEstimator
	feeStrategy         chain.FeeStrategy
	notificationService NotificationService
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
}

// NewLsps1SettlementService creates a new settlement monitoring service
func NewLsps1SettlementService(
	client lightning.Client,
	openingStore common.OpeningStore,
	historyStore history.Store,
	config *config.NodeConfig,
	feeEstimator chain.FeeEstimator,
	feeStrategy chain.FeeStrategy,
	notificationService NotificationService,
) *Lsps1SettlementService {
	return &Lsps1SettlementService{
		client:              client,
		openingStore:        openingStore,
		historyStore:        historyStore,
		config:              config,
		feeEstimator:        feeEstimator,
		feeStrategy:         feeStrategy,
		notificationService: notificationService,
	}
}

// Start begins monitoring invoice settlements
func (s *Lsps1SettlementService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	invoiceChan, errChan, err := s.client.SubscribeInvoices()
	if err != nil {
		log.Error().Err(err).Msg("LSPS1 Settlement: Failed to subscribe to invoices")
		return err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Info().
			Str("component", string(common.LogComponentSettlement)).
			Str("action", string(common.LogActionSuccess)).
			Msg("LSPS1 Settlement: Service started, monitoring invoices")

		// Recover stuck orders in a separate goroutine to avoid blocking Start()
		go s.recoverStuckOrders()

		for {

			select {
			case <-s.ctx.Done():
				log.Info().
					Str("component", string(common.LogComponentSettlement)).
					Msg("LSPS1 Settlement: Service stopped")
				return

			case err := <-errChan:
				if err != nil {
					log.Error().Err(err).Msg("LSPS1 Settlement: Invoice subscription error")
					// TODO: Implement reconnection logic with exponential backoff
				}

			case update := <-invoiceChan:
				if update == nil {
					continue
				}
				if update.State == lightning.InvoiceStateSettled {
					s.processSettledInvoice(update)
				}
			}
		}
	}()

	return nil
}

// Stop gracefully shuts down the settlement service
func (s *Lsps1SettlementService) Stop() {
	log.Info().
		Str("component", string(common.LogComponentSettlement)).
		Msg("LSPS1 Settlement: Stopping service...")
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	log.Info().
		Str("component", string(common.LogComponentSettlement)).
		Msg("LSPS1 Settlement: Service stopped successfully")
}

// processSettledInvoice handles a settled invoice event
func (s *Lsps1SettlementService) processSettledInvoice(update *lightning.InvoiceUpdate) {
	pr := update.PaymentRequest
	if len(pr) > 20 {
		pr = pr[:20] + "..."
	}
	log.Debug().
		Str("payment_request", pr).
		Uint64("amount_mloki", update.AmountMloki).
		Msg("LSPS1 Settlement: Processing settled invoice")

	// Lookup order by payment invoice
	order, err := s.openingStore.GetLsps1OrderByPaymentInvoice(update.PaymentRequest)
	if err != nil {
		// Not all invoices are LSPS1 orders, skip silently
		log.Trace().Err(err).Msg("LSPS1 Settlement: Invoice not found in LSPS1 orders (expected for non-LSPS1 invoices)")
		return
	}

	// Check if order is in CREATED state (prevent duplicate processing)
	if order.OrderState != string(Lsps1OrderStateCreated) {
		log.Warn().
			Str("order_id", order.OrderId).
			Str("current_state", order.OrderState).
			Msg("LSPS1 Settlement: Order not in CREATED state, skipping")
		return
	}

	// Update order state to PAID
	err = s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid))
	if err != nil {
		log.Error().
			Err(err).
			Str("order_id", order.OrderId).
			Msg("LSPS1 Settlement: Failed to update order state")
		return
	}

	log.Info().
		Str("component", string(common.LogComponentSettlement)).
		Str("action", string(common.LogActionSuccess)).
		Str("order_id", order.OrderId).
		Str("peer_id", order.PeerId).
		Int64("client_balance", order.ClientBalanceLoki).
		Msg("LSPS1 Settlement: Order marked as PAID")

	// Notify client of state change
	if s.notificationService != nil {
		go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStatePaid), "", "")
	}

	// Trigger automatic channel opening
	go s.openChannelForOrder(order)
}

func (s *Lsps1SettlementService) openChannelForOrder(order *common.Lsps1Order) {
	log.Info().
		Str("component", string(common.LogComponentSettlement)).
		Str("action", string(common.LogActionRequest)).
		Str("order_id", order.OrderId).
		Str("peer_id", order.PeerId).
		Msg("LSPS1 Settlement: Attempting automatic channel opening")

	peerId, err := hex.DecodeString(order.PeerId)
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to decode peer ID")
		// Ideally notify failure here too, but can't without peerId maybe?
		// We have peerId string in order.PeerId.
		if s.notificationService != nil {
			go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStateFailed), "", "failed to decode peer id")
		}
		return
	}

	// Check if peer is connected
	connected, err := s.client.IsConnected(peerId)
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to check peer connectivity")
		if s.notificationService != nil {
			go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStateFailed), "", fmt.Sprintf("failed to check connectivity: %v", err))
		}
		return
	}

	if !connected {
		log.Info().
			Str("component", string(common.LogComponentSettlement)).
			Str("order_id", order.OrderId).
			Str("peer_id", order.PeerId).
			Msg("LSPS1 Settlement: Peer not connected, skipping automatic open (will be handled by JIT or manual)")
		// Not a failure per se, just deferred. Do not notify FAILED.
		return
	}

	// Estimate fee
	fee, err := s.feeEstimator.EstimateFeeRate(context.Background(), s.feeStrategy)
	if err != nil || fee == nil {
		log.Warn().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Fee estimation failed, using target conf from config")
		fee = &chain.FeeEstimation{
			TargetConf: &s.config.TargetConf,
		}
	}

	capacity := uint64(order.LspBalanceLoki+order.ClientBalanceLoki) + uint64(s.config.AdditionalChannelCapacity)
	if capacity < s.config.MinChannelCapacityLoki {
		capacity = s.config.MinChannelCapacityLoki
	}

	// Open channel
	log.Info().
		Str("component", string(common.LogComponentSettlement)).
		Str("action", string(common.LogActionRequest)).
		Str("order_id", order.OrderId).
		Uint64("capacity_loki", capacity).
		Uint64("push_loki", uint64(order.ClientBalanceLoki)).
		Msg("LSPS1 Settlement: Calling OpenChannel")

	channelPoint, err := s.client.OpenChannel(&lightning.OpenChannelRequest{
		Destination:     peerId,
		CapacityLoki:    capacity,
		PushLoki:        uint64(order.ClientBalanceLoki),
		AnnounceChannel: order.AnnounceChannel,
		FeeLokiPerVByte: fee.LokiPerVByte,
		TargetConf:      fee.TargetConf,
		RoutingPolicy: &lightning.RoutingPolicy{
			Ppm:           uint32(s.config.FeeRate * 1_000_000),
			BaseMloki:     s.config.BaseFeeMloki,
			TimeLockDelta: s.config.TimeLockDelta,
		},
	})

	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to open channel")

		// Update state to FAILED
		dbErr := s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStateFailed), string(Lsps1OrderStatePaid))
		if dbErr != nil {
			log.Error().Err(dbErr).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to update order state to FAILED")
		}

		if s.notificationService != nil {
			go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStateFailed), "", fmt.Sprintf("failed to open channel: %v", err))
		}
		return
	}

	log.Info().
		Str("component", string(common.LogComponentSettlement)).
		Str("action", string(common.LogActionSuccess)).
		Str("order_id", order.OrderId).
		Str("channel_point", channelPoint.String()).
		Msg("LSPS1 Settlement: Channel opening initiated")

	// Update order with funding details and set to COMPLETED
	// Wait a bit for the channel to be registered in LND if needed, but OpenChannel returns immediately with the point
	outpointIdx := int(channelPoint.Index)
	txid := channelPoint.Hash.String()

	err = s.openingStore.UpdateLsps1OrderFunding(order.OrderId, txid, outpointIdx)
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to update order funding details")
	}

	err = s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStateCompleted), string(Lsps1OrderStatePaid))
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to update order state to COMPLETED")
	}

	// Notify client of COMPLETED state with channel point
	if s.notificationService != nil {
		go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStateCompleted), channelPoint.String(), "")
	}
}

// recoverStuckOrders finds and retries orders that were PAID but never fulfilled
// (e.g. because flspd crashed before opening the channel).
func (s *Lsps1SettlementService) recoverStuckOrders() {
	log.Info().
		Str("component", string(common.LogComponentSettlement)).
		Msg("LSPS1 Settlement: Checking for stuck PAID orders to recover...")

	orders, err := s.openingStore.GetAllPaidLsps1Orders()
	if err != nil {
		log.Error().Err(err).Msg("LSPS1 Settlement: Failed to retrieve stuck orders")
		return
	}

	if len(orders) == 0 {
		log.Debug().Msg("LSPS1 Settlement: No stuck orders found")
		return
	}

	log.Info().
		Str("component", string(common.LogComponentSettlement)).
		Int("count", len(orders)).
		Msgf("LSPS1 Settlement: Found %d stuck orders, recovering...", len(orders))

	for _, order := range orders {
		// Process each order sequentially to avoid overwhelming LND on startup
		s.openChannelForOrder(order)
	}
}

