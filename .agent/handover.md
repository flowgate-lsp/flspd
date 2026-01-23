# Handover Document: LSPS5 over Nostr Implementation

This document summarizes the work done on implementing LSPS5 compliant notifications via HTTPS and Nostr transports.

## Completed Work

### 1. `nostr-lsps5` Library
- **Location**: `flspd/nostr-lsps5` (internal module).
- **Features**:
    - **Nostr Kind**: Using `kind: 31055` for LSPS5 notifications.
    - **Encryption**: NIP-04 supported (NIP-44 planned).
    - **URI Support**: Implements `nostr+lsps5://<pubkey>?relay=<wss://...>` scheme.
    - **Relay Pool**: Uses a persistent `RelayPool` to maintain open connections to client relays.
    - **Testing**: Unit tests achieved **~90% coverage**.
    - **Documentation**: Includes a basic `README.md`.

### 2. `flspd` Daemon Integration
- **Abstract Transport Layer**: Defined `NotificationTransport` interface in `notifications/transport.go`.
- **Transports**:
    - `HTTPSTransport`: Refactored existing webhook logic.
    - `NostrTransport`: New implementation using the `nostr-lsps5` library.
- **Service Refactor**: `NotificationService` now routes notifications based on the registered transport for each webhook.
- **LSPS5 Compliance**:
    - Added signature headers and JSON-RPC 2.0 formatting.
    - Implemented `lsps5.webhook_registered` and other notification methods.
- **Configuration**: Added support for `LSP_NOSTR_PRIVATE_KEY` and `LSP_NOSTR_RELAYS`.
- **Database**: Added `transport` column to `notification_subscriptions` table.
- **gRPC API**: Updated `notifications.proto` to support the `transport` field in subscriptions.

## Current Status

> [!TIP]
> **Build Status: PASSING**
> The circular dependency and type mismatch issues have been resolved by moving `NotificationRegistration` to the `common` package.

- **E2E Tests**: Implemented in `itest/nostr_notification_test.go` using a mock Nostr relay.
- **Verification**: Tests cannot be executed in the current environment due to missing `bitcoind` and `lnd` binaries.

## Missing / Next Steps

1. **Run E2E Tests**:
    - Execute `go test -v -run "TestLspd/.*/testNostrNotification" ./itest` in an environment with `bitcoind` and `lnd` installed.
2. **Finish Library Coverage**:
    - Close the gap to 100% coverage in `nostr-lsps5`.
3. **LokiHub Integration**:
    - See `lokihub_nostr_handover.md` for client-side implementation details.

## Reference Artifacts
- [task.md](file:///root/.gemini/antigravity/brain/107b3784-8111-470d-802e-549e023b5172/task.md)
- [implementation_plan.md](file:///root/.gemini/antigravity/brain/107b3784-8111-470d-802e-549e023b5172/implementation_plan.md)
