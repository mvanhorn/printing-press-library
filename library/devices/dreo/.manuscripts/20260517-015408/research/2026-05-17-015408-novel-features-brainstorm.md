# Dreo Novel Features Brainstorm

## Customer model

**Persona A: "Bedtime-Routine Sam" — sleep-routine homeowner with 4-7 Dreo devices**

*Today (without this CLI):* Sam has a tower fan in the bedroom, a fan in the office, a heater in the bathroom, and a purifier in the living room — all Dreo, all controlled by the Dreo app. At bedtime they open the app, tap "All Devices," and individually toggle each device off (or worse, just leave them running because tapping four times is tedious). They tried Home Assistant once but didn't want to run a Raspberry Pi for what felt like a four-action problem.

*Weekly ritual:* Nightly at ~10:30pm: power off bedroom fan (or set sleep mode), power off office fan, turn bathroom heater to eco, bump bedroom purifier to sleep. Morning at 7am: reverse it.

*Frustration:* No bulk operation. The Dreo app makes you tap each device individually. The "Scenes" feature in the app is finicky and doesn't survive app updates.

**Persona B: "Cron-script Casey" — automation-leaning user who scripts everything but refuses to run HA**

*Today (without this CLI):* Casey has a `~/bin/` full of bash wrappers for Hue, ecobee, Tailscale, and now wants Dreo in there too. They looked at hass-dreo, realized it was a Home Assistant component, and gave up. They tried curling the Dreo endpoints by hand, got tangled in the MD5+OAuth+region-handshake login, and quit.

*Weekly ritual:* Maintains a `crontab` that fires off vendor-specific shell scripts at fixed times (purifier to high at 6pm when cooking, heater to eco overnight, fan off at sunrise).

*Frustration:* WebSocket is the control plane. Every existing reference client buries WS behind a Python/TypeScript module. Casey needs `dreo set bedroom-fan --power off` to work from cron without a long-running daemon.

**Persona C: "Agent-Wrangler Avery" — uses Claude Code / agents to query and act on home state**

*Today (without this CLI):* Avery asks their agent "what's the temperature in each room?" and gets nothing because the agent has no Dreo tool. They want the agent to read sensor state across every device and to act ("turn off everything in the bedroom") through a single MCP surface.

*Weekly ritual:* Multiple times a week: ad-hoc "what's the air quality" / "is the bathroom heater still on" / "what was the bedroom temp last night" queries.

*Frustration:* Whole-house sensor aggregation requires fanning out to N devices — a perfect job for the CLI's local store.

**Persona D: "Debug-Diving Dan" — automation builder reverse-engineering Dreo behavior**

*Today (without this CLI):* Dan needs to know what fields change, when, and in what order. Today they have to add `print()` statements to hass-dreo's command transport and watch its log — which requires running HA.

*Weekly ritual:* When debugging a new automation: run a fan through manual controls, capture the state delta over WebSocket, reverse-engineer which params correspond to which user-visible behavior.

*Frustration:* `tail -f` for WebSocket frames doesn't exist for Dreo. Every reference client owns the WS connection.

## Candidates (pre-cut)

1. `bulk off` — fan-out power-off across device type/room (a, b)
2. `sensors` — whole-house sensor snapshot (a, c)
3. `watch` — live WebSocket state stream as JSON lines (a, b)
4. `sensors record` + `sensors query` — sensor timeseries persisted to local SQLite (c, e)
5. `schedule` — cron-friendly one-shot scheduled command (a, b)
6. `alerts` — threshold-based device alerts (b, c)
7. `anomaly` — find offline / stuck devices (c)
8. `rooms` — group devices by room with rollup state per room (c, e)
9. `scene save` / `scene apply` (a, e)
10. `capabilities` — per-device-model capability reference offline (b)
11. `history <device>` — timeline of recent state changes (c)
12. `devices search` — local FTS across device name/room/model/serial (c)
13. `tail` — alias for `watch --all` (d)
14. `firmware check` (b)
15. `presence` — auto-power-off when no devices report user activity (e)
16. `away mode` — bulk transition (a, e)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Bulk fan-out control | `bulk --type <model> --room <room> <action>` | 9/10 | hand-code | Reads cached `devices` table, filters by type/room, opens one WS connection, sends N control frames in parallel, exits | Brief Top Workflow #1 ("Bulk power off at bedtime / when away — #1 forum ask"); hass-dreo + homebridge-dreo lack any bulk surface (HA-resident) |
| 2 | Whole-house sensor snapshot | `sensors` | 9/10 | hand-code | Iterates cached `devices` with sensor capability, calls `/api/user-device/device/state` per device (or reads last cached state), composes one ranked table of (device, room, temp, humidity, pm25) | Brief Top Workflow #2; no single Dreo endpoint returns aggregated sensors; Persona Avery direct ask |
| 3 | Live WebSocket state stream | `watch <device>` / `watch --all` | 9/10 | hand-code | Opens WSS, subscribes, prints each state-delta frame as JSON line on stdout; Ctrl-C exits cleanly | Brief Top Workflow #5; no existing tool exposes raw WS frames |
| 4 | Sensor timeseries history | `sensors record` + `sensors query --device <d> --since <dur> --metric <m>` | 8/10 | hand-code | `record` subscribes to WS, parses state frames, appends rows to local `sensor_readings (device_sn, ts, metric, value)`; `query` runs parameterized SQL over the local table | Brief Build Priority 7; cross-source local query |
| 5 | Threshold + offline alerts | `alerts` | 7/10 | hand-code | Joins cached `devices` + most-recent `device_state`, filters where `filter_life < 10` OR `water_tank == empty` OR `last_seen > 5m ago` OR `pm25 > threshold` | Brief Top Workflow #4; cross-entity local join |
| 6 | Scene save + apply | `scene save <name>` / `scene apply <name>` | 7/10 | hand-code | `save` snapshots state across (filtered) devices to a local `scenes` table; `apply` reads and replays each as a WS control frame in parallel | Persona Sam's bedtime ritual; Dreo app's scenes feature is finicky; no equivalent in any reference client |
| 7 | Per-room rollup | `rooms` | 6/10 | hand-code | Groups cached `devices` by `room`, joins latest `device_state`, prints per-room aggregates | Persona Avery; cross-entity local join |
| 8 | Local FTS device search | `devices search <q>` | 5/10 | spec-emits | Generator emits FTS over the cached `devices` table | Cross-source local query, mostly generator-default |

### Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `schedule` | Scope creep: needs persistent background or crontab install; reduces to "put `dreo bulk` in your crontab" | bulk (1) |
| `anomaly` | Subsumed by alerts (5) | alerts (5) |
| `capabilities <model>` | Static reference table; Persona Dan's deeper need is solved by `watch` | watch (3) |
| `history <device>` | Pure subset of sensors query with `--device` filter | sensors query (4) |
| `tail` | UX sugar over watch | watch (3) |
| `firmware check` | Thin endpoint mirror; in absorb manifest already | — |
| `presence` | Needs external presence signal; LLM/external-service kill | bulk + scene apply away |
| `away mode` standalone | Reduces to `scene apply away` | scene save/apply (6) |
