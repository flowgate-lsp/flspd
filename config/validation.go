package config

import (
	"fmt"
)

// AbsoluteMinChannelCapacityLoki is the hard floor for channel size enforced by LSP.
const AbsoluteMinChannelCapacityLoki = 20000

// Validate performs a full validation of the NodeConfig.
func (cfg *NodeConfig) Validate() error {
	// 1. Basic Capacity Checks
	if cfg.MinChannelCapacityLoki > 0 && cfg.MinChannelCapacityLoki < AbsoluteMinChannelCapacityLoki {
		return fmt.Errorf("minChannelCapacityLoki must be >= %d (LSP Limit), got %d", AbsoluteMinChannelCapacityLoki, cfg.MinChannelCapacityLoki)
	}
	if cfg.MinChannelCapacityLoki == 0 {
		cfg.MinChannelCapacityLoki = AbsoluteMinChannelCapacityLoki
	}

	if cfg.AdditionalChannelCapacity < 0 {
		return fmt.Errorf("additionalChannelCapacity (%d) cannot be negative", cfg.AdditionalChannelCapacity)
	}

	// 2. LSPS1 Validation
	if err := cfg.ValidateLsps1(); err != nil {
		return fmt.Errorf("LSPS1 config validation failed: %w", err)
	}

	// 3. JIT Validation
	if err := cfg.ValidateJit(); err != nil {
		return fmt.Errorf("JIT config validation failed: %w", err)
	}

	return nil
}

func (cfg *NodeConfig) ValidateLsps1() error {
	// Helper to ensure field is set or use default
	setUint16 := func(ptr **uint16, def uint16) uint16 {
		if *ptr == nil {
			*ptr = &def
		}
		return **ptr
	}
	setUint32 := func(ptr **uint32, def uint32) uint32 {
		if *ptr == nil {
			*ptr = &def
		}
		return **ptr
	}
	setUint64 := func(ptr **uint64, def uint64) uint64 {
		if *ptr == nil {
			*ptr = &def
		}
		return **ptr
	}
	setBool := func(ptr **bool, def bool) bool {
		if *ptr == nil {
			*ptr = &def
		}
		return **ptr
	}

	// 1. Set Defaults for Minimal Config
	setUint16(&cfg.Lsps1MinRequiredChannelConfirmations, 0)
	setUint16(&cfg.Lsps1MinFundingConfirmsWithinBlocks, 1)
	setBool(&cfg.Lsps1SupportsZeroChannelReserve, true)
	setUint32(&cfg.Lsps1MaxChannelExpiryBlocks, 201600) // ~140 days
	setUint32(&cfg.Lsps1OrderExpirySeconds, 86400)      // 24h

	minClient := setUint64(&cfg.Lsps1MinInitialClientBalanceLoki, 0)
	maxClient := setUint64(&cfg.Lsps1MaxInitialClientBalanceLoki, 100_000_000)
	minLsp := setUint64(&cfg.Lsps1MinInitialLspBalanceLoki, 0)
	maxLsp := setUint64(&cfg.Lsps1MaxInitialLspBalanceLoki, 100_000_000)
	minChan := setUint64(&cfg.Lsps1MinChannelBalanceLoki, cfg.MinChannelCapacityLoki)
	maxChan := setUint64(&cfg.Lsps1MaxChannelBalanceLoki, 100_000_000)

	// 2. Check Logical Constraints (Min <= Max)
	if minClient > maxClient {
		return fmt.Errorf("lsps1MinInitialClientBalanceLoki (%d) > lsps1MaxInitialClientBalanceLoki (%d)", minClient, maxClient)
	}
	if minLsp > maxLsp {
		return fmt.Errorf("lsps1MinInitialLspBalanceLoki (%d) > lsps1MaxInitialLspBalanceLoki (%d)", minLsp, maxLsp)
	}
	if minChan > maxChan {
		return fmt.Errorf("lsps1MinChannelBalanceLoki (%d) > lsps1MaxChannelBalanceLoki (%d)", minChan, maxChan)
	}

	// 3. Network Compatibility Check
	padding := uint64(cfg.AdditionalChannelCapacity)
	if minChan+padding < cfg.MinChannelCapacityLoki {
		return fmt.Errorf("LSPS1 incompatibility: minChannelBalanceLoki (%d) + padding (%d) < network floor (%d)", minChan, padding, cfg.MinChannelCapacityLoki)
	}

	// 4. Business Floor vs Physical Floor
	if minChan < cfg.MinChannelCapacityLoki {
		return fmt.Errorf("LSPS1 config error: lsps1MinChannelBalanceLoki (%d) cannot be lower than network floor (%d)", minChan, cfg.MinChannelCapacityLoki)
	}

	return nil
}

func (cfg *NodeConfig) ValidateJit() error {
	if !cfg.JitEnabled {
		return nil
	}

	// 1. Check Max Client Delay
	if cfg.JitMaxClientToSelfDelay == 0 {
		cfg.JitMaxClientToSelfDelay = 10080 // DefaultMaxClientToSelfDelay
	}

	// 2. Check Min Payment Size vs Floors
	padding := uint64(cfg.AdditionalChannelCapacity)
	minPaymentSat := cfg.MinPaymentSizeMloki / 1000

	// Check against Physical Network Floor
	if minPaymentSat+padding < cfg.MinChannelCapacityLoki {
		return fmt.Errorf("JIT incompatibility: minPaymentSizeMloki (%d loki) + padding (%d) < network floor (%d)", minPaymentSat, padding, cfg.MinChannelCapacityLoki)
	}

	// Check against Business Floor
	if cfg.JitMinChannelBalanceLoki != nil {
		if minPaymentSat+padding < *cfg.JitMinChannelBalanceLoki {
			return fmt.Errorf("JIT business violation: minPaymentSizeMloki (%d loki) + padding (%d) < jitMinChannelBalanceLoki (%d)", minPaymentSat, padding, *cfg.JitMinChannelBalanceLoki)
		}

		// Check that Business Floor is at least Physical Network Floor
		if *cfg.JitMinChannelBalanceLoki < cfg.MinChannelCapacityLoki {
			return fmt.Errorf("JIT configuration error: jitMinChannelBalanceLoki (%d) cannot be lower than network floor (%d)", *cfg.JitMinChannelBalanceLoki, cfg.MinChannelCapacityLoki)
		}
	}

	// 3. Check Min <= Max
	if cfg.JitMaxPaymentSizeMloki > 0 && cfg.MinPaymentSizeMloki > cfg.JitMaxPaymentSizeMloki {
		return fmt.Errorf("JIT logical error: minPaymentSizeMloki (%d) > jitMaxPaymentSizeMloki (%d)", cfg.MinPaymentSizeMloki, cfg.JitMaxPaymentSizeMloki)
	}

	return nil
}
