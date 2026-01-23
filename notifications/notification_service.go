package notifications

import (
	"context"
	"sync"
	"time"

	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/rs/zerolog/log"
)

var minNotificationInterval time.Duration = time.Second * 30
var cleanInterval time.Duration = time.Minute * 2

type NotificationService struct {
	mtx                 sync.Mutex
	recentNotifications map[string]time.Time
	openingStore        common.OpeningStore
	nodesService        common.NodesService
	transports          map[TransportType]NotificationTransport
}

func NewNotificationService(openingStore common.OpeningStore, nodesService common.NodesService) *NotificationService {
	s := &NotificationService{
		openingStore:        openingStore,
		recentNotifications: make(map[string]time.Time),
		nodesService:        nodesService,
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
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Str("pubkey", pubkey).
			Msg("Notifications: Failed to fetch webhooks")
		return false, err
	}

	log.Debug().
		Str("component", string(common.LogComponentNotifications)).
		Str("action", string(common.LogActionRequest)).
		Str("pubkey", pubkey).
		Int("webhooks_count", len(webhooks)).
		Msg("Notifications: Dispatching lsps5.payment_incoming to all webhooks")

	var mtx sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(webhooks))
	notified := false
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)

	for _, w := range webhooks {
		go func(w *common.Lsps5Webhook) {
			defer wg.Done()
			currentNotified := s.deliverOne(ctx, lightning.Lsps5MethodPaymentIncoming, nil, w)
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
		Str("component", string(common.LogComponentNotifications)).
		Str("action", string(common.LogActionResponse)).
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
	webhook *common.Lsps5Webhook,
) bool {
	log.Debug().
		Str("component", string(common.LogComponentNotifications)).
		Str("action", string(common.LogActionRequest)).
		Str("pubkey", pubkey).
		Str("url", webhook.Url).
		Msg("Notifications: Sending lsps5.webhook_registered to newly registered webhook")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)
	defer cancel()

	return s.deliverOne(ctx, lightning.Lsps5MethodWebhookRegistered, nil, webhook)
}

// NotifyExpirySoon sends the lsps5.expiry_soon notification to all registered webhooks
// with the block height timeout parameter.
func (s *NotificationService) NotifyExpirySoon(
	pubkey string,
	timeout int,
	nodePrivKey interface{},
) (bool, error) {
	webhooks, err := s.openingStore.GetLsps5Webhooks(pubkey)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
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
		go func(w *common.Lsps5Webhook) {
			defer wg.Done()
			currentNotified := s.deliverOne(ctx, lightning.Lsps5MethodExpirySoon, params, w)
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
		go func(w *common.Lsps5Webhook) {
			defer wg.Done()
			currentNotified := s.deliverOne(ctx, lightning.Lsps5MethodLiquidityManagementRequest, nil, w)
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
		go func(w *common.Lsps5Webhook) {
			defer wg.Done()
			currentNotified := s.deliverOne(ctx, lightning.Lsps5MethodOnionMessageIncoming, nil, w)
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
	webhook *common.Lsps5Webhook,
) bool {
	s.mtx.Lock()
	transport, ok := s.transports[TransportType(webhook.Transport)]
	s.mtx.Unlock()

	if !ok {
		log.Error().
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Str("transport", webhook.Transport).
			Msg("Notifications: Unsupported transport type")
		return false
	}

	err := transport.Send(ctx, webhook.Url, method, params)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", string(common.LogComponentNotifications)).
			Str("action", string(common.LogActionFailure)).
			Str("url", webhook.Url).
			Str("method", method).
			Str("transport", string(transport.Type())).
			Msg("Notifications: Delivery failed")
		return false
	}

	return true
}

func paymentIdentifier(pubkey string, paymentHash string) string {
	return pubkey + "|" + paymentHash
}
