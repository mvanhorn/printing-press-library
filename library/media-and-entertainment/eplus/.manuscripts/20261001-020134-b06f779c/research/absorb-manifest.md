# Approved eplus discovery scope
User explicitly preauthorizes sensible read-only scope and routine gates; no routine questions. User explicitly restricts agents to one builder plus one fresh reviewer, overriding the novel-features delegation step. No competing authoritative wrapper contributes features.

| # | Feature | Best source | Our Implementation | Added value |
|---|---|---|---|---|
| 1 | Domestic keyword/artist/date/region/category/venue discovery | Domestic public search | eplus-pp-cli events search | Bounded compact session summaries |
| 2 | Event/session/sale detail and booking handoff | Domestic SSR detail | eplus-pp-cli events detail | Stable identity, separate lottery windows and inventory |
| 3 | International tour discovery | International public catalog | eplus-pp-cli international search | Separate offering, explicit limited catalog coverage |
| 4 | International product/schedule/prices/conditions | International public detail | eplus-pp-cli international detail | Event-specific overseas conditions, missing data explicit |
| 5 | Public source policy guidance | Official FAQ | eplus-pp-cli policies | Source-backed access/payment/collection guidance |
| 6 | Sale window planning | Sale round fields | (behavior in eplus-pp-cli events detail) exact JST sale windows and lottery deadline | Deadlines preserved without false availability |
| 7 | Cached observation/freshness | Read-only HTTP | (behavior in eplus-pp-cli events search) --fresh and timestamped short cache | No inferred inventory, cache explicitly marked |
| 8 | Small projection and request budgets | Agent workflow requirements | (behavior in eplus-pp-cli events search) --select, --limit, --pages | Predictable output and bounded requests |

All rows are shipping scope; no stubs. Unsupported account/checkout/entry mutations excluded by user's read-only boundary. Prices/fees/eligibility absent upstream are explicit unknown; not a stub. Novel planning value delivered by identity, deadline, international restrictions, projection and observation semantics within these commands. All hand-code.

## In-command transcendence clarification on resume

The approved planning behavior in rows 6–8 is one in-command capability, not a new standalone feature. `research.json` now tracks it so the Press gate can check its real command instead of skipping an empty novelty list. No approved scope changes.

| Name | Command | Implementation |
|---|---|---|
| Sale-round planning | eplus-pp-cli events detail | Separate sessions/rounds, exact JST deadlines, unknown availability/access, compact projection and observations |
