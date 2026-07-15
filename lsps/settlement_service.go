package lsps

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/flokiorg/go-flokicoin/wire"
	"github.com/flowgate-lsp/flspd/chain"
	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/history"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/platform/config"
	"github.com/flowgate-lsp/flspd/platform/logging"
	"github.com/flowgate-lsp/flspd/ports"
	"github.com/rs/zerolog/log"
)

// Lsps1SettlementService monitors LND invoice settlements and processes LSPS1 orders
type Lsps1SettlementService struct {
	client       lightning.Client
	openingStore ports.OpeningStore
	historyStore history.Store
	// config is an atomic ref, not a raw pointer, so it can be hot-swapped
	// by the dashboard's config-update handler without racing settlement's
	// own reads here (C-3). Load() it once per logical operation.
	config              *config.NodeConfigRef
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
	openingStore ports.OpeningStore,
	historyStore history.Store,
	config *config.NodeConfigRef,
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
			Str("component", string(logging.LogComponentSettlement)).
			Str("action", string(logging.LogActionSuccess)).
			Msg("LSPS1 Settlement: Service started, monitoring invoices")

		// Recover stuck orders in a separate goroutine to avoid blocking Start()
		go s.recoverStuckOrders()

		for {

			select {
			case <-s.ctx.Done():
				log.Info().
					Str("component", string(logging.LogComponentSettlement)).
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
		Str("component", string(logging.LogComponentSettlement)).
		Msg("LSPS1 Settlement: Stopping service...")
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
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

	// Update order state to PAID. This is a compare-and-set on the CREATED
	// state we just observed: if a concurrent writer (e.g. the startup
	// recovery sweep in recoverSettledCreatedOrders) already transitioned
	// this order, updated is false and we must not proceed — otherwise
	// both paths would call openChannelForOrder for the same order (F-1).
	updated, err := s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid), string(Lsps1OrderStateCreated))
	if err != nil {
		log.Error().
			Err(err).
			Str("order_id", order.OrderId).
			Msg("LSPS1 Settlement: Failed to update order state")
		return
	}
	if !updated {
		log.Info().
			Str("order_id", order.OrderId).
			Msg("LSPS1 Settlement: Order already transitioned out of CREATED by a concurrent writer, skipping")
		return
	}

	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
		Str("action", string(logging.LogActionSuccess)).
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

func (s *Lsps1SettlementService) openChannelForOrder(order *core.Lsps1Order) {
	// Snapshot once so this channel-open decision is made against one
	// internally consistent config even if the dashboard updates it
	// concurrently mid-flight (C-3).
	cfg := s.config.Load()

	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
		Str("action", string(logging.LogActionRequest)).
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
			Str("component", string(logging.LogComponentSettlement)).
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
			TargetConf: &cfg.TargetConf,
		}
	}

	capacity := uint64(order.LspBalanceLoki+order.ClientBalanceLoki) + uint64(cfg.AdditionalChannelCapacity)
	if capacity < cfg.MinChannelCapacityLoki {
		capacity = cfg.MinChannelCapacityLoki
	}

	// Open channel
	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
		Str("action", string(logging.LogActionRequest)).
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
			Ppm:           uint32(cfg.FeeRate * 1_000_000),
			BaseMloki:     cfg.BaseFeeMloki,
			TimeLockDelta: cfg.TimeLockDelta,
		},
	})

	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to open channel")

		// recoverStuckOrders calls openChannelForOrder for any PAID order
		// with no FundingTxid recorded yet -- which includes the case
		// where an *earlier* call to this same function already got past
		// the OpenChannel() call below (broadcasting a real funding tx
		// LND now holds as a pending channel) but crashed before
		// UpdateLsps1OrderFunding could persist that fact. Retrying
		// OpenChannel then collides with LND's own single-pending-channel-
		// per-peer limit and fails here -- reporting FAILED at this point
		// would silently orphan a channel the LSP already paid on-chain
		// fees to open. Check for it before giving up.
		if s.reconcileUnclaimedChannel(order, peerId) {
			return
		}

		// Update state to FAILED
		_, dbErr := s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStateFailed), string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid))
		if dbErr != nil {
			log.Error().Err(dbErr).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to update order state to FAILED")
		}

		if s.notificationService != nil {
			go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStateFailed), "", fmt.Sprintf("failed to open channel: %v", err))
		}
		return
	}

	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
		Str("action", string(logging.LogActionSuccess)).
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

	_, err = s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStateCompleted), string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid))
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to update order state to COMPLETED")
	}

	// Notify client of COMPLETED state with channel point
	if s.notificationService != nil {
		go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStateCompleted), channelPoint.String(), "")
	}
}

// recoverStuckOrders finds and retries orders that were never fulfilled:
//  1. PAID orders: flspd crashed after marking PAID but before opening the channel.
//  2. CREATED orders whose LND invoice is already SETTLED: flspd crashed after the
//     invoice was settled in LND but before it could write PAID to the DB. The
//     SubscribeInvoices replay handles this eventually, but an explicit LND lookup
//     on startup makes recovery deterministic.
func (s *Lsps1SettlementService) recoverStuckOrders() {
	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
		Msg("LSPS1 Settlement: Checking for stuck PAID orders to recover...")

	paidOrders, err := s.openingStore.GetAllPaidLsps1Orders()
	if err != nil {
		log.Error().Err(err).Msg("LSPS1 Settlement: Failed to retrieve stuck PAID orders")
		return
	}

	if len(paidOrders) == 0 {
		log.Debug().Msg("LSPS1 Settlement: No stuck orders found")
	} else {
		log.Info().
			Str("component", string(logging.LogComponentSettlement)).
			Int("count", len(paidOrders)).
			Msgf("LSPS1 Settlement: Found %d stuck PAID orders, recovering...", len(paidOrders))

		for _, order := range paidOrders {
			// F-8: a PAID order that already has funding recorded means
			// openChannelForOrder's OpenChannel call already succeeded on
			// some earlier run — the only way it's still PAID (not
			// COMPLETED) is that the COMPLETED state write itself failed
			// or never ran (e.g. flspd crashed between the two writes).
			// Calling openChannelForOrder again here would try to open a
			// SECOND channel for the same order. Reconcile against actual
			// LND state instead of blindly retrying.
			if order.FundingTxid != nil && order.FundingOutpoint != nil {
				s.reconcileFundedOrder(order)
				continue
			}
			s.openChannelForOrder(order)
		}
	}

	// Also check CREATED orders: if the invoice was settled in LND while flspd
	// was down, the order never progressed. Look each one up directly in LND.
	s.recoverSettledCreatedOrders()
}

// reconcileFundedOrder handles a PAID order that already has funding details
// recorded (F-8): rather than risk opening a duplicate channel, it verifies
// the previously-opened channel is actually active in LND and, if so,
// transitions the order straight to COMPLETED. If the channel can't be
// found (e.g. force-closed before recovery ran), this deliberately does
// NOT fall back to retrying OpenChannel — that ambiguous case (was the
// channel ever really there?) needs a human to look, not a guess that
// risks a duplicate open.
func (s *Lsps1SettlementService) reconcileFundedOrder(order *core.Lsps1Order) {
	peerId, err := hex.DecodeString(order.PeerId)
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to decode peer ID during funded-order reconciliation")
		return
	}

	outpoint, err := lightning.NewOutPointFromString(fmt.Sprintf("%s:%d", *order.FundingTxid, *order.FundingOutpoint))
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Stored funding outpoint is invalid, cannot reconcile")
		return
	}

	channelPoint := fmt.Sprintf("%s:%d", *order.FundingTxid, *order.FundingOutpoint)
	if _, err := s.client.GetChannel(peerId, *outpoint); err != nil {
		log.Warn().Err(err).
			Str("order_id", order.OrderId).
			Str("channel_point", channelPoint).
			Msg("LSPS1 Settlement: Order has recorded funding but no matching active channel found in LND — leaving PAID for manual review, not retrying OpenChannel")
		return
	}

	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
		Str("order_id", order.OrderId).
		Str("channel_point", channelPoint).
		Msg("LSPS1 Settlement: Recorded funding verified active in LND — reconciling order to COMPLETED")

	_, err = s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStateCompleted), string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid))
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to reconcile order state to COMPLETED")
		return
	}

	if s.notificationService != nil {
		go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStateCompleted), channelPoint, "")
	}
}

// reconcileUnclaimedChannel looks for exactly one channel (LND's
// ListChannels already merges in still-pending, not-yet-confirmed opens --
// see LndClient.ListChannels) to order's peer that isn't already recorded
// as the funding for some other order, and if found, treats it as the
// result of an earlier, crashed openChannelForOrder attempt: OpenChannel
// broadcast the funding transaction successfully, but the crash landed
// before UpdateLsps1OrderFunding could persist that. Reconciling here
// instead of retrying OpenChannel avoids both a collision with LND's own
// pending-channel limit and, worse, silently reporting FAILED for an
// order whose channel the LSP already paid on-chain fees to open.
//
// Deliberately conservative: zero or more than one unclaimed candidate is
// ambiguous, so this returns false and lets the caller fall through to
// its normal FAILED handling rather than guessing which channel (if any)
// belongs to this order -- the same philosophy reconcileFundedOrder
// already applies to the has-FundingTxid case above.
func (s *Lsps1SettlementService) reconcileUnclaimedChannel(order *core.Lsps1Order, peerId []byte) bool {
	channels, err := s.client.ListChannels()
	if err != nil {
		log.Warn().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to list channels while checking for an unclaimed prior attempt")
		return false
	}

	var peerChannels []*wire.OutPoint
	for _, ch := range channels {
		if ch.ChannelPoint == nil || !bytes.Equal(ch.PeerId, peerId) {
			continue
		}
		peerChannels = append(peerChannels, ch.ChannelPoint)
	}
	if len(peerChannels) == 0 {
		return false
	}

	// Only worth the extra DB round-trip once there's at least one
	// channel to this peer that might need attributing.
	claimed, err := s.claimedChannelPoints()
	if err != nil {
		log.Warn().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to list existing order funding records while checking for an unclaimed prior attempt")
		return false
	}

	var candidates []*wire.OutPoint
	for _, point := range peerChannels {
		if !claimed[point.String()] {
			candidates = append(candidates, point)
		}
	}

	if len(candidates) != 1 {
		if len(candidates) > 1 {
			log.Warn().
				Str("order_id", order.OrderId).
				Int("candidate_count", len(candidates)).
				Msg("LSPS1 Settlement: Multiple unclaimed channels found for this peer, cannot disambiguate which (if any) belongs to this order — leaving FAILED for manual review")
		}
		return false
	}

	point := candidates[0]
	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
		Str("order_id", order.OrderId).
		Str("channel_point", point.String()).
		Msg("LSPS1 Settlement: Found an unclaimed channel to this peer — treating as an earlier attempt's crash-orphaned open and reconciling instead of retrying OpenChannel")

	if err := s.openingStore.UpdateLsps1OrderFunding(order.OrderId, point.Hash.String(), int(point.Index)); err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to record reconciled funding details")
		return false
	}
	_, err = s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStateCompleted), string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid))
	if err != nil {
		log.Error().Err(err).Str("order_id", order.OrderId).Msg("LSPS1 Settlement: Failed to update reconciled order state to COMPLETED")
		return false
	}

	if s.notificationService != nil {
		go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStateCompleted), point.String(), "")
	}
	return true
}

// claimedChannelPoints returns the set of channel points (as
// wire.OutPoint.String()'s "hash:index" form) already recorded as funding
// for some Lsps1Order, so reconcileUnclaimedChannel never mis-attributes a
// channel that's already accounted for.
func (s *Lsps1SettlementService) claimedChannelPoints() (map[string]bool, error) {
	claimed := make(map[string]bool)
	const pageSize = 500
	for offset := 0; ; offset += pageSize {
		orders, err := s.openingStore.ListLsps1Orders(pageSize, offset)
		if err != nil {
			return nil, err
		}
		for _, o := range orders {
			if o.FundingTxid != nil && o.FundingOutpoint != nil {
				claimed[fmt.Sprintf("%s:%d", *o.FundingTxid, *o.FundingOutpoint)] = true
			}
		}
		if len(orders) < pageSize {
			break
		}
	}
	return claimed, nil
}

// recoverSettledCreatedOrders queries LND for any CREATED order whose invoice
// is already SETTLED, then processes it as if the settlement event just arrived.
func (s *Lsps1SettlementService) recoverSettledCreatedOrders() {
	createdOrders, err := s.openingStore.GetAllCreatedLsps1Orders()
	if err != nil {
		log.Error().Err(err).Msg("LSPS1 Settlement: Failed to retrieve CREATED orders for LND check")
		return
	}
	if len(createdOrders) == 0 {
		return
	}

	log.Debug().
		Str("component", string(logging.LogComponentSettlement)).
		Int("count", len(createdOrders)).
		Msg("LSPS1 Settlement: Checking CREATED orders against LND for missed settlements")

	for _, order := range createdOrders {
		if order.PaymentInvoice == "" {
			continue
		}
		state, err := s.client.LookupInvoiceState(s.ctx, order.PaymentInvoice)
		if err != nil {
			log.Warn().Err(err).
				Str("order_id", order.OrderId).
				Msg("LSPS1 Settlement: Could not look up invoice state in LND, skipping")
			continue
		}

		stateStr := "OPEN"
		switch state {
		case lightning.InvoiceStateSettled:
			stateStr = "SETTLED"
		case lightning.InvoiceStateCanceled:
			stateStr = "CANCELED"
		case lightning.InvoiceStateAccepted:
			stateStr = "ACCEPTED"
		}

		log.Info().
			Str("component", string(logging.LogComponentSettlement)).
			Str("order_id", order.OrderId).
			Str("invoice_state", stateStr).
			Int64("order_total_loki", order.OrderTotalLoki).
			Msg("LSPS1 Settlement: CREATED order invoice state")

		switch state {
		case lightning.InvoiceStateSettled:
			log.Info().
				Str("component", string(logging.LogComponentSettlement)).
				Str("order_id", order.OrderId).
				Str("peer_id", order.PeerId).
				Msg("LSPS1 Settlement: Found CREATED order with settled invoice — recovering")
			s.processSettledOrder(order)
		case lightning.InvoiceStateCanceled:
			log.Warn().
				Str("component", string(logging.LogComponentSettlement)).
				Str("order_id", order.OrderId).
				Msg("LSPS1 Settlement: Invoice is CANCELED — order cannot be fulfilled")
		}
	}
}

// processSettledOrder is the DB-update + channel-open logic factored out of
// processSettledInvoice so that recoverSettledCreatedOrders can reuse it
// without needing the full InvoiceUpdate.
func (s *Lsps1SettlementService) processSettledOrder(order *core.Lsps1Order) {
	// Compare-and-set on CREATED, mirroring processSettledInvoice: this is
	// the other side of the F-1 race, where the live SubscribeInvoices
	// event and this startup recovery sweep can both observe the same
	// order as CREATED. Only the winner proceeds to open a channel.
	updated, err := s.openingStore.UpdateLsps1OrderState(order.OrderId, string(Lsps1OrderStatePaid), string(Lsps1OrderStatePaid), string(Lsps1OrderStateCreated))
	if err != nil {
		log.Error().Err(err).
			Str("order_id", order.OrderId).
			Msg("LSPS1 Settlement: Failed to update order state to PAID during recovery")
		return
	}
	if !updated {
		log.Info().
			Str("order_id", order.OrderId).
			Msg("LSPS1 Settlement: Order already transitioned out of CREATED by a concurrent writer, skipping recovery")
		return
	}

	log.Info().
		Str("component", string(logging.LogComponentSettlement)).
		Str("order_id", order.OrderId).
		Str("peer_id", order.PeerId).
		Msg("LSPS1 Settlement: Order marked as PAID (recovery)")

	if s.notificationService != nil {
		go s.notificationService.NotifyOrderStateChanged(order.PeerId, order.OrderId, string(Lsps1OrderStatePaid), "", "")
	}

	s.openChannelForOrder(order)
}
