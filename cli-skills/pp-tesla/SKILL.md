---
name: pp-tesla
description: "Every Tesla mobile-app feature, plus a charging-cost ledger and supercharger queue watcher no other Tesla CLI ships. Trigger phrases: `is my tesla ready`, `precondition my tesla`, `how much did i spend on charging this month`, `supercharger queue at`, `send this address to my tesla`, `list my tesla keys`, `use tesla`, `run tesla`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tesla-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/tesla/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Tesla — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tesla-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install tesla --cli-only
   ```
2. Verify: `tesla-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Tesla owner-API as JSON-first commands, with a local SQLite that remembers every vehicle state. Get a single `ready` yes/no for departure, a tariff-aware charging cost ledger that replaces TeslaMate's Docker stack, and a supercharger queue watcher you can subscribe to from an agent. For 2018+ Models S/X and pre-2021 Models 3/Y, commands route end-to-end on plain REST; newer vehicles get a clear shim message via `tesla reachability`.

## When to Use This CLI

Use tesla-pp-cli when you want a JSON-first CLI for your Tesla owner account: vehicle state inspection, climate/lock/charge commands, charging cost analytics, and an MCP surface for AI agents to drive your car. Faster than the iOS app for one-off commands, more agent-friendly than TeslaPy, and ships a local SQLite that delivers TeslaMate-style analytics (drives, charges, vampire drain, cost ledger) without standing up Postgres+Grafana. Best fit for owners with pre-2021 Models S/X or 3/Y on the old REST endpoints; newer vehicles get a clear shim via `tesla reachability`.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Composite truth from local state
- **`ready`** — Single yes/no answer to "can I leave in 5 minutes?" with reasoned blocker list - SOC vs trip distance, plugged-in, doors closed, sentry off, cabin within 3F of target, no mid-install update

  _Most-asked Tesla owner question; the iOS app makes it a 6-tap diagnostic, agents need a one-call truth_

  ```bash
  tesla ready 5YJ3E1EA6XXXXXXXX --json
  ```

### Local analytics that beat the dashboard
- **`cost ledger`** — Per-session cost, monthly spend, home-vs-Supercharger ratio with tariff-window aware $/kWh

  _Charging cost is the #1 Tesla-owner spreadsheet exercise; this is the first pure-CLI replacement for the TeslaMate dashboard_

  ```bash
  tesla cost ledger --since 2026-04-01 --group supercharger --json
  ```
- **`cost what-if`** — "If you had only charged at home you would have saved $X over the last 90 days." Re-runs charge rows with home $/kWh substituted for Supercharger sessions

  _Most Tesla owners suspect they could save by charging at home but lack proof; this puts a dollar figure on it_

  ```bash
  tesla cost what-if --only-home --since 90d --json
  ```
- **`timeline`** — Stitched drives + charges from synced vehicle_states polls - start/end, distance, energy, efficiency, address-resolved lat/lng

  _Lets agents reason over the same data TeslaMate exposes, without standing up Docker+Postgres+Grafana_

  ```bash
  tesla timeline --since "last week" --json
  ```
- **`vampire`** — SOC delta vs idle time, flags suspicious sentry sessions or app-wake events

  _Warranty-dispute evidence and rogue-sentry detection; TeslaMate dashboard only otherwise_

  ```bash
  tesla vampire --threshold 1.5pct/24h --since 30d --json
  ```

### Charging intelligence nobody else exposes
- **`supercharger watch`** — Single-poll snapshot of free stalls at a saved Supercharger; --watch repeats every N seconds with JSON-lines transitions

  _Avoids the I-90 Issaquah queue at peak; pageable from an agent on a drive_

  ```bash
  tesla supercharger watch 1000 --free-stalls 2 --watch --json
  ```

### Security audit nobody else does
- **`keys audit`** — Lists every enrolled key with last-seen, role, form-factor; flags keys not seen >90d as stale candidates for removal

  _Security-minded owners worry about old phones and abandoned NFC cards still pairing with their car; this is the quarterly review_

  ```bash
  tesla keys audit --stale-after 90d --json
  ```

### Reachability mitigation
- **`doctor`** — Detects signed-command-required errors, classifies the vehicle as REST-friendly vs signed-command-required, prints tesla-control enrollment URL when needed

  _Tesla's 2024-2026 signed-command rollout is the #1 user-facing landmine for any Tesla CLI; this is the only one that surfaces it clearly_

  ```bash
  tesla doctor --json
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 58 API entries from 60 total network entries
- Protocols: firebase (75% confidence), rest_json (75% confidence)
- Auth signals: bearer_token — headers: authorization; cookie — cookies: _abck, bm_sz; api_key — query: key
- Generation hints: browser_clearance_required, requires_browser_auth
- Candidate command ideas: create_auto_conditioning_start — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/command/auto_conditioning_start traffic.; create_auto_conditioning_stop — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/command/auto_conditioning_stop traffic.; create_diagnostic — Derived from observed POST /mobile-app/macgyver/diagnostic traffic.; create_door_lock — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/command/door_lock traffic.; create_door_unlock — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/command/door_unlock traffic.; create_fireperf:fetch — Derived from observed POST /v1/projects/tesla-prod/namespaces/fireperf:fetch traffic.; create_hermes — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/jwt/hermes traffic.; create_keys — Derived from observed POST /api/{id}/users/keys traffic.

## Command Reference

**charging** — Supercharger queue position and charging status

- `tesla-pp-cli charging` — Current Supercharger queue position (which stall, ETA)

**diagnostics** — Vehicle diagnostic feature config

- `tesla-pp-cli diagnostics` — Vehicle diagnostic feature flags

**energy_sites** — Powerwall and solar energy site config

- `tesla-pp-cli energy_sites create-set-backup-reserve` — Set Powerwall backup reserve percent
- `tesla-pp-cli energy_sites create-set-operation` — Set Powerwall operation mode
- `tesla-pp-cli energy_sites get-calendar-history` — Historical energy data per day/month
- `tesla-pp-cli energy_sites get-live-status` — Real-time Powerwall solar/grid/battery power
- `tesla-pp-cli energy_sites get-rate-tariffs` — Utility rate plans for Powerwall scheduling
- `tesla-pp-cli energy_sites get-site-info` — Energy site config (capacity, type, address)
- `tesla-pp-cli energy_sites get-site-status` — Backup-reserve and operation mode

**logs** — Client-side telemetry sink

- `tesla-pp-cli logs` — Forward client-side log events (mostly internal use)

**notification_preferences** — Push notification settings

- `tesla-pp-cli notification_preferences` — Update push notification preferences

**products** — Vehicles, Powerwalls, and solar products owned by the user

- `tesla-pp-cli products` — List vehicles, Powerwalls, and solar products owned

**users** — Account config, feature flags, orders, and key/credential management

- `tesla-pp-cli users create-keys` — Add a virtual phone key or BLE key
- `tesla-pp-cli users get-app-config` — Mobile app feature flags and runtime config
- `tesla-pp-cli users get-feature-config` — Account-level feature gating
- `tesla-pp-cli users get-orders` — Tesla orders (new vehicle deliveries, accessory purchases)

**vehicles** — Per-vehicle data, climate, locks, remote start, and Hermes telemetry token

- `tesla-pp-cli vehicles create-actuate-trunk` — Open/close trunk or frunk; which_trunk = front | rear
- `tesla-pp-cli vehicles create-add-charge-schedule` — Add a charging schedule (location + time window)
- `tesla-pp-cli vehicles create-add-precondition-schedule` — Add a preconditioning schedule
- `tesla-pp-cli vehicles create-adjust-volume` — Set volume to specific level
- `tesla-pp-cli vehicles create-auto-conditioning-start` — POST /api/1/vehicles/{vehicle_id}/command/auto_conditioning_start
- `tesla-pp-cli vehicles create-auto-conditioning-stop` — POST /api/1/vehicles/{vehicle_id}/command/auto_conditioning_stop
- `tesla-pp-cli vehicles create-bioweapon-mode` — Toggle bioweapon defense mode (HEPA recirculation)
- `tesla-pp-cli vehicles create-cancel-software-update` — Cancel pending software update
- `tesla-pp-cli vehicles create-charge-max-range` — Set charge limit to Max Range (100%)
- `tesla-pp-cli vehicles create-charge-port-door-close` — Close the charge port
- `tesla-pp-cli vehicles create-charge-port-door-open` — Open the charge port
- `tesla-pp-cli vehicles create-charge-standard` — Set charge limit to Standard (~90%)
- `tesla-pp-cli vehicles create-charge-start` — Start charging
- `tesla-pp-cli vehicles create-charge-stop` — Stop charging
- `tesla-pp-cli vehicles create-climate-keeper-mode` — Climate keeper mode: 0=off, 1=on, 2=dog, 3=camp
- `tesla-pp-cli vehicles create-door-lock` — POST /api/1/vehicles/{vehicle_id}/command/door_lock
- `tesla-pp-cli vehicles create-door-unlock` — POST /api/1/vehicles/{vehicle_id}/command/door_unlock
- `tesla-pp-cli vehicles create-erase-user-data` — Erase guest-session user data
- `tesla-pp-cli vehicles create-flash-lights` — Flash headlights
- `tesla-pp-cli vehicles create-guest-mode` — Enable/disable Guest mode
- `tesla-pp-cli vehicles create-hermes` — POST /api/1/vehicles/{vehicle_id}/jwt/hermes
- `tesla-pp-cli vehicles create-honk-horn` — Honk the horn
- `tesla-pp-cli vehicles create-max-defrost` — Toggle max defrost
- `tesla-pp-cli vehicles create-media-next-favorite` — Next favorite station
- `tesla-pp-cli vehicles create-media-next-track` — Next track
- `tesla-pp-cli vehicles create-media-prev-favorite` — Previous favorite station
- `tesla-pp-cli vehicles create-media-prev-track` — Previous track
- `tesla-pp-cli vehicles create-media-toggle-playback` — Toggle play/pause
- `tesla-pp-cli vehicles create-media-volume-down` — Volume down
- `tesla-pp-cli vehicles create-media-volume-up` — Volume up
- `tesla-pp-cli vehicles create-navigation-gps-request` — Send lat/lng directly to navigation
- `tesla-pp-cli vehicles create-remote-start-drive` — POST /api/1/vehicles/{vehicle_id}/command/remote_start_drive
- `tesla-pp-cli vehicles create-remove-charge-schedule` — Remove charging schedule by id
- `tesla-pp-cli vehicles create-remove-precondition-schedule` — Remove preconditioning schedule by id
- `tesla-pp-cli vehicles create-reset-valet-pin` — Reset Valet PIN
- `tesla-pp-cli vehicles create-schedule-software-update` — Schedule the pending software update
- `tesla-pp-cli vehicles create-seat-heater-request` — Set seat heater level (0-3) at a seat position (0=driver, 1=passenger, 2-5=rear)
- `tesla-pp-cli vehicles create-set-charge-limit` — Set charge limit percent (50-100)
- `tesla-pp-cli vehicles create-set-charging-amps` — Set charging amps draw
- `tesla-pp-cli vehicles create-set-sentry-mode` — Enable/disable Sentry mode
- `tesla-pp-cli vehicles create-set-temps` — Set driver and passenger climate temps (Celsius)
- `tesla-pp-cli vehicles create-set-valet-mode` — Enable/disable Valet mode with optional PIN
- `tesla-pp-cli vehicles create-share` — Send navigation destination to the car (address or coordinates)
- `tesla-pp-cli vehicles create-speed-limit-activate` — Activate Speed Limit mode
- `tesla-pp-cli vehicles create-speed-limit-clear-pin` — Clear Speed Limit PIN
- `tesla-pp-cli vehicles create-speed-limit-deactivate` — Deactivate Speed Limit mode
- `tesla-pp-cli vehicles create-speed-limit-set-limit` — Set Speed Limit value (mph)
- `tesla-pp-cli vehicles create-steering-wheel-heater-request` — Toggle steering wheel heater
- `tesla-pp-cli vehicles create-sun-roof-control` — Sun roof: state = vent | close
- `tesla-pp-cli vehicles create-trigger-homelink` — Trigger HomeLink garage door
- `tesla-pp-cli vehicles create-wake-up` — Wake the vehicle from sleep
- `tesla-pp-cli vehicles create-window-control` — Vent or close all windows; command = vent | close
- `tesla-pp-cli vehicles get-compose-image` — Compose a rendered image of the configured vehicle
- `tesla-pp-cli vehicles get-data-charge-state` — Charge state subset (faster than full vehicle_data)
- `tesla-pp-cli vehicles get-data-climate-state` — Climate state subset
- `tesla-pp-cli vehicles get-data-drive-state` — Drive state (location, speed, shift) subset
- `tesla-pp-cli vehicles get-data-gui-settings` — GUI settings (units, time format)
- `tesla-pp-cli vehicles get-data-vehicle-config` — Vehicle config (model, options, trim)
- `tesla-pp-cli vehicles get-data-vehicle-state` — Vehicle state (locks, software, sentry) subset
- `tesla-pp-cli vehicles get-mobile-enabled` — Is mobile control enabled in the car
- `tesla-pp-cli vehicles get-nearby-chargers` — Nearby Supercharger sites with stall availability subfield
- `tesla-pp-cli vehicles get-release-notes` — Release notes for the current/queued software update
- `tesla-pp-cli vehicles get-service-data` — Open service appointments and recent service history for this vehicle
- `tesla-pp-cli vehicles get-vehicle-data` — Full vehicle state snapshot: charge, climate, drive, GUI, vehicle config, and software


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tesla-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Snap then ready

```bash
tesla snap --all --json && tesla ready 5YJ3E1EA6XXXXXXXX --json
```

Capture vehicle state for every car on the account, then check whether your primary vehicle is ready to drive.

### Morning ready check

```bash
tesla ready 5YJ3E1EA6XXXXXXXX --json --select ready,blockers
```

One JSON object: is the car ready to drive, and if not, why not. Use `--select` to limit response size for agents.

### Monthly charging-cost report

```bash
tesla cost ledger --since 30d --group supercharger,home --json --select total_usd,sessions,kwh_added,supercharger_ratio
```

Per-month breakdown with dotted-path `--select` so the agent sees only the rollup fields, not every session.

### Preconditioning before leaving

```bash
tesla vehicles create-auto-conditioning-start 5YJ3E1EA6XXXXXXXX --json
```

Start climate preconditioning. Run `tesla ready <vin>` ten minutes later to confirm cabin warmed up.

### Supercharger queue watcher

```bash
tesla supercharger watch 1000 --free-stalls 2 --watch --json
```

Subscribe to free-stall transitions at a saved site; emits one JSON line per transition. Pipe to an agent for alerting.

### Local SOC history via analytics

```bash
tesla-pp-cli analytics --select battery_level,captured_at --limit 30 --json
```

Pull the 30 most recent vehicle-state rows from the local SQLite for trend analysis; same data TeslaMate would surface in Grafana.

### Recipe: Remote unlock from anywhere (Fleet path)

```bash
tesla auth fleet-template --gen-key --dest ./tesla-keys-host
cd tesla-keys-host && vercel deploy --prod
# Register the resulting <your-host>.vercel.app at developer.tesla.com, copy client_id and client_secret.
tesla auth fleet-register --public-key-domain <your-host>.vercel.app --client-id <id> --client-secret-file <secret-file>
tesla auth fleet-login
tesla command unlock --vehicle Stella --send
```

Run once to scaffold the public-key host, deploy it, register at Tesla, and complete user OAuth. After that, `unlock` (and any other signed command) works from anywhere with internet. Per-call cost: $0.001 plus a $0.02 wake if the car is asleep. Inside the $10/mo personal-use credit, that lands at roughly $0 net.

### Recipe: Cheap remote charge control via Hermes

```bash
tesla auth login --via tesla-auth
tesla auth ble-pair --vin <your-vin>
go install github.com/teslamotors/vehicle-command/cmd/tesla-http-proxy@latest
tesla-http-proxy -key-file ~/.config/tesla-pp-cli/private.pem -port 4443 -cert auto &
tesla relay start
tesla command set-charge-limit --vehicle Snowflake --percent 80 --send
```

iOS-app PKCE login, one-time BLE key enrollment at the car, then a local proxy plus relay. Subsequent `tesla command set-charge-limit`, `tesla command climate-on`, `tesla command honk`, and `tesla command media-toggle-playback` ride free over Hermes. Unlock and trunk are not available on this path; for those, use Fleet.

### Recipe: Switching between paths

The `--via` flag picks which transport handles a signed command:

```bash
tesla command set-charge-limit --vehicle Snowflake --percent 80 --send --via hermes
tesla command unlock --vehicle Stella --send --via fleet
tesla command flash-lights --vehicle Snowflake --send --via ble
```

Defaults: pre-2021 vehicles use REST, post-2021 vehicles use whichever signed path is enrolled. If both Hermes and Fleet are enrolled, Hermes wins for infotainment commands (cheaper) and Fleet wins for unlock/lock/trunk (Hermes does not carry these). Override per-call with `--via`.

### Recipe: Deploy your tesla agent to a cloud Mac mini

```bash
# On laptop:
tesla auth export --out tesla-bundle.enc
scp tesla-bundle.enc user@mac-mini:~/

# On Mac mini:
tesla auth import ~/tesla-bundle.enc
tesla command honk --vehicle Snowflake --send --via fleet
```

Export creates an Argon2id plus AES-256-GCM bundle of the REST bearer, the Fleet bearer, and the enrolled ECDSA keypair. Copy it over Tailscale or any secure channel. Import on the remote, enter the passphrase, and the same Fleet creds plus the same enrolled key work from the new host. The car does not need to re-enroll; key identity rides on the keypair.

### Path-decision flowchart

Text form (rendered consistently across SKILL.md viewers):

```
Is the vehicle pre-2021?
  Yes -> REST (default path, no extra setup)
  No  -> Continue.

Is the car within Bluetooth range of this host?
  Yes -> BLE (free, all commands, no internet needed)
  No  -> Continue.

Do you need unlock, lock, trunk, or another non-infotainment signed command?
  Yes -> Fleet API (roughly $0/mo within personal-use credit)
  No  -> Hermes (free, infotainment commands only)

Deploying to a second host (cloud Mac mini, Pi)?
  Use the chosen path above, then `tesla auth export` and `tesla auth import` on the remote.
```

## Auth Setup

Run `tesla auth login` and the CLI opens Tesla's real login page in your browser. Log in there (Tesla owns MFA, captcha, SMS codes - we never see them), Tesla redirects you to a 404 page on auth.tesla.com after success, you paste that URL back here. The CLI extracts the auth code via PKCE, exchanges it for tokens, and stores them in `~/.config/tesla-pp-cli/config.toml` (mode 0600). Bearer tokens last 8h; the CLI silently re-mints them on 401 using the long-lived refresh token, so once you log in you don't need to think about it. Headless / CI fallbacks: `auth login --paste` reads a pre-captured refresh token from stdin, `auth login --refresh-token <tok>` skips stdin, and `auth login --via tesla-auth` subprocesses into adriankumpf/tesla_auth (if you have it installed and prefer the native window). Newer vehicles on Tesla's signed-command protocol need separate key enrollment - see `tesla reachability` for diagnosis.

Run `tesla-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tesla-pp-cli charging --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tesla-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tesla-pp-cli feedback --stdin < notes.txt
tesla-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.tesla-pp-cli/feedback.jsonl`. They are never POSTed unless `TESLA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TESLA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
tesla-pp-cli profile save briefing --json
tesla-pp-cli --profile briefing charging
tesla-pp-cli profile list --json
tesla-pp-cli profile show briefing
tesla-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `tesla-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add tesla-pp-mcp -- tesla-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tesla-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tesla-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tesla-pp-cli <command> --help`.
