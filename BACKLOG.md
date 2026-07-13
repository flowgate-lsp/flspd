# flspd Engineering Backlog — Stabilize, Secure, Redesign

This file consolidates every finding from the 2026-07-13 review (test/architecture
audit + repo structure/config UX review) into one tracked, phase-gated task list.
It is the single source of truth for this work — update statuses here rather than
letting decisions live only in chat or in the review artifacts.

## How to use this file

- **Work top to bottom.** Phases are ordered on purpose: a later phase assumes
  the gate of every earlier phase is met. Do not start Phase 3+ work while
  Phase 0/1 gates are open — see "Why this order" below.
- Each task has an ID, so it can be referenced from commit messages and PRs
  (e.g. `fix(interceptor): repair mock drift (F-3)`).
- Mark tasks `[x]` when merged to `main` with a passing CI run, not when a PR
  is opened.
- If you find something new, add it under the right phase with the next free
  ID in that series rather than opening a side conversation about it.

## Why this order

The test suite is currently red (two suites panic on run), so nothing can
prove whether a change broke a money path. Every redesign item (config model,
`common/` split, tree move) is far riskier without that signal. The order is:
**contain the live exposure → get the suite green → fix bugs that already
exist on today's structure → raise coverage on the critical path → only then
redesign config and the tree on top of a suite that will catch regressions.**
Config precedes structure: the `common/` package split (Phase 6) should land
on the already-cleaned config model (Phase 5), not migrate the 40-field
number-soup and then get reorganized a second time.

---

## Status summary

| Phase | Theme | Gate to proceed | Status |
|---|---|---|---|
| 0 | Contain live secret exposure | Secrets rotated, file untracked | ☐ Not started |
| 1 | Green the build + safety net | `go test ./... -race` passes in CI | ☐ Not started |
| 2 | Fix defects on current structure | All P2 items merged w/ regression test | ☐ Not started |
| 3 | Unit-test coverage buildout | Critical-four packages ≥70–80% | ☐ Not started |
| 4 | YAML-driven integration tests | Runner + 3 seed scenarios merged | ☐ Not started |
| 5 | Config model + secrets-by-reference | New model shipped behind compat shim | ☐ Not started |
| 6 | Repo structure / arbo migration | Tree matches target layout, build green throughout | ☐ Not started |
| — | Performance backlog (parallel, non-blocking) | — | ☐ Not started |

---

## Phase 0 — Containment (do first, today, independent of everything else)

### SEC-1: Rotate and purge the committed production secret
**Severity:** Critical · **Blocks:** everything (do not defer)

**Finding:** `data/nodes.prod.yaml` is tracked in `HEAD` (added in commit
`be7495d`, "chore: support Wumbo capacity and redesign settings UI") despite
`data/` being listed in `.gitignore`. The ignore rule has no effect on a file
that was already committed/force-added. Contents confirmed live, not
placeholders:
- `flspdPrivateKey` — 64-char hex, real
- `flnd.macaroon` — 586-char hex, real
- `flnd.cert` — 1628-char hex PEM, real
- A live auth token
- Production host IP (195.201.235.46)

**Scope of work:**
1. Confirm whether the repo has ever been pushed to a remote / cloned by
   others. If yes, treat every value above as compromised regardless of
   whether history is later scrubbed.
2. **Rotate all four**: regenerate the LSP `flspdPrivateKey`, bake a fresh
   LND macaroon, reissue the auth token, and confirm whether the TLS cert
   needs reissuing (it's the LND gRPC cert — rotate if it was generated
   specifically for this deployment).
3. `git rm --cached data/nodes.prod.yaml` and commit — stops future tracking;
   file remains on disk locally.
4. Scrub history with `git filter-repo` (preferred) or BFG Repo-Cleaner to
   remove the file from all past commits. Coordinate force-push with anyone
   else who has a clone (their clones will need to be re-fetched/rebased).
5. Add a pre-commit or CI check that fails the build if any file under
   `data/` (or any path matching common secret patterns) is staged for commit,
   so this class of mistake can't silently recur before Phase 5 lands the
   structural fix.

**Acceptance criteria:** old key/macaroon/token are confirmed non-functional
against the live node; `git log --all -- data/nodes.prod.yaml` returns nothing
after the history scrub; a secret-scan guard exists in CI.

---

## Phase 1 — Green the build, build the safety net

Nothing in Phase 2+ should start until this phase's gate is met:
`go test ./... -race` passes with no panics, and CI runs it on every PR.

### T1-1: Repair `interceptor` suite (root cause of F-3)
**Severity:** High (blocking) · **Location:** `interceptor/intercept_handler_test.go:80` vs `interceptor/intercept_handler.go:632`

**Scope of work:** `TestResolvePayment_JIT_Success` asserts the old
proportional fee model (`outgoing = 19,920,000`). The code now grosses up
with `outgoing = input / (1 + rate)`, yielding `19,920,318`. Recompute the
expected value from the current formula. While in this file, extract the fee
math out of `resolvePayment` into a pure function (e.g.
`computeJitOutgoingAmount(input, minFee, proportional)`) so it can be
table-tested independently of the `MockStore` — this also sets up T3-2 and
the shared-fee-model work in Phase 5.

**Acceptance criteria:** `go test ./interceptor/...` passes; new pure function
has its own table test covering the values above.

### T1-2: Repair `dashboard` suite (root cause of F-4)
**Severity:** High (blocking) · **Location:** `dashboard/service_test.go:349` vs `dashboard/service.go:812`

**Scope of work:** `getLiquidityMetrics` calls
`nodes[0].Client.WalletBalance(ctx)`; the `get_summary` subtest's
`MockLightningClient` never registers `.On("WalletBalance", ...)`. Add the
missing expectation returning a realistic `*lightning.WalletBalanceResult`.

**Acceptance criteria:** `go test ./dashboard/...` passes with no panic.

### T1-3: Stand up CI running the full suite with the race detector
**Severity:** High (blocking)

**Scope of work:** Add (or fix) a CI job running `go build ./...`,
`go vet ./...`, and `go test ./... -race -count=1` on every PR and on `main`.
Gate merges on it. If CI already exists but doesn't run `-race` or doesn't
fail the pipeline on test panics, fix that first — a red suite that doesn't
fail CI is why F-3/F-4 went unnoticed.

**Acceptance criteria:** a PR that reintroduces either T1-1 or T1-2's bug
fails CI.

### T1-4: Characterization tests for the two money paths
**Severity:** High (blocking for Phase 5/6)

**Scope of work:** Before any redesign work touches `Intercept` (interceptor)
or the settlement state machine (`lsps/settlement_service.go`), pin their
*current* observable behavior with tests: given a fixed set of inputs
(payment sizes, fee params, order states), assert the exact `InterceptResult`
/ order-state transitions produced today. These are not "good" tests in the
design sense — they're a tripwire so a later refactor that changes behavior
fails loudly instead of silently. They can be superseded by the proper
table-driven tests in Phase 3.

**Acceptance criteria:** at least one characterization test exists per
public entrypoint of `Interceptor` and `Lsps1SettlementService`.

---

## Phase 2 — Fix defects that exist on today's structure

Each item below ships as its own PR with the regression test that proves the
fix, independent of any redesign. Do not bundle these with Phase 5/6 changes.

### F-1: Settlement recovery can open two channels for one order
**Severity:** Critical · **Location:** `lsps/settlement_service.go:145,392` · `postgresql/lsps1_settlement_store.go:35`

**Scope of work:** `UpdateLsps1OrderState` is an unconditional
`UPDATE ... WHERE order_id=$1` — no expected-state precondition, so it's not
an optimistic lock. `recoverSettledCreatedOrders()` (startup sweep) and the
live `SubscribeInvoices` replay can both observe state `CREATED`, both pass
the guard, and both call `openChannelForOrder`. Change the PAID transition to
a compare-and-set: `UPDATE ... SET order_state='PAID' WHERE order_id=$1 AND
order_state='CREATED'`, and only proceed to `openChannelForOrder` when
`RowsAffected() == 1`. Apply the same CAS discipline anywhere else
`UpdateLsps1OrderState` drives a one-time side effect.

**Acceptance criteria:** a test that races the startup-recovery path against
a concurrent `SubscribeInvoices` event for the same order asserts exactly one
channel-open call.

### F-2: Nil-pointer dereference on payments with null fee params
**Severity:** High · **Location:** `postgresql/intercept_store.go:53-61`

**Scope of work:** `PaymentInfo` only assigns `extParams` when the
`opening_fee_params` column is non-null, then unconditionally dereferences
`extParams.Token` / `.OrderId` / `&extParams.Params`. Guard the nil case and
return zero values (or, if no code path should ever produce a null here,
make the column `NOT NULL` with a sensible default and add a migration).

**Acceptance criteria:** a store test inserts a `payments` row with
`opening_fee_params IS NULL` and asserts `PaymentInfo` returns without panic.

### C-1: Dashboard `get_config` leaks node secrets to the browser
**Severity:** Critical · **Location:** `dashboard/service.go:659` (`handleGetConfig`)

**Scope of work:** `handleGetConfig` returns `nodes[0].NodeConfig` verbatim,
including `FlspdPrivateKey`, `Lnd.Macaroon`, `Lnd.Cert`. Introduce a
secrets-stripped DTO (business-policy fields only) and return that instead.
This becomes trivial once Phase 5 removes secrets from `NodeConfig` entirely,
but should not wait for Phase 5 — patch it now with a manual field allowlist.

**Acceptance criteria:** a test asserts the JSON returned by `get_config`
contains none of the secret field names/values.

### C-2: Dashboard `update_config` deletes other nodes on save
**Severity:** High · **Location:** `dashboard/service.go:689` (`handleUpdateConfig`)

**Scope of work:** The handler marshals `[]*config.NodeConfig{&params.Config}`
— just the one edited node — and overwrites `nodes.yaml`. On any multi-node
deployment this silently drops every other node from the persisted file.
Change to read-modify-write: load the existing node list, replace the
matching node by identity (pubkey or index), marshal the full list back.

**Acceptance criteria:** a test with a 2+ node config asserts both nodes are
present in the file after updating one.

### C-3: Config hot-swapped without synchronization under HTLC readers
**Severity:** High · **Location:** `dashboard/service.go:715` (`*nodes[0].NodeConfig = params.Config`)

**Scope of work:** The interceptor and settlement goroutines read
`NodeConfig` fields on the HTLC path while the dashboard writes the whole
struct in place with no lock — a data race on the struct that decides fee
and capacity for real payments. Store config behind an `atomic.Pointer` (or
an `RWMutex`-guarded accessor) and swap the pointer on update so readers
always see one consistent snapshot.

**Acceptance criteria:** `go test -race` covering a concurrent
read-while-write scenario passes clean.

### F-5: Three different fee-rounding rules for one concept
**Severity:** Medium · **Location:** `lsps/server.go:300` (`computeOpeningFee`, ceil) vs `lsps/handler_lsps1.go:312` (floor) vs interceptor's gross-up (T1-1)

**Scope of work:** Same logical "opening fee" is computed three different
ways depending on protocol path. Consolidate into one shared function used
by LSPS1, LSPS2, and the interceptor. (This is also step 2 of the Phase 5
config-model work — landing it here in Phase 2 as a pure bugfix, ahead of
the broader config redesign, removes risk from that later phase.)

**Acceptance criteria:** one fee function, fuzz-tested for overflow and
rounding; all three call sites use it; F-3's characterization test still
passes with the unified result.

### F-6: Nostr relay connection race leaks sockets
**Severity:** Medium · **Location:** `notifications/transport_nostr.go:151` (`RelayPool.GetOrConnect`)

**Scope of work:** Load-miss → connect → store has no per-URL lock; two
concurrent notifications to the same relay both dial and both `Store`,
leaking one live connection. Guard per-URL with `singleflight` or a keyed
mutex.

**Acceptance criteria:** `-race` test with N goroutines hitting one relay URL
concurrently shows a single connection created.

### F-7: NIP-98 dashboard auth skips payload-hash verification
**Severity:** Medium · **Location:** `auth/nostr.go:127` (commented-out block)

**Scope of work:** The body hash is never verified, so a validly signed auth
event for one request can be replayed against a different POST body inside
the ±60s window; there's also no replay/nonce cache. Enforce the `payload`
tag hash for methods with a body; add a short-lived seen-event-ID cache to
reject replays inside the validity window.

**Acceptance criteria:** table test covering valid / expired / tampered-body
/ replayed events.

### F-8: Settlement error paths fail silently, leaving orders stuck
**Severity:** Medium · **Location:** `lsps/settlement_service.go:146,274`

**Scope of work:** If `UpdateLsps1OrderFunding` or the COMPLETED transition
fails *after* a channel has already opened on-chain, the error is logged and
dropped — the order stays PAID forever despite the channel existing. Extend
the recovery sweep (already present for PAID/CREATED) to reconcile PAID
orders against actual on-chain/LND channel state, not just retry the open.

**Acceptance criteria:** test simulating a DB write failure post-open,
asserting the next recovery sweep reconciles the order to COMPLETED.

---

## Phase 3 — Unit-test coverage buildout

Ordered by blast radius, per the original audit. Target: **70–80% on the
four critical/high packages**, green everywhere, before chasing a global
number. Run with `-race`.

### T3-1: interceptor — the money path
Table-test `resolvePayment` (LSPS2 gross-up, LSPS1 activation, underpayment,
not-found, probe detection) and the fee/capacity math in `openChannel`
(reuse the pure function from T1-1/F-5). Fake `InterceptStore` +
`lightning.Client`; assert `InterceptResult` actions and failure codes.

### T3-2: lsps settlement — lift from 44.6%
Cover `recoverStuckOrders`, `recoverSettledCreatedOrders`, and the F-1
concurrency case with two racing processors against a CAS-aware fake store.

### T3-3: postgresql store tests
Introduce a container-backed store test harness (reuse itest's
`PostgresContainer`). Cover: CAS updates (F-1), `PaymentInfo` null-params
(F-2), SCID int64/uint64 round-trips, `ExpireStaleOrders`, reserved-liquidity
SQL.

### T3-4: lnd adapter mapping tests
Pure mappers (`mapScidsFromChannel`, `extractChannelPoint`, invoice-state /
failure-code conversions, `OpenChannelRequest` → `lnrpc`) are fake-free and
high-value; test directly, no mocks needed.

### T3-5: notifications
Backoff schedule, queue status transitions, the relay race (F-6), transport
selection, HTTPS non-200 handling. Fake `Store` + an `httptest` server.

### T3-6: common + config + lightning + auth
Promise sign/verify round-trip and tamper-rejection, `ValidateWebhookUrl`
(private-IP/SSRF matrix), config floor/min-max validation, SCID string
codec, NIP-98 valid/expired/tampered (F-7).

---

## Phase 4 — YAML-driven integration tests

### T4-0: Devnet backing infrastructure (done)
`test/devnet/` — a persistent, inspectable, native-Flokicoin regtest
network (`lokid` + three named `flnd` roles: lsp/client/payer, connected
via neutrino — no bitcoin-core anywhere) for interactive testing and for
backing the scenario runner below. Distinct from `itest/lntest`'s own
ephemeral subprocess-based regtest miner (which uses literal `bitcoind`
and is fine for CI, but isn't the native chain and isn't left running
between test runs). See `test/devnet/README.md`. Verified working
end-to-end: all three roles connect, sync, and can be funded via
`./collect-addresses.sh` + `./generate.sh`.

### T4-1: Generic scenario runner
Add `itest/scenarios/` + `itest/runner_test.go` that globs
`scenarios/*.yaml`, decodes into a typed `Scenario` struct (unknown keys are
a hard error), and runs each as a named subtest. Step/assertion verbs are a
registry (`map[string]StepFn` / `map[string]AssertFn`) wrapping today's
harness calls (`mempoolApi.SetFees`, `MockRelay`, `PostgresContainer`, LND
node control).

### T4-2: Seed scenarios
1. `lsps2_jit_happy_path.yaml` — buy → pay → zero-conf channel open.
2. `lsps1_order_settle_and_open.yaml` — manual order → settle → auto-open.
3. `lsps1_recovery_after_restart.yaml` — reproduces F-1: settle invoice, kill
   the LSP mid-flight, restart, assert **exactly one** channel opens (not
   two) and the order reaches COMPLETED.

### T4-3: External-party fault injection
Extend the scenario schema to script relay failure modes
(`reject_then_accept`), LND fault injection (`kill_lsp` at a given step), and
mempool fee-congestion fixtures — this is what exercises F-6/F-8-shaped bugs
under CI rather than only in production.

**Acceptance criteria for Phase 4:** the 3 seed scenarios pass in CI; adding
a new scenario requires zero Go code, only a new YAML file.

---

## Phase 5 — Config model + secrets-by-reference

Ship behind a compatibility shim that still accepts today's flat field names
for one release, so existing deployments aren't broken on upgrade.

### T5-1: Secrets-by-reference
Replace inline `flspdPrivateKey` / `macaroon` / `cert` values in
`NodeConfig`/`LndConfig` with `{from_env: VAR}` / `{from_file: /path}`
resolvers. This is what makes C-1 fixable at the root, makes `nodes.yaml`
safe to commit as a real example, and directly prevents a recurrence of
SEC-1.

### T5-2: One unit at the operator surface
Amounts in `loki` everywhere at the config/API boundary (flspd's native
base unit — 1 FLC = 100,000,000 loki, converting to mloki internally
where the code already does); fees as `%` + floor `loki`; routing as
`ppm` + `base_loki`. Retire `permyriad` and the bare `0.000001` fraction
from anything a human edits directly. **flspd is Flokicoin, not Bitcoin —
never introduce `sats`/`BTC` naming here or anywhere else in the config,
docs, or code; the unit names are `loki` and `FLC`.**

### T5-3: One shared fee model
Confirm F-5's unified fee function (landed in Phase 2) is the single
implementation referenced by the new `opening_fee: {rate_percent, min_loki}`
config block — no parallel fee math anywhere.

### T5-4: Derived channel-size bounds
Replace the six independent LSPS1/JIT min/max balance fields with one
`channel_size: {min_loki, max_loki, padding_loki}` block; derive the
protocol-specific bounds from it with validated defaults.

### T5-5: Risk profile presets
Add `profile: conservative|balanced|aggressive` that expands to the
confirmation/CLTV-delta/expiry/zero-conf bundle; any single value stays
overridable under an `advanced:` block.

### T5-6: Operator visibility tooling
Add `flspd config check` (validate, human-readable errors), `flspd config
show --effective` (print the fully-resolved config, nothing hidden), and
`flspd config explain fees` (plain-language fee example for a given channel
size).

### T5-7: Re-skin `Settings.tsx` to the tiered model
Connect / Business policy / Advanced cards per the config-UX report,
consuming the secrets-stripped config from C-1 and the new tiered schema.
Fee model as a live-computed sentence + small table; channel size as a dual
slider; product toggles; profile dropdown with read-only expansion.

**Acceptance criteria:** a fresh operator can configure a working node
touching ≤10 fields; every previously-supported field is still settable
under `advanced:`; `nodes.sample.yaml` is rewritten to the new shape and is
safe to commit verbatim (no secrets).

---

## Phase 6 — Repository structure migration

Leaf-first, one `git mv` per PR, `go build ./...` green after every step.
Do not start until Phase 5 lands — this phase should split `common/` and
relocate config onto the *already-clean* model, not migrate the old shape
twice.

### T6-1: Quick, safe wins (can happen anytime, low risk)
Delete `format/` (3 LOC), fold its one constant into wherever `platform/`
config/logging ends up. Add a `make clean` / `just clean` target that clears
`data/ dist/ tmp/ *.test` from the working tree.

### T6-2: Introduce `internal/`
Move the clearly-private adapters (`lnd`, `postgresql`→`adapters/postgres`,
`mempool`) under `internal/adapters/`. No public API depends on these being
importable from outside.

### T6-3: Split `common/` into `core/`, `ports/`, `services/`
The largest single change: separate domain types (`core/order`,
`core/channel`, `core/fees`) from interface definitions (`ports/`) from
orchestration logic currently sitting in `common`'s services
(`OpeningService`, `NodesService` → `services/`). Do this alone, in its own
PR window, with the Phase 3 tests as the regression net.

### T6-4: Consolidate transport, platform, proto
`transport/` for inbound surfaces (grpc, lsps json-rpc, dashboard http,
auth); `platform/` for config/logging/build; `api/proto/` as the one home
for `.proto` + generated code (currently interleaved with hand-written logic
in `rpc/`, `notifications/`).

### T6-5: Relocate samples, deploy, frontend
`configs/examples/` for the sample YAML/env files (out of repo root);
`deploy/` for Dockerfiles + compose; `web/` for the frontend; `test/`
housing `itest/`, the new `scenarios/` from Phase 4, and `devnet/`
(already landed at `test/devnet/` — see T4-0).

**Acceptance criteria:** tree matches the target layout from the structure
report; `go build ./...`, `go vet ./...`, and the full test suite pass after
every individual PR, not just at the end.

---

## Performance backlog (non-blocking, run in parallel once Phase 1 gate is met)

These are latency/CPU/memory notes from the original audit. None are
correctness bugs; none block the phases above, but they're easiest to land
alongside the Phase 3 tests for the same code.

- **P-1** `Intercept` polls `GetChannel` (1s) and `QueryRoutes` (500ms) on
  fixed timers instead of driving off the existing channel-active event
  subscription in `lnd/client.go` — costs tail latency and idle load.
- **P-2** `GetPeerId` / `IsConnected` do full `ListChannels`/`ListPeers`
  scans on every interception; a cached peer↔scid map refreshed off the
  channel-event stream removes them from the hot path.
- **P-3** `ListPeers` calls `GetNodeInfo` per peer (N+1 gRPC) for UI
  alias/color data — fine for a dashboard poll, wrong if ever called hot.
- **P-4** Notification fan-out spawns one goroutine per webhook per event
  with a 7s context inline on the interception path; route through the
  existing persistent queue asynchronously instead.

---

## Cross-reference to source reports

- Test/architecture audit (F-1…F-8, coverage matrix, itest design):
  https://claude.ai/code/artifact/5566942a-9b66-42d2-beda-c4bacc8f39f4
- Repo structure / config UX review (SEC-1, C-1…C-3, config + arbo redesign):
  https://claude.ai/code/artifact/cd741dd7-ba77-4126-9fbf-48e9756f6621
