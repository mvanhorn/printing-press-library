# Zaptec CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Ingrid — Home EV owner (one Zaptec Go in the garage).** Charges a single EV overnight on a variable-rate (spot/Tibber-style) tariff.
- Today: Uses the Zaptec portal app to glance at status; no idea what charging costs per month — app shows session-by-session but never a monthly total. Stops charging via phone app.
- Weekly ritual: Opens the app a few times to confirm the car charged; at month-end guesses energy cost from the utility bill.
- Frustration: No terminal/cron control (can't pause when prices spike); no rolled-up "kWh / cost this month"; raw state opaque.

**Bjørn — Housing co-op / landlord administrator (1 installation, 8-30 chargers, shared grid feed).** Manages load-balancing current limit and bills residents.
- Today: Logs into portal, clicks each charger, eyeballs the installation report for billing. Adjusts available-current by hand, risks the 15-min guard.
- Weekly ritual: Reviews which chargers are active/offline, exports usage report for resident billing, sanity-checks no charger stuck/offline.
- Frustration: No single "what's charging right now across the installation" view; no headroom view; offline chargers discovered only by complaint.

**Tobias — Fleet/commercial operator + integrator (multiple installations, scripts the API).** Runs depot chargers, wants automation + anomaly catching.
- Today: Only the HA integration (needs running Home Assistant) or hand-rolled scripts; decodes observation IDs manually.
- Weekly ritual: Reviews fleet utilization, hunts chargers that finished with abnormally low energy or long zero-power "stuck" sessions, checks firmware drift before upgrades.
- Frustration: No scriptable CLI; no anomaly detection over session history; utilization/dead-time computed by hand from paginated responses.

## Candidates (pre-cut)

(See survivors and kills below — 14 candidates generated across sources a/b/c; LLM-summary and cost-estimator killed inline for LLM-dependency and reimplementation.)

## Survivors (>= 5/10)

| # | Feature | Command | Score | Persona |
|---|---------|---------|-------|---------|
| 1 | Energy & cost rollup | `cost` | 10/10 | Ingrid, Bjørn |
| 2 | What's-charging-now snapshot | `live` (alias `fleet`) | 9/10 | Bjørn, Tobias |
| 3 | Load-balancing headroom | `current headroom` | 7/10 | Bjørn |
| 4 | Stale / offline charger watch | `chargers stale` | 7/10 | Bjørn, Tobias |
| 5 | Firmware drift | `firmware drift` | 6/10 | Tobias |
| 6 | Session anomaly scan | `sessions anomalies` | 6/10 | Tobias |

Plus: `state --watch` ships as a flag on the absorbed `state` command (not a standalone novel feature).

## Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `sessions summarize` (NL) | LLM dependency; pipe `--json \| claude` | cost rollup |
| `cost estimate <kWh>` | Reimplementation, no leverage | cost rollup |
| `utilization` | Sibling-killed: folds into cost/stale/live | cost / live / stale |
| `offpeak` | 0 research backing (no tariff data in spec) | cost rollup |
| `cost top` leaderboard | Subsumed by `cost --by charger` | cost rollup |
| `report export --csv` | Subsumed by absorbed report + `cost --json` | report |
| `state diff` | 4/10; needs snapshot-history infra no persona demanded | live |
| `state --watch` | Thin poll wrapper; ship as flag | absorbed state |
