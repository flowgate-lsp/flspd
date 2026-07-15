package lsps

import (
	"context"
	"time"

	"github.com/flowgate-lsp/flspd/ports"
	"github.com/rs/zerolog/log"
)

// OrderCleanupService runs a background job to expire stale LSPS1 orders
type OrderCleanupService struct {
	store    ports.OpeningStore
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewOrderCleanupService creates a new cleanup service
func NewOrderCleanupService(store ports.OpeningStore, interval time.Duration) *OrderCleanupService {
	return &OrderCleanupService{
		store:    store,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the cleanup loop
func (s *OrderCleanupService) Start() {
	log.Info().Msgf("OrderCleanupService: Starting with interval %v", s.interval)

	// Run immediately on startup
	s.runCleanup()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	defer close(s.doneCh)

	for {
		select {
		case <-ticker.C:
			s.runCleanup()
		case <-s.stopCh:
			log.Info().Msg("OrderCleanupService: Stopping")
			return
		}
	}
}

// Stop signals the cleanup service to stop
func (s *OrderCleanupService) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

func (s *OrderCleanupService) runCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = ctx // For future use if needed

	count, err := s.store.ExpireStaleOrders()
	if err != nil {
		log.Error().Err(err).Msg("OrderCleanupService: Failed to expire stale orders")
		return
	}

	if count > 0 {
		log.Info().Int64("expired_count", count).Msg("OrderCleanupService: Expired stale LSPS1 orders")
	} else {
		log.Debug().Msg("OrderCleanupService: No stale orders to expire")
	}
}
