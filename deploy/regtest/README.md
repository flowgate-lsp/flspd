# flspd as lokihub's regtest test LSP

lokihub's regtest stack runs flspd as its test LSP (LP-13): one flnd node of
its own, LSPS1, LSPS2 (JIT channels) and LSPS5, and a NIP-LSP announcement
(kind 10735) on the stack's relay that lokihub builds its LSP list from.
Regtest only. Nothing here brings a stack up; lokihub's compose does.

| file | for |
|---|---|
| `../Dockerfile.regtest` | the image (`just image-regtest`) |
| `flnd-lsp.conf.sample` | the LSP's flnd node |
| `nodes.regtest.sample.yaml` | flspd's node config, announcing on (`NODES_CONFIG_PATH`) |
| `flspd.regtest.env.sample` | flspd's environment |
| `compose.lokihub.sample.yml` | the services to merge into lokihub's `docker-compose.dev.yml` |

## Image

Until go.mod pins an nmilat release with `niplsp` (BACKLOG T8-5), the build
takes an nmilat checkout as a second build context:

```sh
just image-regtest /path/to/nmilat            # tags flspd:regtest
just image-regtest /path/to/nmilat flspd:rc   # another tag
```

That runs `docker build -f deploy/Dockerfile.regtest --build-context
nmilat=<path> --build-arg COMMIT=<short sha> -t <tag> .` from the flspd
checkout. The nmilat checkout must carry `niplsp` (nmilat
`spec-sync/parity-1` or later). The image runs `/bin/flspd` and listens on
9090 (gRPC) and 8080 (dashboard).

## What lokihub's compose passes

Services (`compose.lokihub.sample.yml`):

- **`flnd-lsp`**: lokihub's flnd build, config `flnd-lsp.conf.sample`
  (`rpclisten` 10007, `requireinterceptor=true`, zero-conf and scid-alias),
  data at `./data/regtest/flnd-lsp`. It needs a **fixed IPv4 address** on
  `lokihub-dev` (the sample uses `172.29.40.40` in `172.29.40.0/24`):
  NIP-LSP `addr` takes an IP, not a hostname, and the announced address must
  be one the hub's flnd can dial.
- **`flspd-db`**: Postgres 15 for flspd alone (user, password and database
  `flspd`).
- **`flspd`**: the image, with:
  - `NODES_CONFIG_PATH=/flspd/nodes.yaml`, the nodes sample mounted there;
  - flnd-lsp's data directory mounted read-only at `/flnd-lsp` (the sample
    reads `/flnd-lsp/tls.cert` and
    `/flnd-lsp/data/chain/flokicoin/regtest/admin.macaroon`);
  - the environment below;
  - started after `flnd-lsp` is healthy (its admin macaroon exists),
    `flspd-db` is healthy and `relay` started.

Environment (`flspd.regtest.env.sample`):

| variable | value | |
|---|---|---|
| `NODES_CONFIG_PATH` | `/flspd/nodes.yaml` | required |
| `DATABASE_URL` | `postgres://flspd:flspd@flspd-db:5432/flspd?sslmode=disable` | required |
| `AUTO_MIGRATE_DATABASE` | `true` | required on a fresh database |
| `FLSPD_PRIVATE_KEY` | 64 hex, from `flspd genkey` (`docker run --rm flspd:regtest genkey`) | required; flspd's own key, not the node key |
| `NOSTR_ANNOUNCE_RELAYS` | `ws://relay:5500` | required while announcing; comma list |
| `LISTEN_ADDRESS` | `0.0.0.0:9090` | flspd's gRPC |
| `LSP_NOSTR_PRIVATE_KEY` | hex or nsec | optional: LSPS5's Nostr transport |
| `DASHBOARD_ADMIN_PUBKEY`, `DASHBOARD_LISTEN_ADDRESS` | hex pubkey, `:8080` | optional: dashboard |
| `MEMPOOL_API_BASE_URL` | unset | fees then come from flnd |

flspd refuses to start while announcing is on and `NOSTR_ANNOUNCE_RELAYS`
is empty or the macaroon lacks `signer:generate` (admin.macaroon has it).

## What the stack must do for the LSP to sell

- Fund flnd-lsp's on-chain wallet (LSPS1 orders and JIT channels open from
  it) and mine so its coins confirm, as the stack does for `payer`.
- Nothing more for the announcement: flspd publishes it on start, signed by
  flnd-lsp's node key, and republishes on any change and every 7 days.

## What lokihub sees

On `ws://relay:5500`, one kind-10735 event authored by flnd-lsp's x-only
node key, with `n` `flokicoin`, `network` `regtest`, `addr`
`172.29.40.40:9735`, `status` `open`, `protocol` 1, 2 and 5, the LSPS1
options, the LSPS2 fee row and `lsps5 max_webhooks`, and `r`
`ws://relay:5500`. Its values equal flspd's live LSPS answers over that
address. The dashboard (host port 8090 in the sample) shows the last
event, each relay's answer and the self-check.

Pausing, resuming or unlisting from the dashboard republishes with `status`
`paused`, `open` or `closed`; switching `announce.enabled` off publishes
`closed` once.
