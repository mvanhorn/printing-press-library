# Fandango CLI

**Find and compare official Fandango showtimes, then hand users the licensed purchase link.**

Search official Fabric Origin Fandango inventory and turn it into agent-ready movie plans. Compare theaters, formats, dates, and saved interests without scraping Fandango.

## Install

The recommended path installs both the `fandango-pp-cli` binary and the `pp-fandango` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install fandango
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install fandango --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install fandango --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install fandango --agent claude-code
npx -y @mvanhorn/printing-press-library install fandango --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/fandango/cmd/fandango-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fandango-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install fandango --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fandango --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fandango --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install fandango --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fandango-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FANDANGO_SUBSCRIPTION_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/fandango/cmd/fandango-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fandango": {
      "command": "fandango-pp-mcp",
      "env": {
        "FANDANGO_SUBSCRIPTION_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set FANDANGO_SUBSCRIPTION_KEY to a Fabric Origin subscription key approved for the licensed Fandango Showtimes and Ticketing API. Access requires a paid Fabric Origin core subscription and Fandango approval.

## Quick Start

```bash
# Check configuration before using licensed inventory.
fandango-pp-cli doctor --dry-run

# Discover supported nearby theaters.
fandango-pp-cli fandango get-theaters-theaters-get --zip-code 10001 --radius 10 --limit 10 --agent

# Turn inventory into practical bookable choices.
fandango-pp-cli movie-plan --zip-code 10001 --date 2026-07-25 --after 18:00 --before 22:00 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Plan a movie night
- **`movie-plan`** — Rank practical nearby screenings inside a date and time window and return purchase links.

  _Use it when an agent must turn raw inventory into a short list of bookable plans._

  ```bash
  fandango-pp-cli movie-plan --zip-code 10001 --date 2026-07-25 --after 18:00 --before 22:00 --agent
  ```
- **`starting-soon`** — Find nearby screenings beginning within an immediate time window.

  _Use it when the user asks what they can still make tonight._

  ```bash
  fandango-pp-cli starting-soon --zip-code 10001 --within 90m --agent
  ```

### Compare screenings
- **`format-find`** — Compare a movie's available presentation formats across nearby theaters.

  _Use it for IMAX, premium-format, or amenity-first decisions._

  ```bash
  fandango-pp-cli format-find --movie-id 12345 --id-provider fandangoApi --zip-code 10001 --agent
  ```
- **`theater-compare`** — Compare theaters by movie coverage, showtime density, operating span, and ticket-link coverage.

  _Use it when the venue is the decision rather than a particular screening._

  ```bash
  fandango-pp-cli theater-compare --theater-ids 100,200 --date 2026-07-25 --agent
  ```
- **`movie-availability`** — Show where and when one movie is playing, grouped by theater and display date.

  _Use it when an agent needs every usable option for one title._

  ```bash
  fandango-pp-cli movie-availability --movie-id 12345 --id-provider fandangoApi --zip-code 10001 --agent
  ```

### Personal movie routine
- **`watchlist-showtimes`** — Match a local movie watchlist against currently bookable nearby showtimes.

  _Use it for recurring personalized checks that a one-off endpoint cannot answer._

  ```bash
  fandango-pp-cli watchlist-showtimes --zip-code 10001 --movies "The Fantastic Four,Superman" --agent --select matches.title,matches.theater,matches.start,matches.purchase_url
  ```

## Recipes

### Plan tomorrow night

```bash
fandango-pp-cli movie-plan --zip-code 10001 --date 2026-07-25 --after 18:00 --before 22:00 --agent
```

Ranks usable evening choices and includes official ticket links.

### Find something starting soon

```bash
fandango-pp-cli starting-soon --zip-code 10001 --within 90m --agent
```

Filters nearby inventory against the current time.

### Narrow watchlist results

```bash
fandango-pp-cli watchlist-showtimes --zip-code 10001 --movies "The Fantastic Four,Superman" --agent --select matches.title,matches.theater,matches.start,matches.purchase_url
```

Returns only the high-value fields an agent needs.

## Usage

Run `fandango-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FANDANGO_CONFIG_DIR`, `FANDANGO_DATA_DIR`, `FANDANGO_STATE_DIR`, or `FANDANGO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FANDANGO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FANDANGO_HOME=/srv/fandango
fandango-pp-cli doctor
```

Under `FANDANGO_HOME=/srv/fandango`, the four dirs resolve to `/srv/fandango/config`, `/srv/fandango/data`, `/srv/fandango/state`, and `/srv/fandango/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "fandango": {
      "command": "fandango-pp-mcp",
      "env": {
        "FANDANGO_HOME": "/srv/fandango"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FANDANGO_DATA_DIR` overrides an explicit `--home` for that kind. Use `FANDANGO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FANDANGO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `fandango-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### fandango

Manage fandango

- **`fandango-pp-cli fandango get-geo-location-city-geo-location-by-city-get`** - Gets geo-location data for a city.
- **`fandango-pp-cli fandango get-geo-location-postal-code-geo-location-by-postal-code-get`** - Gets geo-location data for a postal code.
- **`fandango-pp-cli fandango get-movie-by-id-movie-by-id-get`** - Gets a movie.
- **`fandango-pp-cli fandango get-movie-display-dates-moviedisplay-dates-get`** - Gets display dates for a movie based on geolocation.
- **`fandango-pp-cli fandango get-movie-showtime-groupings-movieshowtime-groupings-get`** - Gets movie showtimes grouped by date, theater, format, and amenities.
- **`fandango-pp-cli fandango get-movies-movies-get`** - Search for movies available.
- **`fandango-pp-cli fandango get-showtime-by-id-showtime-by-id-get`** - Get showtime details by showtime id.
- **`fandango-pp-cli fandango get-showtimes-showtimes-get`** - Gets showtimes by geolocation/theater and date.
- **`fandango-pp-cli fandango get-theater-display-dates-theaterdisplay-dates-get`** - Gets display dates for a theater.
- **`fandango-pp-cli fandango get-theater-showtime-groupings-theatershowtime-groupings-get`** - Gets theater showtimes grouped by date, movie, format, and amenities.
- **`fandango-pp-cli fandango get-theater-theater-by-id-get`** - Gets a theater.
- **`fandango-pp-cli fandango get-theaters-theaters-get`** - Gets theaters close to a geolocation.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`fandango-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`fandango-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`fandango-pp-cli learnings list`** - Inspect taught rows
- **`fandango-pp-cli learnings forget <query>`** - Undo a teach
- **`fandango-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`fandango-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`fandango-pp-cli teach-pattern`** - Install a query/resource template up front
- **`fandango-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FANDANGO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `fandango-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
fandango-pp-cli fandango get-geo-location-city-geo-location-by-city-get --country example-value --state example-value --city example-value

# JSON for scripting and agents
fandango-pp-cli fandango get-geo-location-city-geo-location-by-city-get --country example-value --state example-value --city example-value --json

# Filter to specific fields
fandango-pp-cli fandango get-geo-location-city-geo-location-by-city-get --country example-value --state example-value --city example-value --json --select id,name,status

# Dry run — show the request without sending
fandango-pp-cli fandango get-geo-location-city-geo-location-by-city-get --country example-value --state example-value --city example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
fandango-pp-cli fandango get-geo-location-city-geo-location-by-city-get --country example-value --state example-value --city example-value --agent
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
fandango-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `fandango-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/fandango-pp-cli/config.toml`; `--home`, `FANDANGO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FANDANGO_SUBSCRIPTION_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `fandango-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `fandango-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FANDANGO_SUBSCRIPTION_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The API returns unauthorized or forbidden.** — Confirm FANDANGO_SUBSCRIPTION_KEY belongs to a paid Fabric Origin account explicitly approved for Fandango.
- **Showtime inventory appears stale.** — Run fandango-pp-cli sync --full; licensed ticketing information should be refreshed at least every 24 hours.
