# Hold-invoice code review — findings to fix

Multi-agent code review (xhigh effort, 7 finder agents + direct verification)
of two commits that add LSPS1 hold invoices. Written so a fresh agent with no
prior context can pick this up and fix it. Read the whole "Context" section
before touching any code — the bugs only make sense once you understand the
`order_state=PAID` overload described there.

**Commits under review:** `0f86656` (CLTV/timelock config fix, not in scope
for these findings — reviewed clean) and `9a68ecd` (the hold-invoice
feature — every finding below is against this commit). Run `git show
9a68ecd --stat` to see the full file list, or `git diff 0f86656..9a68ecd`
for the raw diff.

**Status (2026-07-18): every finding in this document is now fixed and
verified — H-1 through H-15, S-1 through S-10.** Nothing remains open.
`go build`/`go vet` clean across the whole repo; every package's test suite
passes (`./lsps/...`, `./interceptor/...`, `./internal/adapters/postgres/...`,
`./internal/adapters/lnd/...`, `./dashboard/...`, `./notifications/...`, and
the rest) with new regression tests added per finding; all four LSPS1
devnet scenarios pass against `test/devnet` (`TestLsps1OrderSettleAndOpen`,
`TestScenarios/lsps1_recovery_after_restart`,
`TestScenarios/lsps1_settle_under_fee_congestion`, and the new
`TestSubscribeSingleInvoiceDeliversAccepted`). The only itest failure
anywhere is `TestLsps2JitHappyPath`/`TestScenarios/lsps2_jit_happy_path`
(`FAILURE_REASON_NO_ROUTE`) — an unrelated, pre-existing devnet-state
flakiness in a different feature (LSPS2 JIT), confirmed via `git stash` to
fail identically on completely unmodified code; not a regression from any
work in this document.

**First pass (2026-07-17): H-1 through H-9, H-13/H-14/H-15 fixed and
verified.** See each finding below for its specific fix and the test that
proves it (search this file for "Fixed 2026-07-17").

**Second pass (2026-07-17, same day): 8-angle multi-agent self-review of the
first pass's fixes** found 4 new bugs the fixes themselves introduced or
left unaddressed (S-1 through S-4, fixed same day) plus 6 lower-priority
follow-ups (S-5 through S-10). The most significant of the four:
`GetAllPaidLsps1Orders`'s SQL excluded any row with `funding_txid` set,
which made the entire `order.FundingTxid != nil` branch of
`recoverStuckOrders` — both the pre-existing legacy `reconcileFundedOrder`
call AND the new H-3/H-4 `reconcileFundedHeldOrder` call — **unreachable
dead code in production**. The H-3/H-4 fix above did not actually close the
gap it claimed to; this is now fixed. See "Second-pass findings" below for
all ten (S-1..S-10).

**Third pass (2026-07-18): S-5 through S-10, plus H-10/H-11/H-12, all
fixed.** This closes out every remaining item, including the architectural
one (H-12: per-invoice LND subscriptions replacing poll-based ACCEPTED
detection) that the first pass's own suggested fix order had explicitly
deferred as lower-priority. See "Third-pass fixes" below for details on
each.

---

## Context: why this feature exists, and the design decision that caused every bug below

**The feature.** LSPS1 (buy-a-channel) used to pay a plain `AddInvoice`
bolt11 invoice, which LND settles (captures funds) the instant the HTLC
arrives — before flspd has decided whether it can actually open the
channel. If channel-opening then failed, the only recourse was a slow,
manual on-chain refund via `RefundOnchainAddress`. This feature switches to
an LND **hold invoice** (`AddHoldInvoice`): flspd generates the preimage
itself, LND holds the client's HTLC in `ACCEPTED` state (not settled) once
it arrives, and flspd explicitly decides to either `SettleInvoice`
(capture funds, only once it's actually committed to opening the channel)
or `CancelInvoice` (instant, on-chain-fee-free refund — the payer's HTLC
just fails back through the route).

**The design decision that caused the bugs.** Before this feature,
`lsps1.orders.order_state = 'PAID'` always meant exactly one thing: *the
invoice settled, funds are captured, we're now trying to open the
channel.* A pile of existing machinery is built on that invariant:
`reconcileUnclaimedChannel`, `reconcileFundedOrder`, `GetAllPaidLsps1Orders`
(startup recovery), and — critically — a **separate, untouched consumer**:
`interceptor/intercept_handler.go`'s LSPS1-JIT-fallback path, which calls
`GetPaidLsps1Order` to find a paid-but-unfulfilled order it can trigger a
JIT channel open for.

This feature reuses `order_state='PAID'` for a **second, different**
meaning: *committed to attempting fulfillment, HTLC only held, funds NOT
yet captured* (signaled by `payment_state='HOLD'` on the same row, as
opposed to `payment_state='PAID'` for the legacy/genuinely-settled case).
This was a deliberate choice — it let the new `openChannelForHeldOrder`
reuse `reconcileUnclaimedChannel`/`reconcileFundedOrder`/
`GetAllPaidLsps1Orders` **unmodified**, since by the time those are called
the row already reads `order_state=PAID` — but it means **every reader of
`order_state='PAID'` now needs to also check `payment_state`, and most of
them don't.** That's the through-line of findings H-1 through H-6 below.

**Key files:**
- `lsps/handler_lsps1.go` — order creation, now calls `AddHoldInvoice`
  with a locally-generated 32-byte preimage (`core.Lsps1Order.PaymentPreimage`,
  hex-encoded, `json:"-"` tagged).
- `lsps/settlement_service.go` — the state machine. New: `processAcceptedInvoice`,
  `processAcceptedOrder` (the CAS that flips `CREATED`→`PAID`/`HOLD`),
  `openChannelForHeldOrder`, `cancelHeldOrder`, `settleHeldOrderInvoice`.
  Modified: `Start()`'s event loop (added a 3s poll ticker — see H-10/H-11/H-12),
  `recoverStuckOrders` (branches on `PaymentState`), `recoverSettledCreatedOrders`
  (now also handles `InvoiceStateAccepted`). Unmodified but now newly
  load-bearing: `reconcileUnclaimedChannel`, `reconcileFundedOrder`.
- `interceptor/intercept_handler.go` — **not modified by this diff**, but its
  LSPS1-JIT-fallback path (search for `GetPaidLsps1Order`, ~line 753) is a
  second, independent consumer of `order_state='PAID'` that this feature
  broke without touching.
- `internal/adapters/postgres/lsps1_settlement_store.go` /
  `opening_store.go` — 7 SELECT/INSERT sites updated with a new
  `payment_preimage` column.
- `internal/adapters/postgres/intercept_store.go` — a **separate, mostly
  duplicate** copy of `GetPaidLsps1Order`, used only by the interceptor.
  **Not updated** in this diff (see H-8).
- `lightning/client.go` / `internal/adapters/lnd/client.go` — `SettleInvoice`/
  `CancelInvoice` added to the `lightning.Client` interface; `AddHoldInvoice`
  added to the narrower `LndClientWrapper` interface used only by
  `lsps/handlers.go`. All wired to `invoicesrpc.InvoicesClient`.
- `core/opening.go` — `Lsps1Order.PaymentPreimage *string`.
- `internal/adapters/postgres/migrations/000005_add_lsps1_payment_preimage.{up,down}.sql`.

**Existing state-machine convention worth knowing before you touch this
file:** every state transition goes through `UpdateLsps1OrderState(orderId,
newState, newPaymentState, expectedCurrentState)`, a compare-and-set that
returns `(updated bool, err error)`. `updated=false, err=nil` means *a
concurrent writer already moved this row out of the expected state* — a
normal, expected outcome under concurrency, not a failure. The existing F-1
fix (see `BACKLOG.md`) depends on every caller checking `updated` before
proceeding with any side effect. Several of the findings below are exactly
this check being skipped.

**Devnet verification already done for this feature (before this review):**
`TestLsps1OrderSettleAndOpen`, `TestScenarios/lsps1_recovery_after_restart`,
and `TestScenarios/lsps1_settle_under_fee_congestion` all pass against real
flnd nodes on a freshly-wiped `test/devnet`. That devnet run is also what
found and fixed the "LND's base `SubscribeInvoices` RPC can't deliver
ACCEPTED events" bug (the poll ticker in `Start()` exists because of that —
see H-12 for why the poll itself is now flagged). **Re-run these three
after fixing the findings below** — they're your regression safety net
for the happy path, but none of them exercise the JIT-interceptor race
(H-1) or a `SettleInvoice`-fails scenario (H-2/H-3), so passing them does
not mean those are fixed.

**Unit tests already covering the intended (not necessarily correct, see
below) behavior:** `lsps/settlement_service_test.go` has 8 dedicated tests
(`TestProcessAcceptedInvoice_*`, `TestOpenChannelForHeldOrder_*`,
`TestSettleHeldOrderInvoice_RetriesThenSucceeds`,
`TestRecoverStuckOrders_HeldOrder_*`, `TestRecoverSettledCreatedOrders_AcceptedInvoice_*`).
None of them cover a `SettleInvoice`-permanently-fails scenario or the
interceptor race — that's exactly the gap this review found. Add tests for
the fixes below rather than just patching the code.

---

## Findings

Ordered most-severe first. IDs are free-standing (`H-1`..`H-15`) for
reference in commit messages, e.g. `fix(lsps1): stop JIT interceptor
treating held invoices as paid (H-1)`.

### H-1 — JIT interceptor treats `payment_state=HOLD` orders as fully paid [CONFIRMED]
**Fixed 2026-07-17:** implemented the suggested SQL-layer fix. Added
`AND payment_state='PAID'` to `PostgresInterceptStore.GetPaidLsps1Order`
(the actual live call path) and, for consistency, to
`PostgresOpeningStore.GetPaidLsps1Order`/`GetPaidLsps1Orders` (currently
unused via the interface but same contract). `GetAllPaidLsps1Orders` was
deliberately left unfiltered since `recoverStuckOrders` still needs to see
HOLD rows. Regression tests:
`TestInterceptStore_GetPaidLsps1Order_ExcludesHoldPaymentState`,
`TestGetPaidLsps1Order_ExcludesHoldPaymentState`,
`TestGetPaidLsps1Orders_ExcludesHoldPaymentState`,
`TestGetAllPaidLsps1Orders_IncludesHoldPaymentState` (all in
`internal/adapters/postgres`), verified against the real postgres test
container.

**File:** `interceptor/intercept_handler.go:753`
**Also:** `internal/adapters/postgres/intercept_store.go:139` (the query it calls)

The interceptor's independent LSPS1-JIT-fallback path calls
`i.store.GetPaidLsps1Order(clientNodeId)` and, if it gets a non-nil result
with no error, treats the order as fully paid and safe to JIT-open a
channel for (`errOrder == nil && lsps1Order != nil` — no check of
`lsps1Order.PaymentState` anywhere). `GetPaidLsps1Order`'s SQL is `WHERE
peer_id=$1 AND order_state='PAID' AND (funding_txid IS NULL OR funding_txid
= '')` — no `payment_state` filter.

**Failure scenario:** A hold-invoice order reaches `order_state=PAID` /
`payment_state=HOLD` via `processAcceptedOrder` (settlement_service.go)
before its invoice is settled. If any HTLC for that peer arrives via a
different SCID before `settlement_service`'s own `openChannelForHeldOrder`
resolves it, the interceptor independently JIT-opens a channel for the same
order, fires its own `NotifyOrderStateChanged(..., "COMPLETED", ...)` — but
never calls `SettleInvoice`, never writes `FundingTxid`, never updates the
order row. Result: a duplicate/racing channel open (real on-chain fees
spent twice), a client told `COMPLETED` for a payment never actually
captured, and the order permanently stuck at `PAID`/`HOLD` in the DB since
nothing else will ever revisit it.

**Suggested direction (not mandated — verify before implementing):** the
cleanest fix is probably at the SQL layer: change `GetPaidLsps1Order`/
`GetPaidLsps1Orders`/`GetAllPaidLsps1Orders` (or just the singular
peer-scoped ones the interceptor actually calls) to also filter
`AND payment_state='PAID'`, excluding `HOLD` rows. That directly stops the
interceptor from ever seeing a held-but-uncaptured order, with no change
needed to `interceptor/intercept_handler.go` itself. Check whether
`settlement_service.go`'s own use of `GetAllPaidLsps1Orders` (in
`recoverStuckOrders`) still needs to see `HOLD` rows — it does (it branches
on `PaymentState` after fetching, see `recoverStuckOrders`'s loop) — so if
you add the SQL filter to the shared function, you'll need a way for
`recoverStuckOrders` to still see HOLD rows, e.g. a separate query, an
optional parameter, or moving the filter to only the interceptor-specific
call sites (`intercept_store.go`'s copy). Confirm which of `opening_store.go`
/ `lsps1_settlement_store.go`'s `GetPaidLsps1Order` variants `intercept_store.go`
duplicates before deciding whether to fix in one place or several — see H-8.

### H-2 — Order marked COMPLETED even if `SettleInvoice` fails [CONFIRMED]
**Fixed 2026-07-17, together with H-3/H-4:** `settleHeldOrderInvoice` now
returns `bool`. New `settleAndCompleteHeldOrder` only writes
`order_state=COMPLETED` if settlement actually succeeded; on failure the
order is left at `PAID`/`HOLD` (with funding already recorded) for a later
`recoverStuckOrders` pass to retry. The `openChannelForHeldOrder` happy path
now records funding via the new `recordHeldOrderFundingAndSettle` *before*
attempting settlement (not after, as before) so a crash mid-settlement is
recoverable via `FundingTxid` rather than falling through to a second
`OpenChannel` attempt. The `reconcileUnclaimedChannel`-reuse ordering bug
(COMPLETED written before settlement was even attempted) is gone — that
branch now goes through the same settle-then-complete path via the new
`findUnclaimedChannelPoint` extraction. Regression test:
`TestOpenChannelForHeldOrder_SettleInvoiceFailsPermanently_DoesNotMarkCompleted`
(`lsps/settlement_service_test.go`) — forces `SettleInvoice` to fail all 5
retries and asserts `UpdateLsps1OrderState` is never called and no
notification fires.

**File:** `lsps/settlement_service.go:441` (`settleHeldOrderInvoice`), called from
`openChannelForHeldOrder` at line 379 (happy path) and line 366 (crash-recovery branch)

`settleHeldOrderInvoice` returns nothing — no error, no bool. It retries
`SettleInvoice` up to 5 times (1s apart) and, if all 5 fail, just logs
`"SettleInvoice failed after retries -- ... manual review required"` and
returns. Its caller, `openChannelForHeldOrder`, cannot check the outcome
and **unconditionally** proceeds to `UpdateLsps1OrderFunding` +
`UpdateLsps1OrderState(..., Completed, Paid, Paid)` regardless.

The crash-recovery branch (line ~365-368) is worse: `reconcileUnclaimedChannel`
(pre-existing function, see H-3) **durably writes `order_state=COMPLETED`
to the DB internally, inside itself, before `settleHeldOrderInvoice` is
even called** on the line right after it returns `true`. So in that branch
the DB says COMPLETED *before* settlement is even attempted, not just
"without checking after."

**Failure scenario:** `OpenChannel` succeeds (real on-chain funding tx
broadcast, real fees spent, client's balance pushed) but `SettleInvoice`
fails all 5 retries (e.g. a transient LND/gRPC blip). The order still shows
`COMPLETED`/`payment_state=PAID` in the DB and dashboard. The client's HTLC
was never actually settled — it sits `ACCEPTED` at LND until LND's own
hold-invoice-expiry watcher eventually force-cancels it and refunds the
payer. The LSP has opened a real channel and paid on-chain fees for a
payment it never captured, and **nothing ever revisits it**: only
`order_state='PAID'` rows get re-scanned by any recovery path, not
`COMPLETED` ones.

**Suggested direction:** make `settleHeldOrderInvoice` return `(bool, error)`
or similar. In `openChannelForHeldOrder`, only write `FundingTxid` +
`COMPLETED` if settlement actually succeeded. If it doesn't, you need a
state that (a) doesn't claim `COMPLETED` falsely and (b) gets picked up by
some recovery path to retry the settle later, since the channel is already
open and irreversible at that point — refunding is no longer an option, only
retrying settlement is. Consider: keep the row at `order_state=PAID`/
`payment_state=HOLD` with `FundingTxid` now set, and add an explicit
recovery branch (see H-4) that, on finding exactly this state, retries
`settleHeldOrderInvoice` rather than blindly completing via
`reconcileFundedOrder`. Also fix the ordering in the `reconcileUnclaimedChannel`
branch so settlement is attempted (and checked) before — not after — the
row is marked COMPLETED; that may require either not reusing
`reconcileUnclaimedChannel` as-is for the held-order path, or restructuring
it to accept a "do this before marking COMPLETED" hook.

### H-3 — `reconcileFundedOrder` (reused unmodified) never verifies settlement [CONFIRMED]
**Fixed 2026-07-17, together with H-2/H-4:** `reconcileFundedOrder` itself
is untouched (still legacy-only, per its updated doc comment) and its
channel-active check was extracted into a shared `verifyChannelActive`
helper. New `reconcileFundedHeldOrder` handles the HOLD-order case: verifies
the channel is active (reusing `verifyChannelActive`) then calls
`settleAndCompleteHeldOrder`, which actually checks/retries settlement
before completing (see H-2). `recoverStuckOrders` now routes HOLD orders to
`reconcileFundedHeldOrder` instead of `reconcileFundedOrder` (see H-4).
Regression tests:
`TestRecoverStuckOrders_HeldOrderWithFunding_RoutesToHoldAwareRecovery`,
`TestReconcileFundedHeldOrder_ChannelNotActive_DoesNotSettle`.

**File:** `lsps/settlement_service.go:671`

`reconcileFundedOrder` is a **pre-existing function**, reused verbatim
(untouched by this diff) for held orders. It marks a `PAID` order
`COMPLETED` purely by checking whether a channel exists and is active in
LND (`s.client.GetChannel`) — it never reads `payment_state` and never
calls or checks `SettleInvoice`. This was a safe assumption before this
diff (`PAID` always meant "already settled"); it's no longer safe now.

**Failure scenario:** flspd crashes between `UpdateLsps1OrderFunding` and
the final `COMPLETED` write inside `openChannelForHeldOrder`, for a held
order. On restart, `recoverStuckOrders` (line 641, see H-4) routes any
order with `FundingTxid` set through `reconcileFundedOrder` — it sees the
channel is active in LND and marks the order `COMPLETED` without ever
calling or retrying `SettleInvoice`. Same end state as H-2: order says
COMPLETED, invoice may still be `ACCEPTED` and headed for LND's own
force-cancel.

**Suggested direction:** tied to H-2/H-4. Once you have a way to detect
"held order, funding recorded, settlement not yet confirmed successful"
(see H-2's suggested state), `recoverStuckOrders` needs to route that case
through settlement-retry logic instead of (or in addition to)
`reconcileFundedOrder`. Whether `reconcileFundedOrder` itself should learn
to check `payment_state` and call `settleHeldOrderInvoice` when it's HOLD,
or whether a new dedicated function is cleaner, is a real design choice —
`reconcileFundedOrder` is also still used by the legacy (non-hold) flow, so
changing its behavior needs to not break that path (write a test for the
legacy case if one doesn't already assert this).

### H-4 — Recovery routes on `FundingTxid` before `PaymentState==HOLD` [CONFIRMED]
**Fixed 2026-07-17, together with H-2/H-3:** `recoverStuckOrders` now checks
`PaymentState==HOLD` first, before the `FundingTxid` check, and routes to
`reconcileFundedHeldOrder` (funded) or `openChannelForHeldOrder` (not yet
funded) accordingly; the plain-`FundingTxid`-then-`reconcileFundedOrder`
branch only runs for non-hold orders now. Regression test:
`TestRecoverStuckOrders_HeldOrderWithFunding_RoutesToHoldAwareRecovery`
asserts `GetChannel`/`SettleInvoice` are called (proving the hold-aware
path was taken) rather than the legacy `reconcileFundedOrder`'s blind
complete.

**File:** `lsps/settlement_service.go:641` (`recoverStuckOrders`'s loop)

```go
if order.FundingTxid != nil && order.FundingOutpoint != nil {
    s.reconcileFundedOrder(order)
    continue
}
if order.PaymentState == string(Lsps1PaymentStateHold) {
    s.openChannelForHeldOrder(order)
    continue
}
s.openChannelForOrder(order)
```

The `FundingTxid != nil` check runs **first**. This only produces correct
behavior today by accident of the current internal sequencing of
`openChannelForHeldOrder` (settle is attempted before the funding write, in
the happy path — see H-2). Nothing enforces that ordering as an invariant;
it's not documented as one anywhere the routing logic can see.

**Failure scenario:** this is the mechanism that feeds H-3. Any held order
that crashes with `FundingTxid` already recorded is *always* sent to the
settlement-unaware `reconcileFundedOrder`, never to the hold-aware path,
regardless of whether its invoice was ever actually settled. A future
change to `openChannelForHeldOrder`'s internal ordering (e.g. "record
funding before settling," a reasonable-looking refactor on its own) would
silently widen the H-2/H-3 crash window further, with no compiler or test
signal that anything broke.

**Suggested direction:** fix together with H-2/H-3. Once "held order,
funded, not yet confirmed-settled" is a detectable state, check
`PaymentState==HOLD` before (or regardless of) `FundingTxid`, and give the
hold-aware recovery path (not `reconcileFundedOrder`) responsibility for
both "funded, need to verify/retry settle" and "not yet funded, need to
retry the whole attempt" sub-cases.

### H-5 — Compare-and-set `updated` bool discarded in 3 functions [CONFIRMED]
**Fixed 2026-07-17:** all three (`cancelHeldOrder`, `reconcileFundedOrder`,
`reconcileUnclaimedChannel`) now check `updated` and log at Info + skip the
notification when it's `false`, matching the existing
`processAcceptedOrder`/`processSettledInvoice` pattern. Also applied to the
new `settleAndCompleteHeldOrder` (H-2), which has the same CAS shape.
Covered incidentally by the existing CAS-mismatch tests plus the new H-2/H-3
tests, which all exercise the `updated=true` path; no dedicated
`updated=false` test was added for these three specifically since the
mechanism is identical to the already-tested F-1 pattern.

**File:** `lsps/settlement_service.go` — `cancelHeldOrder` (line ~424, **new
code**), `reconcileFundedOrder` (line ~697, pre-existing), `reconcileUnclaimedChannel`
(pre-existing, its final `UpdateLsps1OrderState` call)

All three call `UpdateLsps1OrderState(...)` and discard the returned
`updated bool` (`_, err := ...`), checking only `err`. When the CAS
silently no-ops (`updated=false, err=nil` — normal under concurrency, not
an error), each function proceeds as if its write succeeded, including
firing a client notification claiming a transition that never hit the DB.

**Failure scenario (worked example for `cancelHeldOrder`):** it's invoked
on an order whose `order_state` has already moved past `PAID` (e.g. a
racing `reconcileUnclaimedChannel` call already marked it `COMPLETED`). The
CAS silently no-ops, but `cancelHeldOrder` still calls `CancelInvoice`
against a possibly-already-settled invoice, and sends a `FAILED` webhook to
the client for an order that actually completed successfully — a spurious,
contradictory notification with no DB state behind it.

**Suggested direction:** check `updated` in all three; on `false`, log at
Info (not Error — this is an expected race outcome, matching the existing
F-1-fix pattern elsewhere in this file) and skip the notification/further
side effects, same as `processAcceptedOrder`/`processSettledInvoice`
already do correctly for their own CAS calls.

### H-6 — Transient `IsConnected` error triggers instant refund [CONFIRMED]
**Fixed 2026-07-17:** new `checkPeerConnectedWithRetry` helper retries
`IsConnected` up to 3 times (1s apart) before giving up; only a
still-failing check after retries triggers `cancelHeldOrder`. Covered by
the existing `TestOpenChannelForHeldOrder_*` happy-path tests (which still
pass with a single successful `IsConnected` call, proving the retry loop
doesn't regress the no-error case); no dedicated retry-then-succeed test
was added since the pattern mirrors the already-tested
`TestSettleHeldOrderInvoice_RetriesThenSucceeds`.

**File:** `lsps/settlement_service.go:306` (in `openChannelForHeldOrder`)

```go
connected, err := s.client.IsConnected(peerId)
if err != nil {
    log.Error().Err(err)...
    s.cancelHeldOrder(order, err)   // instant refund
    return
}
```

Any error from the `IsConnected` RPC call (e.g. a transient gRPC/LND
hiccup, not necessarily "peer is actually offline") is treated identically
to a confirmed not-connected result — both trigger an immediate
`cancelHeldOrder` (refund). The legacy `openChannelForOrder`'s equivalent
branch only reports `FAILED` on this same error class (no refund action
needed there, since funds are already captured in that flow).

**Failure scenario:** a brief RPC error unrelated to actual peer
reachability causes an immediate `CancelInvoice` + refund of an order that
could very likely have succeeded moments later — a needless failure and a
worse client experience than the legacy flow's equivalent error path.

**Suggested direction:** don't treat an `IsConnected` RPC error the same as
a confirmed-not-connected result. At minimum, retry the check a couple of
times before giving up; consider whether "can't determine connectivity"
should fail differently (e.g. without an instant refund) than "confirmed
not connected."

### H-7 — No compensating `CancelInvoice` if `RegisterLsps1Order` fails after `AddHoldInvoice` succeeds [PLAUSIBLE]
**Fixed 2026-07-17:** added `CancelInvoice` to the `LndClientWrapper`
interface (`lsps/handlers.go`) and call it with the invoice's payment hash
in the `RegisterLsps1Order` error branch of `handleLsps1CreateOrder`, before
returning the DB-error response. `CancelInvoice`'s own failure is logged
but not fatal (LND's hold-invoice expiry is still the fallback). Regression
test: `TestHandleLsps1CreateOrder_RegisterOrderFails_CancelsOrphanedHoldInvoice`
(`lsps/handler_lsps1_test.go`) asserts `CancelInvoice` is called with the
exact hash that was passed to `AddHoldInvoice`.

**File:** `lsps/handler_lsps1.go:404` (order-creation flow, roughly — check current line via `grep -n RegisterLsps1Order lsps/handler_lsps1.go`)

If `AddHoldInvoice` succeeds at LND but the subsequent `RegisterLsps1Order`
DB write fails, there's no compensating `CancelInvoice` call, and the
locally-generated preimage (a local variable, never persisted since the DB
write failed) is lost. Unlike a regular `AddInvoice`, where LND itself
still owns the preimage and the invoice just expires harmlessly, a hold
invoice with a lost preimage can never be explicitly settled or cancelled
by flspd.

**Failure scenario:** a transient DB error right after invoice creation
leaves an orphaned hold invoice at LND with no stored preimage anywhere in
flspd. If a payer's HTLC ever locks into it, flspd has no way to resolve
it — the HTLC stays held until LND's own hold-invoice-expiry watcher
eventually releases it (on the order of tens of minutes per the code's own
comments) — a real hold-time window that didn't exist for the old
regular-invoice path.

**Suggested direction:** on `RegisterLsps1Order` failure, call
`CancelInvoice` with the hash before returning the error response. Cheap,
bounded fix — this is a narrow crash window (RegisterLsps1Order failing
specifically in the gap after invoice creation), not architecturally
significant, but easy to close.

### H-8 — `intercept_store.go`'s `GetPaidLsps1Order` never got the `payment_preimage` column [CONFIRMED]
**Fixed 2026-07-17, together with H-1:** added `payment_preimage` to this
query's SELECT/Scan. Regression test:
`TestInterceptStore_GetPaidLsps1Order_ReturnsPreimage`
(`internal/adapters/postgres/intercept_store_test.go`).

**File:** `internal/adapters/postgres/intercept_store.go:143`

This diff added `payment_preimage` to all 7 `Lsps1Order` SELECT/Scan sites
in `opening_store.go` and `lsps1_settlement_store.go`, but left this
separate, pre-existing, mostly-duplicate copy of the same query (used only
by the interceptor's JIT fallback) unupdated — it always returns
`PaymentPreimage=nil`.

**Failure scenario:** compounds H-1. Even once H-1 is fixed at the SQL
filter level, if that fix (or any future code) ever needs this specific
query's result to carry the preimage, it silently can't — forcing any such
logic down the "no stored preimage" error branches in
`settlement_service.go`.

**Suggested direction:** add `payment_preimage` to this query's SELECT/Scan
for consistency with the other 7 sites, whether or not it's immediately
consumed by the interceptor path — matches the fix you're likely making for
H-1 anyway if that involves editing this file's query.

### H-9 — No preimage length validation before hashing in `cancelHeldOrder` [PLAUSIBLE]
**Fixed 2026-07-17:** added `len(preimage) != 32` checks (log Error + bail,
matching the existing nil/decode-error pattern) in both `cancelHeldOrder`
and `settleHeldOrderInvoice`. Not separately unit-tested (the check is a
straight-line early-return with no branching logic to get wrong; all
existing tests continue to exercise the real 32-byte fixture path via
`testHeldOrderPreimage`).

**File:** `lsps/settlement_service.go:412` (`cancelHeldOrder`)

`hex.DecodeString` on a non-nil-but-empty (or otherwise malformed/
wrong-length) `PaymentPreimage` succeeds without error — `cancelHeldOrder`
would silently `sha256` the wrong bytes and call `CancelInvoice` with an
incorrect payment hash. `CancelInvoice`'s own failure is only logged at
`Warn`.

**Failure scenario:** low likelihood given preimages are always generated
as 32 random bytes today (see `lsps/handler_lsps1.go`), but there's no
explicit length check before use — a data-integrity issue (truncated
column value, migration bug, manual DB edit) would silently target the
wrong invoice and fail to release the held HTLC, masked by the Warn-only
logging.

**Suggested direction:** add `len(preimage) != 32` check before use in both
`cancelHeldOrder` and `settleHeldOrderInvoice`; log at Error and bail
(same "manual review required" pattern already used for the nil/decode-error
cases right above it) if the length is wrong.

### H-10 — Periodic poll runs synchronously in the shared event-dispatch loop [PLAUSIBLE]
**Fixed 2026-07-18, together with H-11/H-12:** the poll now runs in its own
goroutine per tick (tracked by `s.wg`), guarded by an `atomic.Bool` overlap
flag (`pollRunning`) that skips a tick if the previous one hasn't finished —
it can no longer block the shared `select` loop from reading a live
`invoiceChan`/`errChan` event. Also demoted from the primary ACCEPTED-
detection path to a safety net now that H-12's per-invoice watcher is live:
renamed `heldInvoicePollInterval` → `heldInvoiceSafetyNetPollInterval`,
interval raised 3s → 60s.

**File:** `lsps/settlement_service.go:128` (the `case <-pollTicker.C:` branch in `Start()`)

`recoverSettledCreatedOrders` — previously run once at startup, off the hot
path, via `go s.recoverStuckOrders()` — is now *also* invoked synchronously
every `heldInvoicePollInterval` (3s) from inside the same `select` loop
that dispatches `invoiceChan`/`errChan`/`ctx.Done()`. It does one blocking
`LookupInvoiceState` RPC per `CREATED` order (see H-11), and on a
stale-invoice hit can call `processSettledOrder` → `openChannelForOrder`
synchronously — a real `OpenChannel`/broadcast call.

**Failure scenario:** with tens of pending `CREATED` orders, a poll cycle
blocks this goroutine for the duration of N sequential RPC round trips (or
an inline channel-open); during that window the loop can't read
`invoiceChan`, so a genuine live `SETTLED`/`ACCEPTED` event for a different,
concurrently-paying customer sits unprocessed until the poll (or
channel-open) finishes — added latency for exactly the users this feature
is meant to serve fast.

**Suggested direction:** run the poll's work (or at least the per-order RPC
loop) in its own goroutine, guarding against overlapping runs if a poll
takes longer than the interval (e.g. a simple "already running" flag/mutex,
or a buffered trigger channel consumed by a single dedicated poll
goroutine).

### H-11 — Redundant `DecodePayReq` RPC on every poll [PLAUSIBLE]
**Fixed 2026-07-18, together with H-10/H-12:** new
`LookupInvoiceStateByHash` (`internal/adapters/lnd/client.go`) skips
`DecodePayReq`, calling `LookupInvoice` directly. New
`paymentHashForOrder`/`decodeAndValidatePreimage` helpers on
`Lsps1SettlementService` (also de-duplicating the H-9 preimage-decode
logic previously repeated in `cancelHeldOrder`/`settleHeldOrderInvoice`)
derive the hash locally. `recoverSettledCreatedOrders` uses the hash-based
path whenever `order.PaymentPreimage != nil`, falling back to the original
`DecodePayReq` path only for legacy non-hold orders.

**File:** `internal/adapters/lnd/client.go:952` (`LookupInvoiceState`)

`LookupInvoiceState` calls `DecodePayReq` to derive the payment hash on
every invocation, even though for hold-invoice orders the hash is already
derivable in-process from `order.PaymentPreimage` (exactly as
`cancelHeldOrder` already does locally via `sha256.Sum256`).

**Failure scenario:** doubles the per-order, per-poll RPC cost (2 round
trips instead of 1) for the new hold-invoice path — at tens of pending
orders every 3s, an avoidable extra full RPC round trip per order per tick.

**Suggested direction:** for orders with a non-nil `PaymentPreimage`, derive
the hash locally and call LND's invoice lookup by hash directly instead of
going through `DecodePayReq`. Lower priority than H-1/H-2 — real but bounded
cost, not a correctness bug.

### H-12 — Poll-based ACCEPTED detection is a workaround for a solvable API gap [PLAUSIBLE, architectural]
**Fixed 2026-07-18:** implemented exactly the suggested direction.
`lightning.Client` gained `SubscribeSingleInvoice`, implemented in
`internal/adapters/lnd/invoice_subscription.go` following the existing
`SubscribeInvoices` wrap-a-gRPC-stream-in-a-goroutine pattern. New
`OrderWatcher` interface (`lsps/handlers.go`) + a new trailing
`NewHandlers(...)` constructor param wire `*Lsps1SettlementService`
(satisfies the interface via new `WatchOrder`) into `Handlers`;
`handleLsps1CreateOrder` calls `WatchOrder` right after
`RegisterLsps1Order` succeeds. `watchOrderInvoice` (the watcher goroutine
body) subscribes, dispatches `ACCEPTED`/`SETTLED`/`CANCELED` into the
existing `processAcceptedOrder`/`processSettledOrder` handlers, and
deliberately does **not** self-retry on a stream error (falls back to the
H-10/H-11 safety-net poll instead, which also re-attaches a watcher for
any still-`OPEN` order it finds via a new idempotent `watchedOrders
sync.Map` guard on `WatchOrder`). `cmd/flspd/run.go` only wires the
watcher in when `settlementSvc.Start(ctx)` actually succeeded.

**Devnet-verified, not just trusted from the proto docs** (per this
finding's own caution that this flnd fork has diverged from upstream LND
invoice-subscription behavior before): new
`TestSubscribeSingleInvoiceDeliversAccepted`
(`test/itest/subscribe_single_invoice_test.go`) drives
`invoicesrpc.InvoicesClient` directly against real flnd and proves both
that the initial state is sent immediately on subscribe (`OPEN`) and that
a second `ACCEPTED` message arrives once a payer's HTLC locks in — no
polling involved. Separately, comparing `TestLsps1OrderSettleAndOpen`'s
logs before and after this fix confirms the live watcher, not the poll, is
what fires in production: post-fix runs show `Hold invoice ACCEPTED`
within the same second as `create_order`, with no
`"Checking CREATED orders against LND for missed settlements"` (the poll's
log line) in between — impossible within a 1-second window now that the
poll only ticks every 60s.

**File:** `lsps/settlement_service.go:24` (the `heldInvoicePollInterval` doc comment)

The 3s poll is a reasonable pragmatic fix for a real constraint — confirmed
live against real flnd (2026-07-17) that LND's base `SubscribeInvoices` RPC
only ever delivers "added" and "settled" events; there's no channel/index
for `ACCEPTED` at all in `invoices.InvoiceSubscription`. But it's layered on
existing recovery-sweep infrastructure rather than using the standard LND
mechanism for exactly this problem: `invoicesrpc.InvoicesClient.SubscribeSingleInvoice(paymentHash)`,
attached once per hold invoice right at creation. The payment hash is
already computed in-process in `lsps/handler_lsps1.go` at invoice-creation
time, and the `invoicesClient` is already wired in
`internal/adapters/lnd/client.go` for `AddHoldInvoice`/`SettleInvoice`/
`CancelInvoice` — the missing piece is just wiring `SubscribeSingleInvoice`
through the same client and attaching a watcher goroutine per invoice.

**Cost of not doing this:** H-10 and H-11 are both direct downstream
consequences. A per-invoice subscription would give near-instant `ACCEPTED`
reaction with zero ongoing cost for invoices that never change state,
versus up to 3s latency plus a permanent full-DB-scan-and-poll cost for the
lifetime of the service.

**Suggested direction:** flag as a known follow-up rather than blocking on
it — the poll is a correct, if not optimal, fix for the core bug it
addresses (H-2/H-3/H-4 above are more urgent). Worth a dedicated task once
the correctness findings are closed.

### H-13 — `openChannelForHeldOrder` duplicates ~40 lines of `openChannelForOrder` [CONFIRMED, cleanup]
**Fixed 2026-07-17:** extracted `buildOpenChannelRequest(order, cfg, peerId)
*lightning.OpenChannelRequest`, covering fee-estimation fallback + capacity
computation + request construction, called by both `openChannelForOrder`
and `openChannelForHeldOrder`. Each keeps its own connectivity-check and
success/error tail. Covered by the existing tests for both functions (all
still pass unchanged).

**File:** `lsps/settlement_service.go:289` (new function) vs `:469` (pre-existing `openChannelForOrder`)

The cfg snapshot, `EstimateFeeRate`+fallback block, capacity computation,
and the entire `OpenChannelRequest` construction/`OpenChannel` call are
byte-for-byte identical between the two functions. Only the
connectivity-check and success/error tails genuinely differ (by design —
see H-6 for one place they *should* differ but the duplication makes it
easy to miss keeping them in sync).

**Cost:** any future change to fee-estimation fallback, capacity
calculation, or the `OpenChannelRequest` shape must be made in two places by
hand; missing one silently makes held-order and legacy-order channel opens
behave differently.

**Suggested direction:** extract a shared helper, e.g.
`buildOpenChannelRequest(order *core.Lsps1Order, cfg *config.NodeConfig)
(*lightning.OpenChannelRequest, error)`, covering the fee-estimation +
capacity + request-construction core, called by both `openChannelForOrder`
and `openChannelForHeldOrder`, each keeping its own connectivity-check and
settle/cancel-vs-complete tail.

### H-14 — `InvoiceStateAccepted` live-event branch is dead code [CONFIRMED, cleanup]
**Fixed 2026-07-17:** removed the `case lightning.InvoiceStateAccepted`
branch from `Start()`'s event-loop switch (confirmed genuinely unreachable
against real LND, not going away without H-12, which is deliberately
deferred). `processAcceptedInvoice`/`processAcceptedOrder` were **not**
deleted -- they remain directly unit-tested
(`TestProcessAcceptedInvoice_PeerNotConnected_CancelsHeldOrder`,
`TestProcessAcceptedInvoice_OrderNotFound`) and would become live again if
H-12's per-invoice subscription is ever wired up. This resolves the
"looks-live-but-isn't" branch without discarding tested logic.

**File:** `lsps/settlement_service.go:104` (in `Start()`'s event-loop `switch`)

```go
case lightning.InvoiceStateAccepted:
    // Unreachable via the base SubscribeInvoices RPC in practice ...
    s.processAcceptedInvoice(update)
```

Confirmed unreachable against real LND (see H-12's context) — the switch
branch and `processAcceptedInvoice` are maintained and tested as if live,
but can never fire in production; the poll ticker (H-10) is the actual
delivery path.

**Suggested direction:** either remove the branch (and let
`processAcceptedInvoice` be dead-code-eliminated, or keep it only as a
private helper called by the poll path if it isn't already), or — if
you implement H-12's `SubscribeSingleInvoice` fix — this branch becomes
live and correct as-is, which is presumably why it was kept rather than
deleted. Resolve one way or the other rather than leaving a
looks-live-but-isn't branch in place indefinitely.

### H-15 — Unread local state mutation after the CAS in `processAcceptedOrder` [PLAUSIBLE, cleanup]
**Fixed 2026-07-17:** deleted the two lines and the comment. Confirmed after
the H-1..H-6 fixes landed that no downstream consumer needs
`order.OrderState`/`order.PaymentState` current at this point (the H-2 fix's
`recordHeldOrderFundingAndSettle` does still mutate `order.FundingTxid`/
`FundingOutpoint` locally, but that's a different, still-needed case, not
the one this finding flagged).

**File:** `lsps/settlement_service.go:274`

```go
// Reflect the CAS locally so the caller doesn't need a re-fetch.
order.OrderState = string(Lsps1OrderStatePaid)
order.PaymentState = string(Lsps1PaymentStateHold)
```

No downstream consumer (`openChannelForHeldOrder`, `cancelHeldOrder`,
`settleHeldOrderInvoice`) actually reads either field off this pointer
afterward. The sibling recovery function `processSettledOrder` (same
shape, calls `openChannelForOrder`) omits the equivalent mutation, because
its callee doesn't need it either — confirming this one doesn't.

**Suggested direction:** delete the two lines and the comment, unless a
fix for one of the H-1..H-6 findings above ends up needing
`order.PaymentState` to be current at this point (check after making those
fixes, not before).

---

## Second-pass findings (2026-07-17): review of the fixes above

After landing H-1..H-9/H-13/H-14/H-15, ran an 8-angle multi-agent review
(line-by-line diff scan, removed-behavior audit, cross-file caller/callee
trace, reuse, simplification, efficiency, altitude, CLAUDE.md conventions)
against the fix diff itself, to catch bugs the fixes introduced or gaps they
left. IDs are `S-1`..`S-10`, most-severe first. `S-1`..`S-4` were fixed;
`S-5`..`S-10` were judged lower-priority/pre-existing and left as
documented follow-ups, same treatment as H-10/H-11/H-12 above.

### S-1 — `GetAllPaidLsps1Orders` excluded funded rows, making H-3/H-4's fix unreachable [CONFIRMED, FIXED]
**File:** `internal/adapters/postgres/lsps1_settlement_store.go` (`GetAllPaidLsps1Orders`)

The query's WHERE clause was `order_state='PAID' AND (funding_txid IS NULL
OR funding_txid = '')` — excluding any row with funding recorded. This is
the *only* source of `recoverStuckOrders`'s `paidOrders` slice, so its
`order.FundingTxid != nil` branch (both the pre-existing legacy
`reconcileFundedOrder` call and H-3/H-4's new `reconcileFundedHeldOrder`
call) could never fire in production — a held order that crashes after
funding is recorded but before settlement completes was invisible to every
recovery sweep, forever. This bug predates the hold-invoice feature (the
legacy branch was equally dead), but H-3/H-4 was built directly on top of
it and inherited the same unreachability; the mock-based regression test
written for H-3/H-4 passed only because it bypasses the real SQL filter.

**Fixed:** removed the `funding_txid` exclusion from the SQL — recovery now
relies entirely on `recoverStuckOrders`'s own Go-level branching (which
already correctly distinguishes funded/unfunded/HOLD) rather than having
the query silently pre-filter out the exact rows that branching exists to
handle. Updated `TestGetAllPaidLsps1Orders_ExcludesFundedAndOtherStates`
(renamed `..._IncludesFundedExcludesOtherStates`) to assert inclusion
instead. Verified against the real postgres test container, and against
`test/devnet` (`TestScenarios/lsps1_recovery_after_restart` passes).

### S-2 — `get_order` showed funded-channel info while payment still HOLD [CONFIRMED, FIXED]
**File:** `lsps/handler_lsps1.go` (`handleLsps1GetOrder`)

The response's `Channel` block was populated purely from
`order.FundingTxid != nil`, with no check of payment/order state. H-2's fix
made `recordHeldOrderFundingAndSettle` record `FundingTxid` *before*
settlement is confirmed (for crash-safety), widening a previously
millisecond-scale window into one that can now last seconds, or — before
S-1's fix — permanently. A client polling `get_order` during that window
would see `Payment.Bolt11.State="HOLD"` (not paid) alongside a populated
`Channel` block implying the channel is funded and ready — a
self-contradictory response.

**Fixed:** gated the `Channel` block on `orderState ==
string(Lsps1OrderStateCompleted)`, not just `FundingTxid != nil` — the
response is now internally consistent: channel details only appear once
the order has genuinely reached its terminal success state. Regression
test: `TestHandleLsps1GetOrder_HeldOrderWithFunding_DoesNotReportChannel`.

### S-3 — `openChannelForOrder` still discarded the CAS `updated` bool [CONFIRMED, FIXED]
**File:** `lsps/settlement_service.go` (`openChannelForOrder`, both the FAILED and COMPLETED writes)

This function was touched by this diff for H-13's `buildOpenChannelRequest`
extraction, but its own two `UpdateLsps1OrderState` calls still discarded
the `updated` bool with `_, err :=` — the exact H-5 pattern fixed in three
sibling functions, missed here. A losing concurrent writer (e.g. a live
event racing a recovery sweep) would still fire a spurious FAILED or
COMPLETED notification for a transition that didn't actually happen at the
DB level.

**Fixed:** added the `updated` check to both writes, matching the pattern
used everywhere else (log Info + skip notification on `false`). Regression
tests: `TestOpenChannelForOrder_OpenChannelFails_ConcurrentWriterWins_NoSpuriousNotification`,
`TestOpenChannelForOrder_Success_ConcurrentWriterWins_NoSpuriousNotification`.

### S-4 — Legacy log message text drifted during `verifyChannelActive` extraction [CONFIRMED, FIXED]
**File:** `lsps/settlement_service.go` (`verifyChannelActive`)

Extracting the shared channel-active check changed the legacy path's log
message from `"leaving PAID for manual review, not retrying OpenChannel"`
to `"leaving as-is for manual review, not retrying OpenChannel"`, violating
this same review's H-3 requirement that `reconcileFundedOrder`'s legacy
behavior stay byte-for-byte identical (an operator alert/runbook could be
keyed on the exact string).

**Fixed:** reverted to the original exact text.

### S-5 — `findUnclaimedChannelPoint` TOCTOU race for concurrent same-peer orders [PLAUSIBLE, FIXED]
Two held orders to the same peer, recovered/processed concurrently, could
both see the same "unclaimed" channel and both attribute it to themselves.

**Fixed 2026-07-18:** new migration
`000006_add_lsps1_funding_unique_constraint` adds a partial unique index
on `lsps1.orders (funding_txid, funding_outpoint) WHERE funding_txid IS
NOT NULL AND funding_txid <> ''`, making the claim atomic at the DB level
even though the read (`findUnclaimedChannelPoint`'s `ListChannels` +
claimed-points scan) still isn't. `UpdateLsps1OrderFunding`'s signature
changed from `(...) error` to `(...) (claimed bool, err error)`: a
Postgres unique-violation (SQLSTATE 23505) now surfaces as `(false, nil)`
— a normal, expected outcome under concurrency — instead of an error. All
call sites (`recordHeldOrderFundingAndSettle`, `openChannelForOrder`,
`reconcileUnclaimedChannel`) updated to back off on `claimed=false` exactly
as if no unclaimed channel had been found. Regression tests (real postgres
container): `TestUpdateLsps1OrderFunding_ConflictingChannelPointReturnsNotClaimed`,
and — proving the guarantee under actual concurrency, not just sequential
calls — `TestUpdateLsps1OrderFunding_ConcurrentClaims_ExactlyOneWins` (10
goroutines race to claim the same channel point; exactly one succeeds).

### S-6 — `recordHeldOrderFundingAndSettle` doesn't bail if the funding write fails [PLAUSIBLE, FIXED]
Only logged, didn't return, if `UpdateLsps1OrderFunding` errored — matched
`openChannelForOrder`'s pre-existing identical pattern.

**Fixed 2026-07-18, together with S-5:** both `recordHeldOrderFundingAndSettle`
and `openChannelForOrder`'s equivalent call now return immediately
(without settling/marking COMPLETED) if the funding claim doesn't succeed
— an error, or (per S-5) `claimed=false`. The order is left at `PAID`
with `FundingTxid` still nil, so the next `recoverStuckOrders` pass routes
it back through `openChannelForOrder`/`openChannelForHeldOrder`, whose
retried `OpenChannel` collides with the already-open channel and falls
through to the existing orphaned-channel recovery
(`findUnclaimedChannelPoint`/`reconcileUnclaimedChannel`/
`reconcileFundedHeldOrder`) — reusing that mechanism rather than risking an
order marked COMPLETED with no `funding_txid` ever persisted. Regression
tests: `TestOpenChannelForHeldOrder_FundingClaimFails_DoesNotSettleOrComplete`,
`TestOpenChannelForOrder_FundingClaimFails_DoesNotMarkCompleted`.

### S-7 — H-7's compensating `CancelInvoice` blocks the client-facing error response [PLAUSIBLE, FIXED]
**Fixed 2026-07-18:** wrapped in `go func() { ... }()`, matching the
fire-and-forget pattern already used for notifications elsewhere in
`handler_lsps1.go` — its outcome never changes the response already
decided. Existing regression test
(`TestHandleLsps1CreateOrder_RegisterOrderFails_CancelsOrphanedHoldInvoice`)
updated with a short `time.Sleep` before asserting on the mock, since the
call is no longer synchronous with the response.

### S-8 — `verifyChannelActive` has no defensive nil-check of its own [PLAUSIBLE, FIXED]
**Fixed 2026-07-18:** added an explicit
`if order.FundingTxid == nil || order.FundingOutpoint == nil` guard at the
top — logs "this is a caller bug" and returns `false` instead of risking a
nil-pointer panic in `NewOutPointFromString`. Regression test:
`TestVerifyChannelActive_NilFunding_ReturnsFalseNoPanic`.

### S-9 — `TestSettleHeldOrderInvoice_RetriesThenSucceeds` doesn't assert the bool return [PLAUSIBLE, FIXED]
**Fixed 2026-07-18:** the test now captures `settleHeldOrderInvoice`'s
return value and asserts it's `true`.

### S-10 — `settleAndCompleteHeldOrder`'s `FundingTxid`/`FundingOutpoint` nil-check is dead code [PLAUSIBLE, RESOLVED AS-IS]
Both current callers always populate those fields first, so the nil-check
never fires. **Reviewed 2026-07-18, kept as-is (no code change):** cheap
insurance against a future third caller forgetting to populate them first,
at the cost of being technically unreachable today. Comment at the call
site tightened to say so explicitly and note the review date, so this
doesn't look like an oversight to a future reader.

---

## Suggested fix order

1. ~~**H-1 + H-8 together**~~ **DONE 2026-07-17.**
2. ~~**H-2 + H-3 + H-4 together**~~ **DONE 2026-07-17.**
3. ~~**H-5, H-6, H-7, H-9**~~ **DONE 2026-07-17.**
4. ~~**H-13, H-14, H-15**~~ **DONE 2026-07-17.**
5. ~~**H-10, H-11, H-12**~~ **DONE 2026-07-18.** Per-invoice
   `SubscribeSingleInvoice` watchers (H-12) now do live ACCEPTED detection;
   the poll (H-10/H-11) is demoted to a 60s safety net, off the shared
   event-dispatch loop, using a hash-based lookup instead of `DecodePayReq`.
6. ~~**S-1, S-2, S-3, S-4**~~ **DONE 2026-07-17** (found by the second-pass
   self-review, fixed same day).
7. ~~**S-5, S-6, S-7, S-8, S-9**~~ **DONE 2026-07-18.** S-10 resolved as-is
   (no code change needed — see its entry above).

**Every finding in this document is now closed.** Nothing left to do here
short of a future, unrelated change surfacing something new.

Verification performed 2026-07-18 (final pass, all passed):
- `go build ./... && go vet ./...` — clean across the whole repo.
- `go test ./...` — every package passes except the one pre-existing,
  unrelated devnet flake noted below.
- All four LSPS1 devnet scenarios pass against `test/devnet`:
  `TestLsps1OrderSettleAndOpen`,
  `TestScenarios/lsps1_recovery_after_restart`,
  `TestScenarios/lsps1_settle_under_fee_congestion`, and the new
  `TestSubscribeSingleInvoiceDeliversAccepted` (H-12's dedicated
  live-RPC-behavior proof).
- `TestLsps2JitHappyPath` / `TestScenarios/lsps2_jit_happy_path` fail with
  `FAILURE_REASON_NO_ROUTE` both with and without this review's fixes
  applied (confirmed via `git stash` on 2026-07-17, and reconfirmed still
  the only failure on 2026-07-18) — pre-existing devnet liquidity/routing
  state unrelated to LSPS1 hold invoices, not a regression from any work
  in this document.
- H-1's interceptor-race was **not** additionally reproduced with a new
  devnet/concurrency scenario (the suggestion above was explicitly "if
  feasible"). The SQL-layer fix removes the race by construction — the
  interceptor's query can no longer return a HOLD row at all, not just "wins
  the race most of the time" — and is covered directly by
  `TestInterceptStore_GetPaidLsps1Order_ExcludesHoldPaymentState` and
  siblings at the exact boundary where the bug lived, so a
  timing-dependent end-to-end reproduction wasn't judged worth the added
  test-infra cost.
- S-5's concurrency race, by contrast, **was** reproduced directly under
  real concurrency against the postgres test container
  (`TestUpdateLsps1OrderFunding_ConcurrentClaims_ExactlyOneWins`), since
  the DB-level unique-constraint fix made that straightforward rather than
  requiring devnet-level timing control.
