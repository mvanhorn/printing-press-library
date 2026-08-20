# WalkingPad CLI Brief

Target kind: **BLE physical device** (not an HTTP API). Routed through Device Sniff.
Live device available: **WalkingPad C2**, probe + actuate approved.
Family goal: work with **any compatible WalkingPad** (A1 / C2 / R1 / R2 / X21 share the protocol).

## Device Identity
- Product: KingSmith WalkingPad treadmill (C2 confirmed; A1/R1 PRO reported identical per ph4 README).
- Advertised BLE name contains `walkingpad` (case-insensitive match). kabirdos uses hint `WalkingPad`.
- Single-client constraint: only one BLE central at a time. The official phone app must be disconnected or the laptop cannot connect (observed "wedged BLE" pathology when the app held the central role).
- macOS note: CoreBluetooth requires scanning by `service_uuids` before connect; addresses are CoreBluetooth UUIDs, not MAC.

## Reachability Risk
- **Low.** Two independent community libraries implement and ship against this protocol; user has the physical device on hand for live confirmation. No remote endpoint, no anti-bot, no auth server.

## Mapping Research Gate — SATISFIED
Two independent community sources map actions to payloads:
1. `ph4r05/ph4-walkingpad` (`ph4_walkingpad/pad.py`) — canonical protocol reverse-engineering (Google OSS Peer Bonus 2022). Ground-truth command framing + status parsing.
2. `kabirdos/walkingpad-tracker` (`src/walkingpad/`) — tracking app built on ph4; corroborates UUIDs, speed clamp, operational timeouts; supplies the persistence/health data model.

### GATT topology
- Service: `0000fe00-0000-1000-8000-00805f9b34fb`
- Notify char (status): `0000fe01-0000-1000-8000-00805f9b34fb`  (subscribe for notifications)
- Write char (commands): `0000fe02-0000-1000-8000-00805f9b34fb`  (write commands)
- Also present: GAP `1800`, Device Info `180a`.

### Command framing
`[0xF7, group, ...payload..., crc, 0xFD]`
- Header `0xF7` (247), footer `0xFD` (253).
- CRC: `cmd[-2] = sum(cmd[1:-2]) % 256` (sum of all bytes between header and crc, mod 256). Invalid CRC → belt ignores command.
- Min command spacing ~0.69s; poll cadence ~0.75–0.8s.

### Control commands (group 0xA2 = 162)
| Action | Payload (pre-CRC) | Notes | Safety |
|---|---|---|---|
| ask_stats | `F7 A2 00 00 A2 FD` | request current status notification | read-only |
| set speed | `F7 A2 01 <spd> FF FD` | spd = km/h×10, clamp 5–60 (0.5–6.0 km/h) | physical-effect |
| switch mode | `F7 A2 02 <mode> FF FD` | mode: 0=auto, 1=manual, 2=standby | physical-effect (mode/wake) |
| start belt | `F7 A2 04 01 FF FD` | starts the belt | physical-effect |
| stop belt | `F7 A2 01 00 FF FD` | set speed 0 | physical-effect |
| (162_3_7) | `F7 A2 03 07 AC FD` | unknown semantic; metadata-only | unknown |

`wake` = switch to manual mode (brings a connected pad out of standby so the belt can run).

### History command (group 0xA7 = 167)
| Action | Payload (pre-CRC) | Notes | Safety |
|---|---|---|---|
| ask last record | `F7 A7 AA FF 50 FD` (mode 0) / `F7 A7 AA 00 51 FD` (mode 1) | belt's stored last-run; a second query clears it | read-only |

### Preferences (group 0xA6 = 166)
Encoding: `F7 A6 <key> <stype> <v2> <v1> <v0> AC FD` where value is 3-byte big-endian.
| Key | Pref | Values |
|---|---|---|
| 3 | max speed | km/h×10 | configuration-risk |
| 4 | start speed | km/h×10 | configuration-risk |
| 5 | start intelli (auto-start) | 0/1 | configuration-risk |
| 6 | sensitivity (auto mode) | 1=high,2=med,3=low | configuration-risk |
| 7 | display | 7-bit mask | configuration-risk |
| 8 | units | 0=km,1=miles | configuration-risk |
| 9 | child lock | 0/1 | configuration-risk |
| 1 | target | stype=type(0 none,1 dist,2 cal,3 time), value | configuration-risk |

### Status notification (current) — prefix `F8 A2` (248,162)
- `[2]` belt_state, `[3]` speed (×0.1 km/h), `[4]` mode (0 auto/1 manual/2 standby)
- `[5:8]` time seconds (3-byte BE), `[8:11]` distance in 10m units (÷100 = km), `[11:14]` steps
- `[14]` app_speed (÷30), `[16]` controller button, `[17]` CRC, `[18]` footer
- Calories are **NOT** reported by the belt (must be computed from a user profile).

### Last-record notification — prefix `F8 A7` (248,167)
- `[8:11]` time, `[11:14]` distance, `[14:17]` steps.

## Top Workflows
1. Control a walk hands-free from the laptop/terminal: start, set speed, stop, wake from standby.
2. Live monitor: poll/subscribe current speed/distance/steps/time while walking.
3. Record sessions to a local store and review history (today / by date / trends / streaks).
4. Compute calories the belt won't report (from a user weight/height/age/sex profile).
5. Export daily totals for downstream use (Apple Health bridge / JSON for shortcuts/agents).

## Table Stakes (absorb from both libraries)
- ph4: switch mode, start/stop, set arbitrary speed (0.5–6.0, finer than the native 0.5 step), all prefs, ask current state, ask last stored record, continuous stats logging to file.
- kabirdos: capture/record sessions, today totals, sessions-by-date, daily export, streaks, daily series/trends, reconnect/backoff, stale-link detection, calorie/health export bridge.

## Data Layer
- Primary entities: `sessions` (start/end, duration_s, distance_m, steps, avg/max speed, origin offsets) and `samples` (per-tick speed/distance/steps/belt_state).
- Session boundary: detect start/end by belt counter resets; keep one session across transient reconnects.
- Derived: daily totals, daily series (trend), current streak, calorie estimate.
- FTS/search: low value for numeric telemetry; offer SQL over sessions/samples instead.

## Codebase Intelligence
- Speed unit: km/h×10 integer; clamp 5–60. `kmh_to_pad_speed = clamp(round(kmh*10),5,60)`.
- Distance unit: 10m (÷100 → km). Time: seconds. Big-endian 3-byte ints.
- Connect ~1–3s healthy; cap 20s (wedged-handle). Disconnect cap 5s. Stale-data timeout ~15s → reconnect.
- Auto-discover by name when no address given; `WALKINGPAD_ADDRESS` env override.

## User Vision
- C2 owner; wants broad WalkingPad-family compatibility, not C2-only.

## Product Thesis
- Name: **walkingpad-pp-cli** ("walkingpad")
- Why it should exist: the only agent-native, single-binary WalkingPad controller with a built-in local session database. Control the belt and own your walk history offline — no phone app, no cloud, no Python env. Speeds the native app won't let you set, calories the belt won't report, and a history store the official app loses on power-cut.

## Build Priorities
1. P0 data layer: sessions + samples store, session recorder, SQL access.
2. P1 absorb: connect/scan, status, monitor(subscribe), start/stop/speed/wake/mode, all prefs, ask-last-record, doctor.
3. P2 transcend: session recording, today/sessions/trends/streak, calorie compute from profile, health/daily export, "since" window.
4. P3 polish: safety gating (dry-run/confirm on physical-effect), help examples, descriptions.

## Safety stance (from device-sniff-ble.md)
- Label: read-only / physical-effect / configuration-risk / unknown.
- Physical-effect + configuration-risk writes require dry-run preview or explicit `--confirm`/`--yes` before non-verify replay.
- verify/dogfood must NOT actuate real hardware (verify-mode no-op).
- MCP read-only annotations only on genuinely non-actuating commands.
