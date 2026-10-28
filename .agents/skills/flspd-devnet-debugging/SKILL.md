---
name: flspd-devnet-debugging
description: Devnet bring-up, test/itest YAML scenario conventions, and root-cause debugging methodology for flspd. Use whenever running or writing a scenario under test/itest/scenarios/, debugging a flaky or failing devnet-backed test, investigating a Lightning protocol bug against real flnd nodes, or before restarting/wiping devnet state. Always consult before touching test/devnet/ or test/itest/.
---

# flspd Devnet & Integration-Test Debugging

`test/devnet/` + `test/itest/` is flspd's YAML-driven integration test system: real `lokid` (Flokicoin) + real `flnd` nodes + real Postgres, no bitcoind anywhere. This **replaced** an older `itest/lntest` harness that was bitcoind-coupled and has been fully deleted (2026-07) — if you ever see a reference to `itest/lntest`, `GetBitcoindBinary`, or `--bitcoin.node=bitcoind`, that's dead/orphaned. Don't resurrect it; see `flnd-fork-gotchas` for why it was architecturally impossible to fix instead of just missing a binary.

## Devnet bring-up sequence

1. `test/devnet/setup.sh` — seeds configs.
2. `docker compose up -d --build` (in `test/devnet/`).
3. Poll each flnd role's RPC until healthy.
4. `test/devnet/collect-addresses.sh` + `test/devnet/generate.sh 150` — mining bootstrap.
5. Run tests.

Don't inline the bring-up commands' internals into other docs/scripts — reference these script paths and their one-line purpose; the scripts themselves are the source of truth and will drift out from under a copy.

## Restart vs. full wipe — never wipe without asking first

A plain container restart is **not** sufficient once real channel/wallet state exists: `lokid` wipes its own regtest chain on restart, but `flnd`'s persistent wallet state stays pinned to the old chain tip — the two get out of sync. Recovering from this needs a **full wipe** of all `test/devnet/data/*` directories and a from-scratch rebuild, not just a restart.

Devnet state is disposable session-to-session, but **never wipe it unprompted mid-session** — always ask first. Other scenarios or the user's own manual testing may depend on the current state.

## YAML scenario runner conventions (`test/itest/`)

- **Strict decoding**: unknown top-level keys, step verbs, or per-verb params are hard errors at load time (`scenario.go`) — typos surface immediately, not as a silent no-op.
- **Zero-Go-code scenarios**: step/assert verbs live in registries (`scenario_steps.go`'s step functions, `scenario_asserts.go`'s assert functions). Adding a new scenario should mean writing a new `.yaml` file under `test/itest/scenarios/`, not touching Go.
- **`StartDevnet` (`devnet.go`) is idempotent** and never restarts the `chain` container — restarting it would wipe its own DB.
- **`FlspdProcess` (`flspd_process.go`) sets `Pdeathsig: SIGKILL`** plus a `pgrep`-based stray-process sweep, because `go test -timeout` kills the test process without running `t.Cleanup` — without this, timed-out runs leak orphaned `flspd` processes.
- **One `FakeMempoolServer` per scenario, not shared**: any scenario using the `set_mempool_fees` step gets its own instance (gated by `Scenario.usesStep("set_mempool_fees")` in `runner_test.go`), so fee state from one scenario can never leak into another's assertions. This was a near-miss regression — don't go back to a single shared fake server.
- **Relative, not absolute, state assertions**: the devnet is long-lived across many runs, so an assertion like `channel_count: 1` breaks immediately once other scenarios have already opened channels against the same peers. Snapshot a baseline first and assert *deltas* (`count_channels` + `channel_count_delta`), never an absolute count.
- **itest is deliberately excluded from PR-blocking CI**: `workflow_dispatch` + nightly only. It checks out and builds several sibling repos from source plus a live devnet — realistically 10-20+ minutes with materially more inherent flakiness risk than a unit suite. This was an explicit call, not an oversight — don't try to add it to the push/PR-blocking path.

**Known accepted gap** (don't rediscover this as a new bug): devnet channels/balances accumulate unboundedly across scenario runs with no per-run reconciliation. Some scenarios can show `FAILED` only when run in the full batch, not in isolation. Nobody has specced a fix yet (would need per-scenario chain/DB isolation, or an explicit cleanup step) — this is a real open backlog item, not something to half-fix inside a debugging session.

## Debugging methodology

- **Throwaway probe programs**: when a bug's cause is ambiguous, write a standalone `cmd/*_probe_tmp/` Go program that calls the *real* production client code, run it to get a true A/B result against the suspect code path, then delete it — never commit it.
- **Revert-and-verify discipline**: confirm every fix by reverting it and proving the regression test then actually fails. A fix that "looks right" but was never proven to matter isn't verified.
- **Don't stop at the first plausible explanation.** The canonical example here is the LSPS2 zero-conf JIT investigation (see table below): three separate plausible hypotheses were each rigorously tested and refuted before the real cause was found — and even after fixing it, three more distinct bugs were found stacked on top, and a long-standing "flakiness" turned out to be mundane, not a bug at all.
- **Check resource levels directly before hypothesizing a protocol bug.** `listchannels`' `local_balance`, `pendingchannels`, etc. — on a long-lived, reused devnet, an under-funded or already-pending-limited channel produces resource-sounding errors (no route, insufficient balance) that look exactly like flakiness but aren't a code bug.
- **A single successful RPC call is not proof its effect is live.** E.g. `UpdateChannelPolicy` can return success while the policy silently hasn't taken effect for minutes. Verify via a poll of *observable* state (not just "the call didn't error"), and retry the call itself inside that poll if needed.
- **When flspd's behavior matches a known bug class, check upstream LND's own itest suite** for the equivalent scenario and how it verifies success — this is what cracked the `QueryRoutes` vs `DescribeGraph` bug (see `flnd-fork-gotchas`).

## What NOT to do

- Never wipe the shared devnet without asking first.
- Never commit a `cmd/*_probe_tmp/` probe program.
- Never assert an absolute channel/balance count on this devnet — always relative to a snapshot baseline.
- Don't resurrect `itest/lntest`, `GetBitcoindBinary`, or `--bitcoin.node=bitcoind` — permanently removed, architecturally incompatible (see `flnd-fork-gotchas`).

## Concrete example: the LSPS2 zero-conf JIT investigation

| Step | Finding |
|---|---|
| Hypothesis 1 | Connection identity / gRPC stream separation — tested with 5 independently-controlled probe variants, all failed identically → **refuted** |
| Hypothesis 2 | The devnet's `flnd` build being on an in-progress `sync/lnd-2026-07` upstream-reconciliation branch — A/B tested against flnd's stable `main` (the exact version flspd's `go.mod` pins), identical failure → **refuted** |
| Hypothesis 3 | Accumulated multi-session devnet state — wiped and re-provisioned from scratch, identical failure on a chain with zero history → **refuted** |
| Real cause | Found by comparing against upstream LND's own zero-conf itest, which verifies channel readiness via `DescribeGraph`, never `QueryRoutes` — `QueryRoutes`'s pathfinding reads an in-memory graph cache that lags for a freshly-added zero-conf edge |
| After the fix | 3 more distinct real bugs found stacked on top (JIT capacity not budgeting the channel reserve; test client invoicing gross instead of net; `UpdateChannelPolicy`'s `TimeLockDelta` silently not applying) |
| The "flakiness" | The long-chased "payer-side pathfinding flakiness" across many sessions turned out to be ordinary insufficient local balance on a long-lived, reused, never-topped-up test channel — not a separate bug at all |
