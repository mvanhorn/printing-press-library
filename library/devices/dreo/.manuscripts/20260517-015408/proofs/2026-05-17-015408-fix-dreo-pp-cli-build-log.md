# Dreo CLI - Phase 3 build acceptance report

Working dir: `/Users/tmchow/printing-press/.runstate/lexical-conjuring-kahan-622724c9/runs/20260517-015408/working/dreo-pp-cli`

## Environment note

`DREO_USERNAME` and `DREO_PASSWORD` were **not** present in this subprocess's environment despite the user statement. Verified via `env | grep -E 'DREO|USER'` returning only `USER=tmchow`, and via `${DREO_USERNAME+set}` parameter expansion. As a result, *live* (real-API) acceptance for `auth login`, `devices list` (against the API), `set` (real WS frame), `bulk` (real WS frames), `watch` (real updates), `sensors --live`, and `scene apply` (real WS frames) **could not be exercised against the production account**. Every command was, however, verified via:

1. The full `--dry-run` path (which constructs the actual frame/request and emits it, with no network).
2. Behavior against a seeded local SQLite store containing 4 devices and their state.
3. `PRINTING_PRESS_VERIFY=1` short-circuits.
4. `PRINTING_PRESS_DOGFOOD=1` curtailment.
5. Negative paths (bad flag values, missing args, scene-not-found).

If the user can re-run the live commands with credentials in env, the report at the end of this file lists the exact commands.

---

## 1. `internal/dreoauth.Login` (foundation)

| Test | Outcome |
| ---- | ------- |
| Compiles (`go build ./internal/dreoauth/...`) | PASS |
| `Login()` with empty username/password returns error | PASS (constructed `errors.New`) |
| Region-mismatch retry path encoded in `Login()` | PASS (logical review; live retry not exercised) |
| MD5 of password computed via `crypto/md5` + `hex.EncodeToString` | PASS |

**Live verification (could not run):**
```bash
# Requires DREO_USERNAME/DREO_PASSWORD in env
./dreo-pp-cli auth login --json
```
Expected output: `{"authenticated":true,"region":"NA"|"EU",...}` and `~/.config/dreo-pp-cli/config.toml` populated with `access_token` and `region`.

## 2. `internal/dreows.Connect` (foundation)

| Test | Outcome |
| ---- | ------- |
| Compiles | PASS |
| Constructs `wss://wsb-{us,eu}.dreo-tech.com/websocket?accessToken=...` | PASS (verified in source) |
| Keepalive goroutine sends `"2"` every 15s | PASS |
| `Send()` emits proper control envelope `{devicesn, method:"control", params, timestamp}` | PASS |
| `parseFrame()` is tolerant of `reported` / `params` / `state` / `data` sub-shapes | PASS (`merge()` flattens every non-protocol key) |

**Live verification (could not run):**
```bash
# Requires an access token
./dreo-pp-cli watch --all --duration 20s
```
Expected: at least one JSON line on stdout containing `devicesn` and one or more flattened fields.

## 3. `internal/store` (foundation)

| Test | Outcome |
| ---- | ------- |
| `store.Open()` creates tables (`devices`, `device_state`, `sensor_readings`, `scenes`) | PASS |
| FTS5 fallback to LIKE when virtual table unavailable | PASS (try-create, set `useFTS` flag, LIKE path) |
| `UpsertDevice` → `ListDevices` → `SearchDevices` round-trip | PASS (exercised via CLI seed; see `devices search` below) |
| `UpsertDeviceState` → `GetDeviceState` round-trip | PASS (exercised via `alerts`, `rooms`) |
| `AppendSensorReading` → `QuerySensorReadings` with `--since` window | PASS (see sensors query test) |
| `SaveScene` → `LoadScene` → `ListScenes` | PASS (see scene tests) |

## 4. `auth login` subcommand

```
$ ./dreo-pp-cli auth login --help
Exchange Dreo email/password for an access token (cached for subsequent calls)
...
Flags:
      --password string   Dreo password (defaults to $DREO_PASSWORD)
      --username string   Dreo email (defaults to $DREO_USERNAME)
```

```
$ ./dreo-pp-cli auth login
Error: auth login requires DREO_USERNAME and DREO_PASSWORD (or --username/--password)
exit=2
```

```
$ DREO_USERNAME=<test-user> DREO_PASSWORD=fakepass ./dreo-pp-cli auth login --dry-run
would log in as <test-user> against https://app-api-us.dreo-tech.com
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| Refuses to run without creds (exit 2) | PASS |
| Honors --dry-run | PASS |
| Wired into `auth` parent command | PASS |
| Live run + `auth status --json` shows `authenticated:true` | **NOT VERIFIED** (no env creds) |

## 5. `set` subcommand

```
$ ./dreo-pp-cli set fake-sn --power on --speed 4 --dry-run --json
{
  "device": "fake-sn",
  "dry_run": true,
  "endpoint": "wss://dreo-tech.com/websocket",
  "params": {
    "poweron": true,
    "windlevel": 4
  }
}
exit=0
```

```
$ ./dreo-pp-cli set fake --mode auto --oscillate horizontal --target-humidity 45 --rgb-color "#FF8800" --dry-run --json
{
  "device": "fake",
  "dry_run": true,
  "endpoint": "wss://dreo-tech.com/websocket",
  "params": {
    "oscmode": 1,
    "rgbcolor": "#FF8800",
    "shakehorizon": true,
    "targetHumidity": 45,
    "windmode": 4,
    "windtype": 4
  }
}
exit=0
```

```
$ ./dreo-pp-cli set fake --power maybe --dry-run
Error: --power: expected on|off, got "maybe"
exit=2

$ ./dreo-pp-cli set fake --mode bizarre --dry-run
Error: --mode: expected normal|natural|sleep|auto|turbo, got "bizarre"
exit=2

$ ./dreo-pp-cli set fake
Error: at least one state flag is required (--power, --speed, --mode, ...)
exit=2

$ PRINTING_PRESS_VERIFY=1 ./dreo-pp-cli set fake --power on
would set fake: map[poweron:true]
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| `--dry-run` prints the frame | PASS |
| Mode→windmode/windtype mapping | PASS (normal→1, auto→4) |
| Oscillation→oscmode + shakehorizon | PASS |
| RGB color parsed and emitted | PASS |
| Bad flag values rejected with exit 2 | PASS |
| `PRINTING_PRESS_VERIFY` short-circuit | PASS |
| Real WS send observed on a physical fan | **NOT VERIFIED** (no env creds) |

## 6. `bulk` subcommand

```
$ ./dreo-pp-cli bulk --action off --type tower-fan --dry-run --json
{
  "action": "off",
  "devices": [
    { "model": "HTF008S", "name": "Bedroom Fan", "room": "Bedroom", "sn": "HTF008S-AAAA1111" }
  ],
  "dry_run": true,
  "matched": 1,
  "params": { "poweron": false }
}
exit=0
```

```
$ ./dreo-pp-cli bulk --action sleep --all --dry-run
DRY RUN: would send map[poweron:true windmode:3 windtype:3] to 4 devices:
  Bedroom Fan          HTF008S-AAAA1111 (HTF008S)
  Living Room Purifier HAP002S-BBBB2222 (HAP002S)
  Nursery Humidifier   HHM001S-CCCC3333 (HHM001S)
  Office Heater        HSH004S-DDDD4444 (HSH004S)
exit=0
```

```
$ PRINTING_PRESS_VERIFY=1 ./dreo-pp-cli bulk --action off --all
would bulk off across 4 devices
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| Type filter (`tower-fan` → `HTF*`) | PASS (1/4 matched) |
| `--all` covers every cached device | PASS (4/4 matched) |
| Action→params mapping (off / sleep / auto) | PASS |
| `PRINTING_PRESS_VERIFY` short-circuit | PASS |
| Real WS fan-out observed | **NOT VERIFIED** |

## 7. `watch` subcommand

```
$ ./dreo-pp-cli watch --dry-run
DRY RUN: would watch all devices
exit=0
```

```
$ PRINTING_PRESS_VERIFY=1 ./dreo-pp-cli watch --all
would watch: skipped under PRINTING_PRESS_VERIFY
exit=0
```

```
$ PRINTING_PRESS_DOGFOOD=1 timeout 35 ./dreo-pp-cli watch --all
Error: watch: open WS: not authenticated; run `dreo-pp-cli auth login`
dogfood watch exit=5
# (Exited in <1s because no token; dogfood caps the loop at 5s anyway,
# so even with a token the matrix's 30s budget cannot be exceeded.)
```

| Acceptance | Status |
| ---------- | ------ |
| Dry-run is non-network | PASS |
| Verify short-circuit | PASS |
| Dogfood exits cleanly under matrix's 30s timeout | PASS (5s ctx + auth fast-fail) |
| Real-time updates printed as JSON lines | **NOT VERIFIED** (no creds) |

## 8. `sensors` (snapshot)

```
$ ./dreo-pp-cli sensors
NAME                  ROOM         MODEL    TEMP  HUMIDITY  PM2.5
Living Room Purifier  Living Room  HAP002S                  42
Bedroom Fan           Bedroom      HTF008S  72.5  41
Office Heater         Office       HSH004S  64
Nursery Humidifier    Nursery      HHM001S        38
exit=0

$ ./dreo-pp-cli sensors --json
[ {"model":"HAP002S","name":"Living Room Purifier","pm25":42, ...},
  {"humidity":41,"model":"HTF008S","name":"Bedroom Fan","room":"Bedroom","sn":"HTF008S-AAAA1111","temperature":72.5},
  ... ]
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| At least one row with seeded data | PASS (4 rows) |
| Rank by pm25 desc, then temp desc | PASS (Living Room Purifier first; pm25=42) |
| `--json` parses cleanly | PASS |

## 9. `sensors record`

```
$ ./dreo-pp-cli sensors record --help
# Long-running; not driven live without a token. Verify path tested:

$ PRINTING_PRESS_VERIFY=1 ./dreo-pp-cli sensors record
would record: skipped under PRINTING_PRESS_VERIFY
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| Annotated `mcp:hidden=true` | PASS (review of source) |
| Verify short-circuit | PASS |
| Dogfood caps at 10s | PASS (review of source) |
| Live recording into `sensor_readings` table | **NOT VERIFIED** (no token) |

## 10. `sensors query`

```
$ ./dreo-pp-cli sensors query --metric temperature --limit 5
TIME                       SN                METRIC       VALUE
2026-05-17T02:28:38-07:00  HTF008S-AAAA1111  temperature  72
2026-05-17T02:27:38-07:00  HTF008S-AAAA1111  temperature  72.1
...
exit=0

$ ./dreo-pp-cli sensors query --since 30m --json
[ {"sn":"HTF008S-AAAA1111","ts":"...","metric":"temperature","value":72}, ... ]
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| `--metric` filter | PASS |
| `--since` window | PASS |
| `--limit` enforced | PASS |
| `--json` array output | PASS |

## 11. `alerts`

```
$ ./dreo-pp-cli alerts --pm25-above 35
KIND              NAME                  ROOM         VALUE
stale_state       Bedroom Fan           Bedroom      2026-05-17T02:21:38-07:00
pm25_high         Living Room Purifier  Living Room  42
filter_low        Living Room Purifier  Living Room  8
stale_state       Living Room Purifier  Living Room  2026-05-17T02:21:38-07:00
water_tank_empty  Nursery Humidifier    Nursery
stale_state       Nursery Humidifier    Nursery      2026-05-17T02:21:38-07:00
offline           Office Heater         Office       false
stale_state       Office Heater         Office       2026-05-17T02:21:38-07:00
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| `pm25_high` triggers (seeded pm25=42 > threshold=35) | PASS |
| `filter_low` triggers (seeded filterLife=8 < default 10) | PASS |
| `water_tank_empty` triggers (`wrong="Empty"`) | PASS |
| `offline` triggers (device.online=false) | PASS |
| `stale_state` triggers (state fetched 7m ago > default 5m) | PASS |
| `--json` array output | PASS (verified) |

## 12. `scene` (save / apply / list)

```
$ ./dreo-pp-cli scene save evening --all
Saved scene "evening" with 4 devices.
exit=0

$ ./dreo-pp-cli scene list
NAME     DEVICES  SAVED
evening  4        2026-05-17T02:29:26-07:00
exit=0

$ ./dreo-pp-cli scene apply evening --dry-run --json
{
  "devices": 4,
  "dry_run": true,
  "name": "evening",
  "snapshots": {
    "HAP002S-BBBB2222": {"poweron": true, "windlevel": 2},
    "HHM001S-CCCC3333": {"mistlevel": 2, "poweron": true, "targetHumidity": 50},
    "HSH004S-DDDD4444": {"htalevel": 1, "poweron": false, "targetTemperature": 70},
    "HTF008S-AAAA1111": {"oscmode": 1, "poweron": true, "windlevel": 3, "windmode": 1}
  }
}
exit=0

$ ./dreo-pp-cli scene apply nonexistent
Error: scene "nonexistent": sql: no rows in result set
exit=3
```

| Acceptance | Status |
| ---------- | ------ |
| `save` filters by `--all` and writes 4 device snapshots | PASS |
| `list` shows the saved scene | PASS |
| `apply --dry-run` shows extracted controllable fields | PASS |
| `apply <missing>` returns notFoundErr (exit 3) | PASS |
| Read-only fields (temperature, humidity, pm25) excluded from snapshot | PASS (only `poweron`, `windlevel`, etc. in scene) |
| Real WS fan-out from apply | **NOT VERIFIED** |

## 13. `rooms`

```
$ ./dreo-pp-cli rooms
ROOM         DEVICES  ON  AVG_TEMP  AVG_HUMIDITY
Bedroom      1        1   72.5      41.0
Living Room  1        1
Nursery      1        1             38.0
Office       1        0   64.0
exit=0

$ ./dreo-pp-cli rooms --json
[ {"room":"Bedroom","device_count":1,"on_count":1,"avg_temperature":72.5,"avg_humidity":41},
  ... ]
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| Groups by `room` | PASS |
| `on_count` derived from `poweron` | PASS (Office=0/1 because poweron=false; others=1/1) |
| `avg_temperature` / `avg_humidity` mean | PASS |
| `--json` shape | PASS |

## 14. `devices search`

```
$ ./dreo-pp-cli devices search bedroom
NAME         ROOM     MODEL    SN                ONLINE
Bedroom Fan  Bedroom  HTF008S  HTF008S-AAAA1111  yes
exit=0

$ ./dreo-pp-cli devices search HTF008S --json
[ {"sn":"HTF008S-AAAA1111","name":"Bedroom Fan","model":"HTF008S","room":"Bedroom",...} ]
exit=0
```

| Acceptance | Status |
| ---------- | ------ |
| Matches room name | PASS |
| Matches model | PASS (separate run) |
| `--json` output | PASS |
| Wired as `devices search` subcommand | PASS (visible under `devices --help`) |

## Config patch (Config.AuthHeader)

```go
// Before:
if c.DreoUsername != "" {
    return "Bearer " + c.DreoUsername  // BUG: synthesized fake bearer from email
}
// After:
if c.AccessToken != "" {
    return "Bearer " + c.AccessToken
}
return ""
```

Verified by:
- Auth status now reports "Not authenticated" when only `DREO_USERNAME`/`DREO_PASSWORD` are set (no token).
- `auth set-token testtoken123` → `auth status` → reports `source: oauth2` and `Bearer testtoken123` flows through to client requests.
- `Region` field added to `Config` and persisted by `auth login`.

## Build, test, format

```
$ go build -o ./dreo-pp-cli ./cmd/dreo-pp-cli   # PASS
$ go test ./...                                 # 151 passed in 13 packages
$ go vet ./...                                  # No issues found
$ go fmt ./...                                  # PASS
```

## Outstanding live-verification work

Not blockers on the build but reproducible by anyone with credentials in env:

```bash
# Confirm full happy path end to end:
DREO_USERNAME=... DREO_PASSWORD=... ./dreo-pp-cli auth login
./dreo-pp-cli devices list --json | jq 'length'
./dreo-pp-cli devices state --device-sn <sn> --json
./dreo-pp-cli set <some-sn> --power on --wait --json
./dreo-pp-cli bulk --type tower-fan --action off
./dreo-pp-cli watch --duration 10s
./dreo-pp-cli sensors --live --json
./dreo-pp-cli sensors record --duration 30s
./dreo-pp-cli scene save night --all
./dreo-pp-cli scene apply night
```

## Summary

| Bucket | Count |
| ------ | ----- |
| Features delivered (compiles + dry-run path works) | 13/13 |
| Foundation packages | 3/3 (dreoauth, dreows, store) |
| Live-tested end-to-end against the Dreo account | 0/13 (env vars absent in subprocess) |
| Local-state tested end-to-end | 8/13 (devices search, sensors, sensors query, alerts, rooms, scene save/apply/list) |
| Dry-run + verify-env short-circuit verified | 13/13 |
| Tests passing | 151/151 |
| `go vet` clean | yes |
| Build succeeds | yes |
