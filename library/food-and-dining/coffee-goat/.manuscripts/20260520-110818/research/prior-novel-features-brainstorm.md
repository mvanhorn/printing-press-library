# Novel-Features Subagent Output (Audit Trail)

Spawned during Phase 1.5 Step 1.5c.5. Full response below for retro/dogfood debugging.

## Customer model

**P1 — the multi-continent enthusiast (the user).** Buys from 24 roasters across NA/EU/JP/AU. Has 4–8 bags in rotation at any time. Logs brews on V60, Aeropress, and a single-boiler espresso machine. Frustrated that Beanconqueror only knows beans he's purchased, that Coffee Review scores live on a separate site, and that no tool lets him ask "what's the closest current bean to that Sey Gesha I loved." Weekly rituals: Sunday morning catalog sweep, brew-by-brew dial-in, end-of-bag retrospective.

**P2 — Maya, the home barista with a Niche + Linea Mini.** Buys mostly from 3–4 favored roasters but window-shops the global shelf monthly. Pulls 2–4 espressos daily, logs maybe half. Cares deeply about roast date and dial-in efficiency. Her pain: tracking espresso drift as a single bean ages from day-7 to day-21, and knowing when to bin a bag.

**P3 — Devon, the small-roastery competitive-intel hire.** Works at a US roaster, monitors what The Barn, April, La Cabra, and Sey are sourcing — which farms, which lots, which processes, at what price. Currently does this with bookmarks and a spreadsheet. Wants a producer-centric and roaster-style view.

**P4 — "The household agent," operating on behalf of P1 or P2.** Answers natural-language questions: "what should I drink next from the shelf...", "anything new from Sey?", "is my grinder drifting?". Needs every primary action as an MCP tool with deterministic output.

## Candidates (pre-cut)

12 user-locked features (C1-C12) all passed kill/keep checks. Scoring:

| # | Feature | Score | Verdict |
|---|---|---|---|
| C1 | search | 10/10 | prior-keep |
| C2 | dial-in | 10/10 | prior-keep |
| C3 | shelf | 10/10 | prior-keep |
| C4 | watch | 10/10 | prior-keep |
| C5 | compare | 9/10 | prior-keep |
| C6 | twin | 10/10 | prior-keep |
| C7 | blind-cup | 9/10 | prior-keep |
| C8 | fx | 9/10 | prior-reframe (curated `roaster_shipping` static reference flagged) |
| C9 | producer | 10/10 | prior-keep |
| C10 | refill-plan | 10/10 | prior-keep |
| C11 | roaster-style | 9/10 | prior-keep |
| C12 | drift | 9/10 | prior-keep |

Subagent-added candidates (sources a/b/c):
- **C13 `harvest`** — curated harvest-window freshness ranking (7/10). **Note: User explicitly cut "seasons" earlier in scoping conversation — `harvest` is the same concept renamed. Respect user cut; do NOT propose.**
- **C14 `stock-pulse`** — killed (sync-history table not in data model; overlaps with `watch` and `roaster-style`)
- **C15 `whats-next`** — "what should I drink next" recommender (10/10). NEW; not yet surfaced to user.
- **C16 `vintage`** — killed (fold into `producer --years N`)

## Survivors and kills

### Survivors → transcendence table for the manifest

The 12 user-locked features carry forward as-is, with `fx` flagged for the curated shipping table reframe.

**Subagent's additional C15 (`whats-next`) is surfaced to the user at Phase Gate 1.5 as an optional 13th feature.**

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| C13 `harvest` | User explicitly cut "seasons" (same concept) in scoping conversation; respect user direction | n/a |
| C14 `stock-pulse` | Requires sync-history audit table not in data model; value overlaps with `watch` and `roaster-style` | `watch` (#4), `roaster-style` (#11) |
| C16 `vintage` | Pure refactor of `producer` with year filter; redundant standalone | `producer --years N` flag |
| Hugo Tea filtering | Not a feature — sync-layer `product_type` filter already in brief | n/a |
