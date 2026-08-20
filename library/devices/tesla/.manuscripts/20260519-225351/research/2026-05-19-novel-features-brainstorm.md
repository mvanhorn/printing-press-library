## Customer model

**Persona 1: Matt - Daily Driver, 2018 Model X owner.** Today, he opens the Tesla iOS app 4-6 times a day: morning "is it warm yet?", lunchtime "is it still locked?", evening "what was the cost of that supercharge?", late-night "did I leave the windows cracked?". Each answer is buried 3-6 taps deep. Weekly ritual: he preconditions before a 7:45am school drop, supercharges at the same two stalls on the Bellevue-Seattle corridor, runs sentry overnight. Frustration: the app doesn't show cost per session, doesn't tell him whether his cabin is already at temp before he pulls on his coat, and re-asks login every couple of weeks. He wants a one-liner that says "yes you can leave" and a ledger he can pipe to Claude for monthly reconciliation.

**Persona 2: Charging-Cost Skeptic.** Today, he suspects Supercharging is eating his EV savings but has no proof - the app shows kWh per session and a dollar total, but never aggregates, never breaks home-vs-Supercharger ratio, and never models "what if I had only charged at home." Weekly ritual: screenshots the in-car charging tab, drops it into Notes, eyeballs the math at month-end. Frustration: he installed TeslaMate once, hit the Postgres+Grafana wall, gave up. He wants the cost ledger without standing up Docker.

**Persona 3: Agent Operator (the user delegating to Claude).** Today, he wants to ask Claude things like "did Matt drive yesterday?", "is the car ready for my 6pm dinner?", "send the restaurant address to the car." Weekly ritual: chat-driven car interactions via MCP. Frustration: scald/tesla-mcp ships three tools, tesla.async.fyi is closed-source. He wants every command as an MCP tool and the local store queryable by SQL.

**Persona 4: Security-Minded Owner.** Today, he opens Tesla app -> Security -> Locks to scan enrolled keys quarterly. Five phones and three NFC cards over the lifetime of the car, no audit trail of when each was added. Weekly ritual: none - it's quarterly anxiety. Frustration: no way to know which key was last used, when an unknown key appeared, or to bulk-remove stale ones. He wants a `keys audit` command that flags anomalies and writes the audit to disk.

## Candidates (pre-cut)

1. `ready` - `tesla ready <vin>` - composite "can I leave in 5 min?" with reasoned breakdown. Persona 1. Source (e), (c).
2. `cost ledger` - per-session cost, monthly spend, home-vs-Supercharger ratio. Persona 2. Source (e), (c).
3. `cost counterfactual` - "if you only charged at home you would have saved $X." Persona 2. Source (e), (c).
4. `supercharger watch` - free-stall snapshot/poll with JSON-lines transitions. Persona 1/3. Source (a), (b).
5. `supercharger route-score` - score Superchargers along a planned trip. Persona 1. Source (b), (e). KILL: speculative historical-wait data.
6. `timeline` - stitched drives + charges from vehicle_states polls. Persona 2/3. Source (b), (c).
7. `vampire` - SOC delta over idle time, flags suspicious wakes. Persona 4/2. Source (b), (c).
8. `keys audit` - enrolled keys with last-seen + stale flags. Persona 4. Source (a), (b).
9. `fleet` - multi-vehicle aggregate table. Persona 1/3. Source (c). KILL: single-vehicle today.
10. `precondition cron` - launchd plist for scheduled departure. Persona 1. Source (e), (a). KILL: scope creep.
11. `plan-trip` wrapper - charging stops + ETA + cost estimate + push to car. Persona 1/3. Source (b), (e). KILL: thin wrapper over absorbed PLAN_TRIP.
12. `dashcam save-with-note` - DASHCAM_SAVE_CLIP + freeform note logged. Persona 1/4. Source (a). KILL: low weekly use.
13. `charge digest --month` - per-month rollup. Persona 2. Source (c). KILL: subset of cost ledger.
14. `location history` - clustered locations. Persona 2/4. Source (c). KILL: needs reverse-geocoding.
15. `MCP tool surface` - mandatory floor per PP convention; not novel.
16. `reachability doctor` - signed-command-required detection + shim URL. Persona 1/3/4. Source (e).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Ready-to-drive composite | `tesla ready <vin>` | 9/10 | hand-code | Reads cached vehicle_states row + last VEHICLE_DATA call; evaluates SOC/plug/locks/sentry/climate/update predicates locally; returns single JSON {ready, blockers:[...]} | Brief Top Workflows #1; no competitor CLI composites these signals |
| 2 | Charging-cost ledger | `tesla cost ledger [--since] [--group home|supercharger]` | 9/10 | hand-code | Joins local `charges` + `tariffs` tables; reads cost_usd/$_per_kwh/tariff_window columns populated by sync from CHARGING_HISTORY + CHARGING_DOWNLOAD_CSV | Matt's #1 stated priority; TeslaMate proves demand, cobanov/teslamate-mcp proves agent demand, neither runs without Postgres |
| 3 | Cost counterfactual | `tesla cost what-if --only-home` | 7/10 | hand-code | Re-runs charges rows substituting home $/kWh for Supercharger sessions; emits delta | Brief product thesis; no competitor offers |
| 4 | Supercharger watch | `tesla supercharger watch <site_id> [--free-stalls N] [--watch]` | 7/10 | hand-code | Single poll of NEARBY_CHARGING_SITES stall-availability subfield; --watch repeats with JSON-lines transitions | Brief workflow #5; Tesla shows in-car only; Matt's stated supercharger priority |
| 5 | Drive-and-charge timeline | `tesla timeline --since "last week"` | 8/10 | hand-code | Stitches consecutive vehicle_states rows where shift_state/charging_state transitions; emits drives + charges JSON | TeslaMate's killer feature; cobanov gates it on Postgres |
| 6 | Vampire-drain monitor | `tesla vampire [--threshold 1.5pct/24h]` | 6/10 | hand-code | SOC delta over time windows from vehicle_states where shift_state=null/charging=disconnected | TeslaMate dashboard; warranty-dispute use case |
| 7 | Keys audit | `tesla keys audit` | 6/10 | hand-code | Joins keys_enrolled with commands_log for last-seen per key; flags >90d as stale | Brief workflow #11; persona-4 anxiety |
| 8 | Reachability doctor | `tesla doctor` | 6/10 | hand-code | Pings vehicle_data + command/honk; classifies REST-OK / signed-required / token-expired; prints shim URL | Brief Reachability Risk: signed-command rollout is the #1 user-facing landmine; no community CLI explains it |

### Killed candidates

| Candidate | Kill reason | Closest surviving sibling |
|-----------|-------------|---------------------------|
| Precondition cron | Scope creep, OS-coupled (launchd/systemd) | `ready` covers "is the cabin warm" |
| Supercharger route-score | Speculative historical-wait data, unverifiable | `supercharger watch` ships verifiable subset |
| Fleet aggregate | Single-vehicle account today; weekly-use=0 | `tesla vehicles list --table` (P1 absorbed) |
| Plan-trip wrapper | Mostly wrapper over absorbed PLAN_TRIP | `nav send` (absorbed) |
| Dashcam save-with-note | Low weekly frequency; verifiability low | None - clean kill |
| Location history geocoded | External reverse-geocoding not in brief | `timeline` includes start/end lat/lng |
| Charge digest --month | Subset of cost ledger | `cost ledger --group month` |
| MCP tool surface | Floor feature per PP convention | Every survivor exposed via MCP automatically |
