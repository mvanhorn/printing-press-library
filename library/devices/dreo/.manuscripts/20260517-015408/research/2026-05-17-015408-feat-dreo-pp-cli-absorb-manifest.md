# Dreo CLI Absorb Manifest

## Tools found (all reverse-engineered; no official open API)

| Tool | Lang | Stars | Status | Role |
|------|------|-------|--------|------|
| [JeffSteinbok/hass-dreo](https://github.com/JeffSteinbok/hass-dreo) | Python | ~200 | Active | Reference HA integration; most-covered device models |
| [zyonse/homebridge-dreo](https://github.com/zyonse/homebridge-dreo) | TypeScript | ~80 | Active | Homebridge/HomeKit integration |
| [dreo-team/hass-dreoverse](https://github.com/dreo-team/hass-dreoverse) | Python | <50 | Active | OFFICIAL Dreo HA integration |
| [dreo-team/pydreo-client](https://github.com/dreo-team/pydreo-client) | Python | <50 | Active | OFFICIAL Dreo Python SDK |
| pydreo-community (PyPI) | Python | n/a | Active | Standalone install of hass-dreo's pydreo lib |

**No general-purpose Dreo CLI exists today.** Every existing client is bundled inside a HA / Homebridge / SDK integration. This is greenfield.

## Absorbed (match or beat everything that exists) — 34 features

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | List devices | hass-dreo `pydreo/__init__.py:get_devices()` | `devices list` reads `/api/v2/user-device/device/list`, caches to local SQLite | Offline list, JSON/table/CSV, --select |
| 2 | Read device state snapshot | hass-dreo `commandtransport.py:get_state()` | `state <device>` reads `/api/user-device/device/state?deviceSn=` | Offline cached, --live to force fresh |
| 3 | Power on/off | hass-dreo `set_power_state()` | `set <device> --power on\|off` sends WS `{params:{poweron:bool}}` | --dry-run, --wait, bulk fan-out |
| 4 | Set fan speed/level (1-N) | hass-dreo `set_speed()` | `set <device> --speed N` sends WS `{params:{windlevel:N}}` | Range validation per-model |
| 5 | Set fan mode (normal/natural/sleep/auto/turbo) | hass-dreo `set_mode()` | `set <device> --mode <m>` sends WS `{params:{windmode:n}}` | Mode-name → int alias |
| 6 | Oscillation (h/v/both/off) | hass-dreo OscillationMode | `set <device> --oscillate horizontal\|vertical\|both\|off` (bitwise) | Word inputs |
| 7 | Oscillation angle | hass-dreo `pydreoheater.py` | `set <device> --oscillate-angle N` (`shakehorizonangle` or `oscangle`) | Per-model validation |
| 8 | Timer on/off | hass-dreo `set_timer()` | `set <device> --timer-on/off <mins>` | Human time ("30m", "2h") |
| 9 | Heater level (1-3) | hass-dreo `pydreoheater.py` | `set <device> --heat-level N` | Type-checked |
| 10 | Heater mode (coolair/hotair/eco) | hass-dreo | `set <device> --heat-mode <m>` | Mode-name resolution |
| 11 | Target temperature (heaters, AC) | hass-dreo `pydreoac.py` | `set <device> --target-temp N --unit C\|F` | Unit conversion |
| 12 | Target humidity (humidifiers, dehumidifiers) | hass-dreo `pydreohumidifier.py` | `set <device> --target-humidity N` | 30-90 range |
| 13 | Fog level (humidifiers) | hass-dreo | `set <device> --fog-level N` | 0-6 range |
| 14 | Mist level (humidifiers) | hass-dreo | `set <device> --mist-level N` | 1-3 range |
| 15 | Ceiling fan direction | hass-dreo `pydreoceilingfan.py` | `set <device> --direction forward\|reverse` | Boolean alias |
| 16 | Light on/off | hass-dreo | `set <device> --light on\|off` | Per-model |
| 17 | Light dimming | hass-dreo | `set <device> --light-level N` | Range per model |
| 18 | Color temperature | hass-dreo | `set <device> --color-temp N` | Kelvin range |
| 19 | RGB color/mode (humidifiers) | hass-dreo `set_rgb_*()` | `set <device> --rgb-color #hex --rgb-mode static\|cycle\|breath --rgb-level N` | Hex parsing |
| 20 | Child lock | hass-dreo | `set <device> --child-lock on\|off` | Per-model |
| 21 | Display/LED toggles | hass-dreo | `set <device> --display/--led-always-on` | |
| 22 | Voice/mute | hass-dreo | `set <device> --voice/--mute` | |
| 23 | Temperature unit (C/F) | hass-dreo | `set <device> --unit C\|F` | |
| 24 | Sensor readings | hass-dreo state map | Embedded in `state` output | Plus aggregate `sensors` view |
| 25 | Filter life (purifiers) | hass-dreo `pydreoair*.py` | Embedded in `state` output | |
| 26 | Water tank status | hass-dreo | Embedded in `state` output | |
| 27 | Get persistent settings | hass-dreo `/api/user-device/setting GET` | `settings get <device>` | |
| 28 | Update persistent settings | hass-dreo `/api/user-device/setting PUT` | `settings update <device> --json '{...}'` | --dry-run shows diff |
| 29 | Auth login (email/MD5(password) → OAuth) | All 3 SDKs | `auth login` (or auto on first use); token to `~/.config/dreo/token.json` | Auto-refresh on 401 |
| 30 | Region discovery | hass-dreo `_re_login()` | Honors region from response; `--region us\|eu` override | |
| 31 | WebSocket state subscription | hass-dreo `commandtransport.py` | Embedded in `watch` (novel) | |
| 32 | Token refresh on 401 | hass-dreo `_re_login()` | Transparent to user | |
| 33 | Firmware check | dreo-cloudcutter | `firmware check <device>` reads `/api/upgrade/device/check` | Read-only; --json |
| 34 | Doctor health check | new (no existing Dreo CLI) | `doctor` verifies env vars, login round-trip, region, device count | New |

No stubs in absorbed scope. Every row is shipping scope.

## Transcendence (novel — only possible with our approach) — 8 features

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Bulk fan-out control | `bulk --type <model> --room <room> <action>` | 9/10 | hand-code | Reads cached `devices` table, filters by type/room, opens one WS connection, sends N control frames in parallel | Top Workflow #1; hass-dreo + homebridge-dreo lack any bulk surface (HA-resident) |
| 2 | Whole-house sensor snapshot | `sensors` | 9/10 | hand-code | Iterates cached `devices` with sensor capability, reads state per device, composes one ranked table | Top Workflow #2; no single Dreo endpoint returns aggregated sensors |
| 3 | Live WebSocket state stream | `watch <device>` / `watch --all` | 9/10 | hand-code | Opens `wss://wsb-{region}.dreo-tech.com/websocket`, subscribes, prints each state-delta frame as JSON line | Top Workflow #5; no existing tool exposes raw WS frames |
| 4 | Sensor timeseries history | `sensors record` + `sensors query --device <d> --since <dur> --metric <m>` | 8/10 | hand-code | `record` ingests WS frames into `sensor_readings (device_sn, ts, metric, value)`; `query` runs parameterized SQL | Brief Build Priority 7; cross-source local query |
| 5 | Threshold + offline alerts | `alerts` | 7/10 | hand-code | Joins cached `devices` + most-recent `device_state`, filters on filter-life/water-tank/heartbeat/PM2.5 | Top Workflow #4; cross-entity local join |
| 6 | Scene save + apply | `scene save <name>` / `scene apply <name>` | 7/10 | hand-code | `save` snapshots state to local `scenes` table; `apply` replays as WS control frames in parallel | Persona Sam's bedtime ritual; Dreo app scenes are finicky |
| 7 | Per-room rollup | `rooms` | 6/10 | hand-code | Groups cached `devices` by `room`, joins latest `device_state`, prints per-room aggregates | Persona Avery; cross-entity local join |
| 8 | Local FTS device search | `devices search <q>` | 5/10 | spec-emits | Generator emits FTS over cached `devices` (name, room, model, serial) | Cross-source local query, generator-default |

## Build commitment

- **34 absorbed features** — match/beat every existing Dreo client. Generator emits most from spec.
- **8 transcendence features**: 7 hand-code (`bulk`, `sensors`, `watch`, `sensors record`/`query`, `alerts`, `scene save`/`apply`, `rooms`) + 1 spec-emits (`devices search`).
- Hand-code commitment: ~7 new Cobra command files + `root.go` wiring (typical 80-150 LoC each).
- No stubs anywhere in shipping scope.
