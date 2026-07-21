---
name: flnd-fork-gotchas
description: Concrete divergences between flnd (Flokicoin's fork of LND) / flncli and upstream Lightning Labs LND. Use whenever calling flnd's gRPC API, invoking flncli, choosing an HTLC-interceptor failure code, working with channel-open/zero-conf/scid-alias flags, or reasoning about capacity/reserve math. Never assume Bitcoin-LND parity in either direction — verify via --help or source first.
---

# flnd / flncli Fork Gotchas

`flnd` is Flokicoin's fork/rebrand of Lightning Labs' LND. It is **inconsistently** renamed — some things changed, some didn't, with no reliable rule for which. Don't pattern-match from upstream Bitcoin-LND muscle memory in *either* direction (assuming a rename happened, or assuming it didn't) — verify each specific flag/field via `--help` or the actual source before using it.

## Divergences from upstream LND

| Area | What's different |
|---|---|
| Macaroon rotation | `deletemacaroonid 0` is refused. Deleting only `macaroons.db` leaves a signature mismatch. Rotation requires deleting **both** `macaroons.db` and every `*.macaroon` file, then restarting. |
| Bitcoin backend | `--bitcoin.node` only accepts `btcd`/`bitcoind`/`neutrino`/`nochainbackend` — there is no `lokid` mode at all. This isn't a missing-binary gap; it's why the old bitcoind-shaped `itest/lntest` harness was architecturally impossible to fix and was fully removed rather than patched. |
| Sync payment RPC | No `SendPaymentSync` — use `routerrpc.SendPaymentV2`. |
| Interceptor failure codes | `interceptedForward.FailWithCode` only implements `TEMPORARY_CHANNEL_FAILURE`, `InvalidOnion*`, and `ExpiryTooSoon`. Sending any other code doesn't just fail that one HTLC — it **kills the entire interceptor gRPC stream**, causing an infinite reconnect/redeliver loop. Check the accepted set before adding a new failure path. |
| `option-scid-alias` / zero-conf | Negotiated **per-peer-connection**, not per-channel. Enabling both on every role broke gossip for an ordinary (non-JIT) payer↔LSP channel too — `funding/manager.go`'s alias-channel re-announce path deleted-then-failed-to-readd the edge. Strip these flags from any role that never actually receives a JIT channel. |
| Channel-readiness queries | **The single highest-value lesson here**: `QueryRoutes` reads an in-memory `graphCache` that lags for a freshly-added zero-conf edge. `DescribeGraph`/`GetChanInfo` read the graph DB directly and are correct immediately. Poll the DB-backed RPC for "is this channel ready" — never `QueryRoutes`. |
| Channel reserve | A mandatory ~1% reserve is enforced at the link level (`RequiredRemoteMaxValue`) regardless of advertised policy. A channel can never forward more than `capacity - capacity/100` in one direction — capacity math must budget for this explicitly, not just add flat padding. |
| CLI flag naming | Not a safe pattern-match either direction. `flncli queryroutes` has only a whole-loki `--amt`, no `--amt_msat` sub-unit flag at all (unlike real Bitcoin-LND, which has `--amt_msat`) — but *other*, unrelated flags in the same `--help` output still say "msat," not "mloki." Verify every flag individually; neither "assume renamed to mloki" nor "assume it kept the Bitcoin name" is a safe shortcut. |

## Version pin note

flspd's `go.mod` pins `flnd` at `v0.1.21-beta`, which is exactly flnd's `main` branch HEAD. A separate, in-progress `sync/lnd-2026-07` upstream-LND-reconciliation branch also exists on `flokiorg/flnd` — that branch is **not** what's pinned. Relevant when choosing what to A/B test a suspected flnd-side bug against.

## What NOT to do

- Don't send an interceptor failure code outside the accepted set — it kills the whole stream, not just one HTLC.
- Don't enable `option-scid-alias`/zero-conf globally across every role — scope it to roles that actually receive JIT channels.
- Don't trust `QueryRoutes` for channel-readiness freshness — poll `GetChanInfo`/`DescribeGraph` instead.
- Don't assume flncli's flags mirror Bitcoin-LND's naming, and don't assume they don't — check each one.
