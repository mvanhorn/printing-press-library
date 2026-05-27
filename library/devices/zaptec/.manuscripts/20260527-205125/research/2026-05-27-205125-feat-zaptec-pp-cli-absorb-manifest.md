# Zaptec CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

The only incumbents are the Home Assistant integration (`custom-components/zaptec`, polling-based, needs a running HA) and evcc's read-only Zaptec driver. No dedicated CLI or MCP server exists. We match the HA integration's full control + telemetry surface and the API's full endpoint surface, with `--json`, `--select`, typed exit codes, `--dry-run`, offline SQLite, and baked command/observation decoding.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List chargers | API `/api/chargers` + HA | `chargers list` | offline store, --json/--select, search |
| 2 | Charger details | API `/api/chargers/{id}` | `chargers get` | --select |
| 3 | Charger live state (decoded) | API `/api/chargers/{id}/state` + HA sensors | `state <id>` | decodes numeric observation IDs → plain English; `--watch` poll flag |
| 4 | Pause charging | sendCommand StopCharging(502)/StopChargingFinal(506) + HA | `pause <id>` | --dry-run, typed exit |
| 5 | Resume charging | sendCommand ResumeCharging(507) + HA | `resume <id>` | --dry-run |
| 6 | Start charging | sendCommand StartCharging(501) | `start <id>` | --dry-run |
| 7 | Stop charging | sendCommand StopCharging(502) | `stop <id>` | --dry-run |
| 8 | Restart charger | sendCommand RestartCharger(102) | `restart <id>` | --dry-run |
| 9 | Unlock connector | sendCommand UnlockConnector(708) | `unlock <id>` | --dry-run |
| 10 | Authorize / Deauthorize | HA buttons, DeauthorizeAndStop | `authorize`/`deauthorize <id>` | --dry-run |
| 11 | Update charger settings | API `/api/chargers/{id}/update` | `chargers update` | --dry-run, named flags |
| 12 | List installations | API `/api/installation` | `installations list` | offline |
| 13 | Installation details | API `/api/installation/{id}` | `installations get` | --select |
| 14 | Installation hierarchy | API `/api/installation/{id}/hierarchy` | `installations hierarchy` | tree view |
| 15 | Available-current / load balance | API `/api/installation/{id}/update` + HA available-current | `current set <inst> --amps N` | **15-min re-send guard** warns before tripping the API constraint |
| 16 | Charge history | API `/api/chargehistory` | `history` + `sync` to SQLite | offline analytics base |
| 17 | Installation usage report | API `/api/chargehistory/installationreport` | `report <inst>` | --json/--csv |
| 18 | Session details | API `/api/session/{id}` | `sessions get <id>` | |
| 19 | Session priority | API `/api/session/{id}/priority` | `sessions priority <id>` | --dry-run |
| 20 | Firmware status (fleet) | API `/api/chargerFirmware/installation/{id}` | `firmware list <inst>` | fleet view |
| 21 | Upgrade firmware | sendCommand UpgradeFirmware(200) | `firmware upgrade <id>` | --dry-run |
| 22 | Constants decode | API `/api/constants` (public) | baked offline cache + `constants` helper | command/observation/setting name↔id lookup |
| 23 | Sync | (foundation) | `sync` chargers/installations/sessions → SQLite | enables all transcendence |
| 24 | Search / SQL | (foundation) | `search`, `sql` over local store | FTS over charger/installation/session |
| 25 | Doctor | (foundation) | `doctor` — token validity, API reachability, store health | uses public `/api/constants` |
| 26 | Auth | OAuth2 password grant | `auth login/status/logout` | env `ZAPTEC_USERNAME`/`ZAPTEC_PASSWORD`; token cache + refresh |

Every absorbed row ships fully (no stubs). Commands 4-10, 21 are hand-built wrappers over `POST /api/chargers/{id}/sendCommand/{commandId}` using baked command IDs; the rest are generator-emitted from the spec.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|--------------------------|---------|
| 1 | Energy & cost rollup | `cost [--by month\|charger] [--from --to] [--json]` | 10/10 | Aggregates the local `charge_sessions` table into per-period/per-charger kWh + cost + duration totals. No single Zaptec API call returns a monthly total; the portal shows only session-by-session. | Ingrid, Bjørn |
| 2 | What's-charging-now snapshot | `live` (alias `fleet`) | 9/10 | Joins synced `chargers` with a one-shot live state fetch, decodes operation mode + power/current/phase via baked constants into one table. No portal or HA single-view equivalent. | Bjørn, Tobias |
| 3 | Load-balancing headroom | `current headroom <installation>` | 7/10 | Sums decoded active per-charger current draw and subtracts from the installation max available-current → uncommitted amps. Operators currently tune the 15-min-guarded limit blind. | Bjørn |
| 4 | Stale / offline charger watch | `chargers stale [--minutes N]` | 7/10 | Pure local query over synced charger/state rows where operation mode is Disconnected/Unknown or last update exceeds a threshold. HA is polling-only with no alerting. | Bjørn, Tobias |
| 5 | Firmware drift | `firmware drift <installation>` | 6/10 | Groups the firmware list by version and flags chargers behind the fleet's modal version before scheduling upgrades. Turns a raw version list into an upgrade decision. | Tobias |
| 6 | Session anomaly scan | `sessions anomalies [--since]` | 6/10 | Threshold SQL over local `charge_sessions` flags near-zero-energy, abnormally long, or zero-cost sessions. No incumbent offers session QA. | Tobias |

No stubs. All six are buildable with the synced SQLite store + baked constants + same OAuth2 auth — no external services, no LLM dependency.

## Build scope summary
- **Priority 0 (foundation):** auth (OAuth2 password grant + token cache), SQLite store for chargers/installations/charge_sessions/firmware + baked constants, sync/search/sql/doctor.
- **Priority 1 (absorbed):** 22 features above (incl. 7 sendCommand control wrappers).
- **Priority 2 (transcend):** 6 novel features above.
