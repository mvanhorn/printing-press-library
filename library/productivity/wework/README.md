# WeWork CLI

**Find and book WeWork day-desks from the terminal — cities, buildings, availability, price, and your bookings — with a local store for instant, agent-native queries.**

WeWork's desk booking is a map-and-modal web app with no CLI and no public API. This CLI reverse-engineers the members-portal read API into scriptable commands: resolve any city, search bookable desks by city + date with credits/price/seat availability, browse buildings and amenities, and list your bookings. Cities and locations sync into a local SQLite store so repeat queries are instant and work offline.

Learn more at [WeWork](https://members.wework.com).

Created by [@ptbyrne](https://github.com/ptbyrne) (Paul Byrne).

## Install

The recommended path installs both the `wework-pp-cli` binary and the `pp-wework` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wework
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wework --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wework --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wework --agent claude-code
npx -y @mvanhorn/printing-press-library install wework --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wework-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install wework --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wework --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wework --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install wework --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wework-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WEWORK_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wework": {
      "command": "wework-pp-mcp",
      "env": {
        "WEWORK_TOKEN": "<access-token>",
        "WEWORK_REFRESH_TOKEN": "<refresh-token>",
        "WEWORK_UUID": "<account-uuid>",
        "WEWORK_MEMBER_TYPE": "<member-type>"
      }
    }
  }
}
```

</details>

## Authentication

WeWork uses an Auth0 access token, rotating refresh token, account UUID, and member type. The recommended windowless-runtime setup is a one-time complete-session import:

```bash
# Easiest on a machine with an existing Chrome login: reads a private on-disk
# LevelDB snapshot; does not open or debug a browser window.
wework-pp-cli auth login --chrome
wework-pp-cli auth refresh --force --json

# Push that locally stored session to a remote headless host over SSH stdin.
# The remote host force-refreshes and becomes the sole token-family owner.
wework-pp-cli auth push --ssh-target user@booking-host

# Portable fallback when the source machine cannot import from Chrome:
wework-pp-cli auth session-import --help     # prints the safe browser capture snippet
pbpaste | wework-pp-cli auth session-import --stdin
wework-pp-cli auth whoami --json     # headless_ready=true means the host can sustain itself
```

`auth session-import` requires the complete four-value renewable bundle by default; `--allow-partial` is an explicit repair mode for an existing installation. The former `auth import` spelling remains an alias for compatibility. After import, normal commands refresh automatically near expiry. Refreshes are serialized across concurrent CLI processes so only one process consumes each rotating token. A required refresh failure stops before the API request and points to `auth refresh --force` instead of sending known-stale credentials.

`WEWORK_TOKEN`, `WEWORK_REFRESH_TOKEN`, `WEWORK_UUID`, and `WEWORK_MEMBER_TYPE` can bootstrap a clean headless host. After the first rotation, the private credential file takes ownership so stale environment values cannot replay an already-consumed refresh token.

`auth login --chrome` reads only actual `members.wework.com` LevelDB records from a private temporary snapshot, binds all four values to that origin, validates the WeWork issuer/client, and works while Chrome is running. It does not open a window or use CDP. Once a complete session is stored locally, `auth push --ssh-target user@booking-host` sends it only over SSH stdin; the remote CLI imports it, force-refreshes it, and verifies `headless_ready=true`. That rotation makes the remote host the sole owner, so stop using the source session afterward. If no local session can be imported, `auth handoff --ssh-target user@booking-host` prints the WeWork login link, four-value capture script, and manual stdin-over-SSH command. The booking host never opens a browser. WeWork has not enabled an OAuth device grant or CLI redirect URI, so a phone-style one-click callback is not available. Running `auth login` without flags only prints these headless-first instructions; `--cdp` remains an explicit last-resort browser fallback.

## Quick Start

```bash
# Health check — confirms the binary and config load (no auth needed). Set up auth with: wework-pp-cli auth session-import --help
wework-pp-cli doctor --dry-run

# List bookable cities. --filter matches city identity fields, not nearby addresses.
wework-pp-cli cities --filter "Austin, TX" --limit 5 --data-source live

# Show your upcoming desk bookings
wework-pp-cli bookings

# Search bookable desks in a city on a date
wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Full-text search across every synced WeWork city and building, offline and instant.

  _Reach for this to resolve a place name to concrete city/building UUIDs without hitting the network._

  ```bash
  wework-pp-cli search "austin" --agent
  ```
- **`desks`** — Sort desks by credits or price and filter to only those with open seats.

  _Answers 'what is the cheapest available desk here tomorrow' in one call._

  ```bash
  wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --sort credits --available-only
  ```

### Agent-native plumbing
- **`auth session-import` / `auth refresh` / `auth push` / `auth handoff`** — Bootstrap, remotely transfer, and sustain a rotating Auth0 session on a headless booking host without passing secrets in command arguments.

  _Use `push` when this machine already has a complete session; use `handoff` for the manual capture fallback._

  ```bash
  wework-pp-cli auth push --ssh-target user@booking-host
  wework-pp-cli auth handoff --ssh-target user@booking-host
  wework-pp-cli auth refresh --force --json
  ```
- **`desks`** — Search bookable desks by city name and date, deriving the map bounding box the API requires from cached city geo.

  _The headline command: turns a plain city name + validated YYYY-MM-DD date into structured desk availability with price, seats, and truthful live-source metadata._

  ```bash
  wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --agent
  ```

## Recipes

### Bootstrap a renewable remote booking host

```bash
wework-pp-cli auth login --chrome
wework-pp-cli auth push --ssh-target user@booking-host
```

`auth push` reads the complete local credential bundle, pipes it directly to the remote `auth session-import --stdin`, forces one remote refresh, and fails unless the remote status is both renewable and headless-ready. No secret enters command arguments, environment variables, output, or a transfer file. The forced rotation transfers ownership to the booking host, so do not continue using the source session. If local Chrome import is unavailable, use `auth handoff --ssh-target user@booking-host` for the manual capture workflow instead.

### Cheapest available desk in a city

```bash
wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --sort credits --available-only --agent
```

Ranks bookable desks by credits and drops any with no open seats.

### Compact desk view for agents

```bash
wework-pp-cli desks --city "New York, NY" --date 2026-08-18 --agent --select desks.location.name,desks.credits,desks.seat.available
```

Returns only high-gravity fields from the deeply-nested desk response to keep agent context small.

### Resolve a place to UUIDs offline

```bash
wework-pp-cli search "barton springs" --json
```

Finds the matching city/building from the local store without a network call.

## Usage

Run `wework-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `WEWORK_CONFIG_DIR`, `WEWORK_DATA_DIR`, `WEWORK_STATE_DIR`, or `WEWORK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `WEWORK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export WEWORK_HOME=/srv/wework
wework-pp-cli doctor
```

Under `WEWORK_HOME=/srv/wework`, the four dirs resolve to `/srv/wework/config`, `/srv/wework/data`, `/srv/wework/state`, and `/srv/wework/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "wework": {
      "command": "wework-pp-mcp",
      "env": {
        "WEWORK_HOME": "/srv/wework"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `WEWORK_DATA_DIR` overrides an explicit `--home` for that kind. Use `WEWORK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `WEWORK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `wework-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### common-booking

Manage common booking

- **`wework-pp-cli common-booking book-desk`** - Creates a real desk reservation and charges the member's saved payment account. Verified against a live booking on 2026-08-12 ($47). Recommended flow: quote first, then book. Returns ReservationID + BookingStatus. The generated command should default to --dry-run; require an explicit confirm flag to charge.
- **`wework-pp-cli common-booking cancel-booking`** - Cancels an existing booking. Verified against a live cancellation on 2026-08-12 (full $47 refund confirmed). Full refund if canceled >=24h before start time. Identify the booking with bookingId + reservationId (from list-bookings / bookDesk).
- **`wework-pp-cli common-booking get-booking-details`** - [BEST-EFFORT/UNVERIFIED] Get booking details
- **`wework-pp-cli common-booking list-bookings`** - List my upcoming desk bookings
- **`wework-pp-cli common-booking quote-booking`** - Returns the price breakdown (grandTotal, subTotal, taxes, lineItems) for a prospective desk booking. Safe — no reservation is created and no card is charged. Verified against a live booking on 2026-08-12.

### spaces

Manage spaces

- **`wework-pp-cli spaces list-locations`** - List WeWork buildings in a city with availability
- **`wework-pp-cli spaces search-desks`** - The primary desk-search endpoint. Returns bookable day-desks for a city/date, with capacity, credits, price, seat availability, hours and cancellation policy. Narrow to specific buildings with locationUUIDs; bounds+city define the area.

### wework-yardi

Manage wework yardi

- **`wework-pp-cli wework-yardi get-city-details`** - Get details for a city
- **`wework-pp-cli wework-yardi list-amenities`** - List amenities for locations
- **`wework-pp-cli wework-yardi list-cities`** - Returns every city where WeWork desks can be booked (~842), each with its market geo (latitude/longitude) and nearest location. The geo is used to derive the bounding box that the desk/location search endpoints require.
- **`wework-pp-cli wework-yardi list-locations-by-geo`** - List WeWork buildings within a geo bounding box


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`wework-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`wework-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`wework-pp-cli learnings list`** - Inspect taught rows
- **`wework-pp-cli learnings forget <query>`** - Undo a teach
- **`wework-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`wework-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`wework-pp-cli teach-pattern`** - Install a query/resource template up front
- **`wework-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `WEWORK_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `wework-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wework-pp-cli common-booking get-booking-details

# JSON for scripting and agents
wework-pp-cli common-booking get-booking-details --json
# Filter to specific fields by name
wework-pp-cli common-booking get-booking-details --json --select <field>[,<field>...]

# Dry run — show the request without sending
wework-pp-cli common-booking get-booking-details --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wework-pp-cli common-booking get-booking-details --agent

# CSV rows from the friendly location and desk commands
wework-pp-cli locations --city "Austin, TX" --date 2026-08-18 --csv
wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --csv

# Suppress successful output
wework-pp-cli locations --city "Austin, TX" --quiet
```

The friendly `cities`, `locations`, `desks`, and `bookings` aliases are live-only. They accept `--data-source auto` or `--data-source live`, reject `--data-source local` before making a request, and report `meta.source=live` in agent output. `locations` and `desks` validate `--date` as `YYYY-MM-DD` before any network call.

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
wework-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `wework-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/wework-workplaceone-pp-cli/config.toml`; `--home`, `WEWORK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WEWORK_TOKEN` | per_call | Yes | Set to your API credential. |
| `WEWORK_REFRESH_TOKEN` | bootstrap | No | Rotating refresh token. On first successful rotation, the private credential file owns the replacement. |
| `WEWORK_UUID` | per_call | Yes | Account UUID sent as the `weworkuuid` header. |
| `WEWORK_MEMBER_TYPE` | per_call | Yes | Member type sent as the `weworkmembertype` header. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `wework-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `wework-pp-cli doctor` to check credentials
- Run `wework-pp-cli auth whoami --json` and check `headless_ready`, `request_ready`, `refresh_required`, and `renewable`; do not print token values while troubleshooting.
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / empty results on every command** — Run `wework-pp-cli auth whoami --json`. If `renewable` is true, run `wework-pp-cli auth refresh --force`; otherwise import a complete session with `auth session-import --stdin` or use `auth handoff` from another computer.
- **desks returns nothing for a valid city** — Run 'wework-pp-cli sync' first so the city geo is cached, then retry — the search bounding box is derived from cached city coordinates.
- **book fails or returns an unexpected shape** — The book/cancel endpoints are inferred from discovery, not verified. Run with --dry-run to inspect the request, then verify against your account before using --confirm.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `WEWORK_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
