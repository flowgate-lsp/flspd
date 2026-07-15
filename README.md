# flspd
flspd is a Flokicoin Liquidity Service Provider (FLSP) daemon that provides services to Flokicoin clients.

It is designed to work with an [flnd](https://github.com/flokiorg/flnd) (Flokicoin Lightning Network Daemon) node.

## Deployment

### Docker (Recommended)
The easiest way to run `flspd` is using the provided Docker setup.

1.  **Configure Environment**:
    Copy `sample.env` to `.env` and update the `NODES` and `DATABASE_URL` settings.
    ```bash
    cp sample.env .env
    # Edit .env with your favorite editor
    ```

2.  **Run with Docker Compose**:
    ```bash
    docker-compose up -d
    ```
    This will start `flspd` and a `postgres` database. Ensure your `flnd` node is running and accessible (as configured in `NODES`).

### Manual Install
If you prefer to run `flspd` manually or want to contribute:

1.  **Prerequisites**:
    *   Go 1.24+
    *   PostgreSQL
    *   [flnd](https://github.com/flokiorg/flnd)

2.  **Build**:
    ```bash
    go build -v ./cmd/flspd
    ```

3.  **Run**:
    ```bash
    export NODES='[...]' # See sample.env
    ./flspd
    ```

## Flow for Creating Channels
When a client (Alice) wants to receive a payment but lacks sufficient inbound capacity, she calls the flspd function `RegisterPayment()`, providing:
*   Payment Hash & Secret
*   Destination (Alice's Pubkey)
*   Incoming Amount (what the payer sends)
*   Outgoing Amount (what Alice receives)

The difference represents the LSP fee. `flspd` intercepts the incoming HTLC from the upstream node and opens a channel to Alice on-the-fly (Zero-Conf) to fulfill the payment.

## Integration Tests
Integration tests run against a real, disposable Flokicoin regtest devnet
(native `lokid` + `flnd`, no Bitcoin Core) — see `test/devnet/README.md`
for the devnet itself. You need Docker; the devnet builds `flnd`/`lokid`
from source on first run, no separate binary paths to configure.

Run tests from the root directory:
```bash
go test -timeout 20m -v ./test/itest/...
```
