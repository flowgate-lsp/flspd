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
| 0 | Contain live secret exposure | Untracked, purged, guarded | ✅ Done — confirmed test data, no rotation needed |
| 1 | Green the build + safety net | `go test ./... -race` passes in CI | ◐ T1-1..T1-3 done; T1-4, T1-5 open |
| 2 | Fix defects on current structure | All P2 items merged w/ regression test | ✅ F-1, F-2, C-1, C-2, C-3, F-7, F-9 done; F-5, F-8 (medium) remain |
| 3 | Unit-test coverage buildout | Critical-four packages ≥70–80% | ◐ In progress: F-6, F-7, F-9 done, T3-4/T3-6 done; interceptor 38.9%, lsps 45.9%, postgresql 35.8%, notifications 23.2%, lnd 4.7%, common 36.0%, config 89.3%, lightning 86.9%, auth 76.5% |
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
that was already committed/force-added. Contents had the shape of live
credentials (64-char hex private key, 586-char macaroon, 1628-char cert, an
auth token, a host IP) — **confirmed by the operator (2026-07-14) to be test
data, not a credential set backing any live/production system.** No
credential rotation is needed as a result; leaving the finding and the
history-purge work below documented since it happened regardless (real or
not, tracked secret-shaped data under `data/` is the pattern the CI guard in
item 5 now catches).

**Scope of work:**
1. ~~Confirm whether the repo has ever been pushed to a remote / cloned by
   others. If yes, treat every value above as compromised regardless of
   whether history is later scrubbed.~~ Moot — confirmed test data, not
   live credentials.
2. ~~Rotate all four: regenerate the LSP `flspdPrivateKey`, bake a fresh
   LND macaroon, reissue the auth token, confirm whether the TLS cert needs
   reissuing.~~ **Not needed** — operator confirmed no live system uses
   these values. The rotation procedure verified below is kept for
   reference (real, tested findings about how flnd macaroon rotation
   actually works, useful if this is ever needed for real).
3. `git rm --cached data/nodes.prod.yaml` and commit — stops future tracking;
   file remains on disk locally. **Done** (commit `447dd14`).
4. Scrub history with `git filter-repo` (preferred) or BFG Repo-Cleaner to
   remove the file from all past commits, then force-push. **Done.** Ran
   `git filter-repo --path data/nodes.prod.yaml --invert-paths --force`
   locally (backup bundle taken first), verified the file and the literal
   string `flspdPrivateKey` are absent from every blob across all refs,
   then the operator force-pushed to `origin main` themselves. `origin/main`
   now matches the rewritten local history exactly. Anyone else with an
   existing clone or fork of this repo needs to re-clone or `git fetch &&
   git reset --hard origin/main` — their old clones still contain the
   pre-purge history until they do. If this repo has any forks, those
   still contain the secret in their own history independently and are
   not affected by this purge.
5. Add a pre-commit or CI check that fails the build if any file under
   `data/` (or any path matching common secret patterns) is staged for commit,
   so this class of mistake can't silently recur before Phase 5 lands the
   structural fix. **Done** (commit `194dbdc`) — a `secret-scan` CI job now
   fails on any tracked file under `data/`, plus a general gitleaks pass.

**Verified LND-macaroon rotation procedure** (tested on a disposable devnet
`flnd` node, not assumed from docs — flnd is an LND fork and some
LND-family macaroon mechanics are non-obvious):
- ~~`flncli bakemacaroon --root_key_id=<new-id> ...` then `flncli
  deletemacaroonid 0`~~ does **not** work: flnd explicitly refuses
  (`deleting the default root key ID 0 is not allowed`) — root key 0 (what
  the default `admin.macaroon` uses) can never be individually revoked.
- ~~Delete only `macaroons.db` and let flnd regenerate~~ does **not** fully
  work either: flnd only writes `admin.macaroon` if that file is *missing*.
  Deleting just `macaroons.db` produces a fresh root key but leaves the old
  macaroon file's signature mismatched against it (`signature mismatch
  after caveat verification`) — not a working replacement.
- **What actually works**: stop `flnd`, delete **both**
  `data/chain/flokicoin/<network>/macaroons.db` *and* every `*.macaroon`
  file in that same directory, then restart. flnd generates a brand new
  root key and a fresh, internally-consistent `admin.macaroon` (and the
  other service macaroons) on startup. Every macaroon derived from the old
  root key — including the leaked one — stops verifying immediately.
  Update `nodes.yaml`'s `flnd.macaroon` with the new file's hex content and
  restart flspd.

**Acceptance criteria:** ✅ `git log --all -- data/nodes.prod.yaml` returns
nothing after the history scrub; a secret-scan guard exists in CI.
Credential non-functionality is N/A — operator confirmed these were never
live.

**Status:** ✅ Closed (2026-07-14).

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

**Status:** ✅ Done (commit `af77853`). Extracted `computeJitOutgoingAmount`,
fixed the stale assertion, added a table test covering the minFee-floor
branch that had no prior coverage.

### T1-2: Repair `dashboard` suite (root cause of F-4)
**Severity:** High (blocking) · **Location:** `dashboard/service_test.go:349` vs `dashboard/service.go:812`

**Scope of work:** `getLiquidityMetrics` calls
`nodes[0].Client.WalletBalance(ctx)`; the `get_summary` subtest's
`MockLightningClient` never registers `.On("WalletBalance", ...)`. Add the
missing expectation returning a realistic `*lightning.WalletBalanceResult`.

**Acceptance criteria:** `go test ./dashboard/...` passes with no panic.

**Status:** ✅ Done (commit `9e99139`). Turned out to be four layers of the
same drift, each masked by the previous panic — also needed
`ListLsps1Orders`, `GetBlockHeight`/`ListPeers`, and `GetNodeInfo`
expectations before the full suite went green.

### T1-3: Stand up CI running the full suite with the race detector
**Severity:** High (blocking)

**Scope of work:** Add (or fix) a CI job running `go build ./...`,
`go vet ./...`, and `go test ./... -race -count=1` on every PR and on `main`.
Gate merges on it. If CI already exists but doesn't run `-race` or doesn't
fail the pipeline on test panics, fix that first — a red suite that doesn't
fail CI is why F-3/F-4 went unnoticed.

**Acceptance criteria:** a PR that reintroduces either T1-1 or T1-2's bug
fails CI.

**Status:** ✅ Done (commit `4785220`, `.github/workflows/ci.yml`). There was
no test-running CI at all before this — only manual release/promote
workflows — which is the structural reason F-3/F-4 went unnoticed. itest
is excluded from this job (needs external binaries/docker); see Phase 4.

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

**Status:** ◐ Partially covered, not done. `Lsps1SettlementService`'s state
transitions are already reasonably pinned by the existing
`TestProcessSettledInvoice_*` tests in `lsps/settlement_service_test.go`.
`Interceptor.Intercept()` itself (as opposed to `resolvePayment`, which T1-1
now covers well) still has no direct test — doing so needs a full
`lightning.Client` + `notifications.Notifier` + `history.Store` mock set,
which is real scaffolding work, not a quick addition. Deferred rather than
rushed; revisit before starting Phase 5/6, since those are the phases this
item actually gates.

### T1-5: Full CI/CD pipeline review & modernization
**Severity:** Medium · **Depends on:** T1-3 (done — this extends it)

**Finding:** Beyond "no CI ran tests at all" (T1-3), the two workflows that
do exist have their own gaps worth a proper pass now that a real test
pipeline exists to build on:

- `release.yml` pins `go-version: '1.22'`, while `go.mod` requires `1.26.1`
  — a real version drift, not just cosmetic (differs from `ci.yml`, which
  correctly reads `go-version-file: go.mod`).
- Docker image builds are only ever exercised inside `release.yml`
  (`workflow_dispatch`, manual) — a broken `Dockerfile` or `Dockerfile.dev`
  is invisible until someone manually cuts a release, not on the PR that
  broke it.
- No lint step in CI at all, despite `justfile` already having a `lint`
  recipe (`golangci-lint run ./...`) that nothing calls.
- No frontend CI — `justfile`'s `test-frontend`/`build-frontend` recipes
  exist but nothing runs them on PRs touching `frontend/`.
- Release is entirely manual (`workflow_dispatch`) and gated only on a
  `releasenotes/${VERSION}.md` file existing — there's a changelog
  convention already in place (`releasenotes/*.md` + `VERSION`), but no
  automation ties a merged changelog entry to an actual release.

**Scope of work:**
1. **Fix the version drift.** Point `release.yml` at `go-version-file:
   go.mod` too, so CI and release always build with the same toolchain
   `go.mod` declares — this class of drift is exactly what caused F-3/F-4
   and the itest/lntest `bitcoin.*` vs `flokicoin.*` flag confusion found
   during the earlier review.
2. **Split pipelines by purpose, not by copy-paste** — a fast PR/diff
   pipeline (build, vet, unit tests, lint — target under a few minutes,
   blocks merge) versus a heavier pipeline (Docker image build validation,
   itest/devnet-backed integration tests from Phase 4) that can run on
   `main` and on release tags without slowing down every PR.
3. **Validate Docker builds on every PR** that touches `Dockerfile`,
   `Dockerfile.dev`, `go.mod`, or `go.sum` — at minimum `docker build` for
   both, ideally using the `test/devnet` sibling-checkout build pattern as
   a template for caching. Catches breakage before release time, not
   during it.
4. **Wire up lint and frontend CI** using the existing `justfile` recipes
   rather than inventing new ones — `just lint`, `just test-frontend`,
   `just build-frontend`.
5. **Changelog-driven auto-release.** Formalize what `release.yml`
   currently checks manually: on merge to `main`, detect whether `VERSION`
   changed and `releasenotes/${VERSION}.md` exists, and if so trigger the
   existing release pipeline automatically instead of requiring a manual
   `workflow_dispatch`. Keep the changelog file as the source of truth
   (human-written release notes gating the release), just remove the
   manual trigger step once that file lands.
6. **Docker build best practices pass** while touching these files:
   multi-stage build caching (buildx cache-from/cache-to, already used
   ad-hoc in `test/devnet`'s images), pinned base image digests rather
   than floating tags, and a basic image vulnerability scan (e.g. `trivy`)
   as a non-blocking CI report to start.

**Acceptance criteria:** PR pipeline (build/vet/test/lint) completes in a
few minutes and blocks merge on failure; a broken Dockerfile fails on the
PR that broke it, not at release time; a merged changelog entry with a
bumped `VERSION` triggers a release without a human clicking
`workflow_dispatch`; `go-version` is declared in exactly one place
(`go.mod`) across all workflows.

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

**Status:** ✅ Done (commit `e13656c`). `UpdateLsps1OrderState` is now a CAS
(added `expectedCurrentState`, returns `updated bool`); all four call sites
check the result. Regression test races the two real code paths against a
mutex-protected fake store 10x under `-race`.

### F-2: Nil-pointer dereference on payments with null fee params
**Severity:** High · **Location:** `postgresql/intercept_store.go:53-61`

**Scope of work:** `PaymentInfo` only assigns `extParams` when the
`opening_fee_params` column is non-null, then unconditionally dereferences
`extParams.Token` / `.OrderId` / `&extParams.Params`. Guard the nil case and
return zero values (or, if no code path should ever produce a null here,
make the column `NOT NULL` with a sensible default and add a migration).

**Acceptance criteria:** a store test inserts a `payments` row with
`opening_fee_params IS NULL` and asserts `PaymentInfo` returns without panic.

**Status:** ✅ Done (commit `b4ff434`). Also stood up `postgresql/`'s first
real test harness (`TestMain` spins up a disposable postgres container) —
seeds Phase 3's postgresql coverage work. Confirmed the regression test
reproduces the exact original panic when the fix is reverted.

### C-1: Dashboard `get_config` leaks node secrets to the browser
**Severity:** Critical · **Location:** `dashboard/service.go:659` (`handleGetConfig`)

**Scope of work:** `handleGetConfig` returns `nodes[0].NodeConfig` verbatim,
including `FlspdPrivateKey`, `Lnd.Macaroon`, `Lnd.Cert`. Introduce a
secrets-stripped DTO (business-policy fields only) and return that instead.
This becomes trivial once Phase 5 removes secrets from `NodeConfig` entirely,
but should not wait for Phase 5 — patch it now with a manual field allowlist.

**Acceptance criteria:** a test asserts the JSON returned by `get_config`
contains none of the secret field names/values.

**Status:** ✅ Done (commit `1058b7c`). `stripConfigSecrets` strips
`flspdPrivateKey`/`tokens`/`flnd` via a JSON marshal-delete-remarshal round
trip rather than a hand-maintained DTO struct. `TestHandleGetConfig_StripsSecrets`.

### C-2: Dashboard `update_config` deletes other nodes on save
**Severity:** High · **Location:** `dashboard/service.go:689` (`handleUpdateConfig`)

**Scope of work:** The handler marshals `[]*config.NodeConfig{&params.Config}`
— just the one edited node — and overwrites `nodes.yaml`. On any multi-node
deployment this silently drops every other node from the persisted file.
Change to read-modify-write: load the existing node list, replace the
matching node by identity (pubkey or index), marshal the full list back.

**Acceptance criteria:** a test with a 2+ node config asserts both nodes are
present in the file after updating one.

**Status:** ✅ Done (commit `1058b7c`). Builds the full config list from
live in-memory nodes, replaces only the edited one. `TestHandleUpdateConfig_PreservesOtherNodes`.

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

**Status:** ✅ Done (commit `1058b7c`). Introduced `config.NodeConfigRef`
(an `atomic.Pointer[NodeConfig]` wrapper) — the structural fix behind the
bulk of that commit's diff, since every long-lived holder of `NodeConfig`
(interceptor, settlement service, lnd, cmd/flspd wiring, and their tests)
now goes through it, `Load()`-ing once per logical operation instead of
caching a raw pointer. `TestNodeConfig_ConcurrentReadWriteNoRace` passes
clean under `-race`.

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

**Status:** ✅ Done (commit `d6cc9e5`). Wrapped in `singleflight.Group` keyed
by URL. Confirmed against the original code first: 20 concurrent calls for
one URL produced 20 real connections, every run (5/5) — reliably
reproducible, not a rare flake.

### F-7: NIP-98 dashboard auth skips payload-hash verification
**Severity:** Medium · **Location:** `auth/nostr.go:127` (commented-out block)

**Scope of work:** The body hash is never verified, so a validly signed auth
event for one request can be replayed against a different POST body inside
the ±60s window; there's also no replay/nonce cache. Enforce the `payload`
tag hash for methods with a body; add a short-lived seen-event-ID cache to
reject replays inside the validity window.

**Acceptance criteria:** table test covering valid / expired / tampered-body
/ replayed events.

**Status:** ✅ Done (commit `7d0ce90`). Payload-hash now enforced for any
method carrying a body (matched against `ComputePayloadHash`); added a
short-lived (180s) in-memory replay cache keyed on `event.GetID()`
(recomputed from content, not the unverified client-supplied `ID` field —
go-nostr's `CheckSignature` doesn't check that field, so trusting it would
have made the replay cache itself spoofable). Table tests cover valid /
expired / future-timestamp / tampered-signature / wrong-kind / URL-mismatch
/ method-mismatch / missing-payload-hash / wrong-payload-hash / replayed /
query-param-fallback. Each of the two new checks confirmed to fail its
targeted tests when temporarily no-op'd, and pass with the fix restored.

### F-8: Settlement error paths fail silently, leaving orders stuck
**Severity:** Medium · **Location:** `lsps/settlement_service.go:146,274`

**Scope of work:** If `UpdateLsps1OrderFunding` or the COMPLETED transition
fails *after* a channel has already opened on-chain, the error is logged and
dropped — the order stays PAID forever despite the channel existing. Extend
the recovery sweep (already present for PAID/CREATED) to reconcile PAID
orders against actual on-chain/LND channel state, not just retry the open.

**Acceptance criteria:** test simulating a DB write failure post-open,
asserting the next recovery sweep reconciles the order to COMPLETED.

### F-9: Outpoint byte-order bug — channel-sync path stored reversed txids
**Severity:** High · **Location:** `lightning/outpoint.go` (`NewOutPointFromString`)

**Scope of work:** `NewOutPointFromString` hex-decoded LND's display-order
`ChannelPoint`/`FundingTxidStr` string straight into `chainhash.Hash.SetBytes`
without reversing it — `chainhash.Hash.String()` byte-reverses for display,
but `SetBytes` does not reverse on the way in, so the correct parse path is
`chainhash.Decode`/`NewHashFromStr`, not raw `hex.DecodeString`. Practical
effect: every outpoint reaching the DB via the channel-sync path
(`lnd.ListChannels` → `history.ChannelUpdate` → `postgresql` `channels`
table) was byte-reversed relative to outpoints built from raw
`FundingTxidBytes` on the htlc/forwarding path (`lnd/client.go:320` →
`open_channel_htlcs` table). Any `bytes.Compare`-based correlation between
the two — notably `cmd/flspd_revenue_cli/revenue.go`'s forward↔htlc/channel
matching — would silently fail to match rows for the *same* physical
channel. `dashboard/service.go`'s `activeChannels` map keyed by
`.String()` would also show the wrong (double-reversed) channel point to
operators.

**Status:** ✅ Fixed (commit `3f80c8b`). `NewOutPointFromString` now routes
through `chainhash.Decode`. Added `TestOutPoint_StringAndBytesConstructorsAgree`
proving the string- and bytes-based constructors now produce identical
`wire.OutPoint` values for the same underlying txid.

**Follow-up needed (flagging, not doing autonomously):** any *already
persisted* `channels` table rows written before this fix have the old
byte-reversed hash baked in and would need a backfill (re-derive from a
fresh `ListChannels` sync, or reverse the stored bytes in place) to
reconcile with `open_channel_htlcs`/revenue data written via the correct
path. Per earlier confirmation that current DB contents are test data only,
no live backfill is believed necessary right now — but confirm before
relying on any existing `channels` table data for revenue reporting.

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

**Status:** ◐ In progress (commits `258c5e2`, `78cd852`). `resolvePayment`
well covered by T1-1's table tests. Added: a full `lightning.Client` +
`notifications.Notifier` testify mock, real `openChannel` coverage
(network/business floor rejections, JIT vs non-JIT routing-policy and
zero-conf selection, fee-estimator-failure fallback, client-error
propagation — replacing the old `TestOpenChannel_CapacityLogic` test that
only re-derived the formula inline rather than calling the real method),
`notifyAndWait`'s full offline/reconnect state machine, both
`discord.go` webhook senders, and `NewInterceptHandler`'s pubkey decode.
`Interceptor.Intercept()` itself (the streaming HTLC entrypoint) is still
untested — same scaffolding gap as T1-4, appropriately Phase 4's territory
since it needs devnet-realistic HTLC flows, not deeper mocking.
`interceptor` package coverage: 12.2% → 38.9%.

### T3-2: lsps settlement — lift from 44.6%
Cover `recoverStuckOrders`, `recoverSettledCreatedOrders`, and the F-1
concurrency case with two racing processors against a CAS-aware fake store.

**Status:** ✅ F-1 concurrency case done (commit `e13656c`,
`TestF1_ConcurrentSettlementAndRecoveryOpenExactlyOneChannel`).
`recoverStuckOrders`/`recoverSettledCreatedOrders` still need direct
coverage. `lsps` package coverage: 45.9%.

### T3-3: postgresql store tests
Introduce a container-backed store test harness (reuse itest's
`PostgresContainer`). Cover: CAS updates (F-1), `PaymentInfo` null-params
(F-2), SCID int64/uint64 round-trips, `ExpireStaleOrders`, reserved-liquidity
SQL.

**Status:** ◐ In progress (commits `b4ff434`, `589235a`, `6dbcbf2`). Own
harness landed (`TestMain` + disposable postgres container, not itest's —
keeps postgresql/ independently testable without an itest dependency).
Done: `PaymentInfo` null-params (F-2), CAS atomicity at the SQL level (F-1,
20 concurrent goroutines on one real row), `GetReservedLiquidity`,
`ExpireStaleOrders`, `PostgresOpeningStore`'s LSPS1 order registry
(register/get/list/count incl. client+state+expiry-cutoff filters), LSPS5
webhook upsert, LSPS2 buy registration round-trip, fee-params settings
(incl. the two global default rows the initial migration seeds), and
`NotificationsStore` in full (register/upsert/unsubscribe/expire,
enqueue+JSONB round-trip, pending/backoff transitions). Still open: SCID
int64/uint64 round-trips, `revenue_cli_store.go`, `history_store.go`
(CopyFrom-based batch inserts — more involved, `channels`/`open_channel_htlcs`
tables are also where F-9's fix should be re-verified end-to-end once
Phase 4's devnet-backed itests exist). `postgresql` coverage: 11.7% → 35.8%.

### T3-4: lnd adapter mapping tests
Pure mappers (`mapScidsFromChannel`, `extractChannelPoint`, invoice-state /
failure-code conversions, `OpenChannelRequest` → `lnrpc`) are fake-free and
high-value; test directly, no mocks needed.

**Status:** ✅ Done. `lnd/mappers_test.go`: `mapScidsFromChannel` (zero-conf vs
standard, alias handling), `extractChannelPoint` (string vs bytes vs invalid
txid), `convertInvoiceState` (all 4 states). The gRPC-calling methods
(`ListChannels`, `OpenChannel`, etc.) remain untested — they need a fake LND
gRPC server or the devnet (Phase 4), not pure unit tests. `lnd` coverage:
0% → 4.7%.

### T3-5: notifications
Backoff schedule, queue status transitions, the relay race (F-6), transport
selection, HTTPS non-200 handling. Fake `Store` + an `httptest` server.

**Status:** ◐ In progress (commits `d6cc9e5`, `a28c8e6`). Done: F-6 fixed +
proven, `HTTPSTransport.Send` (signing verified cryptographically, non-200
handling, invalid-key rejection), `Notify` dispatch + dedup window,
`processPending` success/backoff transitions. Still open: `expiry_watcher.go`,
`cleanup.go`, `server.go`'s gRPC handlers, Nostr transport's event-creation
path. `notifications` coverage: 0% → 23.2%.

### T3-6: common + config + lightning + auth
Promise sign/verify round-trip and tamper-rejection, `ValidateWebhookUrl`
(private-IP/SSRF matrix), config floor/min-max validation, SCID string
codec, NIP-98 valid/expired/tampered (F-7).

**Status:** ✅ Done (commits `aa586fa`, `3f80c8b`, `86175cb`, `7d0ce90`).
`common` (promise sign/verify round-trip + tamper/wrong-signer/malformed
rejection, `paramsHash` determinism, `ValidateWebhookUrl`, `isPrivateIP`
full SSRF matrix, `ParsePrivateKey`) — coverage 0% → 36.0%. `lightning`
(`ShortChannelID`/`OutPoint` round-trip + invalid-format tables,
`SignLsps5Message`/`VerifyMessage` round trip, found and fixed F-9 along
the way) — coverage 0% → 86.9%. `config` (floor/min-max validation on
`Validate`/`ValidateLsps1`/`ValidateJit`, network-floor vs business-floor
relationships) — coverage 0% → 89.3%. `auth` (NIP-98
valid/expired/tampered/replayed table tests, fixed F-7 alongside) —
coverage 0% → 76.5%.

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
