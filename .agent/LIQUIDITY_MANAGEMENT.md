# Liquidity Management

## Overview
The Liquidity Manager (`lsps/liquidity_manager.go`) prevents the LSP from over-committing its on-chain funds. It is critical for LSPS1 (Channel Orders) and LSPS2 (JIT Channels) where the LSP is the funder.

## Logic
**Available Liquidity** = `Confirmed Wallet Balance` - `Reserved Liquidity`

### Reserved Liquidity
We define "Reserved Liquidity" as the sum of `LspBalanceLoki` for all orders in the `CREATED` state.
- **Why**: When an order is created, we promise to fund it, but the funding transaction hasn't been broadcast yet. Once funded, the balance is deducted from the wallet (or locked in UTXOs), but until then, we must account for it separately.

### Failure Modes
- **LSPS1**: If insufficient, `create_order` returns error code `1` (Client rejected).
- **LSPS2**: If insufficient, `buy` returns error code `203` (Payment size too large/check failed).

## Implementation Details
- **Interface**: `LiquidityManager` (in `lsps/liquidity_manager.go`)
- **Store**: `OpeningStore` needs `GetReservedLiquidity()` method.
- **Store Implementation**: `postgresql/opening_store.go` uses `SELECT SUM(lsp_balance_loki) ... WHERE order_state = 'CREATED'`.
- **LND Integration**: `lightning.Client` interface uses `WalletBalance()` to get confirmed/unconfirmed balance.

## Testing
- **Matrix Tests**: `lsps/liquidity_manager_test.go` covers permutations of:
  - Sufficient/Insufficient balance.
  - Zero/Non-Zero reserved amounts.
  - LND/DB errors.
