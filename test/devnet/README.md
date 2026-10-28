# flspd integration-test devnet

A private, disposable Flokicoin regtest network for local development and
for backing the YAML-driven integration tests (see `BACKLOG.md` Phase 4).
Everything here is built from source (never a production/release image),
mirroring the sibling-checkout convention the top-level
`deploy/docker-compose.dev.yml` already uses for its single-instance dev `flnd`.

**Native Flokicoin only — no Bitcoin Core anywhere.** The chain backend is
`lokid` (go-flokicoin's own full node daemon), and `flnd` connects to it
in neutrino mode over P2P — the same chain-backend mode flspd's
single-instance dev setup already uses against real mainnet
(`../../data/flnd/flnd.conf.sample`); here it just points at a private
regtest peer instead.

## Topology

| Service | What | Container | Host RPC port | Host P2P port |
|---|---|---|---|---|
| `chain` | `lokid`, regtest | `flspd-devnet-chain` | 19001 | 19002 |
| `flnd-lsp` | LSP-side node | `flspd-devnet-flnd-lsp` | 20001 | 20011 |
| `flnd-client` | buys inbound liquidity / JIT channels | `flspd-devnet-flnd-client` | 20002 | 20012 |
| `flnd-payer` | third-party payer, for multi-hop/MPP tests | `flspd-devnet-flnd-payer` | 20003 | 20013 |

Need another peer (e.g. an offline-peer scenario)? Copy one of the
`flnd-*` blocks in `docker-compose.yml` — give it its own
`container_name`, data volume, and two free host ports.

## Quick start

```bash
./setup.sh                  # one-time: creates data/ dirs + sample configs
docker compose up -d        # builds (first run is slow) and starts everything
./collect-addresses.sh      # fetches an address from each flnd node, wires
                             # them into the chain's miningaddr list, and
                             # restarts `chain` ONCE to pick them up
./generate.sh 150           # mine 150 blocks on demand (100+ for coinbase
                             # maturity) — funds all three nodes
./flncli.sh lsp walletbalance
```

`flncli.sh <role> <args...>` wraps `flncli` with the right macaroon/cert
paths for a given role, e.g. `./flncli.sh client getinfo`.

## Important: regtest wipes on every `chain` restart

`lokid` unconditionally deletes its regtest database on startup
(`removeRegressionDB` in `go-flokicoin/lokid.go` — inherited btcd-family
behavior; unlike `bitcoind`, which persists regtest state across
restarts). Practical implications:

- Run `collect-addresses.sh` **once**, right after the first `docker
  compose up -d`, before anything of value exists — that's the only time
  restarting `chain` is harmless.
- After that, mine additional blocks with `./generate.sh <n>` (an RPC
  call, no restart) — never `docker compose restart chain` mid-session, or
  every mined block and wallet balance is gone.
- `docker compose down` + a fresh `up -d` always starts from genesis;
  treat each session as disposable, matching how the itest Go harness
  already treats its own regtest miner.

## Data & volumes

All state lives under `test/devnet/data/` (already covered by the
repo-wide `data/` `.gitignore` rule — verified: `git check-ignore -v
test/devnet/data/...` matches). Nothing here is ever committed. To wipe
everything: `docker compose down -v && rm -rf data && ./setup.sh`.

## Why not just reuse `itest/lntest`?

The Go itest harness (`itest/lntest/miner.go`, `lnd_node.go`) already
spins up regtest nodes as ephemeral subprocesses per `go test` run — but
it uses literal `bitcoind` as its regtest backend, which is fine for
exercising Lightning-level logic in CI but isn't the native chain. This
devnet is for interactive, persistent, inspectable use (manual testing,
building/debugging the Phase 4 YAML scenario runner, verifying backlog
fixes by hand) — a real `lokid` node you can `docker exec` into, leave
running across a session, and restart on demand, without waiting on a
full `go test` cycle.
