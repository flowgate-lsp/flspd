# Development Guide

## Build & Run
### Manual
```bash
go build -v ./cmd/flspd
export NODES='...' 
./flspd
```
See `README.md` for Docker instructions.

## Testing

### Unit Tests
Located next to source files (e.g., `lsps/handler_lsps1.go` -> `lsps/handler_lsps1_test.go`).
- **Mocking**: We use strict interface-based design to allow mocking.
- **Mocks Location**: Often found in `*_test.go` files or `testing/` utils. In `lsps` package, `server_test.go` and `settlement_service_test.go` contain reusable mocks (`MockLightningClient`, `MockOpeningStore`).
- **Running Tests**:
  ```bash
  go test -v ./lsps/...
  ```

### Integration Tests
Located in `itest/`.
- Require running binaries (`flnd`, `lokid`).
- Typically run via Docker or specialized test runner script.

## Coding Standards
- **Interfaces**: Define interfaces in `common/` or consumer packages (e.g., `lsps/LiquidityManager`) to decouple implementation.
- **Dependency Injection**: Use constructor injection (e.g., `NewServer(...)`) to pass dependencies. This facilitates testing with mocks.
- **Hexagonal Architecture**: Keep core logic (handlers) separate from infrastructure (LND, Postgres).

## Troubleshooting
- **Lacks implementation**: If you see "missing method" errors in mocks, you likely need to update the mock struct in `server_test.go` or similar test files.
