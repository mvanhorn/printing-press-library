# Tesla CLI

**Every Tesla mobile-app feature, plus a charging-cost ledger and supercharger queue watcher no other Tesla CLI ships.**

The Tesla owner-API as JSON-first commands, with a local SQLite that remembers every vehicle state. Get a single `ready` yes/no for departure, a tariff-aware charging cost ledger that replaces TeslaMate's Docker stack, and a supercharger queue watcher you can subscribe to from an agent. For 2018+ Models S/X and pre-2021 Models 3/Y, commands route end-to-end on plain REST; newer vehicles get a clear shim message via `tesla reachability`.

Learn more at [Tesla](https://owner-api.teslamotors.com).

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `tesla-pp-cli` binary and the `pp-tesla` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tesla
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tesla --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tesla --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tesla --agent claude-code
npx -y @mvanhorn/printing-press-library install tesla --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/tesla/cmd/tesla-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tesla-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tesla --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-tesla --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install tesla --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tesla-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TESLA_AUTH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tesla": {
      "command": "tesla-pp-mcp",
      "env": {
        "TESLA_AUTH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Run `tesla auth login` and the CLI opens Tesla's real login page in your browser. Log in there (Tesla owns MFA, captcha, SMS codes - we never see them), Tesla redirects you to a 404 page on auth.tesla.com after success, you paste that URL back here. The CLI extracts the auth code via PKCE, exchanges it for tokens, and stores them in `~/.config/tesla-pp-cli/config.toml` (mode 0600). Bearer tokens last 8h; the CLI silently re-mints them on 401 using the long-lived refresh token, so once you log in you don't need to think about it. Headless / CI fallbacks: `auth login --paste` reads a pre-captured refresh token from stdin, `auth login --refresh-token <tok>` skips stdin, and `auth login --via tesla-auth` subprocesses into adriankumpf/tesla_auth (if you have it installed and prefer the native window). Newer vehicles on Tesla's signed-command protocol need separate key enrollment - see `tesla reachability` for diagnosis.

## Signed-command paths overview

Teslas built in 2021 and later (Model 3/Y from 2021, Model S/X from late 2022, Cybertruck, all Model 3 Highland) do not accept plain REST commands at the owner-API endpoint. They require Tesla's Vehicle Command Protocol (VCP), where every command is signed with an ECDSA key the car has explicitly enrolled. The signing happens client-side; only the signed request reaches Tesla's edge.

VCP has three transports, and this CLI supports all three. Pick the one that matches your hardware, network position, and budget.

| Transport | Reach | Cost | Command coverage | Setup time |
|-----------|-------|------|------------------|------------|
| BLE | within roughly 30ft of the car | $0 | every signed command | 10 min |
| Hermes (signaling.vn.teslamotors.com) | anywhere with internet | $0 (uses iOS-app OAuth) | infotainment only (charge, climate, honk, media) | 5 min |
| Fleet API (developer.tesla.com) | anywhere with internet | roughly $0/mo inside Tesla's $10/mo personal-use credit | every signed command including unlock/lock/trunk | 30 min |

BLE needs a laptop, Raspberry Pi, or other Linux/macOS host that can sit within Bluetooth range of the parked car. Hermes is Tesla's own mobile-app signaling backend, the same backbone the iOS app uses for push commands; this CLI reuses your iOS-app OAuth bearer to reach it. Fleet API is Tesla's official partner-app surface, registered through developer.tesla.com.

### Officially supported vs pragmatic

Fleet API is Tesla's official developer surface, documented, billed, and stable. The Hermes path uses an iOS-app bearer credential reaching Tesla's mobile-app signaling backend. The Hermes path works today but is unofficial; Tesla could change or remove it without notice. Use Fleet for commands you depend on (unlock, lock, daily automations). Use Hermes for routine infotainment commands where you would rather not pay per call.

## Quick start: Hermes free path

Free internet control for charge, climate, honk, and media commands. Uses your iOS-app OAuth, no Fleet registration, no per-call billing.

1. `tesla auth login --via tesla-auth`

   iOS-app PKCE flow. Stores the bearer this CLI needs for both REST and Hermes.

2. `tesla auth ble-pair --vin <your-vin>`

   Enroll this CLI's public key on the car over BLE. Run from a laptop within roughly 30ft of the car. The car prompts for an NFC tap to confirm.

3. Install and run tesla-http-proxy:

   ```bash
   go install github.com/teslamotors/vehicle-command/cmd/tesla-http-proxy@latest
   tesla-http-proxy -key-file ~/.config/tesla-pp-cli/private.pem -port 4443 -cert auto
   ```

4. `tesla relay start && tesla command set-charge-limit --vehicle <name> --percent 80 --send`

   The relay forwards signed commands through Hermes. Replace `<name>` with the friendly name the CLI prints under `tesla products`.

## Quick start: Fleet API path

Full coverage including unlock, lock, trunk, and any other signed command, callable from anywhere with internet. Roughly $0/mo for typical hobbyist use after the $10/mo personal-use credit.

1. `tesla auth fleet-template --gen-key --dest ./tesla-keys-host`

   Scaffolds the public-key host (a Vercel project that serves `/.well-known/appspecific/com.tesla.3p.public-key.pem`) and generates an ECDSA P-256 keypair under `~/.config/tesla-pp-cli/`.

2. `cd tesla-keys-host && vercel deploy --prod`

   Deploys the host. Vercel returns the public hostname (something like `<your-host>.vercel.app`). Tesla resolves that hostname to fetch your public key during partner registration and during every signed command.

3. Register your app at developer.tesla.com.

   On the partner account dashboard, register a new app with allowed origin and redirect set to `https://<your-host>.vercel.app`, and copy the `client_id` and `client_secret` Tesla shows you.

4. `tesla auth fleet-register --public-key-domain <your-host>.vercel.app --client-id <id> --client-secret-file <secret-file>`

   Registers your `partner_accounts` entry with Tesla. After this, Tesla's API recognizes your public key for signed commands.

5. `tesla auth fleet-login`

   Browser-based user OAuth, callback on `localhost:8585`. Saves the Fleet user bearer token alongside the REST bearer.

6. `tesla command unlock --vehicle <name> --send`

   Signed remote unlock from anywhere with internet.

## Choosing your path

| Situation | Path |
|-----------|------|
| Pre-2021 Tesla | REST (existing default, no extra setup) |
| Want $0 cost and your car is at home | BLE (run from a laptop or Pi within Bluetooth range) |
| Want $0 and remote control of infotainment commands only | Hermes |
| Want full coverage including unlock from anywhere | Fleet API |
| Want to deploy your agent to a cloud Mac mini | Fleet plus `tesla auth export` then `tesla auth import` |

## Costs

Quoted from developer.tesla.com, May 2026.

- Fleet API: $0.001 per command, $0.02 per wake
- $10/month personal-use credit covers roughly 100 commands plus 2 wakes per day for two vehicles
- Hermes: $0 (uses your iOS-app OAuth)
- BLE: $0 (free forever)

A typical hobbyist driving one or two vehicles lands under $1/mo gross, $0 net after the personal-use credit.

Brokers (Teslemetry, Tessie, TeslaFi) are listed here only for completeness. They are not in the default path. Each broker holds your signing key on your behalf, which the CLI avoids by design; the BYOK (bring your own key) model in this CLI keeps the private key on your host.

### BLE virtual-key enrollment

`tesla auth ble-pair --vin <your-vin>` runs the full BLE handshake: it subprocesses into `tesla-control` (from teslamotors/vehicle-command), points it at the keypair under `~/.config/tesla-pp-cli/`, and waits for the car to prompt for an NFC tap. Bring a Tesla-issued NFC card or the phone key from the iOS app, tap the center console when prompted, and the car records this CLI's public key as a virtual key.

You need the laptop within roughly 30ft of the car for this one-time enrollment. Once enrolled, the same key signs commands over BLE (proximity) and over Fleet API (internet). Hermes uses the same key once `fleet-register` has uploaded it.

### Multi-machine deployments

When you want the same CLI usable from your laptop and from a cloud Mac mini (or a Raspberry Pi, or both), export the credentials and the enrolled key as one bundle and import on the second host.

- `tesla auth export --out tesla-bundle.enc`

  Produces a passphrase-encrypted bundle (Argon2id KDF, AES-256-GCM cipher). The bundle contains the REST bearer, the Fleet bearer if present, and the ECDSA keypair.

- Copy `tesla-bundle.enc` to the remote (scp over Tailscale, or any other secure channel).

- `tesla auth import tesla-bundle.enc` on the remote, then enter the passphrase.

The same Fleet creds and the same enrolled key now work on both machines. The car still recognizes the key because key identity is on the keypair, not the host.

## Quick Start

```bash
# Confirm auth and list your vehicle_id / VINs / energy sites. Run 'tesla auth login' first if needed (see Auth section above).
tesla products --json

# Confirm auth works and list your vehicle_id / VINs / energy site IDs.
tesla products --json

# Classify each vehicle: REST-OK (legacy) or signed-command-required (newer).
tesla reachability --json

# Capture a vehicle_data snapshot for every car; populates the local store for analytics.
tesla snap --all --json

# Single yes/no "can I leave?" with the blocker list.
tesla ready 5YJ3E1EA6XXXXXXXX --json

# Last 30 days of charging cost, home vs Supercharger split.
tesla cost ledger --since 30d --json

```

## Unique Features

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

## Usage

Run `tesla-pp-cli --help` for the full command reference and flag list.

## Commands

### charging

Supercharger queue position and charging status

- **`tesla-pp-cli charging`** - Current Supercharger queue position (which stall, ETA)

### diagnostics

Vehicle diagnostic feature config

- **`tesla-pp-cli diagnostics`** - Vehicle diagnostic feature flags

### energy_sites

Powerwall and solar energy site config

- **`tesla-pp-cli energy_sites create-set-backup-reserve`** - Set Powerwall backup reserve percent
- **`tesla-pp-cli energy_sites create-set-operation`** - Set Powerwall operation mode
- **`tesla-pp-cli energy_sites get-calendar-history`** - Historical energy data per day/month
- **`tesla-pp-cli energy_sites get-live-status`** - Real-time Powerwall solar/grid/battery power
- **`tesla-pp-cli energy_sites get-rate-tariffs`** - Utility rate plans for Powerwall scheduling
- **`tesla-pp-cli energy_sites get-site-info`** - Energy site config (capacity, type, address)
- **`tesla-pp-cli energy_sites get-site-status`** - Backup-reserve and operation mode

### logs

Client-side telemetry sink

- **`tesla-pp-cli logs`** - Forward client-side log events (mostly internal use)

### notification_preferences

Push notification settings

- **`tesla-pp-cli notification_preferences`** - Update push notification preferences

### products

Vehicles, Powerwalls, and solar products owned by the user

- **`tesla-pp-cli products`** - List vehicles, Powerwalls, and solar products owned

### users

Account config, feature flags, orders, and key/credential management

- **`tesla-pp-cli users create-keys`** - Add a virtual phone key or BLE key
- **`tesla-pp-cli users get-app-config`** - Mobile app feature flags and runtime config
- **`tesla-pp-cli users get-feature-config`** - Account-level feature gating
- **`tesla-pp-cli users get-orders`** - Tesla orders (new vehicle deliveries, accessory purchases)

### vehicles

Per-vehicle data, climate, locks, remote start, and Hermes telemetry token

- **`tesla-pp-cli vehicles create-actuate-trunk`** - Open/close trunk or frunk; which_trunk = front | rear
- **`tesla-pp-cli vehicles create-add-charge-schedule`** - Add a charging schedule (location + time window)
- **`tesla-pp-cli vehicles create-add-precondition-schedule`** - Add a preconditioning schedule
- **`tesla-pp-cli vehicles create-adjust-volume`** - Set volume to specific level
- **`tesla-pp-cli vehicles create-auto-conditioning-start`** - POST /api/1/vehicles/{vehicle_id}/command/auto_conditioning_start
- **`tesla-pp-cli vehicles create-auto-conditioning-stop`** - POST /api/1/vehicles/{vehicle_id}/command/auto_conditioning_stop
- **`tesla-pp-cli vehicles create-bioweapon-mode`** - Toggle bioweapon defense mode (HEPA recirculation)
- **`tesla-pp-cli vehicles create-cancel-software-update`** - Cancel pending software update
- **`tesla-pp-cli vehicles create-charge-max-range`** - Set charge limit to Max Range (100%)
- **`tesla-pp-cli vehicles create-charge-port-door-close`** - Close the charge port
- **`tesla-pp-cli vehicles create-charge-port-door-open`** - Open the charge port
- **`tesla-pp-cli vehicles create-charge-standard`** - Set charge limit to Standard (~90%)
- **`tesla-pp-cli vehicles create-charge-start`** - Start charging
- **`tesla-pp-cli vehicles create-charge-stop`** - Stop charging
- **`tesla-pp-cli vehicles create-climate-keeper-mode`** - Climate keeper mode: 0=off, 1=on, 2=dog, 3=camp
- **`tesla-pp-cli vehicles create-door-lock`** - POST /api/1/vehicles/{vehicle_id}/command/door_lock
- **`tesla-pp-cli vehicles create-door-unlock`** - POST /api/1/vehicles/{vehicle_id}/command/door_unlock
- **`tesla-pp-cli vehicles create-erase-user-data`** - Erase guest-session user data
- **`tesla-pp-cli vehicles create-flash-lights`** - Flash headlights
- **`tesla-pp-cli vehicles create-guest-mode`** - Enable/disable Guest mode
- **`tesla-pp-cli vehicles create-hermes`** - POST /api/1/vehicles/{vehicle_id}/jwt/hermes
- **`tesla-pp-cli vehicles create-honk-horn`** - Honk the horn
- **`tesla-pp-cli vehicles create-max-defrost`** - Toggle max defrost
- **`tesla-pp-cli vehicles create-media-next-favorite`** - Next favorite station
- **`tesla-pp-cli vehicles create-media-next-track`** - Next track
- **`tesla-pp-cli vehicles create-media-prev-favorite`** - Previous favorite station
- **`tesla-pp-cli vehicles create-media-prev-track`** - Previous track
- **`tesla-pp-cli vehicles create-media-toggle-playback`** - Toggle play/pause
- **`tesla-pp-cli vehicles create-media-volume-down`** - Volume down
- **`tesla-pp-cli vehicles create-media-volume-up`** - Volume up
- **`tesla-pp-cli vehicles create-navigation-gps-request`** - Send lat/lng directly to navigation
- **`tesla-pp-cli vehicles create-remote-start-drive`** - POST /api/1/vehicles/{vehicle_id}/command/remote_start_drive
- **`tesla-pp-cli vehicles create-remove-charge-schedule`** - Remove charging schedule by id
- **`tesla-pp-cli vehicles create-remove-precondition-schedule`** - Remove preconditioning schedule by id
- **`tesla-pp-cli vehicles create-reset-valet-pin`** - Reset Valet PIN
- **`tesla-pp-cli vehicles create-schedule-software-update`** - Schedule the pending software update
- **`tesla-pp-cli vehicles create-seat-heater-request`** - Set seat heater level (0-3) at a seat position (0=driver, 1=passenger, 2-5=rear)
- **`tesla-pp-cli vehicles create-set-charge-limit`** - Set charge limit percent (50-100)
- **`tesla-pp-cli vehicles create-set-charging-amps`** - Set charging amps draw
- **`tesla-pp-cli vehicles create-set-sentry-mode`** - Enable/disable Sentry mode
- **`tesla-pp-cli vehicles create-set-temps`** - Set driver and passenger climate temps (Celsius)
- **`tesla-pp-cli vehicles create-set-valet-mode`** - Enable/disable Valet mode with optional PIN
- **`tesla-pp-cli vehicles create-share`** - Send navigation destination to the car (address or coordinates)
- **`tesla-pp-cli vehicles create-speed-limit-activate`** - Activate Speed Limit mode
- **`tesla-pp-cli vehicles create-speed-limit-clear-pin`** - Clear Speed Limit PIN
- **`tesla-pp-cli vehicles create-speed-limit-deactivate`** - Deactivate Speed Limit mode
- **`tesla-pp-cli vehicles create-speed-limit-set-limit`** - Set Speed Limit value (mph)
- **`tesla-pp-cli vehicles create-steering-wheel-heater-request`** - Toggle steering wheel heater
- **`tesla-pp-cli vehicles create-sun-roof-control`** - Sun roof: state = vent | close
- **`tesla-pp-cli vehicles create-trigger-homelink`** - Trigger HomeLink garage door
- **`tesla-pp-cli vehicles create-wake-up`** - Wake the vehicle from sleep
- **`tesla-pp-cli vehicles create-window-control`** - Vent or close all windows; command = vent | close
- **`tesla-pp-cli vehicles get-compose-image`** - Compose a rendered image of the configured vehicle
- **`tesla-pp-cli vehicles get-data-charge-state`** - Charge state subset (faster than full vehicle_data)
- **`tesla-pp-cli vehicles get-data-climate-state`** - Climate state subset
- **`tesla-pp-cli vehicles get-data-drive-state`** - Drive state (location, speed, shift) subset
- **`tesla-pp-cli vehicles get-data-gui-settings`** - GUI settings (units, time format)
- **`tesla-pp-cli vehicles get-data-vehicle-config`** - Vehicle config (model, options, trim)
- **`tesla-pp-cli vehicles get-data-vehicle-state`** - Vehicle state (locks, software, sentry) subset
- **`tesla-pp-cli vehicles get-mobile-enabled`** - Is mobile control enabled in the car
- **`tesla-pp-cli vehicles get-nearby-chargers`** - Nearby Supercharger sites with stall availability subfield
- **`tesla-pp-cli vehicles get-release-notes`** - Release notes for the current/queued software update
- **`tesla-pp-cli vehicles get-service-data`** - Open service appointments and recent service history for this vehicle
- **`tesla-pp-cli vehicles get-vehicle-data`** - Full vehicle state snapshot: charge, climate, drive, GUI, vehicle config, and software

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tesla-pp-cli charging

# JSON for scripting and agents
tesla-pp-cli charging --json

# Filter to specific fields
tesla-pp-cli charging --json --select id,name,status

# Dry run — show the request without sending
tesla-pp-cli charging --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tesla-pp-cli charging --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
tesla-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/tesla-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TESLA_AUTH_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tesla-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TESLA_AUTH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`tesla command ...` returns `vehicle command protocol required`** — Your vehicle is on Tesla's new signed-command protocol. Run `tesla doctor` for enrollment URL; install `tesla-control` from teslamotors/vehicle-command for command operations
- **All commands return 401 token expired** — Auto-refresh should handle this transparently. If it doesn't (refresh token revoked), run `tesla auth login` to re-authenticate. To disable auto-refresh entirely, set `TESLA_PP_NO_AUTOREFRESH=1`.
- **vehicle_data calls drain the 12V battery** — Run snapshots on a cron with `tesla snap --all` and avoid keeping the car awake unnecessarily; check `tesla doctor` for asleep-state behavior and battery drain mitigation
- **`tesla cost ledger` shows $0 for Supercharger sessions** — Run `tesla snap --all` to refresh vehicle_data, then `tesla timeline` to stitch new charge sessions into `tesla_charges`; Supercharger pricing may also lag in the upstream CHARGING_HISTORY by up to 24h

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://owner-api.teslamotors.com/api/1/logs
- Capture coverage: 58 API entries from 60 total network entries
- Reachability: standard_http (95% confidence)
- Protocols: firebase (75% confidence), rest_json (75% confidence)
- Auth signals: bearer_token — headers: authorization; cookie — cookies: _abck, bm_sz; api_key — query: key
- Generation hints: browser_clearance_required, requires_browser_auth
- Candidate command ideas: create_auto_conditioning_start — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/command/auto_conditioning_start traffic.; create_auto_conditioning_stop — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/command/auto_conditioning_stop traffic.; create_diagnostic — Derived from observed POST /mobile-app/macgyver/diagnostic traffic.; create_door_lock — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/command/door_lock traffic.; create_door_unlock — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/command/door_unlock traffic.; create_fireperf:fetch — Derived from observed POST /v1/projects/tesla-prod/namespaces/fireperf:fetch traffic.; create_hermes — Derived from observed POST /api/{id}/vehicles/5YJ3E1EA6XXXXXXXX/jwt/hermes traffic.; create_keys — Derived from observed POST /api/{id}/users/keys traffic.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**teslamate-org/teslamate**](https://github.com/teslamate-org/teslamate) — Elixir (8000 stars)
- [**timdorr/tesla-api**](https://github.com/timdorr/tesla-api) — Ruby (2100 stars)
- [**teslamotors/vehicle-command (tesla-control)**](https://github.com/teslamotors/vehicle-command) — Go (650 stars)
- [**tdorssers/TeslaPy**](https://github.com/tdorssers/TeslaPy) — Python (416 stars)
- [**jonasman/TeslaSwift**](https://github.com/jonasman/TeslaSwift) — Swift (255 stars)
- [**cobanov/teslamate-mcp**](https://github.com/cobanov/teslamate-mcp) — Python (127 stars)
- [**scald/tesla-mcp**](https://github.com/scald/tesla-mcp) — Rust (14 stars)
- [**gak/teslatte**](https://github.com/gak/teslatte) — Rust (7 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
