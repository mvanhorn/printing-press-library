# Luma CLI

**Browse, search, and export public Luma events from one Go binary — no account, no API key, plus offline full-text search the public API doesn't have.**

luma reads Luma's public discovery surface (api.luma.com) to list events by city and category, fetch full event and calendar details, and mirror it all into a local SQLite store. On top of the raw API it adds what Luma never exposed publicly: full-text search, geo-radius lookup, cross-city aggregation, change tracking, and ICS export — all agent-native with --json and --select.

Learn more at [Luma](https://api.luma.com).

Created by [@richardadonnell](https://github.com/richardadonnell) (richardadonnell).

## Install

The recommended path installs both the `luma-pp-cli` binary and the `pp-luma` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install luma
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install luma --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install luma --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install luma --agent claude-code
npx -y @mvanhorn/printing-press-library install luma --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/cmd/luma-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/luma-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install luma --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-luma --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-luma --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install luma --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/luma-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/cmd/luma-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "luma": {
      "command": "luma-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account or API key required. luma reads Luma's public, read-only discovery API the website itself uses. It cannot create events, manage guests, or read private/subscribed calendars — that is the paid Luma Plus API's job.

## Quick Start

```bash
# health check — confirms the public API is reachable
luma-pp-cli doctor --dry-run

# upcoming events in San Francisco
luma-pp-cli events list --city sf --limit 10

# list topic categories (AI, Crypto, Tech) and their ids
luma-pp-cli discover categories

# events for a category id from the list above
luma-pp-cli events list --category cat-ai --limit 10

# mirror a city's events locally so search/digest/drift work offline
luma-pp-cli sync --resources events --resource-param events:slug=sf

# offline full-text search across everything synced
luma-pp-cli search "summit"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Query the API can't
- **`agenda`** — One flat, date-sorted list of upcoming events across multiple cities and categories at once.

  _Reach for this when the user asks 'what AI events are on this week' across several cities — no single Luma call can answer it._

  ```bash
  luma-pp-cli agenda --city sf --city nyc --category cat-ai --window 7d --agent
  ```
- **`near`** — Find events within N km of a lat/lng, ranked by distance.

  _Use when the user wants events near a point or venue, which no Luma API call supports._

  ```bash
  luma-pp-cli near --lat 37.77 --lng -122.42 --radius-km 5 --window 14d --agent
  ```

### Take it with you
- **`ics`** — Export a synced/filtered set of events to a .ics calendar file your calendar app can import.

  _Use when the user wants events on their own calendar instead of re-reading JSON._

  ```bash
  luma-pp-cli ics --city sf --category cat-ai --window 30d --out ai.ics
  ```

### Track over time
- **`watch`** — Show what changed on synced events since the previous sync — new, removed, filling up, sold out, rescheduled.

  _Reach for this to monitor events over time; a single API call can never show change._

  ```bash
  luma-pp-cli watch --city sf --category cat-ai --agent
  ```
- **`calendars compare`** — Side-by-side upcoming-event counts and total guest counts across several calendars (communities).

  _Use to benchmark which communities are most active or drawing the biggest crowds._

  ```bash
  luma-pp-cli calendars compare cal-AAA cal-BBB cal-CCC --window 14d --agent
  ```

## Recipes


### Event detail, narrowed for an agent

```bash
luma-pp-cli events get --event-id evt-XXXX --agent --select api_id,event.name,event.start_at,event.timezone,guest_count,ticket_count
```

Event detail is large and deeply nested; --select pulls only the fields you need so an agent doesn't burn context.

### AI events across three cities this week

```bash
luma-pp-cli agenda --city sf --city nyc --category cat-ai --window 7d --agent
```

Unions several cities filtered to a category into one flat, date-sorted list the API can't return.

### Events near a point

```bash
luma-pp-cli near --lat 37.77 --lng -122.42 --radius-km 5 --window 14d --agent
```

Haversine radius search over synced event coordinates — no Luma endpoint offers this.

### Export a city to your calendar

```bash
luma-pp-cli ics --city sf --out sf.ics
```

Writes an ICS file you can import into any calendar app.

## Usage

Run `luma-pp-cli --help` for the full command reference and flag list.

## Commands

### calendars

Fetch Luma calendar (community) details

- **`luma-pp-cli calendars`** - Get a calendar (community) by its api_id (cal-...)

### discover

Discover featured events, places, and categories on Luma

- **`luma-pp-cli discover categories`** - List event categories (AI, Crypto, Tech, Arts, ...) with upcoming-event counts
- **`luma-pp-cli discover home`** - Featured place, popular places, categories, and calendars on the Luma discover home

### events

Browse and fetch public Luma events

- **`luma-pp-cli events get`** - Get full details for an event by its api_id (evt-...)
- **`luma-pp-cli events list`** - List upcoming events filtered by city (--city/--place-id) or category (--category)

### places

Browse cities/places that host Luma events

- **`luma-pp-cli places calendars`** - List calendars (communities) active in a place
- **`luma-pp-cli places get`** - Get a place (city) by slug (sf, nyc, miami, ...) or place api_id
- **`luma-pp-cli places map`** - Geographic points for events in a place, for mapping


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
luma-pp-cli calendars --calendar-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
luma-pp-cli calendars --calendar-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
luma-pp-cli calendars --calendar-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
luma-pp-cli calendars --calendar-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
luma-pp-cli calendars --calendar-id 550e8400-e29b-41d4-a716-446655440000 --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
luma-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/luma-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **events near / search / digest / drift return nothing** — Run sync first (e.g. luma-pp-cli sync --resources events --resource-param events:slug=sf) — these commands read the local store.
- **unknown city returns empty** — Use a real Luma city slug (sf, nyc, miami, london); run luma-pp-cli discover home to see places.
- **HTTP 429 / rate limited** — Lower --limit or add --max-scan-pages; the public API throttles aggressive paging.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**luma-events-mcp**](https://github.com/montaguegabe/luma-events-mcp) — TypeScript
- [**Lu.Ma .NET SDK**](https://github.com/Zettersten/Lu.Ma) — C#
- [**luma-ai-mcp-server**](https://github.com/bobtista/luma-ai-mcp-server) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
