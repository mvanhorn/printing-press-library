# Hayward OmniLogic — Absorb Manifest

The GOAT CLI matches every feature any competing tool exposes for the OmniLogic partner API, then transcends with a local store + compound-call orchestration nothing else has.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Command | Added Value |
|---|---|---|---|---|
| 1 | Authenticate (email + password → token + refresh) | omnilogic-api 0.5+ (post-Oct-2025 v2 flow) | `auth login` / cached on first run | Token cached in XDG state, auto-refresh on expiry; no re-login per call |
| 2 | List user's sites (`GetSiteList`) | omnilogic-api | `sites list` | `--json`, FTS search, persisted in `sites` table for offline cross-reference |
| 3 | Fetch equipment inventory (`GetMspConfigFile`) | omnilogic-api | `config get` | Parsed into typed equipment rows; versioned snapshots in `msp_config_snapshots` so schedule changes are diffable |
| 4 | List active alarms (`GetAlarmList`) | omnilogic-api | `alarms list` | Persisted as event log with first_seen/last_seen/cleared_at; cloud only returns "current set" |
| 5 | Current telemetry snapshot (`GetTelemetryData`) | omnilogic-api | `telemetry get` | Joins MSP config + alarms so every reading carries its equipment name/type; appended to `telemetry_samples` time-series |
| 6 | Heater on/off (`SetHeaterEnable`) | omnilogic-api | `heater set --enable/--disable` | `--dry-run`, idempotent retries, structured exit codes; logged to `command_log` |
| 7 | Heater setpoint (`SetUIHeaterCmd`) | omnilogic-api | `heater set-temp <F>` | `--dry-run`, range validation against per-heater min/max from MSP config, audit trail |
| 8 | Pump speed (`SetUIEquipmentCmd` for pumps) | omnilogic-api | `pump set-speed <rpm-or-pct>` | Range validation from pump's MinSpeed/MaxSpeed in MSP config; `--dry-run` |
| 9 | Generic equipment on/off + timed run (`SetUIEquipmentCmd`) | omnilogic-api | `equipment set --on/--off [--for <duration>]` | Works for valves, relays, lights, VSPs; `--for 1h` translates to start/end window automatically |
| 10 | Spillover speed (`SetUISpilloverCmd`) | omnilogic-api | `spillover set --speed <pct>` | `--dry-run`, audit trail |
| 11 | Superchlorination (`SetUISuperCHLORCmd`) | omnilogic-api | `superchlor on/off` | `--dry-run`, audit trail, automatic ChlorID resolution from MSP config |
| 12 | ColorLogic light show v1 (`SetStandAloneLightShow`) | omnilogic-api | `light show <show-id>` | Show-name lookup table (no need to know numeric IDs); `--dry-run` |
| 13 | ColorLogic light show v2 (`SetStandAloneLightShowV2` with speed + brightness) | omnilogic-api | `light show <show-id> [--speed N] [--brightness N]` | Auto-detect V2-Active from MSP config; falls back to V1 if not |
| 14 | Chlorinator config (`SetCHLORParams`) | omnilogic-api | `chlorinator set-params --op-mode <m> [--timed-pct N] [--cell-type <t>]` | Reads current config as defaults so partial updates don't reset everything; honors Hayward's `ORPTimout` typo silently |
| 15 | Token refresh (`/auth-service/v2/refresh`) | omnilogic-api | (transparent — runs inside the client) | No user-facing command needed; falls back to fresh login if refresh fails |
| 16 | Per-equipment sensor surface (pump-speed-%, chlor-output-%, salt level, pH, ORP, heater-enabled bool) | HA core omnilogic integration | `telemetry get --json` exposes all these fields; `chemistry get` short-cuts to pH/ORP/salt/temp | Structured JSON with explicit field names; `--select` to pluck just what you need |
| 17 | Heater control surface (missing in HA core, present in HACS fork) | djtimca/haomnilogic HACS | `heater set --enable/--disable`, `heater set-temp` | Always exposed; no HACS dependency |
| 18 | ColorLogic show control (missing in HA core) | djtimca/haomnilogic HACS, openHAB binding | `light show <id> [--speed N] [--brightness N]` | Show-name lookup table beats numeric IDs |
| 19 | Auto-discovery from cloud creds (no manual config) | openHAB haywardomnilogic binding | `sync --full` walks sites → BoWs → equipment automatically | Local store seeded on first run; subsequent calls are offline-fast |
| 20 | Health check (auth + reachability + site list) | n/a (CLI table-stakes) | `doctor` | Standard PP-CLI doctor, plus OmniLogic-specific: token expiry, last successful telemetry, store freshness |
| 21 | Offline search across alarms / equipment / commands | n/a (no wrapper has this) | `search <term>` | FTS5 over alarms, equipment, command_log; the cloud has no search endpoint |
| 22 | SQL inspection of the local store | n/a (no wrapper has this) | `sql 'SELECT ...'` | Read-only SQL over the SQLite store for ad-hoc queries |

**Coverage:** 22 absorbed rows cover every operation exposed by `djtimca/omnilogic-api` 0.5+, every sensor in HA's core integration, every control added by `djtimca/haomnilogic`'s HACS fork, the auto-discovery pattern from openHAB's binding, and the table-stakes operator commands (doctor, search, sql) that no existing wrapper has. No row will ship as a stub.

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| 1 | Pool readiness composite | `status` | Compounds telemetry + alarms + MSP config + setpoints in one call; the app makes you tap through 4 screens to assemble the same answer | 9 |
| 2 | Historical chemistry export | `chemistry log --since <when> [--csv]` | Cloud returns "now" only; users today roll their own InfluxDB to get a weekly pH/ORP/salt/temp log | 9 |
| 3 | Chemistry drift + forecast | `chemistry drift [--forecast]` | Requires append-only time-series + slope math; cloud and app only fire when Hayward's static thresholds are crossed | 8 |
| 4 | Equipment "why isn't this on?" | `why-not-running <equipment>` | Agent-shaped diagnostic joining current alarms, relay state, schedule windows, heater demand, and superchlor lockouts — none of which any wrapper packages | 9 |
| 5 | Schedule change detection | `schedule diff [--since <when>]` | Needs versioned `msp_config_snapshots`; the cloud has no schedule-history endpoint; catches silent service-tech edits | 8 |
| 6 | Multi-site morning sweep | `sweep [--alarms] [--chemistry] [--offline]` | Pool-service businesses today open the app per-site; only a CLI with the stored site list + parallel calls collapses this into one report | 9 |
| 7 | Ready-by (preheat with ETA) | `ready-by <time> [--temp <F>]` | Heat-rate learning (°F/hr per body of water) requires telemetry history; the app has no ETA math and users today guess | 8 |
| 8 | Command audit + replay | `command log [--replay <id>]` | Local audit trail of every Set* issued; `--replay` re-issues a prior command. The cloud doesn't expose user command history. | 7 |
| 9 | Equipment runtime totals | `runtime [--equipment <id>] [--since <when>]` | Pump hours, heater hours, SCG cell hours from telemetry deltas — for maintenance planning and warranty. Not an API field; only the store can compute it. | 7 |

**Group clustering (for README "Unique Features"):**
- **Pool readiness at a glance** — `status`, `ready-by`
- **Local state that compounds** — `chemistry log`, `chemistry drift`, `runtime`, `command log`
- **Diagnostics the cloud can't do** — `why-not-running`, `schedule diff`
- **Multi-site operations** — `sweep`

## Killed candidates (audit trail)
- **`preheat` standalone** — folded into `ready-by`; preheat-alone was a subset.
- **`chemistry forecast` standalone** — folded into `chemistry drift --forecast`; same time-series backbone.
- **`equipment health`** (pump-RPM-vs-flow, heater short-cycling, SCG cell aging detection) — needs years of labeled data, high false-positive risk would erode trust on day one. Maybe v2.

## Stubs
**None.** Every row above is shipping scope. The OmniLogic API is well-mapped, our auth context is in place, and there are no paid tiers or external setups blocking any of these. If implementation hits unexpected scope mid-Phase-3, the contract is to return to this manifest and re-approve a revised scope — not silently downgrade.

## Generation strategy note
The Printing Press generator targets REST+JSON APIs. OmniLogic is XML-RPC-over-HTTP with two-stage auth. The generator will be used for **scaffolding only** (Cobra tree, SQLite store, MCP server, agent_context, doctor, version, README/SKILL templates, helpers). The HTTP client, XML envelope construction, XML response parsing, the operation set, and every novel feature will be hand-built in Phase 3 — the documented pattern for non-REST APIs (per the skill's GraphQL-only guidance). Phase 3 will be larger than usual; expect the long end of the 30-60 minute window.
