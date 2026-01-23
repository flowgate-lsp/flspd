# Project Context: flspd

## Overview
`flspd` is a Liquidity Service Provider (LSP) daemon for the Flokicoin Lightning Network (a fork of Bitcoin Lightning). It works alongside `flnd` (Flokicoin LND) to provide liquidity services like JIT (Just-In-Time) channels to mobile wallets and other clients.

## Technology Stack
- **Language**: Go
- **Database**: PostgreSQL (using `pgx`)
- **Lightning Implementation**: `flnd` (fork of LND)
- **Communication**: gRPC (for interacting with LND), Custom Lightning Messages (for LSPS protocols)

## Architecture
The application follows a hexagonal-like architecture:
- **`cmd/flspd`**: Entry point, wiring dependencies (`run.go`).
- **`lsps/`**: Logic for Lightning Service Provider Specs (LSPS).
    - `server.go`: Main LSPS server loop, handles custom messages.
    - `handler_lsps1.go`: Handles channel orders (LSPS1).
    - `handler_lsps2.go`: Handles JIT channel buying (LSPS2).
    - `liquidity_manager.go`: Manages on-chain liquidity checks.
- **`common/`**: Interfaces and shared types (`opening_store.go`, `nodes_service.go`).
- **`postgresql/`**: Database implementations of interfaces.
- **`lnd/`**: LND client implementation.
- **`lightning/`**: Interfaces/Adapters for Lightning clients (allowing mocking).

## Key Components

### 1. LSPS Handler
Handles incoming custom lightning messages for LSPS protocols:
- **LSPS0**: Transport/Versioning.
- **LSPS1**: Channel Order Service (Client requests a channel manually).
- **LSPS2**: JIT Channels (Client requests a payment, LSP intercepts and opens channel).

### 2. Liquidity Manager
Ensures the LSP node has sufficient on-chain funds before accepting channel opening requests.
- **On-Chain Check**: `Confirmed Wallet Balance` - `Reserved Liquidity`.
- **Reserved Liquidity**: Sum of `LspBalanceLoki` for all `CREATED` orders not yet funded.

### 3. Interceptor
Intersects HTLCs to trigger JIT channel opening (in `interceptor/` package).

## Testing
- **Unit Tests**: Co-located with code (e.g., `lsps/handler_lsps1_test.go`). Heavy use of mocks.
- **Integration Tests**: In `itest/`. Requires Docker and full binaries (`flnd`, `lokid`).
