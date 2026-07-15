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
| 1 | Green the build + safety net | `go test ./... -race` passes in CI | ✅ T1-1..T1-5 done |
| 2 | Fix defects on current structure | All P2 items merged w/ regression test | ✅ All items done: F-1, F-2, C-1, C-2, C-3, F-5, F-7, F-8, F-9, F-10, F-11, F-12, F-13 |
| 3 | Unit-test coverage buildout | Critical-four packages ≥70–80% | ◐ In progress: F-5, F-6, F-7, F-8, F-9, F-12, F-13 done, T1-4/T3-2/T3-3/T3-4/T3-5/T3-6 done; interceptor 58.3%, lsps 50.1%, postgresql 69.1%, notifications 54.3%, lnd 4.7%, common 36.0%, config 93.4%, lightning 86.9%, auth 76.5% |
| 4 | YAML-driven integration tests | Runner + 3 seed scenarios merged | ◐ Foundation built + proven (Devnet/FlspdProcess/LSPS2 client, 2 more real bugs found — F-10 additions); gossip-propagation blocker root-caused + fixed (see T4-1); YAML runner built and working (T4-1/T4-2) with scenario 1 wired through it — known-failing on a distinct zero-conf-balance-sync race, documented, not part of the CI gate; scenarios 2/3 need a new LSPS1 test client, not started; T4-3 fault injection not started |
| 5 | Config model + secrets-by-reference | New model shipped behind compat shim | ✅ T5-1..T5-7 done (secrets-by-reference, unit/fee/channel-size model, profiles, CLI tooling, Settings.tsx re-skin — frontend not browser-verified, no automation tooling available) |
| 6 | Repo structure / arbo migration | Tree matches target layout, build green throughout | ◐ T6-1..T6-3 done (format/ folded into common/, internal/adapters/ introduced, common/ split into core/ports/services/); T6-4 partial (rpc/→api/proto/rpc/, config/→platform/config/, build/→platform/build/, auth/→transport/auth/ done; notifications/ proto split, lsps/ and dashboard/ transport-vs-logic splits, common/ logging split deliberately deferred — genuine symbol-level ambiguity, not mechanical risk); T6-5 partial (configs/examples/, test/itest/ done; deploy/, web/ deliberately deferred — uncommitted dev-tooling state + sibling-checkout paths) |
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

**Status:** ✅ Done (commit `f97a63d`). `Lsps1SettlementService`'s state
transitions were already reasonably pinned by the existing
`TestProcessSettledInvoice_*` tests. Added 8 characterization tests for
`Interceptor.Intercept()`'s own entry-point control flow (unregistered
forward, hard resolvePayment failure, GetPeerId failure, routing to a
different next hop, IsConnected failure, all three probe-detection
branches) — the scaffolding turned out lighter than expected, reusing
`interceptor/`'s already-built `MockStore`/`MockLightningClient`/
`MockNotifier` rather than needing new fakes from scratch. Deliberately
scoped to the entry point's control flow, not the JIT-channel-open +
confirmation-polling path (needs real/mocked time delays, already covered
separately by `openChannel`'s own tests) — matches this item's own
"tripwire, not exhaustive" framing. Found and fixed a real gap in the
existing `MockStore.PaymentInfo` along the way: it silently hardcoded
several return values to nil regardless of what a test configured, making
"registered payment" scenarios impossible to construct through it.
`interceptor` coverage: 38.9% → 58.3%.

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

**Status: done.** All six scope items shipped:

1. `release.yml`'s `build-binaries` job now uses `go-version-file: go.mod`
   (was hardcoded `'1.22'`), matching `ci.yml`.
2. `ci.yml` gained `lint`, `frontend`, and `docker-validate` jobs alongside
   the existing `test`/`secret-scan` — `frontend`/`docker-validate` are
   gated on a new `changes` job (`dorny/paths-filter`) so they only run
   when `frontend/**` or `Dockerfile`/`go.mod`/`go.sum` actually changed.
   `docker-validate` build-only's the production `Dockerfile` (context `.`,
   no push); `Dockerfile.dev` isn't validated in CI since it needs sibling
   `flokiorg/*` checkouts that don't exist on a plain runner.
3. Added `.golangci.yml` (`golangci-lint` v2 config format). Fixed the
   issues cheap enough to just fix outright: 3 genuinely dead declarations
   in `test/itest/notification_service.go`, 4 ineffectual assignments
   (including one real bug — `postgres/notifications_store.go`'s
   `Unsubscribe` silently discarded the delete's error and always returned
   `nil`), and excluded `frontend/` (a vendored `.go` file under
   `frontend/node_modules` was tripping `govet`). The remaining 68 issues
   (50 `errcheck` — mostly conventionally-ignored `defer x.Close()` —  and
   18 `staticcheck`, mostly deprecation notices) are pre-existing debt too
   large to burn down as part of a CI-wiring pass; the `ci.yml` `lint` job
   uses `golangci-lint-action`'s `only-new-issues: true` so it grandfathers
   that debt and gates only new code. `just lint` still runs the full,
   ungated scan for whoever wants to pay it down.
4. Frontend CI wired to `frontend/`'s own `npm run lint|test|build`
   (equivalent to the `justfile` recipes). Along the way, `npm run test`
   turned up two real pre-existing failures in
   `frontend/src/hooks/useEvents.test.ts` — the test predated an
   `EventsContext`/`EventsProvider` refactor and no longer wrapped the hook
   in a provider, and separately the `vi.mock('./useAuth', ...)` factory
   returned a fresh `getAuthHeader` `vi.fn()` on every call, which (since
   `EventsContext`'s `connect` callback depends on it) retriggered its
   connect/cleanup effect every render and flapped the socket
   connect/disconnect. Fixed both; all 7 frontend tests pass. `npm run
   lint` has ~65 pre-existing eslint errors (`no-explicit-any`,
   `react-hooks/set-state-in-effect`, `react-refresh/only-export-components`
   in `OrderDetail.tsx`/`Settings.tsx`/`router.tsx`) — real issues, but a
   frontend cleanup outside this task's scope, so that step runs
   non-blocking (`continue-on-error: true`) in CI for now.
5. `release.yml` now triggers on `push: branches: [main], paths: [VERSION]`
   (in addition to `workflow_dispatch`) — a merge that bumps `VERSION`
   auto-fires the release; the existing "release notes must exist" check
   still hard-fails the run if `releasenotes/${VERSION}.md` wasn't added in
   the same change.
6. `build-docker` in `release.yml` gained `cache-from`/`cache-to:
   type=gha` buildx caching and a non-blocking (`continue-on-error: true`,
   `exit-code: '0'`) `trivy` scan of the pushed image. `Dockerfile` now
   pins `golang:1.26.1-alpine` (was the floating `golang:alpine` tag —
   switched to the exact toolchain `go.mod` requires) and `alpine:latest`
   to their current digests; added `.github/dependabot.yml` (`gomod`,
   `npm` for `frontend/`, `docker`, `github-actions` ecosystems, weekly) so
   those pins and action versions don't quietly rot. Also bumped
   `softprops/action-gh-release` from `v1` to `v3` — `actionlint` flagged
   `v1` as running on a runtime GitHub Actions no longer supports.

Verified: `go build/vet/test -race` all green; `docker build` of the
pinned `Dockerfile` succeeds locally; `actionlint` clean on all workflow
files; frontend `test`/`build` pass, `lint` runs (non-blocking).

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

**Status:** ✅ Done (commit `60bdf29`, during T5-3). Was never actually
finished despite this entry existing since Phase 2 — `handler_lsps1.go`
still had its own hand-rolled `floor(max(proportional, min))` calculation,
diverging from `computeOpeningFee`'s ceiling-rounded result for the same
inputs. Fixed by routing LSPS1 through the same function (now exported as
`ComputeOpeningFee`, so T5-6's `flspd config explain fees` can reuse it
too). Confirmed via revert-and-verify with a fixture landing on a
capacity/proportional product just past a 1000-mloki boundary (old: 2
loki, new: 3 loki). Added the fuzz test this entry's own acceptance
criteria called for (`FuzzComputeOpeningFee`, 600k+ generated cases
clean). The interceptor's `computeJitOutgoingAmount` (T1-1) is
deliberately NOT merged into the same function — it solves the inverse
problem (given a gross payment, what outgoing amount + fee results,
needed for the JIT gross-up) rather than "fee given a target payment
size," so forcing them into one function would mean restructuring
correct, working, HTLC-money-path-critical logic without a real
correctness gain. `lsps` coverage: 49.0% → 49.4%.

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

**Status:** ✅ Done (commit `f9f06d1`). Found a second bug while fixing
the first: the *existing* recovery sweep for stuck PAID orders called
`openChannelForOrder` unconditionally for every one — meaning any order
stuck in PAID because the COMPLETED write failed (channel already open)
would get a duplicate `OpenChannel` attempt on the next sweep. Fixed by
checking for recorded funding (`FundingTxid`/`FundingOutpoint`, only set
after `OpenChannel` + `UpdateLsps1OrderFunding` both already succeeded)
before retrying: if present, verify the channel is still active via
`GetChannel` and reconcile straight to COMPLETED; if the recorded channel
can't be found active, leave the order in PAID for manual review rather
than guessing (retrying `OpenChannel` here would reintroduce the same
double-open risk). Orders with no recorded funding fall through to the
existing retry path unchanged. Confirmed via revert-and-verify — the new
test panics against the original code (proves `openChannelForOrder` ran
again instead of reconciling) and passes with the fix.

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

### F-10: flspd panics on ordinary graceful shutdown (double-Stop race)
**Severity:** High · **Location:** `cmd/flspd/run.go`'s `stopInterceptors`
closure, `lsps/order_cleanup_service.go`'s `Stop()`

**Scope of work:** `stopInterceptors()` is invoked concurrently from three
independent paths: each interceptor's own goroutine when it stops, the
gRPC server's goroutine when it stops, and the SIGTERM/SIGINT handler.
Under completely ordinary shutdown, the SIGTERM handler stopping the gRPC
server is itself what makes the server's own goroutine observe a stop and
call `stopInterceptors()` a second time. `OrderCleanupService.Stop()` does
an unprotected `close(s.stopCh)`, so the second call panics
(`close of closed channel`) instead of the process exiting cleanly —
every graceful stop crash-loops.

**Status:** ✅ Fixed (commit `b0a5c82`). Gated `stopInterceptors`'s body
behind `sync.Once`. Found by literally running the built binary against
the T4-1 devnet as part of building `itest/FlspdProcess` — the crash
showed up in the process log on every single stop before the fix, and is
gone after it (verified via `TestFlspdProcessSmoke`, which stops/restarts
a real subprocess). This is exactly the kind of defect only a real
integration test (not a mock) catches — flag as a case for prioritizing
T4-2/T4-3 fault-injection scenarios, since they'll exercise this exact
kill/restart path directly.

### F-11: interceptor sends HTLC-fail codes flnd can't actually deliver, breaking the whole interceptor stream
**Severity:** High · **Location:** `lnd/interceptor.go`'s `mapFailureCode`

**Scope of work:** flnd's `interceptedForward.FailWithCode`
(`htlcswitch/interceptable_switch.go`) only implements a FAIL response for
`lnwire.CodeTemporaryChannelFailure`, the three `CodeInvalidOnion*`
variants, and `CodeExpiryTooSoon` — everything else hits its `default:
return ErrUnsupportedFailureCode`. `mapFailureCode` was mapping
`common.FAILURE_TEMPORARY_NODE_FAILURE` (used for, among other paths, LSPS2
JIT underpayment) to `lnrpc.Failure_TEMPORARY_NODE_FAILURE`, and
`common.FAILURE_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS` to its matching
`lnrpc.Failure_*` — both unsupported by flnd's interceptor path. Sending
one doesn't just fail that HTLC: it breaks the *entire* HTLC interceptor
gRPC stream, which reconnects and redelivers the same still-pending HTLC,
hitting the same unsupported-code error again — an infinite reconnect
loop that starves every other HTLC being intercepted at the time, until
the sender's own payment-level timeout eventually gives up (observed:
~35s of "unexpected error in interceptor.Recv()" / reconnect / redeliver
before the sender's 60s timeout won the race).

**Status:** ✅ Fixed (commit pending in this session's later batch).
`mapFailureCode` now always returns `TEMPORARY_CHANNEL_FAILURE` regardless
of the input code — the one value this interceptor path can actually
deliver. Found via `itest/lsps2_jit_test.go`'s real end-to-end JIT flow
against the T4-1 devnet: an underpayment scenario (itself caused by
`routerrpc.SendPaymentV2`'s default MPP splitting, unrelated) triggered
`FAILURE_TEMPORARY_NODE_FAILURE`, and the resulting reconnect-loop was
directly visible in flspd's logs before the fix, gone after.

**Follow-up needed:** the other four `InterceptFailureCode` values
(`FAILURE_AMOUNT_BELOW_MINIMUM`, `FAILURE_INCORRECT_CLTV_EXPIRY`,
`FAILURE_UNKNOWN_NEXT_PEER`, `FAILURE_INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS`)
are now unreachable dead distinctions at the wire level (all collapse to
the same delivered code) — worth a follow-up look at whether flnd exposes
a *different* RPC path for richer failure reasons (e.g. resolving via a
constructed onion failure blob directly rather than the symbolic
`Failure_FailureCode` enum), since collapsing everything to
"temporary channel failure" loses real diagnostic information for clients.

---

### F-12: ExpiryWatcher never escalates past the 144-block milestone
**Severity:** Medium · **Location:** `notifications/expiry_watcher.go`'s
`checkExpiries`

**Scope of work:** `expiryMilestones` is `[144, 72, 24]` (descending,
warning → urgent → critical), and the per-HTLC loop is supposed to notify
every milestone crossed since the last check. It had an unconditional
`break` at the end of the loop body — after handling the *first* threshold
considered, regardless of whether it was newly notified or already seen,
the loop always exited. Since `blocksToExpiry` only decreases over time
and thresholds are checked in descending order, threshold 144 is always
evaluated first; once it's been notified once, every later check sees
`already=true` for 144 and breaks immediately — so 72 ("urgent") and 24
("critical") could never fire for any HTLC that had already crossed 144,
silently losing the escalation behavior the milestone list was designed
for.

**Status:** ✅ Fixed (commit `6c9c773`). Removed the trailing unconditional
`break`, keeping only the early-exit `break` (`blocksToExpiry > threshold`),
so a single check walks through and notifies every crossed-but-not-yet-
notified milestone instead of stopping after the first one considered.
Found while writing unit-test coverage for the `notifications` package
(Phase 3), not via itest — no live system was ever exposed to this, but
any expiry-critical webhook/notification consumer relying on the 72/24
escalation would have silently never received it. Added
`notifications/expiry_watcher_test.go` (7 cases); confirmed via
revert-and-verify that 4 of the 7 fail against the original
unconditional-`break` code.

---

### F-13: flspd_revenue_cli's revenue report generation was 100% broken
**Severity:** High · **Location:** `postgresql/revenue_cli_store.go` — all
three query methods (`GetForwards`, `GetOpenChannelHtlcs`,
`ExportTokenForwardsForExternalNode`)

**Scope of work:** Every query method `cmd/flspd_revenue_cli` calls had a
raw SQL error that fires unconditionally, not an edge case — this CLI tool
could never have produced a report against the real schema, ever:
- `GetForwards` selected `h.peerid_in`/`h.peerid_out` and `GetOpenChannelHtlcs`
  selected `h.peerid`, both from `forwarding_history` — but that table has
  no peerid columns at all (only `chanid_in`/`chanid_out`; peerid only
  exists on `channels`, reachable through the join already present in both
  queries).
- `ExportTokenForwardsForExternalNode`'s `SELECT * FROM (... UNION ALL ...)`
  subquery had no alias, which Postgres rejects outright with "subquery in
  FROM must have an alias".

`flspd_revenue_cli` is a separate CLI binary from the daemon, has no test
coverage, and isn't exercised by CI, so this SQL — whether it was ever
schema-matched to begin with, or the schema changed out from under it —
plausibly went unexecuted since it was written.

**Status:** ✅ Fixed (commit `7796cc2`). `GetForwards` now selects
`c_in.peerid`/`c_out.peerid`; `GetOpenChannelHtlcs`'s inner subquery selects
`c.peerid`; the UNION ALL subquery got an alias. Found and confirmed via
revert-and-verify while writing this file's first-ever test coverage
against a real Postgres container (`postgresql/revenue_cli_store_test.go`,
5 cases) — 4 of 5 fail with these exact SQL errors against the original
code. Also fixed a test-isolation issue this surfaced: three
`history_store_test.go` forward fixtures used orphaned SCIDs with "now"
timestamps, which spuriously broke `sanityCheck`'s nodeid-unscoped query
for any other "now"-windowed test in the package — moved to fixed past
dates. `postgresql` coverage: 54.6% → 69.1%. This closes out T3-3 in
full — Phase 3's postgresql scope is now done.

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

**Status:** ✅ Done (commits `e13656c`, `a0f6321`). F-1 concurrency case
(`TestF1_ConcurrentSettlementAndRecoveryOpenExactlyOneChannel`).
`recoverStuckOrders`/`recoverSettledCreatedOrders` now directly covered:
SETTLED/CANCELED/OPEN invoice-state branches, empty-PaymentInvoice skip,
per-order LookupInvoiceState error isolation, and the early-return when
`GetAllPaidLsps1Orders` itself fails. `lsps` package coverage: 45.9% →
49.0%.

### T3-3: postgresql store tests
Introduce a container-backed store test harness (reuse itest's
`PostgresContainer`). Cover: CAS updates (F-1), `PaymentInfo` null-params
(F-2), SCID int64/uint64 round-trips, `ExpireStaleOrders`, reserved-liquidity
SQL.

**Status:** ✅ Done (commits `b4ff434`, `589235a`, `6dbcbf2`, `4100095`,
`ded3998`, `7796cc2`). Own harness landed (`TestMain` + disposable
postgres container, not itest's — keeps postgresql/ independently
testable without an itest dependency). Covered: `PaymentInfo` null-params
(F-2), CAS atomicity at the SQL level (F-1, 20 concurrent goroutines on
one real row), `GetReservedLiquidity`, `ExpireStaleOrders`,
`PostgresOpeningStore`'s LSPS1 order registry (register/get/list/count
incl. client+state+expiry-cutoff filters), LSPS5 webhook upsert, LSPS2
buy registration round-trip, fee-params settings (incl. the two global
default rows the initial migration seeds), `NotificationsStore` in full
(register/upsert/unsubscribe/expire, enqueue+JSONB round-trip,
pending/backoff transitions), the SCID int64/uint64 sign-flip round-trip
(a bit-63-set SCID through both the LSPS2 buy-registration write path and
`InterceptStore.GetLsps2Order`'s previously-uncovered read path — the
existing round-trip test's own comment claimed to cover this but used a
fixture value that never actually set the sign bit), `history_store.go`'s
CopyFrom-based write paths in full (`UpdateChannels` insert-then-upsert
incl. nil-scid and first_seen-preserved-on-upsert, `InsertForwards`
DO-NOTHING vs `UpdateForwards` DO-UPDATE on conflict, `FetchLndForwardOffset`,
`AddOpenChannelHtlc`) — this is also where F-9's outpoint-byte-order fix
lives, now with direct coverage instead of only pure-function tests —
and `revenue_cli_store.go`, which turned out to be the highest-value find
in this whole subtask: all three of its query methods were completely
broken (F-13), never caught because `flspd_revenue_cli` had zero test
coverage and isn't exercised by CI. `postgresql` coverage: 11.7% → 69.1%.

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

**Status:** ◐ In progress (commits `d6cc9e5`, `a28c8e6`, `f0f638d`,
`6c9c773`, `ef27b55`, `bbea486`). Done: F-6 fixed + proven,
`HTTPSTransport.Send` (signing verified cryptographically, non-200
handling, invalid-key rejection), `Notify` dispatch + dedup window,
`processPending` success/backoff transitions, and all 5 `Notify*` LSPS5
dispatch wrappers (`NotifyWebhookRegistered`, `NotifyExpirySoon`,
`NotifyLiquidityManagementRequest`, `NotifyOrderStateChanged`,
`NotifyOnionMessageIncoming` — fan-out dispatch, store-error propagation,
and `NotifyOrderStateChanged`'s params shape). `expiry_watcher.go`: 7
cases covering first-milestone, all-milestones-in-one-check, multi-check
escalation, no-re-notify, already-expired skip, outgoing-HTLC skip, and
settled-HTLC milestone eviction — found and fixed F-12 in the process.
`cleanup.go`: 5 cases covering the immediate on-start run, the
now-ExpiryDuration cutoff passed to `RemoveExpired`, periodic re-runs,
prompt shutdown on context cancellation, and that a store error on one
tick doesn't kill the loop. `transport_nostr.go`'s `Send`: validation
error paths (missing privkey, invalid URI, no relays, event-creation
failure) plus two real end-to-end cases against an in-process fake relay
that speaks just enough of the Nostr wire protocol to ACK/NACK a
publish — success-on-ACK and failed-to-publish-on-all-NACK. Only
remaining gap: `server.go`'s gRPC handlers (need grpc stream mocking,
lower value — thin protocol glue over already-tested logic). `notifications`
coverage: 0% → 54.3%.

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
backing the scenario runner below. See `test/devnet/README.md`. Verified
working end-to-end: all three roles connect, sync, and can be funded via
`./collect-addresses.sh` + `./generate.sh`.

**Correction (2026-07-14):** the note this item originally had — that
`itest/lntest`'s own miner "uses literal bitcoind and is fine for CI" —
was wrong and is struck through below in favor of the finding actually
written up under T4-1.

### T4-1: Generic scenario runner — **not just a wrapper, `itest/lntest` can't be reused as-is**
**Finding (2026-07-14):** `itest/lntest`'s existing harness (`miner.go`,
`lnd_node.go`, `test_common.go`) isn't merely "using bitcoind for the
miner" — it configures every `flnd` node with `--bitcoin.node=bitcoind`
plus `--bitcoind.rpchost/rpcuser/rpcpass/zmqpubrawblock/zmqpubrawtx`
(`itest/lntest/lnd_node.go:93-99`), i.e. the *entire* chain-backend wiring
assumes a bitcoind-RPC-dialect daemon. `lokid` (`go-flokicoin`) is a
`btcd` fork, not a bitcoind-RPC-compatible daemon, and `flnd`'s
`--bitcoin.node` flag only accepts `btcd`/`bitcoind`/`neutrino`/
`nochainbackend` (`flnd/lncfg/chain.go:16`) — there is no `lokid` mode.
This is why every `itest/*_test.go` currently fails with `exec: no
command` in this environment (no bitcoind binary at all) — it's not a
missing-binary inconvenience, the harness was never adapted for
Flokicoin and can't be made to work by just installing/renaming a
binary. This is almost certainly *why* T1-4 was deferred with "same
scaffolding gap" — that gap is this.

**Decision:** do not attempt to retrofit `itest/lntest` to speak
btcd-RPC against `lokid` (subprocess-based, bitcoind-shaped wiring throughout,
high effort for uncertain payoff). Build the YAML scenario runner fresh
against `test/devnet/`'s docker-compose infrastructure instead (already
verified working, already neutrino-mode against `lokid`, already what the
user asked for when this repo's own devnet was provisioned). Treat
`itest/lntest` + the ~15 existing `itest/*_test.go` files as orphaned
legacy scaffolding — flag for eventual removal/archival in Phase 6, not a
dependency of Phase 4. Reusable non-chain utilities from `itest/` (
`mempool_api.go`'s fee-fixture API, `nostr_relay.go`'s mock relay,
`postgres.go`'s container harness) remain valid and should still be
reused — only the bitcoind-coupled node-lifecycle code is out.

Add `itest/scenarios/` + `itest/runner_test.go` that globs
`scenarios/*.yaml`, decodes into a typed `Scenario` struct (unknown keys are
a hard error), and runs each as a named subtest. Step/assertion verbs are a
registry (`map[string]StepFn` / `map[string]AssertFn`) driving the
devnet's docker-compose services (start/stop/restart a role, RPC into
`lokid` to mine, drive `flncli` against a role) plus the still-valid
`itest/` utilities (`mempoolApi.SetFees`, `MockRelay`, `PostgresContainer`).

**Status (2026-07-14):** ◐ Foundation built and proven live against the
devnet, YAML layer not started yet. Built as plain Go first (deliberately —
prove the mechanics work before formalizing into a declarative schema):

- `itest/devnet.go` (`Devnet`) — wraps `test/devnet/`'s docker-compose
  services: idempotent `StartDevnet` (never restarts `chain`, which wipes
  its own regtest DB), waits for all 3 roles' `GetInfo`, mines a
  freshness block. `LightningClient(role)`/`RawLightningClient(role)`/
  `RawRouterClient(role)` construct flspd's own production `lnd.LndClient`
  against a role (reusing production gRPC client code, not a parallel
  test-only implementation).
- `itest/flspd_process.go` (`FlspdProcess`) — builds and runs the real
  `cmd/flspd` binary as a subprocess against a devnet role + a disposable
  `PostgresContainer`, with Start/Stop/Kill/Restart. `Pdeathsig` +
  `killStrayFlspdProcesses` (a `pgrep`-based sweep run at every
  `StartDevnet`) exist specifically because `go test -timeout` kills the
  test process without running `t.Cleanup`, which orphans the child —
  found by tripping over exactly this (see F-10's follow-on below).
- `itest/lsps2_client.go` — a minimal LSPS0/LSPS2 buyer-side client:
  `CustomMessageBus` (JSON-RPC over the `LspsMessageId` custom-message TLV,
  mirroring `lsps.Server`'s own wire handling from the other side),
  `Lsps2Client.GetInfo`/`.Buy`, `CreateJitInvoice` (builds the bolt11 with
  the phantom-SCID route hint `lsps2.buy` returns), `PayInvoice`
  (`routerrpc.SendPaymentV2` — `SendPaymentSync` isn't implemented on this
  flnd fork), `ConnectPeer`, `AutoAcceptChannels` (flnd rejects every
  inbound channel open without a registered `ChannelAcceptor`, full stop —
  not specific to zero-conf), `OpenNormalChannel`.
- `lnd.LndClient.RawClient()`/`.RouterClient()` — small, deliberate
  production additions exposing the underlying `lnrpc.LightningClient`/
  `routerrpc.RouterClient` for exactly this kind of test/advanced use;
  flspd itself never calls these as the LSP.

**Two more real bugs found and fixed via this harness** (added to F-10 in
Phase 2, both verified with real before/after runs, not just reasoning):
1. `cmd/flspd/run.go`'s `stopInterceptors` was called from up to three
   concurrent paths and wasn't idempotent — panicked on ordinary graceful
   shutdown every time. Fixed with `sync.Once`.
2. `lnd/interceptor.go`'s `mapFailureCode` produced `lnrpc.Failure_FailureCode`
   values flnd's own `interceptedForward.FailWithCode` doesn't implement
   (only `TEMPORARY_CHANNEL_FAILURE` + onion-invalid + `ExpiryTooSoon` are
   real; `TEMPORARY_NODE_FAILURE`/`INCORRECT_OR_UNKNOWN_PAYMENT_DETAILS`
   return `ErrUnsupportedFailureCode`) — sending one broke the *entire*
   interceptor gRPC stream, which reconnected and redelivered the same
   HTLC into the same bug, looping until the sender's own timeout gave up.
   Fixed by always mapping to `TEMPORARY_CHANNEL_FAILURE`, the one code
   this interceptor path can actually deliver.

**Gossip-propagation blocker: root-caused and fixed (2026-07-15).**
Hypothesis (a) from the original investigation was right. Root cause,
traced into `flokiorg/flnd` source directly:

- `protocol.option-scid-alias`/`protocol.zero-conf` were enabled
  identically on all three devnet roles (`flnd.conf.sample`). These are
  feature bits negotiated *per peer connection* (via `init`), not per
  channel — with both payer and lsp advertising `option-scid-alias`, it
  gets negotiated for *every* channel between them, including the
  ordinary (non-zero-conf, not explicitly private) payer↔lsp routing
  channel `OpenNormalChannel` opens.
- `funding/manager.go`'s post-6-conf announcement path
  (`completeChan.NegotiatedAliasFeature() && !completeChan.IsZeroConf()`)
  treats any such channel as an alias channel needing special handling:
  it calls `f.cfg.AliasManager.DeleteSixConfs` +
  `f.cfg.DeleteAliasEdge`/`f.addToGraph` to rewrite the edge *before*
  `f.announceChannel` broadcasts the real `channel_announcement`. On this
  fork (and reproducibly, so not obviously fork-specific — this is
  upstream-mirrored code, see `git log -p` on `discovery/gossiper.go`),
  that delete-then-re-add sequence left the edge missing from the graph
  DB. Confirmed directly in `flnd-lsp`/`flnd-payer` logs for the exact
  affected SCID: `DISC: ignoring remote ChannelAnnouncement1 for own
  channel` (a real, correct guard in `discovery/gossiper.go` —
  `ProcessRemoteAnnouncement` rejects any `ChannelAnnouncement1` naming
  our own key, since normally the *local* announcement path inserts the
  edge first) followed permanently by `GRPH: unable to add channel: edge
  not found` / `GRDB: UpdateEdgePolicy faild: edge not found` every time
  either side tried to publish a `channel_update` for that SCID — so
  neither side's policy could ever land in either graph, no matter how
  long you wait (confirmed: still zero propagation after 14+ hours / ~560
  trickle cycles on the original channel).
- **Fix:** the payer role doesn't need scid-alias/zero-conf at
  all — it never receives a JIT channel, only routes payments through an
  ordinary public channel. `test/devnet/setup.sh` now strips
  `protocol.option-scid-alias`/`protocol.zero-conf` from the payer's
  config only (mirrors the existing LSP-only `requireinterceptor` sed
  pattern). LSP↔client is unaffected — that negotiation is independent,
  per that connection, and still needs (and still gets) both features for
  real JIT channels.
- **Verified live:** closed the old (poisoned) payer↔lsp channel, applied
  the same config change to the running devnet's
  `data/flnd-payer/flnd.conf` (gitignored, not part of the fix itself —
  `setup.sh` is), restarted `flnd-payer`, opened a fresh channel. Within
  ~50 seconds (well under one 90s trickle cycle) `getnodeinfo` from *both*
  sides showed *both* directions' `channel_update` policy populated —
  first time either side has ever seen the other's policy for a
  payer↔lsp channel on this devnet.

**New blocker found immediately behind it (not yet resolved):** with the
edge now fully gossiped, `TestLsps2JitHappyPath` progresses much further
— flspd's interceptor engages on the *first* HTLC attempt (previously it
never got that far), opens the real zero-conf channel, and **the LSPS5
order reaches `COMPLETED` server-side** — but the run still ends in
`FAILURE_REASON_NO_ROUTE` on the payer's side. This is a different bug
from the gossip gap: the interceptor's own `routeTimeout` (60s,
`interceptor/intercept_handler.go`) polls `QueryRoutes` waiting for the
LSP's *own* router to see the freshly-opened zero-conf channel as having
sufficient local balance — and in the observed run, `QueryRoutes` returned
`insufficient local balance` continuously for the *entire* 60 seconds,
never resolving, before the interceptor gave up and resumed the HTLC
best-effort. That resume and the payer's own give-up landed in the same
second (10:22:12 in the captured run) — a race, not a hang. Ruled out
`routerrpc.BuildRoute`+`SendToRouteV2` as a workaround (BACKLOG's own
previously-recommended next step): this fork's `BuildRouteRequest` proto
(`lnrpc/routerrpc/router.proto`) has no `route_hints`-equivalent field,
only `hop_pubkeys` + `outgoing_chan_id` — it has no way to supply per-hop
fee/cltv data for a hop with no real graph edge, so it fundamentally can't
build a route through a still-phantom JIT hop the way `SendPaymentV2`'s
bolt11 route hints can. Whatever fixes this needs to be on the "why does
the LSP's own zero-conf channel balance never sync into its router within
60s" side, not a payer-side pathfinding workaround. `FlspdProcess`, the
devnet, and the LSPS2 client all remain solid and reusable regardless.

**Also learned the hard way:** this devnet's chain/wallet state is meant
to be treated as disposable session-to-session (see `test/devnet/README.md`)
but is *not* something to wipe unprompted mid-session — did so once while
chasing this NO_ROUTE issue (`docker compose down -v && rm -rf data`,
caught and blocked by the safety classifier on the following command, user
re-authorized a fresh re-provision). Don't repeat that: ask first.

### T4-2: Seed scenarios
**Status (2026-07-15):** ◐ Runner built (T4-1's actual deliverable, not
just the seed scenarios): `test/itest/scenario.go` (`Scenario`/`VerbCall`
types, strict YAML decoding — unknown top-level keys, unknown verbs, and
unknown per-verb params are all hard errors at load time, not silently
ignored), `test/itest/scenario_steps.go` /
`test/itest/scenario_asserts.go` (the `map[string]StepFn` /
`map[string]AssertFn` registries T4-1 specified), and
`test/itest/runner_test.go` (`TestScenarios` globs `scenarios/*.yaml`,
runs each as a named subtest against a fresh flspd + Postgres on the
shared devnet). `test/itest/scenario_test.go` covers the parsing/
validation logic itself with no devnet required. Step verbs implemented
so far: `connect_peer`, `open_channel` (idempotent, mirrors the existing
Go-level test's reuse logic), `mine_blocks`, `auto_accept_channels`,
`lsps2_buy`, `create_jit_invoice`, `pay_invoice`; one assertion verb,
`channel_exists`.

1. `lsps2_jit_happy_path.yaml` — buy → pay → zero-conf channel open. Now
   wired through the actual YAML runner (`test/itest/scenarios/`) rather
   than only the hand-written Go-level proof (`lsps2_jit_test.go`, kept
   alongside it) — confirmed to fail at the exact same step
   (`pay_invoice`) for the exact same reason as the Go-level test: see
   T4-1's "new blocker found immediately behind it." **Known-failing, not
   swept under the rug** — the scenario file's own header comment
   explains why and points back here. Not part of the CI gate either way
   (this whole package is path-filtered out of `ci.yml`'s test job).
2. `lsps1_order_settle_and_open.yaml` — manual order → settle → auto-open.
   **Not started.** Needs a new LSPS1 test client (order creation +
   watching for the on-chain settlement payment) comparable in scope to
   what `lsps2_client.go` took for LSPS2 — nothing like it exists yet in
   `test/itest/`.
3. `lsps1_recovery_after_restart.yaml` — reproduces F-1: settle invoice, kill
   the LSP mid-flight, restart, assert **exactly one** channel opens (not
   two) and the order reaches COMPLETED. **Not started**, same LSPS1
   client dependency as #2, plus needs a `kill_lsp`-style fault-injection
   step (see T4-3, which this scenario also needs regardless of T4-3's
   own broader scope).

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

**Status:** ✅ Done (commit `4b3263f`). Added `config.SecretRef`
(`config/secretref.go`) — accepts a plain inline string (unchanged
backward-compat form) or a `{from_env: VAR}`/`{from_file: /path}`
reference, resolved on demand via `.Resolve()`, never in place. Not
resolved at config-load time on purpose: `dashboard/service.go`'s
`handleUpdateConfig` re-serializes the in-memory `NodeConfig` back to
YAML on every dashboard save, including these fields, copied verbatim
from server memory rather than the (secrets-stripped, C-1) browser
payload — if `Resolve()` mutated the ref or `MarshalYAML` emitted the
resolved value, the actual secret would get written to disk in place of
the reference on the very next save. `from_file` hex-encodes raw file
content automatically unless it's already hex text, so a ref can point
directly at flnd's own `tls.cert`/`*.macaroon` files. `FlspdPrivateKey`
and `LndConfig.Cert`/`Macaroon` now use it; every consumption call site
(`cmd/flspd/run.go`, `lnd/client.go`) updated to `.Resolve()` instead of
using the field as a plain string. `nodes.sample.yaml` now uses
`from_env`/`from_file` for all three fields and is covered by a test
that parses it and asserts none of them are inline. `config` coverage:
89.3% → 91.2%.

### T5-2: One unit at the operator surface
Amounts in `loki` everywhere at the config/API boundary (flspd's native
base unit — 1 FLC = 100,000,000 loki, converting to mloki internally
where the code already does); fees as `%` + floor `loki`; routing as
`ppm` + `base_loki`. Retire `permyriad` and the bare `0.000001` fraction
from anything a human edits directly. **flspd is Flokicoin, not Bitcoin —
never introduce `sats`/`BTC` naming here or anywhere else in the config,
docs, or code; the unit names are `loki` and `FLC`.**

**Status:** ✅ Done (commit `6722bf0`). Added `config/tiered.go`:
`opening_fee: {rate_percent, min_loki}`, `routing_fee: {ppm, base_loki}`,
and `channel_size: {min_loki, max_loki, padding_loki}`, each expanding
into the existing flat fields at `Validate()` time so every downstream
consumer (interceptor, lsps, cmd/flspd) is unchanged — see the
commit message for the exact field mapping and the compat-shim
precedence rule (block-level: a new block, when present, always
overwrites the flat field(s) it maps to). `nodes.sample.yaml` rewritten
to use all three. `permyriad`/bare-fraction fields remain supported
(not removed) for one-release backward compat, per this phase's stated
goal — just no longer what the sample file demonstrates.

### T5-3: One shared fee model
Confirm F-5's unified fee function (landed in Phase 2) is the single
implementation referenced by the new `opening_fee: {rate_percent, min_loki}`
config block — no parallel fee math anywhere.

**Status:** ✅ Done (commit `60bdf29`). F-5 was never actually finished —
see its BACKLOG entry — `handler_lsps1.go` still had its own hand-rolled
fee calculation, diverging from LSPS2's. Fixed it here rather than leaving
a stale "confirmed" note that wasn't true; the new `opening_fee` block
(T5-2) and both LSPS1/LSPS2 handlers now share one function,
`ComputeOpeningFee`, fuzz-tested (`FuzzComputeOpeningFee`).

### T5-4: Derived channel-size bounds
Replace the six independent LSPS1/JIT min/max balance fields with one
`channel_size: {min_loki, max_loki, padding_loki}` block; derive the
protocol-specific bounds from it with validated defaults.

**Status:** ✅ Done (commit `6722bf0`, alongside T5-2/T5-5 — see T5-2's
entry for the shared implementation notes).

### T5-5: Risk profile presets
Add `profile: conservative|balanced|aggressive` that expands to the
confirmation/CLTV-delta/expiry/zero-conf bundle; any single value stays
overridable under an `advanced:` block.

**Status:** ✅ Done (commit `6722bf0`). Three presets in
`config/tiered.go`'s `profilePresets` — `balanced` matches this repo's
own `nodes.sample.yaml` baseline (targetConf 6, minConfs 3, timeLockDelta
400); `conservative` and `aggressive` scale confirmations/CLTV-delta/
expiry windows around it, `aggressive` allowing 0-conf (`minConfs: 0`).
No separate `advanced:` block was needed to make fields individually
overridable — every field a profile touches is only filled in if still
at its zero/nil value after normal YAML unmarshaling, so setting it
directly in the document (at any nesting level flat fields already
support) overrides the preset. Presets are risk-tolerance defaults, not
protocol minimums/maximums, documented as such in `tiered.go`.

### T5-6: Operator visibility tooling
Add `flspd config check` (validate, human-readable errors), `flspd config
show --effective` (print the fully-resolved config, nothing hidden), and
`flspd config explain fees` (plain-language fee example for a given channel
size).

**Status:** ✅ Done (commit `8a5ee00`). All three are config-file-only
(no Postgres/flnd connection), sharing `loadNodeConfigs` with the daemon
itself (extracted out of `runLspd` so there's one loading path, not two
that could drift). `check` runs full `Validate()` *and* attempts to
resolve all three `SecretRef` fields — Validate() deliberately doesn't
touch those (see `config/secretref.go`), so this is the only place a
missing `from_env` var or unreadable `from_file` path gets caught before
the daemon itself starts. `show` prints the fully-expanded config as
YAML — tiered blocks and profile defaults applied, secrets shown in
their reference form (never resolved/printed). Writing `show`'s test
surfaced a real bug: `secretRefYAML`'s yaml tags were missing
`omitempty`, so every ref marshaled with both `from_env` and `from_file`
present (one empty) — cosmetically misleading for exactly the tool meant
to give operators a trustworthy view of their config, even though it
round-tripped fine. Fixed, and strengthened the existing round-trip test
(which only checked post-parse struct equality) to check the raw
marshaled text too. `explain fees` reuses `lsps.ComputeOpeningFee`
(F-5) directly.

### T5-7: Re-skin `Settings.tsx` to the tiered model
Connect / Business policy / Advanced cards per the config-UX report,
consuming the secrets-stripped config from C-1 and the new tiered schema.
Fee model as a live-computed sentence + small table; channel size as a dual
slider; product toggles; profile dropdown with read-only expansion.

**Acceptance criteria:** a fresh operator can configure a working node
touching ≤10 fields; every previously-supported field is still settable
under `advanced:`; `nodes.sample.yaml` is rewritten to the new shape and is
safe to commit verbatim (no secrets).

**Status:** ✅ Done (commit `1a11ff6`). Three tabs: Connect (name, host,
flnd address — the ≤10-field happy path), Business Policy (profile
picker w/ read-only expansion, opening-fee rate+floor w/ a live sentence
+ example table via a new `feeCalc.ts` — clearly commented as a
client-side mirror of `ComputeOpeningFee`/`config/tiered.go` for instant
preview only, not a second source of truth — channel size as a new
dependency-free `DualRangeSlider`, JIT/LSPS1 toggles), and Advanced
(every flat field from the old UI, still fully editable, with a banner
explaining the precedence rule from T5-2). Handles an interaction the
naive design would've gotten wrong: switching the profile dropdown
explicitly clears the profile-controlled flat fields back to their unset
sentinel first, since the backend's `applyProfile` only fills in
still-unset fields — without that, the dropdown would visibly change
but silently not take effect on save. A config that hasn't touched the
tiered blocks yet gets them derived from its current flat values on
first load, so existing deployments see real settings, not blanks.
`nodes.sample.yaml` was already rewritten as part of T5-2..T5-5.
Verified: `tsc --noEmit` clean, production build clean, `feeCalc`'s own
vitest suite passes. **Not verified in an actual browser** — no browser
automation tooling was available in this environment; a manual
click-through is recommended before shipping.

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

**Status:** ✅ Done (commit `53340d9`). `format.TIME_FORMAT` moved to
`common/time_format.go` as `TimeFormat` (idiomatic naming, since it was
being touched anyway) — `platform/` doesn't exist yet (that's T6-4), so
this landed in `common/` as the closest current home, to be moved again
when T6-4 happens. `just clean`/`clean-all` extended, but deliberately
*not* exactly as specced: `data/` (nodes.yaml, flnd data dir, postgres
data dir — live local dev state, not a build artifact) went into
`clean-all` (already the destructive tier, alongside the docker-volume
wipe) rather than plain `clean`, so a routine "clean build artifacts"
command a developer runs reflexively can't silently wipe local devnet
state — see this session's earlier devnet-wipe incident for why that
distinction matters.

### T6-2: Introduce `internal/`
Move the clearly-private adapters (`lnd`, `postgresql`→`adapters/postgres`,
`mempool`) under `internal/adapters/`. No public API depends on these being
importable from outside.

**Status:** ✅ Done (commit `1360196`). Blast radius was small (5 files:
`cmd/flspd/run.go`, `cmd/flspd/migrate.go`,
`cmd/flspd_revenue_cli/{revenue,db}.go`, `itest/devnet.go`), confirming
BACKLOG's own rationale. `postgresql` renamed to `postgres` (package
identifier, not just the directory) to match its new path, per the
`→adapters/postgres` arrow notation — `lnd`/`mempool` kept their package
names, only the path changed. `go:embed`'s relative `migrations/*.sql`
path moved as a unit with the package, no changes needed.

### T6-3: Split `common/` into `core/`, `ports/`, `services/`
The largest single change: separate domain types (`core/order`,
`core/channel`, `core/fees`) from interface definitions (`ports/`) from
orchestration logic currently sitting in `common`'s services
(`OpeningService`, `NodesService` → `services/`). Do this alone, in its own
PR window, with the Phase 3 tests as the regression net.

**Status:** ✅ Done (commit `be7232d`). `core/` — domain types only, no
interfaces: `Node`, `NotificationRegistration`, the intercept domain
(`InterceptRequest`/`InterceptResult`/action+failure-code consts), the
opening/LSPS domain (`OpeningFeeParams`, `Lsps2BuyRegistration`,
`Lsps1Order`, `Lsps5Webhook`), the extra-fee TLV helper. `ports/` —
`InterceptHandler`, `NodesService`, `OpeningStore`, `OpeningService`
interfaces, depending on `core` for their signatures. `services/` —
`CombinedHandler`, `nodesService`, `openingService` (the concrete
implementations, plus the promise sign/verify helpers), depending on
both. `common/` remains as a smaller, genuinely cross-cutting residual —
logging setup + the `LogComponent`/`LogAction` taxonomy (left alone on
purpose, that's T6-4's "platform/" territory, not this PR's), `TimeFormat`,
and the generic `ValidateWebhookUrl`/`ParsePrivateKey` helpers that don't
have one natural domain home. ~46 files referencing `common.*` across the
whole module needed symbol-by-symbol reclassification (never a blind
package-name find-replace) — verified via a zero-remaining-old-references
grep sweep before considering it done, not just "it compiles." Full test
suite green throughout.

### T6-4: Consolidate transport, platform, proto
`transport/` for inbound surfaces (grpc, lsps json-rpc, dashboard http,
auth); `platform/` for config/logging/build; `api/proto/` as the one home
for `.proto` + generated code (currently interleaved with hand-written logic
in `rpc/`, `notifications/`).

**Status (2026-07-15):** ◐ Partially done — the mechanical, low-risk moves
landed; the ones needing real symbol-level business-logic-vs-transport
separation (same rigor T6-3 needed) are deliberately deferred, not rushed.

Landed, each verified with `go build`/`go vet`/full test suite green
before moving to the next:
- `rpc/` → `api/proto/rpc/` (`.proto` + generated `.pb.go`/`_grpc.pb.go`
  + `genproto.sh`, plus `node_context.go` — a small (37-line) piece of
  gRPC auth-context glue kept alongside its proto types rather than
  further split into `transport/grpc/`, since splitting it would mean
  every one of its 3 call sites needing *both* packages imported under
  separate aliases for zero real separation-of-concerns benefit; not the
  purest possible split, but a deliberate, documented one). Package name
  (`rpc`) unchanged, only the import path moved — 3 importers
  (`cmd/flspd/{grpc_server,channel_opener_server}.go`,
  `notifications/server.go`) needed a one-line path fix each.
- `config/` → `platform/config/` (25 importers — the largest file count
  in this pass, but a pure import-path rename since the package name
  (`config`) didn't change; every reference was already unaliased
  `"github.com/flowgate-lsp/flspd/config"`, confirmed before doing a bulk
  sed across all 25). Caught and fixed one real fallout:
  `secretref_test.go`'s `os.ReadFile("../configs/examples/...")` assumed
  `config/` sat one level below repo root — now two levels deep
  (`platform/config/`), needed an extra `../`.
- `build/` → `platform/build/` (4 Go importers, pure path rename). Also
  updated the 4 non-Go `-ldflags -X github.com/flowgate-lsp/flspd/build.tag=...`
  references that inject the version string at build time — these use
  the import path as a bare string, so `git mv` alone doesn't catch them:
  `Dockerfile` (both `flspd` and `flspd_revenue_cli` build stages),
  `.github/workflows/release.yml`, and 4 occurrences across `justfile`'s
  `build-backend`/`build-cli`/`dist` recipes. Verified the injection still
  actually works post-move by building a throwaway binary with the new
  `-X` path and `strings`-grepping the injected tag out of it, not just
  trusting `go build` succeeding.
- `auth/` → `transport/auth/` (1 importer, `dashboard/service.go`) — the
  cleanest case in this pass: NIP-98 HTTP auth verification has no
  separate "business logic" dimension to split out, it *is* the
  transport-layer concern.

**Deliberately not done, same reasoning as T6-5's own precedent below**
(genuine architectural ambiguity needing careful per-symbol work, not
mechanical risk):
- `notifications/`'s `.proto`/generated code vs. hand-written logic split.
  Unlike `rpc/`, this one can't be a clean mechanical move: the
  hand-written files (`server.go`, `cleanup.go`, `expiry_watcher.go`,
  `notification_service.go`, `store.go`) reference the proto-generated
  types (`NotificationsServer`, `EncryptedNotificationRequest`, ...)
  *unqualified*, because today they're in the same package. Moving just
  the generated files to `api/proto/notifications/` means every one of
  those unqualified references across every hand-written file needs to
  become package-qualified — a real, if mechanical-once-planned, sweep
  T6-3's own ~46-file audit is the right template for, not something to
  improvise inside a broader T6-4 pass. `notifications/server.go` (the
  actual gRPC service impl, an inbound surface) is also a `transport/`
  candidate by T6-4's own definition, layering a second decision on top
  of the proto split.
- `lsps/`'s transport (JSON-RPC plumbing) vs. business logic
  (LSPS0/1/2/5 protocol implementation, order/settlement handling) split
  — flagged as genuinely ambiguous when T4-1 first ran into this same
  question; still true here. `lsps/` is mostly protocol business logic
  wearing a JSON-RPC transport on top, not a thin transport layer with
  logic accidentally leaking in — a wholesale move to `transport/lsps/`
  would misclassify the bulk of the package.
- `dashboard/`'s HTTP-handler (transport) vs. config-update/analytics
  (business logic) split — same shape of problem as `lsps/`; recall
  C-1/C-2/C-3 (Phase 2) were all bugs *in* `dashboard/service.go`'s
  config-handling logic, not its HTTP plumbing, which is itself evidence
  this package mixes the two rather than being pure transport.
- `common/logger.go` + `log_enums.go` → `platform/logging/` (the last
  piece of T6-3's own predicted `platform/` destination for these) — not
  attempted this pass. `common`'s log-component/log-action taxonomy is
  referenced from ~18 importers, but unlike `config/`'s move, some of
  those files use *other* `common` symbols too (`TimeFormat`,
  `ValidateWebhookUrl`, `ParsePrivateKey` — the residual T6-3 explicitly
  left with "no natural domain home"), so this isn't a blind
  whole-package path rename either; it needs the same
  file-by-file/symbol-by-symbol classification as the proto splits above.

### T6-5: Relocate samples, deploy, frontend
`configs/examples/` for the sample YAML/env files (out of repo root);
`deploy/` for Dockerfiles + compose; `web/` for the frontend; `test/`
housing `itest/`, the new `scenarios/` from Phase 4, and `devnet/`
(already landed at `test/devnet/` — see T4-0).

**Acceptance criteria:** tree matches the target layout from the structure
report; `go build ./...`, `go vet ./...`, and the full test suite pass after
every individual PR, not just at the end.

**Status:** ◐ Partially done. Landed (commits `e4d0f91`, `1f4667b`):
`configs/examples/` (nodes.sample.yaml, compose.env.sample,
flspd.env.sample — updated justfile's `setup-backend` and
`config/secretref_test.go`'s relative path); `itest/` → `test/itest/`
(fixed `devnetDir()`'s relative path, verified against the *real* devnet
via `TestDevnetSmoke`/`TestFlspdProcessSmoke`, not just a build check;
also fixed README.md's already-stale Integration Tests section, which
still referenced CLI flags from the itest/lntest harness removed earlier
this session). **Deliberately not done**, per an explicit decision this
session: `deploy/` (Dockerfile, Dockerfile.dev, docker-compose.{dev,prod}.yml)
and `web/` (frontend/) relocation. `Dockerfile.dev`/`docker-compose.dev.yml`
carry uncommitted local dev-tooling changes (the air.toml hot-reload
setup) from earlier in this session, and `docker-compose.dev.yml`'s
`flnd` service build context is a sibling-checkout-relative path
(`../../flokiorg/flnd`, resolved *outside this repo* against a workspace
layout) that would need careful re-derivation if the compose file's own
location changes — real risk of silently breaking local dev if gotten
wrong, on top of files already mid-edit. Left for deliberate, focused
handling rather than folding into this session's sweep.

**Re-checked (2026-07-15), still deferred:** the blocker is unchanged.
`Dockerfile.dev`/`docker-compose.dev.yml` still carry the same
uncommitted dev-tooling edits (not this session's — someone else's
in-progress work, not mine to fold into a structural move), and
`docker-compose.dev.yml` now visibly has *three* fragile relative paths
that would all need re-deriving together if it moved: `../../flokiorg/flnd`
(the `flnd` service), `../../` + `flowgate/flspd/Dockerfile.dev` (the
`flspd` service, assumes a specific depth inside a multi-repo workspace
layout), and `./frontend` + `Dockerfile.dev` (the frontend dev service —
meaning `web/`'s relocation and `deploy/`'s are coupled: moving one
without the other breaks this reference regardless). Same call as before:
don't touch it speculatively.

---

## Performance backlog (non-blocking, run in parallel once Phase 1 gate is met)

These are latency/CPU/memory notes from the original audit. None are
correctness bugs; none block the phases above, but they're easiest to land
alongside the Phase 3 tests for the same code.

**Status (2026-07-15):** investigated all four against the current code
(not just the original one-line notes) before touching anything — this
hot path (`interceptor/intercept_handler.go`) is the exact code this
session's T4-1 work spent significant effort characterizing a live,
still-open timing race in (see Phase 4), and P-4 specifically turned out
to have a real behavioral trap once actually read rather than assumed
safe from its one-line description. No code changes made this pass;
verifying that risk *before* changing hot-path timing/concurrency behavior
is the actual deliverable here, given what's already fragile in this area.

- **P-1** `Intercept` polls `GetChannel` (1s) and `QueryRoutes` (500ms) on
  fixed timers instead of driving off the existing channel-active event
  subscription in `lnd/client.go` — costs tail latency and idle load.
  **Confirmed, concrete fix path exists but not attempted:**
  `internal/adapters/lnd/client.go`'s `listenChannelEvents()` +
  `WaitChannelActive` already implement exactly this kind of
  subscription-based wait (`notifyAndWait` already uses
  `WaitChannelActive` for a different case), so P-1 isn't a "build new
  infrastructure" problem, it's a "wire the existing primitive into this
  specific loop" problem. Not done this pass: the `GetChannel` loop needs
  the *specific* `channelPoint` `openChannel` just returned, not just
  "any active channel with this peer" (what `WaitChannelActive` checks),
  and the second inner loop (`QueryRoutes` polling for router balance
  sync) has no existing event-driven equivalent at all — that's the same
  "insufficient local balance never resolves" gap Phase 4 already
  documented as open. Changing this loop's timing without dedicated,
  repeated live-devnet verification (each run takes over a minute) risks
  masking or worsening that already-tracked race rather than just being a
  clean perf win.
- **P-2** `GetPeerId` / `IsConnected` do full `ListChannels`/`ListPeers`
  scans on every interception; a cached peer↔scid map refreshed off the
  channel-event stream removes them from the hot path. **Confirmed real**
  (`internal/adapters/lnd/client.go`'s `GetPeerId`/`IsConnected`, both
  called from `Intercept()`'s hot path) — the channel-event subscription
  infrastructure `listenChannelEvents()` already maintains internally
  could back a cache, but building and invalidating that cache correctly
  is new work, not a wire-up of something existing like P-1's fix is.
- **P-3** `ListPeers` calls `GetNodeInfo` per peer (N+1 gRPC) for UI
  alias/color data — fine for a dashboard poll, wrong if ever called hot.
  **Verified, no action needed:** every call site
  (`grep -rn '\.ListPeers('`) is in `dashboard/service.go` — 5 call
  sites, all dashboard-facing. It is not, in fact, ever called hot today;
  the note's own caveat doesn't currently apply.
- **P-4** Notification fan-out spawns one goroutine per webhook per event
  with a 7s context inline on the interception path; route through the
  existing persistent queue asynchronously instead. **The literal fix
  would be a regression, found by actually reading the call sites:**
  `notifications/notification_service.go`'s `Notify()` already writes to
  a durable, persistent queue (`store.Enqueue` → `notification_queue`
  table) *and* a background poller already exists
  (`NotificationService.Start` → `runWorker`, 10s tick,
  `processPending`/`GetPending` with exponential backoff) — so "enqueue
  and let the async worker deliver it" is mechanically trivial (drop the
  eager `deliverOne` call + `wg.Wait()` in `Notify()`/`enqueueAndDeliver`,
  just enqueue and return). *But* `interceptor/intercept_handler.go`'s
  `notifyAndWait` (line ~424) depends on `Notify()`'s current *synchronous
  delivery* return value as a real fast-fail signal: `if err != nil ||
  !notified { resume immediately }` — skipping the up-to-`NotificationTimeout`
  (default 1 minute) `WaitOnline` block specifically when the push
  notification is known to have failed to *reach* the client (webhook
  down, DNS failure, etc.), not just "wasn't delivered yet." Making
  `Notify()` enqueue-only would make every one of those failed-webhook
  cases block for up to a minute instead of failing fast — a real latency
  regression on exactly the path this was meant to speed up, not a safe
  perf-only change. A correct fix needs to preserve that fast-fail
  signal (e.g. keep one bounded/short synchronous delivery attempt for
  the `notifyAndWait` caller specifically, route only the
  fire-and-forget caller at `intercept_handler.go:173` through pure
  enqueue) rather than changing `Notify()`'s contract wholesale.

---

## Cross-reference to source reports

- Test/architecture audit (F-1…F-8, coverage matrix, itest design):
  https://claude.ai/code/artifact/5566942a-9b66-42d2-beda-c4bacc8f39f4
- Repo structure / config UX review (SEC-1, C-1…C-3, config + arbo redesign):
  https://claude.ai/code/artifact/cd741dd7-ba77-4126-9fbf-48e9756f6621
