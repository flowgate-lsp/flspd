package notifications

import (
	"encoding/hex"
	"sync"
	"time"

	"github.com/flowgate-lsp/flspd/config"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/rs/zerolog/log"
)

const (
	// Check expiry every 10 minutes
	expiryCheckInterval = 10 * time.Minute
	// Notify when expiry is within 144 blocks (approx 24 hours)
	expiryNotificationThresholdBlocks = 144
)

// ExpiryWatcher monitors pending HTLCs and notifies clients when expiry is approaching
type ExpiryWatcher struct {
	client              lightning.Client
	notificationService NotificationService
	nodeConfig          *config.NodeConfig
	quit                chan struct{}
	wg                  sync.WaitGroup
}

// NewExpiryWatcher creates a new expiry watcher service
func NewExpiryWatcher(
	client lightning.Client,
	notificationService NotificationService,
	nodeConfig *config.NodeConfig,
) *ExpiryWatcher {
	return &ExpiryWatcher{
		client:              client,
		notificationService: notificationService,
		nodeConfig:          nodeConfig,
		quit:                make(chan struct{}),
	}
}

// Start begins the monitoring loop
func (w *ExpiryWatcher) Start() {
	w.wg.Add(1)
	go w.checkLoop()
}

// Stop stops the monitoring loop
func (w *ExpiryWatcher) Stop() {
	close(w.quit)
	w.wg.Wait()
}

func (w *ExpiryWatcher) checkLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(expiryCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.quit:
			return
		case <-ticker.C:
			w.checkExpiries()
		}
	}
}

func (w *ExpiryWatcher) checkExpiries() {
	// Get all pending channels/HTLCs
	// This depends on what the Lightning Client interface exposes.
	// Typically we'd list channels and inspect pending HTLCs.
	channels, err := w.client.ListChannels()
	if err != nil {
		log.Error().Err(err).Msg("ExpiryWatcher: Failed to list channels")
		return
	}

	currentBlockHeight, err := w.client.GetBlockHeight()
	if err != nil {
		log.Error().Err(err).Msg("ExpiryWatcher: Failed to get block height")
		return
	}

	for _, ch := range channels {
		for _, htlc := range ch.PendingHtlcs {
			// Only interested in incoming HTLCs that we are holding?
			// Or outgoing ones? Typically incoming ones that are waiting for user to come online.
			// Let's assume incoming HTLCs waiting for settlement.

			if htlc.Incoming {
				blocksToExpiry := int(htlc.ExpirationHeight) - int(currentBlockHeight)

				if blocksToExpiry > 0 && blocksToExpiry <= expiryNotificationThresholdBlocks {
					// Notify!
					// We need the pubkey of the peer.
					pubkey := hex.EncodeToString(ch.PeerId)

					// Avoid spamming? The NotificationService dedupes based on payment hash+interval.
					w.notificationService.NotifyExpirySoon(pubkey, blocksToExpiry, nil)
				}
			}
		}
	}
}
