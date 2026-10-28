# flspd

Instructions for any AI coding agent working in this repo.

## Standing rules

- **Flokicoin, never Bitcoin.** Native units are **loki** / **mloki** / **FLC**, never sats/msats/BTC — in code, logs, and prose. (One deliberate exception: the LSPS wire protocol's own `_sat`/`_msat` field-name suffixes are spec-mandated and stay as-is — see the `flspd-lsps-protocol` skill.)
- **Never wipe the shared devnet without asking first** — see the `flspd-devnet-debugging` skill before touching `test/devnet/` state.
- **Reference the `BACKLOG.md` task ID in commit messages** while that document is the active working doc (e.g. `fix(interceptor): repair mock drift (F-3)`). A new finding gets the next free ID in the relevant phase rather than a side conversation.

## Skills

Deeper, task-triggered reference material lives under [.agents/skills/](.agents/skills/):

| Skill | Consult when |
|---|---|
| [flspd-lsps-protocol](.agents/skills/flspd-lsps-protocol/SKILL.md) | Implementing/modifying an LSPS0/1/2/5 method, the fee formula, JIT invoice/capacity math, or the settlement state machine |
| [flspd-devnet-debugging](.agents/skills/flspd-devnet-debugging/SKILL.md) | Running/writing a `test/itest` scenario, chasing a flaky devnet-backed test, or debugging a protocol bug against real flnd nodes |
| [flnd-fork-gotchas](.agents/skills/flnd-fork-gotchas/SKILL.md) | Calling flnd's gRPC API or `flncli`, choosing an interceptor failure code, or reasoning about zero-conf/reserve math |
| [flspd-dashboard-design](.agents/skills/flspd-dashboard-design/SKILL.md) | Designing or building a screen in `web/`, or wiring a new dashboard data binding |

## Authoring new skills

Keep each `SKILL.md` thin and single-purpose — push bulk detail into a `references/` subfolder alongside it. Use paths **relative to the skill's own file**; never hardcode an absolute machine path — it breaks for every other clone, machine, or teammate. Favor `MUST`/`MUST NOT` and tables over narrative prose.
