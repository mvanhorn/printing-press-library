# Tesla CLI Brief

Research date: 2026-05-19. Single-vehicle owner-side CLI build target.

## API Identity

Tesla operates two parallel HTTP surfaces:

- **Owner API** (legacy, reverse-engineered): `https://owner-api.teslamotors.com/api/1/`. Used by the iOS/Android Tesla app. Bearer-token, OAuth2 from `auth.tesla.com`, 8h access-token TTL, refresh tokens long-lived. Scopes: `email offline_access openid phone`. The HAR Matt captured is this surface.
- **Fleet API** (official, partner-registered): same base path, but requires partner registration at developer.tesla.com, public-key enrollment, and the [vehicle-command](https://github.com/teslamotors/vehicle-command) signed-message proxy for most vehicle commands on Models 3/Y built after late 2021 and all Models S/X / Cybertruck refresh forward.

Personal owner accounts can still drive the Owner API endpoints with an app-extracted refresh token. Commands route to vehicle: REST -> Tesla cloud -> vehicle. Telemetry routes via JWT/hermes WebSocket.

## Reachability Risk

- **2023-10-09**: Tesla deprecated many Owner-API command endpoints. Honking/flashing/locking/charging-commands on signed-command-enabled vehicles now return errors when sent through plain REST. ([scald/tesla-mcp](https://github.com/scald/tesla-mcp) README, [TeslaPy](https://github.com/tdorssers/TeslaPy) limitations).
- **2024-2026 trajectory**: TeslaPy README states "the Owner API will stop working as vehicles begin requiring end-to-end command authentication using the Tesla Vehicle Command Protocol." Pre-2021 S/X are the only vehicles guaranteed to still accept REST commands.
- **Matt's Model X** (VIN 5YJ3E1EA6XXXXXXXX, build year `J=2018` per the 10th VIN char) is a **2018 Model X**. That predates the signed-command rollout. REST commands SHOULD still work end-to-end for this vehicle. This is a meaningful reachability win: the CLI can ship without bundling the [vehicle-command](https://github.com/teslamotors/vehicle-command) Go proxy and still drive Matt's car. Plan for a graceful "signed-command required" detection branch so anyone with a newer vehicle gets a clear shim message.
- Polling `vehicle_data` aggressively wakes the car and drains the 12V. Use the `online`/`asleep` summary + cached state. Fleet Telemetry is the modern stream alternative but requires partner registration.

## Auth Landscape

- **Owner API**: get a refresh token from a real Tesla iOS/Android login (Matt already logged into tesla.com, and the iOS HAR contains the bearer). Exchange refresh -> 8h access token via `https://auth.tesla.com/oauth2/v3/token`.
- **Fleet API**: requires partner application registration, public-key upload to `https://tesla.com/_ak/<domain>`, and the `tesla-control` proxy or [vehicle-command](https://github.com/teslamotors/vehicle-command) Go library for signing.
- **BLE direct** (no internet, no Tesla cloud): [vehicle-command](https://github.com/teslamotors/vehicle-command) supports BLE pairing + signed commands. Useful when the vehicle is asleep at home and you want sub-second latency.
- **For this CLI**: ship the Owner API refresh-token flow as primary, with the option to paste a token directly. Document Fleet API enrollment for future-proofing but do not require it on day one.

## Top Workflows

Matt's stated priorities and the obvious owner workflows from app HAR:

1. "Is the car ready to drive?" - SOC, plugged-in, climate state, range, software update pending, sentry status, location.
2. Precondition the cabin before leaving. Today's iOS app needs ~6 taps; CLI needs 1.
3. Set/inspect charge limit, start/stop charge, set amps.
4. Lock/unlock, honk, flash, vent windows, open frunk/trunk/charge-port.
5. Charging cost analytics: $/kWh across home + supercharger sessions, monthly spend, energy-added per session, supercharger queue/wait at favorite stalls.
6. Drives + efficiency over time (TeslaMate territory but with a CLI grain).
7. Multi-vehicle aggregate (Matt has one car today but family-shared accounts are common).
8. Send navigation destination from terminal/Slack to the car.
9. Schedule departure / scheduled charging for off-peak rates.
10. Service appointment + recall + software update visibility.
11. Key rotation: list enrolled keys, add/remove a phone or NFC card.

## Table Stakes (competitor feature inventory)

### timdorr/tesla-api

[github.com/timdorr/tesla-api](https://github.com/timdorr/tesla-api), 2.1k stars. Ruby gem plus the canonical reverse-engineered docs site at `tesla-api.timdorr.com`. Endpoint groups: vehicle commands, vehicle state, energy sites/products, charging, climate, media, valet/security, scheduled features, calendar/navigation, OAuth. The docs site is the lingua franca every other wrapper references. No native signed-command support; treats Owner API as canonical.

### tdorssers/TeslaPy

[github.com/tdorssers/TeslaPy](https://github.com/tdorssers/TeslaPy), 416 stars. Most-maintained Python wrapper. Vehicle class methods:

- `stream`, `api`, `get_vehicle_summary`, `available`, `sync_wake_up`
- `decode_option`, `option_code_list`, `decode_vin`
- `get_vehicle_data`, `get_vehicle_location_data`, `get_nearby_charging_sites`
- `get_service_scheduling_data`, `get_charge_history`, `get_charge_history_v2`
- `mobile_enabled`, `compose_image`, `last_seen`, `dist_units`, `temp_units`, `gui_time`
- `command()` wrapper that maps to every Owner-API command endpoint in `endpoints.json`

Product / Battery class: `get_site_info`, `get_site_data`, `get_calendar_history_data`, `get_history_data`, `set_operation`, `set_backup_reserve_percent`, `set_import_export`, `get_tariff`, `set_tariff`, `create_tariff`.

**Total endpoints in `endpoints.json`: ~1156** spanning vehicle, charging, climate, media, energy, navigation, valet, user, product, service, financing, commerce, referral, upgrades, subscriptions, safety-rating, hermes telemetry. Docstrings do not flag signed-command requirement; README does.

### teslamotors/vehicle-command

[github.com/teslamotors/vehicle-command](https://github.com/teslamotors/vehicle-command), 650 stars. **Official Tesla SDK.** CLI binary is `tesla-control`. Full command list from [cmd/tesla-control/commands.go](https://github.com/teslamotors/vehicle-command/blob/main/cmd/tesla-control):

`valet-mode-on`, `valet-mode-off`, `unlock`, `lock`, `drive`, `climate-on`, `climate-off`, `climate-set-temp`, `add-key`, `add-key-request`, `remove-key`, `rename-key`, `list-keys`, `get`, `post`, `honk`, `ping`, `flash-lights`, `keep-accessory-power`, `low-power-mode`, `charging-set-limit`, `charging-set-amps`, `charging-start`, `charging-stop`, `charging-schedule`, `charging-schedule-cancel`, `charging-schedule-add`, `charging-schedule-remove`, `media-set-volume`, `media-volume-up`, `media-volume-down`, `media-next-favorite`, `media-next-track`, `media-previous-track`, `media-previous-favorite`, `media-toggle-playback`, `software-update-start`, `software-update-cancel`, `sentry-mode`, `wake`, `tonneau-open`, `tonneau-close`, `tonneau-stop`, `trunk-open`, `trunk-move`, `trunk-close`, `frunk-open`, `charge-port-open`, `charge-port-close`, `autosecure-modelx`, `session-info`, `seat-heater`, `steering-wheel-heater`, `product-info`, `auto-seat-and-climate`, `windows-vent`, `windows-close`, `body-controller-state`, `guest-mode-on`, `guest-mode-off`, `erase-guest-data`, `precondition-schedule-add`, `precondition-schedule-remove`, `state`.

**62 commands.** Auth: OAuth bearer + ECDSA private key (private_key.pem) enrolled in vehicle keychain. Signed-command: yes, required. BLE transport available. Notable Cybertruck-only: `tonneau-*`. Model-X-only: `autosecure-modelx`.

### jonasman/TeslaSwift

[github.com/jonasman/TeslaSwift](https://github.com/jonasman/TeslaSwift), 255 stars. Swift wrapper, both Owner API and Fleet API. Public surface is thinner than TeslaPy:

- Auth: `authenticateWebNativeURL`, `authenticateWebNative`, `authenticateWeb`, `reuse`, `getPartnerToken`, `registerApp`, `urlToSendPublicKeyToVehicle`
- Vehicles: `getVehicles`
- Streaming: `openStream`, `closeStream`
- Combine/async extensions

Per-command methods live in extension files keyed off the timdorr endpoint names; the README references full Owner-API command coverage. Signed-command: no native impl (relies on Tesla's Fleet API proxy when used against signed vehicles).

### teslamate (derived metrics)

[github.com/teslamate-org/teslamate](https://github.com/teslamate-org/teslamate), 8k stars. Elixir + Postgres + Grafana telemetry stack (not a CLI - inspiration for derived analytics). 19+ dashboards:

- Drives: distance, energy-net, energy-gross, efficiency, drive details, drive stats
- Charges: energy-added, energy-used, charge-level, charging stats, charge details, cost per kWh
- Battery: health, projected range, vampire drain, range degradation
- Software: update history
- Location: address resolution, geofences, lifetime-visited map
- Operational: online/asleep timeline, total mileage, statistics overview
- Multi-vehicle per account
- Sleep-friendly polling (does not wake car)
- MQTT publishing for Home Assistant/Node-Red/Telegram

### Tesla MCP servers

- [scald/tesla-mcp](https://github.com/scald/tesla-mcp), 14 stars. Three tools: `wake_up`, `refresh_vehicles`, `debug_vehicles`. REST-only, deprecated commands blocked. Tiny.
- [cobanov/teslamate-mcp](https://github.com/cobanov/teslamate-mcp), 127 stars. 18 query tools over a TeslaMate Postgres database: `get_basic_car_information`, `get_current_car_status`, `get_software_update_history`, `get_battery_health_summary`, `get_battery_degradation_over_time`, `get_daily_battery_usage_patterns`, `get_tire_pressure_weekly_trends`, `get_monthly_driving_summary`, `get_daily_driving_patterns`, `get_longest_drives_by_distance`, `get_total_distance_and_efficiency`, `get_drive_summary_per_day`, `get_efficiency_by_month_and_temperature`, `get_average_efficiency_by_temperature`, `get_unusual_power_consumption`, `get_charging_by_location`, `get_all_charging_sessions_summary`, `get_most_visited_locations` + `get_database_schema` + `run_sql`. Read-only, presupposes a running TeslaMate.
- [tesla.async.fyi](https://tesla.async.fyi/) - hosted MCP, no public source.

### Other CLIs found

- [barnybug/tesla-cli](https://github.com/barnybug/tesla-cli), 5 stars. Go. Commands: `login`, `vehicles`, `state`, `charge`. Username+password auth. No signed-command. Skeletal.
- [gak/teslatte](https://github.com/gak/teslatte), 7 stars. Rust. Commands: `vehicles`, `vehicle <id> charge-start|charge-stop|set-charge-limit|set-charging-amps|charge-standard|charge-max-range|charge-port-door-open|charge-port-door-close|set-scheduled-charging|set-scheduled-departure|honk-horn|flash-lights|vehicle-data`, `energy-sites`, `energy-site`, `powerwall`. No signed-command (roadmap item).
- [teslajs/tesla-cli](https://github.com/teslajs/tesla-cli). Node.js wrapper over teslajs library. Not actively maintained.
- [jarijokinen/tesla-cli-ruby](https://github.com/jarijokinen/tesla-cli-ruby). Ruby wrapper over timdorr's gem.
- [aditprab/tesla-cl-tools](https://github.com/aditprab/tesla-cl-tools). Tiny.

None of the community CLIs ship signed-command support today. `tesla-control` is the only one that does, and it lacks all the high-level workflow ergonomics (no analytics, no JSON-by-default, no local store, no "ready-to-drive" composite).

## Data Layer

Local SQLite under `~/.tesla-pp/` for:

- `vehicles` (vin, name, model, year_decoded, last_summary_json, last_seen_at)
- `vehicle_states` (vin, captured_at, raw_json, soc, range_mi, inside_temp_c, outside_temp_c, locked, sentry, online_state, charging_state, charge_limit_soc, charge_amps, est_battery_range_mi, plugged_in_bool, latitude, longitude, odometer)
- `drives` (vin, started_at, ended_at, start_lat, start_lng, end_lat, end_lng, distance_mi, energy_used_kwh, efficiency_wh_per_mi, start_soc, end_soc) - built by tailing `vehicle_data` and stitching consecutive online sessions where shift_state moves
- `charges` (vin, started_at, ended_at, start_soc, end_soc, energy_added_kwh, location_lat, location_lng, location_name, fast_charger_type, charger_phases, charger_voltage, charger_actual_current, max_charger_power, cost_usd, $/kWh, tariff_window) - stitched from charge_state polls
- `supercharger_sessions` (subset of `charges` where fast_charger_type=Tesla); join against `nearby_charging_sites` snapshots
- `software_updates` (vin, version, install_started_at, install_completed_at)
- `commands_log` (vin, ts, command, args_json, latency_ms, http_status, error) - every send for audit + retry
- `tariffs` (location_or_label, time_window, $/kWh, source) - user-configured for home charging cost
- `keys_enrolled` (vin, pubkey_hash, role, form_factor, added_at, last_seen)
- `agent_runs` (run_id, command, started_at, args_json) - PP convention

Plus a tiny `tokens` table for refresh + access token + expiry. Token storage in macOS Keychain when available, fallback to file mode 600.

## Codebase Intelligence (DeepWiki findings)

Not consulted; deferred. Key signals already captured from primary sources above: vehicle-command is the official forward path; TeslaPy is the de-facto Python reference and its `endpoints.json` is the closest thing to a canonical Owner-API catalog; teslamate has cracked derived-metric ergonomics; cobanov/teslamate-mcp proves there is appetite for an AI-native query surface but it is gated on running TeslaMate first.

## User Vision

The user (Matt) is the owner: Tesla Model X, VIN 5YJ3E1EA6XXXXXXXX, currently logged into tesla.com in his browser. He cares about: charging cost tracking, preconditioning, supercharger queue, agent-native output.

## Source Priority

N/A - single source.

## Product Thesis

The Tesla CLI tier today is either (a) skeletal `tesla-control`-style command pipes with no memory of your car, or (b) heavy Docker stacks like TeslaMate that demand a Postgres and a Grafana before they answer "should I leave the house now."

A Printing Press Tesla CLI wins by sitting in the middle: agent-native JSON, a local SQLite that remembers every command and every state poll, a single `ready` command that beats six iOS taps, and charging-cost analytics out of the box without needing TeslaMate. For Matt's 2018 Model X it ships immediately on REST; for newer cars we shim out to `tesla-control` rather than rebuild signed-command crypto.

## Build Priorities

### Priority 0 (foundation)

- Token bootstrap: paste-a-refresh-token + `tesla-pp login --refresh-token-file`; auto-refresh on 401; Keychain on darwin.
- `tesla-pp vehicles list / get` mapping to `PRODUCT_LIST` + `VEHICLE_LIST` + `VEHICLE_SUMMARY`.
- `tesla-pp state <vin>` -> `VEHICLE_DATA` with category flags `--include charge,climate,drive,vehicle,gui,location,vehicle_config,closures,charge_schedule_data,preconditioning_schedule_data`.
- Wake handling: every command takes `--wake` (default off); silent-no-wake mode for sleep-friendly polling like teslamate.
- SQLite store with the schema above. Every command append-only logged.
- Agent-native JSON output by default; `--text` for human; never print to stderr what an agent needs to parse.
- Signed-command detection: if the API returns the "vehicle command protocol required" error, surface a one-line shim that points at `tesla-control` and the enrollment URL. Do NOT silently fail.

### Priority 1 (absorb everything)

Every command in `tesla-control` plus the workflow-style endpoints from TeslaPy:

- Doors / locks: `lock`, `unlock`, `trunk open|close|toggle`, `frunk open`, `charge-port open|close`, `windows vent|close`, `sun-roof vent|close`
- Climate: `climate on|off`, `climate temp <c|f>`, `climate keeper <off|on|dog|camp>`, `climate bioweapon <on|off>`, `climate cop <fan_only|on|off>` + `cop temp`, `defrost max`
- Seats/wheel: `seat-heater <pos> <level>`, `seat-cool <pos> <level>`, `wheel-heater <on|off>`, `auto-seat-climate <pos>`
- Charging: `charge start|stop`, `charge limit <pct>`, `charge amps <a>`, `charge standard|max`, `charge schedule <hh:mm>`, `charge schedule-cancel`
- Scheduled departure: `depart-at <hh:mm> [--precondition] [--off-peak]`
- Media: `media play|pause|next|prev|next-fav|prev-fav|vol up|down|set <0-10>`
- Sentry / valet / guest: `sentry on|off`, `valet on <pin>|off`, `guest on|off`, `guest erase-data`, `speed-limit set <mph> <pin>|on|off|clear-pin`
- Navigation: `nav send "<address>"`, `nav send-coords <lat> <lng>`, `nav send-supercharger <id>`, `nav waypoints <a;b;c>`
- Software: `update install [--in 2h]`, `update cancel`, `release-notes`
- HomeLink: `homelink trigger`
- Honk/flash/wake/ping
- Drive: `drive start` (remote_start_drive)
- Image: `compose --view front|side|rear|side_rear` -> PNG to stdout or `--out file.png`
- Keys: `keys list`, `keys add <pubkey> <role> <form-factor>`, `keys remove <pubkey>`, `keys rename <pubkey> <name>`
- Energy / Powerwall (best-effort even though Matt has no Powerwall): `energy sites`, `energy site <id>`, `energy site <id> tariff get|set`, `energy site <id> reserve <pct>`, `energy site <id> mode self|backup|autonomous`
- Service: `service appointments`, `service recalls`, `service history`
- Charging history: `charge-history` (timdorr + Tesla Electric endpoints)
- Nearby chargers: `chargers nearby [--lat --lng] [--radius]`
- Diagnostics: `diag mobile-enabled`, `diag last-seen`, `diag option-codes`, `diag vin-decode`

Approximately 60-70 absorbed surface commands; matches `tesla-control` and exceeds [teslatte](https://github.com/gak/teslatte) / [barnybug/tesla-cli](https://github.com/barnybug/tesla-cli).

### Priority 2 (transcendence candidates - 7+ scored ideas with one-line "why")

1. **`tesla-pp ready`** - composite "can I leave in 5 min?" answer: SOC vs trip distance, plugged-in, doors closed, sentry off, cabin within 3 deg of target, no pending update mid-install, tire-pressure warnings. **Why**: every owner asks this 5x/week; nobody else gives a single yes/no with reasons.
2. **Charging-cost ledger** - import home tariff windows (TOU) and supercharger session pricing from `CHARGING_HISTORY` + `CHARGING_DOWNLOAD_CSV`; output `$/kWh`, monthly spend, supercharger-vs-home ratio, "if you had only charged at home X you would have saved $Y." **Why**: TeslaMate computes this but requires a whole Docker stack; cobanov/teslamate-mcp queries it but requires TeslaMate to exist. Standalone CLI is unique.
3. **Precondition automation** - `tesla-pp precondition --depart 7:45a [--days mon-fri]` writes a launchd/systemd timer that wakes the car at depart-minus-N and pings climate-on; cancels if not plugged in to avoid drain. **Why**: native `precondition-schedule-add` exists but is location-anchored; this is calendar-anchored and locally observable.
4. **Supercharger queue intelligence** - poll `NEARBY_CHARGING_SITES` + Tesla's stall-availability subfield around frequent stops; emit `tesla-pp supercharger watch <site>` that alerts when free stalls > threshold or when projected wait drops; also "tonight's-route" mode that scores upcoming superchargers along nav route. **Why**: Tesla shows availability in-car only; nobody surfaces it as an agent feed.
5. **Drive-and-charge timeline reconstruction** - stitch `vehicle_data` polls into drives + charges identical to TeslaMate but in pure-Go SQLite, no Postgres. `tesla-pp timeline --since "last week"` returns JSON of drives, charges, vampire drain, address-resolved start/end. **Why**: Lets agents reason over the same data TeslaMate exposes without standing up TeslaMate.
6. **Key-rotation workflow** - `tesla-pp keys audit` reports every enrolled key + last-seen, flags unknown ones, offers a guided `tesla-pp keys remove --stale --older-than 90d`. Pairs with vehicle-command on signed-required cars. **Why**: Tesla app shows keys but no audit story; security-minded owners want a quarterly review.
7. **Ready-for-trip planner** - `tesla-pp plan-trip "Seattle to Bend"` calls `PLAN_TRIP` + `NAVIGATION_ROUTE`, returns charging stops, ETA, projected arrival SOC, cost estimate using current Supercharger pricing, with a `--send-to-car` flag that pushes the route. **Why**: Tesla app has it; nobody has it from a terminal/agent.
8. **Vampire-drain monitor** - delta SOC vs idle time, with a `tesla-pp vampire --threshold 1.5pct/24h` alert. **Why**: TeslaMate dashboard only; useful for warranty disputes and finding rogue sentry sessions.
9. **Sentry + dashcam clip workflow** - `tesla-pp dashcam save` triggers `DASHCAM_SAVE_CLIP` and records the timestamp + location into the store, enabling later `tesla-pp dashcam history` queries. **Why**: Saving a clip today is a frantic-tap action; CLI captures the why-I-saved metadata while it is still fresh.
10. **Multi-vehicle aggregate `tesla-pp fleet`** - even Matt is one-car today, but family-shared accounts are normal; show all vehicles' SOC, plugged status, lock state, in one table. **Why**: future-proofs for a 2-car household.
11. **MCP tool surface** - every priority-1 command + the transcendence ones exposed as MCP tools, naturally agent-callable. **Why**: Mandatory for PP, plus tesla-async.fyi proves market demand.

Top three to lead with: `ready` (#1), charging-cost ledger (#2), supercharger queue intelligence (#4). These all (a) require the local store to deliver, (b) cannot be reproduced by piping `tesla-control` outputs, and (c) show up immediately on Matt's 2018 Model X without signed-command enrollment.
