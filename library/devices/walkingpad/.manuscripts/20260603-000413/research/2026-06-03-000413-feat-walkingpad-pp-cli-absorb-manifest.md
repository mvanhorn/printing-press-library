# WalkingPad CLI — Absorb Manifest

Target: BLE device (WalkingPad family). Sources: `ph4r05/ph4-walkingpad` (protocol), `kabirdos/walkingpad-tracker` (tracking app). Live device: C2 confirmed.

Architecture: the device generator emits a **replay-transport safety scaffold** (`capabilities`, `status`, fixed-payload safety-gated actuators) + the `Transport`/`Session` interfaces. Live BLE, parameterized commands, response parsing, and the history/analytics layer are hand-built on top. Replay transport stays the default so `verify`/`dogfood` never actuate hardware; `--live` selects the real BLE backend; `PRINTING_PRESS_VERIFY=1` forces no-op even with `--live`.

## Absorbed (match or beat both libraries)
| # | Feature | Best Source | Our Implementation | Safety | Buildability | Added Value |
|---|---------|-------------|--------------------|--------|--------------|-------------|
| 1 | Auto-discover pad | ph4 Scanner / kabirdos PadClient | `walkingpad-pp-cli scan` | read-only | hand-code | matches by `fe00` service UUID — works for C2 (`KS-BLC2`, which name-matching misses) and the whole family |
| 2 | Current status | ph4 ask_stats + WalkingPadCurStatus | (generated) `status` + live transport | low-risk-write | spec-emits + hand-code (transport+parse) | `--json`/`--select`, replay-safe default |
| 3 | Live monitor stream | kabirdos capture loop | `walkingpad-pp-cli monitor --live` | low-risk-write | hand-code | continuous JSON lines, bounded under dogfood |
| 4 | Start belt | ph4 start_belt | (generated) `start` | physical-effect | spec-emits | safety-gated `--dry-run`/`--confirm-physical-effect`, verify no-op |
| 5 | Stop belt | ph4 stop_belt | (generated) `stop` | physical-effect | spec-emits | same |
| 6 | Set speed (0.5–6.0, 0.1 steps) | ph4 change_speed | `walkingpad-pp-cli speed <kmh> --live` | physical-effect | hand-code | finer than native 0.5 step |
| 7 | Switch mode | ph4 switch_mode | `walkingpad-pp-cli mode <auto\|manual\|standby> --live` | physical-effect | hand-code | one parameterized command |
| 8 | Wake from standby | ph4 switch_mode(manual) | (generated) `wake` | physical-effect | spec-emits | convenience |
| 9 | Last stored record | ph4 ask_hist | `walkingpad-pp-cli last-record --live` | low-risk-write | hand-code | parses `F8 A7` frame, `--json` |
| 10 | Pref: max speed | ph4 set_pref_max_speed | `walkingpad-pp-cli prefs max-speed <kmh> --live` | configuration-risk | hand-code | gated |
| 11 | Pref: start speed | ph4 set_pref_start_speed | `walkingpad-pp-cli prefs start-speed <kmh> --live` | configuration-risk | hand-code | gated |
| 12 | Pref: auto-start (intel) | ph4 set_pref_inteli | `walkingpad-pp-cli prefs auto-start <on\|off> --live` | configuration-risk | hand-code | gated |
| 13 | Pref: sensitivity | ph4 set_pref_sensitivity | `walkingpad-pp-cli prefs sensitivity <high\|med\|low> --live` | configuration-risk | hand-code | gated |
| 14 | Pref: units | ph4 set_pref_units_miles | `walkingpad-pp-cli prefs units <km\|miles> --live` | configuration-risk | hand-code | gated |
| 15 | Pref: child lock | ph4 set_pref_child_lock | `walkingpad-pp-cli prefs child-lock <on\|off> --live` | configuration-risk | hand-code | gated |
| 16 | Stats logging to store | ph4 --stats / kabirdos recorder | (behavior in `walkingpad-pp-cli record --live`) | low-risk-write | hand-code | records a walk to the local history store |
| 17 | Today totals | kabirdos today | `walkingpad-pp-cli today` | read-only | hand-code | offline, `--json` |
| 18 | Sessions by date | kabirdos sessions | `walkingpad-pp-cli sessions [--date]` | read-only | hand-code | offline, `--json` |
| 19 | Daily export (health bridge) | kabirdos export/health_export | `walkingpad-pp-cli export [--path]` | read-only | hand-code | `daily.json` + `yesterday.json` for the iPhone Health Shortcut |
| 20 | Capabilities/safety metadata | (ours) | (generated) `capabilities` | read-only | spec-emits | safety classes + evidence refs |
| 21 | Doctor / health check | (ours; ph4 OSX notes) | `walkingpad-pp-cli doctor` | read-only | hand-code | BLE adapter + permission + pad-reachable + verify-state |

Deferred from absorb (evidence too thin / low value for v1): `set_pref_display` (7-bit mask semantics undocumented), `set_pref_target` value semantics per type (kept out of v1 unless requested), `ask_profile` PAYLOADS_255 (semantics unknown → would be `unknown` safety, metadata-only).

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|--------------------------|
| T1 | Local walk-session history (single static binary, no Python/daemon/cloud) | `record` / `today` / `sessions` | hand-code | belt loses last-run on power-cut; ph4/kabirdos need a running Python daemon. Ours persists sessions+samples to a local JSONL store from one binary |
| T2 | Streak tracking | `streak` | hand-code | requires a cross-day local history join no single belt query provides |
| T3 | Trends over N days | `trends [--days N]` | hand-code | requires historical daily rollups |
| T4 | Calorie computation from a profile | `calories [--days N]` | hand-code | the belt does **not** report calories; compute from weight/age/height/sex via MET |
| T5 | Family-generic service-UUID discovery | `scan` | hand-code | C2 advertises `KS-BLC2`, not "WalkingPad"; matching `fe00` works across the whole family |
| T6 | Agent-native control surface | all (`--json`/`--select`, MCP read-only, verify no-op) | hand-code/spec | no existing WalkingPad tool is agent-native or safe-by-default |

## Hand-code commitment
- **Spec-emits (generated):** `capabilities`, `status` (shell), `start`, `stop`, `wake` — 5 command shells + safety harness.
- **Shared hand-built infra:** live BLE transport (`device.Transport`/`Session` live impl over tinygo bluetooth, single-client + timeout + verify-no-op guards), frame codec (CRC + 3-byte BE parse), history store (JSONL sessions+samples + rollups), profile config + MET calorie model.
- **Hand-code commands (~17):** `scan`, live `status` parse, `monitor`, `speed`, `mode`, `last-record`, `prefs` (parent + max-speed/start-speed/auto-start/sensitivity/units/child-lock), `record`, `today`, `sessions`, `trends`, `streak`, `calories`, `export`, `doctor`.

## Safety stance
- Physical-effect (`start`/`stop`/`speed`/`mode`/`wake`) and configuration-risk (`prefs *`) writes require `--dry-run` preview or `--confirm` (`--confirm-physical-effect`) before non-verify replay.
- All live commands no-op under `PRINTING_PRESS_VERIFY=1`; replay transport is the default so verify/dogfood never actuate the belt.
- MCP read-only annotations only on genuinely non-actuating commands (`capabilities`, `status`, `monitor`, `last-record`, `today`, `sessions`, `trends`, `streak`, `calories`, `doctor`, `scan`).
