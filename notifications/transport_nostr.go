package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"

	nostrlsps5 "github.com/flowgate-lsp/nostr-lsps5"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog/log"
)

// NostrTransport implements NotificationTransport for Nostr encrypted LSPS5 delivery.
type NostrTransport struct {
	lspPrivKey string // hex private key
	lsps5      *nostrlsps5.LSPS5
	relayPool  *RelayPool
}

// NewNostrTransport creates a new Nostr transport
func NewNostrTransport(lspPrivKey string) *NostrTransport {
	return &NostrTransport{
		lspPrivKey: lspPrivKey,
		lsps5:      nostrlsps5.NewLSPS5(lspPrivKey),
		relayPool:  NewRelayPool(),
	}
}

// Type returns TransportTypeNostr
func (t *NostrTransport) Type() TransportType {
	return TransportTypeNostr
}

// Send delivers a notification via Nostr kind 31055.
// targetUri is the recipient's Nostr URI (nostr+lsps5://...).
func (t *NostrTransport) Send(ctx context.Context, targetUri string, method string, params interface{}) error {
	if t.lspPrivKey == "" {
		return fmt.Errorf("LSP Nostr private key not configured")
	}

	// 1. Parse URI
	targetPubkey, relays, err := nostrlsps5.ParseLsp5Uri(targetUri)
	if err != nil {
		return fmt.Errorf("invalid nostr URI: %v", err)
	}

	if len(relays) == 0 {
		return fmt.Errorf("no relays specified in URI")
	}

	// 2. Create Event (Pure Logic - returns unsigned)
	ev, err := t.lsps5.CreateNotificationEvent(targetPubkey, method, params)
	if err != nil {
		return fmt.Errorf("failed to create notification event: %v", err)
	}

	// 3. Sign the event (Transport responsibility)
	if err := ev.Sign(t.lspPrivKey); err != nil {
		return fmt.Errorf("failed to sign event: %v", err)
	}

	// 4. Publish to Relays (Transport Logic)
	var wg sync.WaitGroup
	var success bool
	var mtx sync.Mutex

	for _, url := range relays {
		wg.Add(1)
		go func(relayUrl string) {
			defer wg.Done()
			relay, err := t.relayPool.GetOrConnect(ctx, relayUrl)
			if err != nil {
				log.Warn().Err(err).Str("relay", relayUrl).Msg("Failed to connect to Nostr relay")
				return
			}

			// Publish
			err = relay.Publish(ctx, *ev)
			if err != nil {
				log.Warn().Err(err).Str("relay", relayUrl).Msg("Failed to publish to Nostr relay")
				return
			}

			mtx.Lock()
			success = true
			mtx.Unlock()
			log.Debug().Str("relay", relayUrl).Msg("Published LSPS5 notification")
		}(url)
	}

	// Wait logic could be optimized to not wait for ALL if one succeeds, but simple WG is fine for now
	// To avoid blocking too long, we can wrap in a channel with timeout if context doesn't handle it
	wg.Wait()

	if !success {
		return fmt.Errorf("failed to publish to any relay")
	}

	return nil
}

// RelayPool manages persistent connections to relays
type RelayPool struct {
	relays sync.Map // map[string]*nostr.Relay
}

func NewRelayPool() *RelayPool {
	return &RelayPool{}
}

func (rp *RelayPool) GetOrConnect(ctx context.Context, url string) (*nostr.Relay, error) {
	if val, ok := rp.relays.Load(url); ok {
		relay := val.(*nostr.Relay)
		if relay.IsConnected() {
			return relay, nil
		}
		// If not connected, maybe try to reconnect? nostr.Relay has internal reconnect logic often?
		// But if CreateConnection failed earlier, it might be dead.
		// For simplicity, if not connected, let's try to connect again or assume the library handles it.
		// go-nostr relays usually maintain connection if created with RelayConnect.
		// If it's closed, we might need to recreate.
		return relay, nil
	}

	// Connect
	// Set a timeout for connection
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	relay, err := nostr.RelayConnect(connectCtx, url)
	if err != nil {
		return nil, err
	}
	rp.relays.Store(url, relay)
	return relay, nil
}
