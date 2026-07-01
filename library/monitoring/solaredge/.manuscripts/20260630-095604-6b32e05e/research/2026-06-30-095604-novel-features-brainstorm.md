# SolarEdge Novel Features Brainstorm (Subagent Audit Trail)

## Customer model

**Dave Reinholt** — owns a 9.8 kW residential rooftop system with a battery, installed three years ago. *Today (without this CLI):* he checks the SolarEdge mobile app maybe once a week to see the daily kWh number, with no way to tell if today's output is normal for the season or a sign one string is degrading. *Weekly ritual:* Sunday morning, glances at the production graph, shrugs, closes the app — no comparison baseline, just today's curve in isolation. *Frustration:* convinced an optimizer is underperforming after a tree grew taller next door, but has no way to compare this month to last year or even last week without exporting CSVs and building his own spreadsheet.

**Priya Anand** — runs a 3-person solar O&M shop monitoring 140 residential and small-commercial SolarEdge sites for a regional installer. *Today (without this CLI):* logs into the slow, non-scriptable installer portal, or uses `solaredge-interface` one site ID at a time. *Weekly ritual:* every Monday pulls overview data across 140 sites to find underperformers, against a 300-req/day-per-site budget she's perpetually near. *Frustration:* has hit 429 rate-limit errors mid-morning because nothing tracks the daily budget, and has no single command for "this week's 5 worst-performing sites" — builds that list by hand today.

**Marcus Webb** — home-automation enthusiast running Home Assistant plus a custom Node-RED/MQTT flow for Grafana. *Today (without this CLI):* hand-rolled a Python cron script hitting `currentPowerFlow`/`overview`, storing raw JSON because Home Assistant doesn't retain long history at his desired resolution. *Weekly ritual:* constantly tweaks polling interval after reading SolarEdge 429 threads (including HA issue #59574 directly), trading freshness against rate-limit risk. *Frustration:* no tool tracks his rate-limit budget or persists power/storage telemetry beyond the API's native 1-week window — he's rebuilding a sync+cache layer that shouldn't be his problem.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Kill/keep note |
|---|------|---------|--------------|---------|--------|-----------------|
| 1 | System health check | `site health <siteId>` | One-shot "is my system OK" combining power flow, energy vs baseline, equipment faults | Dave | (a) | Keep — local join, no LLM |
| 2 | Underperformance vs baseline | `site underperformance <siteId> --since 30d` | Flags days below the site's own trailing historical average | Dave | (b) | Keep — mechanical local aggregation |
| 3 | What changed since X | `site changes <siteId> --since 7d` | Digest of deltas vs prior period: energy, equipment faults, battery cycles | Dave, Marcus | (b) | Keep — local diff query |
| 4 | Rate-limit budget tracker | `budget status` | Requests used/remaining today per site against the 300/day cap | Priya, Marcus | (b) | Keep — pure local derived view, no API endpoint exists for this |
| 5 | Fleet underperformer rollup | `fleet underperformers --since 7d` | Ranks synced sites by underperformance, worst first | Priya | (a) | Keep — cross-site SQL join + sort |
| 6 | Battery health trend | `site battery-health <siteId> --since 90d` | Battery cycle/capacity trend beyond API's 1-week cap | Dave, Priya | (b) | Keep — local aggregation beyond window limit |
| 7 | Equipment fault digest | `equipment faults <siteId>` | Non-nominal equipment filtered from inventory+telemetry | Priya, Dave | (a)/(c) | Keep — local join, mechanical status filter |
| 8 | Live power flow tail | `site tail <siteId> --resource power` | Polls power flow on interval | Marcus, Dave | (c) | Cut risk — thin param of generated `tail` |
| 9 | Sync with rate-limit awareness | sync pre-flight check | Warns/aborts sync before exceeding daily budget | Priya, Marcus | (b) | Fold into `budget status` |
| 10 | Fleet energy rollup (sum) | `fleet energy --since 30d` | Total/average energy across all sites | Priya | (c) | Reframe/cut risk — thin wrapper over bulk energy call |
| 11 | Site comparison | `site compare <id1> <id2> --since 30d` | Side-by-side energy/power/battery stats | Priya | (c) | Keep candidate, not separately scored (subsumed) |
| 12 | Optimizer-level degradation finder | `equipment degradation <siteId>` | Per-optimizer decline vs siblings | Dave | (b) | Cut risk — granularity not confirmed buildable |
| 13 | Cross-resource agent snapshot | `site snapshot <siteId> --json` | Combined overview+power flow+inventory+budget for agents | Dave, Marcus | (a) | Cut risk — same join as `site health` |
| 14 | Stale site detector | `sites stale --since 7d` | Sites with no new data since N days | Priya | (c) | Keep — local sync-cursor query |
| 15 | Inverter firmware/inventory audit | `fleet inventory-audit` | Distinct inverter models/firmware across fleet, outliers | Priya | (c) | Cut risk — niche, weak evidence |
| 16 | Production forecast vs actual | `site forecast-gap <siteId>` | Actual vs weather-adjusted expectation | Dave | (b) | Cut — requires external weather service |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|---------------|----------|-------------------|
| 1 | System health check | `site health <siteId>` | 8/10 | hand-code | Joins locally synced power_series, energy_series, and equipment status into one computed go/no-go view | Product Thesis names this exact gap; Dave persona, Top Workflow #1 | Use this command to get one combined go/no-go status for a site. Do NOT use it for raw live power numbers; use 'site current-power-flow' instead, or for raw summary stats use 'site overview'. |
| 2 | Underperformance vs baseline | `site underperformance <siteId> --since 30d` | 9/10 | hand-code | Computes each day's energy against the site's own trailing historical average from local energy_series rows | Product Thesis headline differentiator; Data Layer notes API has no baseline feature; Dave persona | Use this command to flag days that are statistically low vs this site's own history. Do NOT use it for short-term deltas; use 'site changes' instead. |
| 3 | What changed since X | `site changes <siteId> --since 7d` | 8/10 | hand-code | Diffs current-period energy, equipment status, and battery cycle counts against the prior period from local tables | Product Thesis names "what changed since yesterday" directly; Marcus + Dave personas | Use this command for a short-term delta digest. Do NOT use it for statistical underperformance analysis; use 'site underperformance' instead. |
| 4 | Rate-limit budget tracker | `budget status` | 9/10 | hand-code | Maintains a local call-count table per (account-token, siteId) and reports used/remaining against the 300/day cap | Reachability Risk: 300/req-day limit, HA issue #59574; Marcus + Priya personas | none |
| 5 | Fleet underperformer rollup | `fleet underperformers --since 7d` | 8/10 | hand-code | Ranks all synced sites by production-vs-own-baseline, sorted worst-first | Top Workflow #5; Priya persona | Use this command to rank multiple sites by underperformance. Do NOT use it for a single site's history; use 'site underperformance' instead. |
| 6 | Battery health trend | `site battery-health <siteId> --since 90d` | 7/10 | hand-code | Aggregates storage_telemetry cycle/charge-discharge data over windows beyond the API's native 1-week cap | Data Layer + absorb manifest flag persistence beyond 1-week window; Dave + Priya personas | Use this command for trends over weeks/months. Do NOT use it for raw recent telemetry; use 'site storage-data' instead. |
| 7 | Equipment fault digest | `equipment faults <siteId>` | 7/10 | hand-code | Joins equipment inventory with inverter_telemetry status codes, filtering to non-nominal entries only | Top Workflow #4; Priya + Dave personas | Use this command for a filtered list of equipment in a non-nominal state. Do NOT use it for full inventory; use 'equipment inventory' instead. Do NOT use it for raw per-serial telemetry; use 'equipment inverter-data' instead. |
| 8 | Stale site detector | `sites stale --since 7d` | 7/10 | hand-code | Compares each synced site's last-received-data timestamp against now across the whole fleet | Reachability Risk (polling-only, no push); Priya's 140-site fleet workflow | Use this command to find sites that stopped reporting. Do NOT use it for a single site's exact data period; use 'site data-period' instead. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Cross-resource agent snapshot | Same join surface as `site health`; output-flag ergonomics already free on any survivor | `site health` |
| Optimizer-level degradation finder | Fails Buildability proof — per-optimizer-string granularity not confirmed in the API | `site changes` |
| Production forecast vs actual | Fails External Service kill check — needs weather/forecast API not in spec | `site underperformance` |
| Fleet energy rollup/sum | Fails Reimplementation check — thin SUM/AVG over one bulk call, no ranking | `fleet underperformers` |
| Inverter firmware/inventory audit | Below 5/10 bar — single-persona, weak evidence, mechanically a GROUP BY | `equipment faults` |
| Live power flow tail | Fails Wrapper-vs-leverage — thin parameterization of generated `tail` command | `sites stale` |
| Sync with rate-limit awareness | Folded into `budget status`; sync pre-flight guard is an enhancement, not a separate feature | `budget status` |
