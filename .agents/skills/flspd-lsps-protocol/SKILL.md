---
name: flspd-lsps-protocol
description: LSP protocol implementation guidance for flspd (LSPS0/1/2/5 over BOLT8 + JSON-RPC). Use whenever implementing or modifying an LSPS method, a JSON-RPC handler, the shared fee formula, JIT-channel invoice/capacity math, the settlement/order state machine, or LSPS5 Nostr webhook logic. Always consult before touching the `lsps` package or the fee model.
---

# flspd LSPS Protocol Implementation

`flspd` implements LSPS0/1/2/5 (Lightning Service Provider Standards) as defined in the upstream bLIP specifications. Strict spec adherence matters — clients are third-party wallets that only understand the wire contract, not this codebase's internals.

## Core Workflow

1. **Specification audit first**: before any code change, read the relevant bLIP in [references/SPECS.md](references/SPECS.md).
2. **Schema alignment**: new JSON-RPC models must follow LSPS0's common schemas ([references/blip-0050.md](references/blip-0050.md)), e.g. `_sat`/`_msat` field-name suffixes.
3. **Interface-first design**: extend the `lightning.Client` and `OpeningStore` interfaces in the `lsps` package rather than reaching into LND directly.
4. **Fault-tolerant settlement**: implement recovery logic for `PAID`-but-`UNFULFILLED` orders (disconnections, peer restarts) — see the reconciliation philosophy below.
5. **Verification**: `go test ./lsps/... -count=1` and `go vet ./...`. These are unit tests only — for end-to-end protocol behavior against real flnd nodes, see the `flspd-devnet-debugging` skill's `test/itest` scenario runner; this skill's tests don't exercise that path.

## Units: wire schema vs. internal convention

flspd runs on **Flokicoin** — internal code, logs, and prose use **loki/mloki/FLC**, never sats/msats/BTC. But the LSPS wire schema's `_sat`/`_msat` field-name *suffixes* are spec-mandated (they're part of the JSON-RPC contract every LSPS client expects) and stay exactly as-is even though the underlying value is denominated in loki. Don't rename them to `_loki` — that would break wire compatibility. Don't confuse this one wire-format exception with the rest of the codebase's unit convention.

## One shared fee formula

`ComputeOpeningFee` must back every protocol path (LSPS1 order pricing, LSPS2 JIT quoting, the interceptor's own fee deduction). A real bug once let three divergent rounding rules (ceil/floor/gross-up) exist simultaneously across these paths, producing inconsistent quotes for the same channel size depending on which code path computed it. Never reintroduce a second fee-rounding implementation — extend the shared one.

## JIT channel invoice & capacity rules (LSPS2)

- This LSP implements `client_trusts_lsp=false`: it forwards `payment_size - fee`, not the gross amount. The invoice must be for the **net** (post-fee) amount, with the fee carried on the route hint's `FeeBaseMsat` — exactly how a real LSPS2 wallet derives it from the quoted `opening_fee_params`. Inverting this (invoicing the gross amount) causes the client's own link to reject the forwarded HTLC as underpaying the onion's declared amount.
- JIT channel capacity must add a **reserve-aware floor**, not just gross payment + flat padding: `ceil(outgoing / 0.99) + padding`. A channel can never forward more than `capacity - capacity/100` in one direction (BOLT2's mandatory ~1% reserve, enforced at the link level regardless of advertised policy) — capacity that's nominally larger than the payment can still be structurally unable to forward it.

## Recovery philosophy: don't guess when ambiguous

Applied consistently across order/channel reconciliation (funded-order recovery, unclaimed-channel recovery after a crash mid-`OpenChannel`): only auto-resolve when **exactly one** reconciliation candidate exists. If multiple candidates match, leave it for manual review — never silently pick one. Silent auto-resolution among ambiguous candidates is how a channel opened for the wrong order, or a double-open, happens.

## Nostr / LSPS5 specifics

- Per-relay-URL `singleflight` is required to prevent duplicate concurrent connections to the same relay from separate notification sends.
- NIP-98 replay-cache keys must be the **recomputed** event ID, not the client-supplied `id` field — go-nostr's signature check doesn't validate that the supplied ID matches the event's actual hash, so keying on it is spoofable.
- For scripted fault-injection scenarios (e.g. a relay that must reject-then-accept on command), use the in-repo `MockRelay` fixture — a real relay can't be told to misbehave on cue. Reach for a live public relay only for a one-off manual smoke test that specifically needs proof against real network infrastructure, and don't hardcode a personal/team test relay's URL into this skill — that's operational config, not durable protocol knowledge.

## Implementation pattern example

```go
// 1. Define Request/Response following LSPS0 suffixes
type CreateOrderRequest struct {
    LspBalanceSat    string `json:"lsp_balance_sat"`
    ClientBalanceSat string `json:"client_balance_sat"`
}

// 2. Implement logic in the Service layer (e.g. Lsps1SettlementService)
func (s *SettlementService) HandleCreateOrder(ctx context.Context, req *CreateOrderRequest) (*OrderResponse, error) {
    // Validate against dynamic info (lsps1.get_info)
    // Persist to store (PostgreSQL)
    // Return LSPS-specific error codes on failure
}
```

## What NOT to do

- **Don't hardcode LND**: never import LND/flnd packages directly into `lsps` business logic — go through the `lightning.Client` port. See `flnd-fork-gotchas` for what's actually different about this LND fork.
- **Don't mix `sat`/`msat`** in the same object without the required suffixes.
- **Don't invert JIT fee placement**: net-amount invoice + fee on the route hint, never gross-amount invoice.
- **Don't auto-pick among multiple reconciliation candidates** — ambiguous cases go to manual review.
- **Don't assume peers stay connected**: always implement level-triggered (not edge-triggered) recovery, and listen for `PEER_ONLINE` to retry stuck channel opens.

## Architectural constraints (MUST)

- Use JSON-RPC 2.0 with a random 80+ bit entropy `id`, per [LSPS0 disconnection handling](references/blip-0050.md).
- Error codes use the `LSPS-number * 100` range convention (e.g. LSPS1 errors are 100-199).
- Idempotent methods (per spec) must handle re-delivery of the same `id` gracefully.
- Use `uint64` for amounts and ISO8601 strings for datetimes.

## Dashboard & monitoring interface

UI/data-binding specs for the operator dashboard live in a separate skill — see `flspd-dashboard-design`.
