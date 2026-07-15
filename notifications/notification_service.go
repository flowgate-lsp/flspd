package notifications

import (
	"context"
	"sync"
	"time"

	"github.com/flowgate-lsp/flspd/core"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/flowgate-lsp/flspd/platform/logging"
	"github.com/flowgate-lsp/flspd/ports"
	"github.com/rs/zerolog/log"
)

// Notifier is an interface for sending LSPS5 webhook notifications and other alerts.
type Notifier interface {
	Notify(pubkey string, paymenthash string, nodePrivKey interface{}) (bool, error)
	NotifyWebhookRegistered(pubkey string, webhook *core.Lsps5Webhook) bool
	NotifyOrderStateChanged(pubkey string, orderId string, state string, channelPoint string, errorMsg string) (bool, error)
	NotifyExpirySoon(pubkey string, timeout int, nodePrivKey interface{}, paymentHash string) (bool, error)
	NotifyLiquidityManagementRequest(pubkey string, nodePrivKey interface{}) (bool, error)
	NotifyOnionMessageIncoming(pubkey string, nodePrivKey interface{}) (bool, error)
}

var minNotificationInterval time.Duration = time.Second * 30
var cleanInterval time.Duration = time.Minute * 2

type NotificationService struct {
	mtx                 sync.Mutex
	recentNotifications map[string]time.Time
	openingStore        ports.OpeningStore
	nodesService        ports.NodesService
	store               Store
	transports          map[TransportType]NotificationTransport
}

func NewNotificationService(openingStore ports.OpeningStore, nodesService ports.NodesService, store Store) *NotificationService {
	s := &NotificationService{
		openingStore:        openingStore,
		recentNotifications: make(map[string]time.Time),
		nodesService:        nodesService,
		store:               store,
		transports:          make(map[TransportType]NotificationTransport),
	}

	// Register default transports
	// Note: Nostr transport will be added manually if configured
	s.transports[TransportTypePush] = NewHTTPSTransport(nil) // Private key set later

	return s
}

// AddTransport adds a custom transport to the service
func (s *NotificationService) AddTransport(t NotificationTransport) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.transports[t.Type()] = t
}

// SetNodePrivKey sets the private key for all transports that need it
func (s *NotificationService) SetNodePrivKey(privKey interface{}) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if t, ok := s.transports[TransportTypePush].(*HTTPSTransport); ok {
		t.nodePrivKey = privKey
	}
}

func (s *NotificationService) Start(ctx context.Context) {
	// Start retry worker
	go s.runWorker(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(cleanInterval):
		}

		s.mtx.Lock()
		for id, lastTime := range s.recentNotifications {
			if lastTime.Add(minNotificationInterval).Before(time.Now()) {
				delete(s.recentNotifications, id)
			}
		}
		s.mtx.Unlock()
	}
}

func (s *NotificationService) runWorker(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processPending(ctx)
		}
	}
}

func (s *NotificationService) processPending(ctx context.Context) {
	items, err := s.store.GetPending(ctx, 50)
	if err != nil {
		log.Error().Err(err).Msg("Notifications: failed to fetch pending items")
		return
	}

	for _, item := range items {
		// Prepare temp webhook
		webhook := &core.Lsps5Webhook{
			Url:       item.TargetURL,
			Transport: item.Transport,
		}

		// Attempt delivery
		// We use a short context for single retry
		retryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		success := s.deliverOne(retryCtx, item.Method, item.Params, webhook)
		cancel()

		var status string
		var nextRetry time.Time

		if success {
			status = "completed"
		} else {
			// Exponential backoff: 10s * 2^attempts
			backoff := time.Duration(10*(1<<item.Attempts)) * time.Second
			if backoff > 24*time.Hour {
				status = "permanent_fail"
			} else {
				status = "failed"
				nextRetry = time.Now().Add(backoff)
			}
		}

		_ = s.store.UpdateStatus(ctx, item.ID, status, nextRetry)
	}
}

// Notify sends an lsps5.payment_incoming notification to all registered webhooks
// for the given pubkey. This is compliant with bLIP-55 (LSPS5) specification.
// nodePrivKey should be a *btcec.PrivateKey for signing the notification.
func (s *NotificationService) Notify(
	pubkey string,
	paymenthash string,
	nodePrivKey interface{},
) (bool, error) {
	s.mtx.Lock()
	lastTime, ok := s.recentNotifications[paymentIdentifier(pubkey, paymenthash)]
	s.mtx.Unlock()
	if ok && lastTime.Add(minNotificationInterval).After(time.Now()) {
		// Treat as if we notified if the notification was recent.
		return true, nil
	}

	webhooks, err := s.openingStore.GetLsps5Webhooks(pubkey)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentNotifications)).
			Str("action", string(logging.LogActionFailure)).
			Str("pubkey", pubkey).
			Msg("Notifications: Failed to fetch webhooks")
		return false, err
	}

	log.Debug().
		Str("component", string(logging.LogComponentNotifications)).
		Str("action", string(logging.LogActionRequest)).
		Str("pubkey", pubkey).
		Int("webhooks_count", len(webhooks)).
		Msg("Notifications: Dispatching lsps5.payment_incoming to all webhooks")

	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(webhooks))
	notified := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)

	for _, w := range webhooks {
		go func(w *core.Lsps5Webhook) {
			defer wg.Done()
			log.Trace().
				Str("component", string(logging.LogComponentNotifications)).
				Str("action", "dispatch_one").
				Str("url", w.Url).
				Str("transport", w.Transport).
				Msg("Notifications: dispatching to single webhook")
			currentNotified := s.enqueueAndDeliver(ctx, pubkey, w, lightning.Lsps5MethodPaymentIncoming, &lightning.Lsps5PaymentIncomingParams{
				PaymentHash: paymenthash,
			})
			if currentNotified {
				mtx.Lock()
				notified = true
				mtx.Unlock()
			}
		}(w)
	}

	wg.Wait()
	cancel()

	log.Debug().
		Str("component", string(logging.LogComponentNotifications)).
		Str("action", string(logging.LogActionResponse)).
		Str("pubkey", pubkey).
		Bool("notified", notified).
		Msg("Notifications: Finished dispatching notifications")
	if notified {
		s.mtx.Lock()
		s.recentNotifications[paymentIdentifier(pubkey, paymenthash)] = time.Now()
		s.mtx.Unlock()
	}
	return notified, nil
}

// NotifyWebhookRegistered sends the lsps5.webhook_registered notification to a single
// newly registered webhook. Per bLIP-55, this is only sent to the newly registered
// webhook, not to all webhooks.
func (s *NotificationService) NotifyWebhookRegistered(
	pubkey string,
	webhook *core.Lsps5Webhook,
) bool {
	log.Info().
		Str("component", string(logging.LogComponentNotifications)).
		Str("action", string(logging.LogActionRequest)).
		Str("pubkey", pubkey).
		Str("url", webhook.Url).
		Msg("Notifications: Sending lsps5.webhook_registered to newly registered webhook")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)
	defer cancel()

	return s.enqueueAndDeliver(ctx, pubkey, webhook, lightning.Lsps5MethodWebhookRegistered, map[string]interface{}{})
}

// NotifyExpirySoon sends the lsps5.expiry_soon notification to all registered webhooks
// with the block height timeout parameter.
func (s *NotificationService) NotifyExpirySoon(
	pubkey string,
	timeout int,
	nodePrivKey interface{},
	paymentHash string,
) (bool, error) {
	webhooks, err := s.openingStore.GetLsps5Webhooks(pubkey)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentNotifications)).
			Str("action", string(logging.LogActionFailure)).
			Str("pubkey", pubkey).
			Msg("Notifications: Failed to fetch webhooks for expiry_soon")
		return false, err
	}

	params := &lightning.Lsps5ExpirySoonParams{Timeout: timeout}

	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(webhooks))
	notified := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)

	for _, w := range webhooks {
		go func(w *core.Lsps5Webhook) {
			defer wg.Done()
			currentNotified := s.enqueueAndDeliver(ctx, pubkey, w, lightning.Lsps5MethodExpirySoon, params)
			if currentNotified {
				mtx.Lock()
				notified = true
				mtx.Unlock()
			}
		}(w)
	}

	wg.Wait()
	cancel()
	return notified, nil
}

// NotifyLiquidityManagementRequest sends the lsps5.liquidity_management_request notification.
func (s *NotificationService) NotifyLiquidityManagementRequest(
	pubkey string,
	nodePrivKey interface{},
) (bool, error) {
	webhooks, err := s.openingStore.GetLsps5Webhooks(pubkey)
	if err != nil {
		return false, err
	}

	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(webhooks))
	notified := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)

	for _, w := range webhooks {
		go func(w *core.Lsps5Webhook) {
			defer wg.Done()
			currentNotified := s.enqueueAndDeliver(ctx, pubkey, w, lightning.Lsps5MethodLiquidityManagementRequest, map[string]interface{}{})
			if currentNotified {
				mtx.Lock()
				notified = true
				mtx.Unlock()
			}
		}(w)
	}

	wg.Wait()
	cancel()
	return notified, nil
}

// OrderStateChangedParams contains parameters for the order_state_changed notification
type OrderStateChangedParams struct {
	OrderId      string `json:"order_id"`
	State        string `json:"state"`
	ChannelPoint string `json:"channel_point,omitempty"`
	Error        string `json:"error,omitempty"`
}

// NotifyOrderStateChanged sends the lsps5.order_state_changed notification
// to all registered webhooks when an order transitions state (PAID, COMPLETED, etc.)
func (s *NotificationService) NotifyOrderStateChanged(
	pubkey string,
	orderId string,
	state string,
	channelPoint string,
	errorMsg string,
) (bool, error) {
	webhooks, err := s.openingStore.GetLsps5Webhooks(pubkey)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentNotifications)).
			Str("action", string(logging.LogActionFailure)).
			Str("pubkey", pubkey).
			Msg("Notifications: Failed to fetch webhooks for order_state_changed")
		return false, err
	}

	params := &OrderStateChangedParams{
		OrderId:      orderId,
		State:        state,
		ChannelPoint: channelPoint,
		Error:        errorMsg,
	}

	log.Debug().
		Str("component", string(logging.LogComponentNotifications)).
		Str("action", string(logging.LogActionRequest)).
		Str("pubkey", pubkey).
		Str("order_id", orderId).
		Str("state", state).
		Int("webhooks_count", len(webhooks)).
		Msg("Notifications: Dispatching lsps5.order_state_changed to all webhooks")

	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(webhooks))
	notified := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)

	for _, w := range webhooks {
		go func(w *core.Lsps5Webhook) {
			defer wg.Done()
			log.Trace().
				Str("component", string(logging.LogComponentNotifications)).
				Str("action", "dispatch_one").
				Str("url", w.Url).
				Str("transport", w.Transport).
				Msg("Notifications: dispatching to single webhook")
			currentNotified := s.enqueueAndDeliver(ctx, pubkey, w, lightning.Lsps5MethodOrderStateChanged, params)
			if currentNotified {
				mtx.Lock()
				notified = true
				mtx.Unlock()
			}
		}(w)
	}

	wg.Wait()
	cancel()
	return notified, nil
}

// NotifyOnionMessageIncoming sends the lsps5.onion_message_incoming notification.
func (s *NotificationService) NotifyOnionMessageIncoming(
	pubkey string,
	nodePrivKey interface{},
) (bool, error) {
	webhooks, err := s.openingStore.GetLsps5Webhooks(pubkey)
	if err != nil {
		return false, err
	}

	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(webhooks))
	notified := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)

	for _, w := range webhooks {
		go func(w *core.Lsps5Webhook) {
			defer wg.Done()
			currentNotified := s.enqueueAndDeliver(ctx, pubkey, w, lightning.Lsps5MethodOnionMessageIncoming, map[string]interface{}{})
			if currentNotified {
				mtx.Lock()
				notified = true
				mtx.Unlock()
			}
		}(w)
	}

	wg.Wait()
	cancel()
	return notified, nil
}

// deliverOne delivers a notification to a single webhook using its preferred transport.
func (s *NotificationService) deliverOne(
	ctx context.Context,
	method string,
	params interface{},
	webhook *core.Lsps5Webhook,
) bool {
	s.mtx.Lock()
	transport, ok := s.transports[TransportType(webhook.Transport)]
	s.mtx.Unlock()

	log.Trace().
		Str("component", string(logging.LogComponentNotifications)).
		Str("action", "deliver_one").
		Str("method", method).
		Str("url", webhook.Url).
		Str("transport_type", webhook.Transport).
		Msg("Notifications: deliverOne started")

	if !ok {
		log.Error().
			Str("component", string(logging.LogComponentNotifications)).
			Str("action", string(logging.LogActionFailure)).
			Str("transport", webhook.Transport).
			Msg("Notifications: Unsupported transport type")
		return false
	}

	err := transport.Send(ctx, webhook.Url, method, params)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(logging.LogComponentNotifications)).
			Str("action", string(logging.LogActionFailure)).
			Str("url", webhook.Url).
			Str("method", method).
			Str("transport", string(transport.Type())).
			Msg("Notifications: Delivery failed")
		return false
	}

	log.Info().
		Str("component", string(logging.LogComponentNotifications)).
		Str("action", string(logging.LogActionSuccess)).
		Str("method", method).
		Str("transport", string(transport.Type())).
		Msg("Notifications: Delivery succeeded")

	return true
}

func (s *NotificationService) enqueueAndDeliver(
	ctx context.Context,
	pubkey string,
	webhook *core.Lsps5Webhook,
	method string,
	params interface{},
) bool {
	// 1. Enqueue
	id, err := s.store.Enqueue(ctx, pubkey, webhook.Url, webhook.Transport, method, params)
	if err != nil {
		log.Error().Err(err).Str("url", webhook.Url).Msg("Notifications: failed to enqueue")
		// If enqueue fails, we might still try to deliver?
		// Or fail safe? Let's try to deliver anyway but we won't have persistence.
		return s.deliverOne(ctx, method, params, webhook)
	}

	// 2. Attempt Delivery
	success := s.deliverOne(ctx, method, params, webhook)

	// 3. Update Status
	var status string
	var nextRetry time.Time

	if success {
		status = "completed"
	} else {
		status = "failed"
		nextRetry = time.Now().Add(10 * time.Second) // Initial retry
	}

	// Use background context for database update to ensure it isn't cancelled by the delivery timeout
	updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = s.store.UpdateStatus(updateCtx, id, status, nextRetry)

	return success
}

func paymentIdentifier(pubkey string, paymentHash string) string {
	return pubkey + "|" + paymentHash
}
