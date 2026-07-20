# Ikon Pass CLI

Ikon Pass CLI for reservation availability, pass and voucher lookup, multi-season usage history, and offline change tracking.

Created by [@stellato](https://github.com/stellato) (Greg Stellato).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `ikon-pp-cli` binary and the `pp-ikon` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ikon
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ikon --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ikon --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ikon --agent claude-code
npx -y @mvanhorn/printing-press-library install ikon --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/ikon/cmd/ikon-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ikon-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ikon --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ikon --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ikon --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ikon --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
ikon-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ikon-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/ikon/cmd/ikon-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ikon": {
      "command": "ikon-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Ikon uses cookie-session auth minted through Auth0 SSO — there's no API key. The public `resorts` command needs nothing. Everything tied to your account (availability, passes, vouchers, usage) reads your `.ikonpass.com` session cookie: run `ikon-pp-cli auth login --chrome` to import it from Chrome, or install the `press-auth` companion for a controlled one-window capture that survives session expiry. Sessions last roughly a day; re-import when commands start returning 401.

## Quick Start

```bash
# confirm the API is reachable — no login needed
ikon-pp-cli doctor --dry-run

# browse every Ikon destination and its numeric id
ikon-pp-cli resorts

# reservation availability for Jackson Hole (id 14) once you're logged in
ikon-pp-cli availability 14

# rank the resorts you've skied most across all your seasons
ikon-pp-cli most-visited

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### History that compounds
- **`most-visited`** — Rank the resorts you ski most across every season you've held an Ikon Pass, with per-season day counts.

  _Reach for this when the user asks where they ski most, how many days they've used over the years, or for a season-by-season breakdown._

  ```bash
  ikon-pp-cli most-visited --agent
  ```
- **`changes`** — After each sync, report which dates opened or closed at your watched resorts since the last snapshot.

  _Use this to see what reservation availability shifted day-over-day without re-reading every resort manually._

  ```bash
  ikon-pp-cli changes
  ```

### Reservation intelligence
- **`plan`** — Across every reservation-required resort, show which have your target dates open — one call instead of checking four resorts by hand.

  _Use this when the user wants to ski a date range and doesn't care which of the reservable mountains it is._

  ```bash
  ikon-pp-cli plan --from 2026-01-10 --to 2026-01-20
  ```
- **`calendar`** — Month grid of open vs full vs blackout vs closed days for a reservable resort.

  _Use this to eyeball a whole month of reservation openings for one resort at a glance._

  ```bash
  ikon-pp-cli calendar 14 --month 2026-01
  ```
- **`watch`** — Poll a resort+date and alert the moment a currently-full day frees up.

  _Use this when a desired reservation date is full and the user wants to grab it if someone cancels._

  ```bash
  ikon-pp-cli watch 14 2026-01-17
  ```

## Recipes


### Lifetime resort ranking for an agent

```bash
ikon-pp-cli most-visited --agent
```

Returns season-joined day counts per resort as JSON an agent can summarize directly.

### Trip planning across reservable resorts

```bash
ikon-pp-cli plan --from 2026-01-10 --to 2026-01-20
```

Shows which of the reservation-required resorts have open days in the window.

### Narrow resort data with field selection

```bash
ikon-pp-cli resorts --agent --select name,id,reservations_enabled
```

The resorts payload is large; --select trims it to just the fields an agent needs to resolve a name to an id.

### Watch a full date for openings

```bash
ikon-pp-cli watch 14 2026-01-17
```

Polls Jackson Hole for 2026-01-17 and alerts when the date frees up.

## Usage

Run `ikon-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Your account profile

- **`ikon-pp-cli account`** - Your Ikon account profile

### availability

Reservation availability for reservation-required resorts

- **`ikon-pp-cli availability <resort_id>`** - Per-pass reservation availability (open/blackout/closed days, remaining slots) for a resort. Reservation-required ids: Deer Valley=12, Jackson Hole=14, Loon=17, Snoqualmie=29.

### benefits

Per-pass benefits

- **`ikon-pp-cli benefits <product_id>`** - Benefits attached to a specific pass

### passes

Your Ikon passes / products

- **`ikon-pp-cli passes`** - List the passes and products on your account

### reservations

Your current reservations

- **`ikon-pp-cli reservations`** - List your current/upcoming reservations

### resorts

Browse Ikon destinations (public — no login required)

- **`ikon-pp-cli resorts`** - List all Ikon destinations with reservation flags, codes, and regions

### usage

Pass usage / redemption history

- **`ikon-pp-cli usage <product_id>`** - Season redemption history for a pass (resort, days used, redemption dates). Each pass = one season.

### vouchers

Friends & Family shared vouchers

- **`ikon-pp-cli vouchers <product_id>`** - Friends & Family shared-voucher summary (remaining / used) for a pass


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ikon-pp-cli account

# JSON for scripting and agents
ikon-pp-cli account --json

# Filter to specific fields
ikon-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
ikon-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ikon-pp-cli account --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
ikon-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ikon-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ikon-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Commands return 401 / 'must be authenticated'** — Your session expired or wasn't imported — run `ikon-pp-cli auth login --chrome` (or `press-auth login ikonpass.com`).
- **'must be authenticated' even right after logging in** — Ikon keeps the session cookie where the basic reader can't see it; install the press-auth companion: 'go install github.com/mvanhorn/cli-printing-press/v4/cmd/press-auth@latest' then 'press-auth login ikonpass.com --login-url https://account.ikonpass.com/login'.
- **Requests blocked / HTML 'Incapsula incident' page** — The default browser-like User-Agent clears Incapsula; if you overrode it, restore a normal Chrome UA.
- **'ikon-pp-cli most-visited' is empty in summer** — Expected off-season — usage populates once the season's redemptions begin; past seasons appear immediately if their passes are on your account.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ikon-reservations-chrome-extension**](https://github.com/evanjohnso/ikon-reservations-chrome-extension) — JavaScript
- [**ikon**](https://github.com/ninamutty/ikon) — JavaScript
- [**ski-reservation-bot**](https://github.com/jjohnson5253/ski-reservation-bot) — Python
- [**dude_wheres_my_lift_ticket**](https://github.com/jeffgebhardt/dude_wheres_my_lift_ticket) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
