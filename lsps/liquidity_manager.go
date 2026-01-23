package lsps

import (
	"context"
	"fmt"

	"github.com/flowgate-lsp/flspd/common"
	"github.com/flowgate-lsp/flspd/lightning"
	"github.com/rs/zerolog/log"
)

// LiquidityManager defines the interface for managing on-chain liquidity
type LiquidityManager interface {
	// GetAvailableOnChainLiquidity returns the confirmed wallet balance minus reserved liquidity
	GetAvailableOnChainLiquidity(ctx context.Context) (uint64, error)

	// GetReservedOnChainLiquidity returns the total liquidity reserved for pending orders
	GetReservedOnChainLiquidity(ctx context.Context) (uint64, error)

	// CheckOnChainLiquidity checks if the requested amount is available
	CheckOnChainLiquidity(ctx context.Context, amountSat uint64) error
}

type liquidityManager struct {
	lndClient lightning.Client
	store     common.OpeningStore
}

func NewLiquidityManager(lndClient lightning.Client, store common.OpeningStore) LiquidityManager {
	return &liquidityManager{
		lndClient: lndClient,
		store:     store,
	}
}

func (m *liquidityManager) GetReservedOnChainLiquidity(ctx context.Context) (uint64, error) {
	// Query the store for the sum of LSP balances in CREATED orders
	reserved, err := m.store.GetReservedLiquidity()
	if err != nil {
		return 0, fmt.Errorf("failed to get reserved liquidity: %w", err)
	}
	return reserved, nil
}

func (m *liquidityManager) GetAvailableOnChainLiquidity(ctx context.Context) (uint64, error) {
	// 1. Get confirmed wallet balance
	balance, err := m.lndClient.WalletBalance(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get wallet balance: %w", err)
	}

	// 2. Get reserved liquidity
	reserved, err := m.GetReservedOnChainLiquidity(ctx)
	if err != nil {
		return 0, err
	}

	confirmed := uint64(balance.ConfirmedBalance)

	if reserved > confirmed {
		log.Warn().
			Uint64("confirmed_balance", confirmed).
			Uint64("reserved_liquidity", reserved).
			Msg("LiquidityManager: Reserved liquidity exceeds confirmed balance!")
		return 0, nil
	}

	return confirmed - reserved, nil
}

func (m *liquidityManager) CheckOnChainLiquidity(ctx context.Context, amountSat uint64) error {
	available, err := m.GetAvailableOnChainLiquidity(ctx)
	if err != nil {
		return err
	}

	if amountSat > available {
		return fmt.Errorf("insufficient liquidity: requested %d sat, available %d sat", amountSat, available)
	}

	return nil
}
