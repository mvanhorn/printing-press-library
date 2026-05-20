# Dreo CLI Brief

## API Identity
- Domain: Smart-home device control (cloud-based). Dreo sells smart tower fans, air circulators, ceiling fans, heaters, air purifiers, humidifiers, dehumidifiers, ACs, evaporative coolers, and ChefMaker cookers.
- Users: Homeowners who automate their Dreo devices via Home Assistant, Homebridge, or HomeKit today, plus scripters/cron-driven automation users who lack a clean CLI surface.
- Data profile: User account → 1..N devices (model + serial). Each device has a typed state blob (power, speed, mode, sensor readings, oscillation) and accepts control commands over WebSocket. Cloud-only — no LAN protocol.

## Reachability Risk
- **Low.** Three independently maintained reverse-engineered clients (hass-dreo, homebridge-dreo, dreo-team/pydreo-client) converge on the same endpoints. Zero open auth/login issues on the largest community repo. Dreo's own engineering team publishes `dreo-team/hass-dreoverse` and a Python SDK using the same endpoints — they are not hostile to third-party clients. Caveats:
  - Hardcoded `client_id`/`client_secret` (Dreo iOS app's; lifted from binary). If Dreo rotates them every third-party client breaks.
  - WebSocket disconnects are common but handled with reconnect/backoff in every reference client.

## Top Workflows
1. **Bulk power off at bedtime / when away** — #1 forum ask. Multi-device fan-out (all tower fans, all heaters).
2. **Read sensor state across devices** — "what's the temperature/humidity/PM2.5 across my house?" — every fan, heater, purifier, humidifier exposes its own sensor; agents asking for a whole-house snapshot want this aggregated.
3. **Scheduled mode/speed changes** — "sleep mode at 10pm, turbo at 7am" — currently done as HA automations; a cron-friendly CLI is the simpler shape.
4. **Filter/water-tank/sensor-threshold alerts** — recurring asks: purifier filter life, humidifier water-empty, "turn purifier on when PM2.5 > X".
5. **Live state stream / debug** — `tail -f` for WebSocket events while debugging automations (extremely useful, no existing tool offers this).

## Table Stakes
- List all devices (model, serial, room, online status, current state summary)
- Read full state for one device (power, speed/level, mode, sensor readings)
- Set one or more state fields on a device (power, speed, mode, oscillation, timer)
- Live state stream over WebSocket
- Bulk operations across device type or room
- Token caching with auto-refresh on 401 (mandatory — every login round-trips OAuth)

## Data Layer
- Primary entities: `devices` (device list + capabilities), `device_state` (most recent state snapshot per device), `sensor_readings` (timeseries of temperature/humidity/PM2.5 if we record them).
- Sync cursor: device list refreshes on demand (small N, no cursor needed). State snapshots are point-in-time and ideally appended via WebSocket subscription for `watch`-style features. Sensor history is novel territory.
- FTS/search: search across device name/room/model/serial. With sensor history, search across time-windowed events ("when did the bedroom fan last go to sleep mode").

## Codebase Intelligence
- Source: hass-dreo + homebridge-dreo + dreo-team/pydreo-client (all reverse-engineered, all agree on the wire shape).
- Auth: email + MD5(password) → OAuth bearer token via `POST /api/oauth/login`. Current hardcoded values (verified live 2026-05-17 against `app-api-us.dreo-tech.com`): `client_id=7de37c362ee54dcf9c4561812309347a`, `client_secret=32dfa0764f25451d99f94e1693498791`, `grant_type=email-password`, `encrypt=ciphertext`, `himei=faede31549d649f58864093158787ec9`, `scope=all`, `acceptLanguage=en`. Headers: `ua: dreo/2.8.2`, `lang: en`, `user-agent: okhttp/4.9.1`. Response includes `region` (e.g. "NA"), `access_token`. Token is a bearer; no per-request signing. A 401 from any endpoint triggers re-login. NOTE: an older `grant_type=openapi` shape with different client_id/secret is documented in older forks but is no longer accepted (live API returns `code: 100019 unsupported grant type`).
- Env var pattern: `DREO_USERNAME` + `DREO_PASSWORD` (canonical). Optional `DREO_REGION` (us|eu) skips region discovery.
- Base URL: `https://app-api-{us|eu}.dreo-tech.com` for REST; `wss://wsb-{us|eu}.dreo-tech.com/websocket` for WS.
- Data model: User → devices (deviceSn is the primary key, model identifies device class). State is a flat map of mixed-type fields per device class.
- Rate limiting: no formal rate limits surfaced in any client. Token caching across runs is mandatory; re-logging on every call is wasteful and a real risk for ban.
- Architecture: **WebSocket is the control plane.** REST is for discovery + initial state. Commands (power, speed, mode, oscillation) are WS frames: `{"devicesn":"<sn>","method":"control","params":{"<key>":<value>},"timestamp":<ms>}`. Keepalive is the literal string `"2"` every 15s (Socket.IO v2 ping).

## Endpoint Inventory
| Op | Method | Path |
|----|--------|------|
| Login | POST | `/api/oauth/login` |
| Device list (v2) | GET | `/api/v2/user-device/device/list` |
| Device list (v1, by room) | GET | `/api/app/index/family/room/devices` |
| Device state | GET | `/api/user-device/device/state?deviceSn=<sn>` |
| Setting GET | GET | `/api/user-device/setting` |
| Setting PUT | PUT | `/api/user-device/setting` |
| Firmware check | POST | `/api/upgrade/device/check` |

Plus the WebSocket control channel for all mutations.

## Device-Type Capability Matrix
| Type | Power | Speed | Mode | Oscillation | Sensors | Other |
|------|-------|-------|------|-------------|---------|-------|
| Tower fan (HTF*) | poweron | windlevel 1-N | windtype/windmode (normal/natural/sleep/auto/turbo) | shakehorizon + angle | temperature, sometimes pm25 | mute, lightsensor, ledalwayson, voiceon |
| Air circulator (HAF/HPF) | poweron | windlevel | preset (HPF reverse) | bitwise oscmode (H=1, V=2) | temperature | child lock |
| Ceiling fan (HCF) | poweron | windlevel | direction (forward/reverse) | n/a | — | light, dimming, color temp |
| Heater (HSH/WH) | poweron | htalevel 1-3 | coolair/hotair/eco/off | oscon + 60/90/120° | temperature, tempoffset | childlockon, ptcon, lighton |
| Air purifier (HAP) | poweron | windlevel | auto/manual/sleep | n/a | PM2.5, filter-life | child lock, display |
| Humidifier (HHM) | poweron | foglevel 0-6 | normal/auto/sleep | n/a | humidity, water-level (Ok/Empty) | target_humidity, RGB lights |
| Dehumidifier (HDH) | poweron | windlevel | auto/manual/dry | n/a | humidity, water-tank-full | target_humidity |
| AC (HAC) | poweron | windlevel | cool/dry/fan/sleep/eco | oscon → 0/2 bitwise | temp, optional humidity | target_temperature C/F, child lock, PTC |
| Evaporative cooler (HEC) | poweron | windlevel | normal/sleep | yes | water level | RGB on some |
| ChefMaker (KCM) | yes | n/a | cooking program | n/a | probe temperature | target temp + cook time |

Constants: `OscillationMode: OFF=0, H=1, V=2, BOTH=3`. `TemperatureUnit: 0=C, 1=F`.

## User Vision
User wants a CLI for their Dreo smart-home devices. No specific feature list — open-ended. Credentials provided for live testing.

## Product Thesis
- Name: **dreo-pp-cli**
- Why it should exist: No general-purpose Dreo CLI exists today. Every existing client is bundled inside a Home Assistant or Homebridge integration — useful if you run HA, useless if you want a one-shot script, cron-driven scheduler, or agent-callable surface. Dreo's own SDK is a Python provider library, not a CLI. A CLI with offline device caching, sensor history, WebSocket live-stream debugging, and bulk fan-out beats every existing tool because every existing tool is *not actually a CLI*.

## Build Priorities
1. **Auth + token cache** — login, refresh on 401, persist `access_token` + `region` to `~/.config/dreo/token.json`. `doctor` confirms it round-trips.
2. **Device discovery** — `devices list` (table + JSON) reads from cloud, caches to local SQLite (`devices` table keyed by `deviceSn`).
3. **State read** — `state <device>` per-device snapshot. `sensors` whole-house aggregated view (temperature/humidity/PM2.5 across every sensor-bearing device).
4. **Control** — `set <device> --power on --speed 4 --mode sleep --oscillate horizontal` translates flag groups to WS frames, with `--wait` to confirm state echo.
5. **Bulk / fan-out** — `bulk --type tower-fan --room bedroom power off` runs the same control across N devices in parallel.
6. **Live watch** — `watch <device>` streams WebSocket state updates as JSON lines; `watch --all` streams everything.
7. **Sensor history** — `sensors record` appends WS state events to a `sensor_readings` table; `sensors query` does time-windowed SQL over them. This is the transcendence play — no other Dreo tool keeps history.
