# Tesla CLI Absorb Manifest

## Scope summary

- API: Tesla Owner API (`https://owner-api.teslamotors.com/api/1/`), bearer-token OAuth from auth.tesla.com.
- Spec source: HAR-derived spec (18 endpoints, 8 resources) merged with TeslaPy `endpoints.json` workflow methods and `teslamotors/vehicle-command` command list.
- Reachability: PASS. Matt's 2018 Model X (VIN suffix `JF139484`) predates the signed-command rollout, so REST commands route end-to-end. Newer-vehicle owners get a clear shim message pointing at `tesla-control`.
- Auth: `tesla auth login` ships the OAuth refresh-token flow (paste or capture); Keychain on darwin, file mode 600 elsewhere; auto-refresh on 401.

## Absorbed (match or beat every command in every existing tool)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Auth: login (paste refresh token) | tesla_auth, TeslaPy | `tesla auth login --paste` | --json, Keychain on darwin, file mode 600 fallback |
| 2 | Auth: status / logout / refresh | TeslaPy | `tesla auth status\|logout\|refresh` | Idempotent, --json |
| 3 | List vehicles + products | timdorr PRODUCT_LIST + TeslaPy.api('VEHICLE_LIST') | `tesla vehicles list` | --json default, offline cached, --table |
| 4 | Vehicle summary | TeslaPy.get_vehicle_summary | `tesla vehicles get <vin>` | Wake-aware, last-cached fallback |
| 5 | Full vehicle data | tesla-control state + TeslaPy.get_vehicle_data | `tesla state <vin> [--include charge,climate,drive,vehicle,gui,location,vehicle_config,closures,charge_schedule,preconditioning_schedule]` | Category filter, --json, persisted to store |
| 6 | Wake vehicle | tesla-control wake | `tesla wake <vin>` | --json, idempotent, --timeout |
| 7 | Lock / unlock | tesla-control lock/unlock | `tesla lock\|unlock <vin>` | --json, mutation logged, --dry-run |
| 8 | Honk + flash | tesla-control honk/flash-lights | `tesla honk\|flash <vin>` | --json, --dry-run, logged |
| 9 | Trunk + frunk | tesla-control trunk-*/frunk-open | `tesla trunk <vin> open\|close\|toggle` / `tesla frunk <vin> open` | --json |
| 10 | Charge port + windows | tesla-control charge-port-*/windows-vent/close | `tesla charge-port <vin> open\|close` / `tesla windows <vin> vent\|close` | --json |
| 11 | Climate on/off + set temp | tesla-control climate-on/off/set-temp | `tesla climate <vin> on\|off\|temp <c\|f>` | Unit detection, --json |
| 12 | Climate keeper modes (dog/camp/COP) | TeslaPy keeper docstrings | `tesla climate <vin> keeper <off\|on\|dog\|camp>` | One-flag mode |
| 13 | Seat + wheel heaters | tesla-control seat-heater/steering-wheel-heater | `tesla seat-heater <vin> <pos> <level>` / `tesla wheel-heater <vin> on\|off` | --json |
| 14 | Defrost max | TeslaPy MAX_DEFROST_MODE | `tesla climate <vin> defrost-max [on\|off]` | --json |
| 15 | Charge start/stop + limit + amps | tesla-control charging-start/stop/set-limit/set-amps | `tesla charge <vin> start\|stop\|limit <pct>\|amps <a>` | --json, --dry-run, logged |
| 16 | Charge schedule add/remove | tesla-control charging-schedule-*/* | `tesla charge <vin> schedule add\|remove\|list` | --json |
| 17 | Charging queue position | iOS HAR `/api/1/charging/queue` | `tesla charging queue <vin>` | Unique to this CLI vs all others |
| 18 | Precondition schedule add/remove | tesla-control precondition-schedule-* | `tesla precondition schedule add\|remove` | --json |
| 19 | Charge history (raw) | TeslaPy.get_charge_history_v2 | `tesla charge history --since` | --json, paginated |
| 20 | Nearby chargers | TeslaPy.get_nearby_charging_sites | `tesla chargers nearby [--lat --lng --radius]` | --json |
| 21 | Software update install/cancel + release notes | tesla-control software-update-start/cancel | `tesla update install [--in 2h]\|cancel\|release-notes` | --json |
| 22 | Sentry / valet / guest / speed-limit | tesla-control sentry-mode/valet-mode-*/guest-mode-*/SPEED_LIMIT | `tesla sentry <vin>` etc. | --json, mutation logged, --dry-run |
| 23 | Remote start drive | iOS HAR command/remote_start_drive | `tesla drive <vin>` | --json |
| 24 | Media commands | tesla-control media-* | `tesla media <vin> play\|pause\|next\|prev\|vol <0-10>\|up\|down\|next-fav\|prev-fav` | --json |
| 25 | Navigation: send address | TeslaPy SHARE_TO_VEHICLE | `tesla nav <vin> send "<address>"` | --json |
| 26 | Plan trip | TeslaPy PLAN_TRIP | `tesla nav <vin> plan-trip "<dest>" [--send]` | Charging-stop projection, --send-to-car |
| 27 | Compose vehicle image | TeslaPy.compose_image | `tesla compose <vin> --view front\|side\|rear\|side_rear --out file.png` | --json metadata + binary out |
| 28 | Keys list / add / remove / rename | tesla-control add-key-request/list-keys/remove-key/rename-key | `tesla keys list\|add\|remove\|rename <vin>` | --json table |
| 29 | Hermes telemetry JWT | iOS HAR `/api/1/vehicles/{id}/jwt/hermes` | `tesla hermes <vin>` | Mint + cache for streaming |
| 30 | Diagnostics flags | iOS HAR /diagnostics | `tesla diagnostics <vin>` | --json |
| 31 | Notification preferences | iOS HAR notification_preferences | `tesla notifications get\|set` | --json |
| 32 | App + feature config | iOS HAR users/app_config + feature_config | `tesla account config\|features` | --json |
| 33 | Tesla orders | iOS HAR users/orders | `tesla orders list` | --json |
| 34 | Products list | iOS HAR products | `tesla products` | Same as vehicles + energy in one call |
| 35 | Energy sites list / info / live | TeslaPy ProductList + get_site_info/data | `tesla energy sites` / `tesla energy site <id> info\|live` | --json |
| 36 | Energy tariff get/set | TeslaPy get_tariff/set_tariff | `tesla energy site <id> tariff get\|set` | --json |
| 37 | Energy backup reserve | TeslaPy set_backup_reserve_percent | `tesla energy site <id> reserve <pct>` | --json |
| 38 | Energy operation mode | TeslaPy set_operation | `tesla energy site <id> mode self\|backup\|autonomous` | --json |
| 39 | Sleep-friendly sync | teslamate convention | `tesla sync --no-wake` | Polls without waking the car |
| 40 | SQL / search / context | PP convention | `tesla sql\|search\|context` | FTS5 + raw SQL on the local store |
| 41 | Doctor | PP convention | `tesla doctor` | See transcendence row 8 - upgraded with signed-command detection |

Stub row:
- (S1) Service appointments + recall + service history endpoints (iOS HAR `/mobile-app/drive/appointments`, `/mobile-app/service/surveys`) — DEFERRED v0.2 because `ownership.tesla.com` is a second base URL that the v0.1 spec model can't carry cleanly. Status: `(stub - requires multi-host transport)`. Same bearer works against the host; only a per-resource `base_url_override` is missing.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|------------------------|
| 1 | Ready-to-drive composite | `tesla ready <vin>` | 9/10 | hand-code | Reads cached vehicle_states + last vehicle_data; evaluates SOC vs typical trip, plugged-in, doors closed, sentry off, cabin within 3F of target, no mid-install update; returns single JSON `{ready: bool, blockers: [...]}`. No competitor CLI composites these signals. |
| 2 | Charging-cost ledger | `tesla cost ledger [--since] [--group home\|supercharger]` | 9/10 | hand-code | Joins local `charges` + `tariffs` tables, with `cost_usd`/`$_per_kwh`/`tariff_window` columns populated by sync from CHARGING_HISTORY + CHARGING_DOWNLOAD_CSV. TeslaMate proves demand; nobody else delivers it without Postgres. |
| 3 | Cost counterfactual | `tesla cost what-if --only-home` | 7/10 | hand-code | Re-runs charges rows substituting home $/kWh for Supercharger sessions; emits delta. Only possible because the ledger is local. |
| 4 | Supercharger queue watch | `tesla supercharger watch <site_id> [--free-stalls N] [--watch]` | 7/10 | hand-code | Polls NEARBY_CHARGING_SITES stall-availability subfield; `--watch` repeats with JSON-lines transitions. Tesla shows in-car only; no third-party CLI/agent exposes it. |
| 5 | Drive-and-charge timeline | `tesla timeline --since "last week"` | 8/10 | hand-code | Stitches consecutive vehicle_states polls where shift_state/charging_state transitions; emits drives + charges JSON. TeslaMate's killer; we deliver it in pure-Go SQLite. |
| 6 | Vampire-drain monitor | `tesla vampire [--threshold 1.5pct/24h]` | 6/10 | hand-code | SOC delta over time windows from vehicle_states where shift_state=null and charging=disconnected. Warranty-dispute + rogue-sentry detection. |
| 7 | Keys audit | `tesla keys audit` | 6/10 | hand-code | Joins keys_enrolled with commands_log to derive last-seen per key; flags >90d as stale. Persona-4 quarterly anxiety; no competitor. |
| 8 | Reachability doctor | `tesla doctor` | 6/10 | hand-code | Pings vehicle_data + command/honk; classifies REST-OK / signed-command-required / token-expired; prints `tesla-control` enrollment URL with one-line shim when signed-required. No community CLI explains the signed-command landmine. |

## Build priorities

- Priority 0 (foundation): auth flow, vehicles/products list, full vehicle_data, sync to local store, doctor command, SQLite schema (vehicles, vehicle_states, drives, charges, supercharger_sessions, software_updates, commands_log, tariffs, keys_enrolled, tokens).
- Priority 1 (absorb): all 41 rows above. Generator emits ~30 of them from the spec; ~10 are hand-coded leaf commands wired to existing client methods.
- Priority 2 (transcendence): all 8 hand-code novel features above.

## Notes for the user at Phase Gate 1.5

- Spec source is sniffed (not catalog) — Tesla owner-api is undocumented officially.
- Reachability is great for this user (2018 Model X). Plan B for signed-command users is documented in `tesla doctor` and the README.
- One stub deferred (service appointments host).
- 8 novel features will require hand-coded Go after generate (~50-150 LoC each plus `root.go` wiring).
- Auth onboarding requires a one-time paste of a Tesla refresh token; planned UX is `tesla auth login --paste` reading from stdin, storing to Keychain on darwin.
