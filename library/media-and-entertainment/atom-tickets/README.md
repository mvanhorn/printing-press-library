# Atom Tickets CLI

**Find bookable Atom movie tickets, compare real offers, and hand users the official checkout link.**

Search official Atom Partner API inventory and turn it into agent-ready movie plans. Compare prices, accessibility attributes, family constraints, preorders, and last-minute availability without scraping Atom.

## Install

The recommended path installs both the `atom-tickets-pp-cli` binary and the `pp-atom-tickets` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install atom-tickets
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install atom-tickets --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install atom-tickets --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install atom-tickets --agent claude-code
npx -y @mvanhorn/printing-press-library install atom-tickets --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/atom-tickets/cmd/atom-tickets-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/atom-tickets-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install atom-tickets --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-atom-tickets --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-atom-tickets --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install atom-tickets --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/atom-tickets-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ATOM_TICKETS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/atom-tickets/cmd/atom-tickets-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "atom-tickets": {
      "command": "atom-tickets-pp-mcp",
      "env": {
        "ATOM_TICKETS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set ATOM_TICKETS_API_KEY to an approved Atom Tickets Partner API key. The CLI sends it in the documented x-api-key header.

## Quick Start

```bash
# Check Partner API configuration.
atom-tickets-pp-cli doctor --dry-run

# Discover supported nearby venues.
atom-tickets-pp-cli partner get-venue-details-by-location --lat 40.7505 --lon -73.9934 --radius 25 --agent

# Turn official inventory into practical bookable choices.
atom-tickets-pp-cli movie-plan --latitude 40.7505 --longitude -73.9934 --after 18:00 --before 22:00 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Plan a movie night
- **`movie-plan`** — Rank bookable nearby showtimes by distance, start time, price, format, rating, and runtime.

  _Use it when an agent must turn raw Atom inventory into a balanced shortlist._

  ```bash
  atom-tickets-pp-cli movie-plan --latitude 40.7505 --longitude -73.9934 --after 18:00 --before 22:00 --agent
  ```
- **`family-fit`** — Rank showtimes satisfying advisory-rating, runtime, ending-time, distance, and budget constraints.

  _Use it for family suitability and schedule constraints rather than an unconstrained recommendation._

  ```bash
  atom-tickets-pp-cli family-fit --latitude 40.7505 --longitude -73.9934 --ratings G,PG --end-before 21:00 --agent
  ```

### Compare bookable options
- **`deal-finder`** — Find the lowest advertised ticket offers across nearby supported venues.

  _Use it when lowest advertised price is the primary decision._

  ```bash
  atom-tickets-pp-cli deal-finder --latitude 40.7505 --longitude -73.9934 --movie "Superman" --agent
  ```
- **`accessible-showtimes`** — Return only bookable nearby showtimes matching required accessibility or seating attributes.

  _Use it when an accommodation such as captions or descriptive video is mandatory._

  ```bash
  atom-tickets-pp-cli accessible-showtimes --latitude 40.7505 --longitude -73.9934 --attribute "Closed Captioning" --agent
  ```

### Opening-weekend discovery
- **`preorder-radar`** — Surface upcoming preorder days across nearby venues with production and venue names resolved.

  _Use it to discover opening-weekend inventory without repeatedly checking theater pages._

  ```bash
  atom-tickets-pp-cli preorder-radar --latitude 40.7505 --longitude -73.9934 --days 30 --agent
  ```
- **`last-call`** — Find soon-starting showtimes that still report inventory and provide direct checkout URLs.

  _Use it when plans form at the last minute and the user needs immediately bookable choices._

  ```bash
  atom-tickets-pp-cli last-call --latitude 40.7505 --longitude -73.9934 --within 90m --agent --select options.production,options.venue,options.start,options.checkout_url
  ```

## Recipes

### Plan a practical movie night

```bash
atom-tickets-pp-cli movie-plan --latitude 40.7505 --longitude -73.9934 --after 18:00 --before 22:00 --agent
```

Ranks balanced, inventory-backed options.

### Find captioned showtimes

```bash
atom-tickets-pp-cli accessible-showtimes --latitude 40.7505 --longitude -73.9934 --attribute "Closed Captioning" --agent
```

Filters official showtime attributes and inventory.

### Narrow last-minute choices

```bash
atom-tickets-pp-cli last-call --latitude 40.7505 --longitude -73.9934 --within 90m --agent --select options.production,options.venue,options.start,options.checkout_url
```

Returns only the fields an agent needs to present immediate options.

## Usage

Run `atom-tickets-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ATOM_TICKETS_CONFIG_DIR`, `ATOM_TICKETS_DATA_DIR`, `ATOM_TICKETS_STATE_DIR`, or `ATOM_TICKETS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ATOM_TICKETS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ATOM_TICKETS_HOME=/srv/atom-tickets
atom-tickets-pp-cli doctor
```

Under `ATOM_TICKETS_HOME=/srv/atom-tickets`, the four dirs resolve to `/srv/atom-tickets/config`, `/srv/atom-tickets/data`, `/srv/atom-tickets/state`, and `/srv/atom-tickets/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "atom-tickets": {
      "command": "atom-tickets-pp-mcp",
      "env": {
        "ATOM_TICKETS_HOME": "/srv/atom-tickets"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ATOM_TICKETS_DATA_DIR` overrides an explicit `--home` for that kind. Use `ATOM_TICKETS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ATOM_TICKETS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `atom-tickets-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### partner

Manage partner

- **`atom-tickets-pp-cli partner get-production-by-vendor-id`** - Get a production by partner vendor ID
- **`atom-tickets-pp-cli partner get-production-details-by-ids`** - Get production details by IDs
- **`atom-tickets-pp-cli partner get-production-ids-by-venue`** - Get production IDs by venue
- **`atom-tickets-pp-cli partner get-showtime-by-vendor-showtime-id`** - Get a showtime by vendor showtime ID
- **`atom-tickets-pp-cli partner get-showtime-by-vendor-venue-and-showtime-ids`** - Get a showtime by vendor venue and showtime IDs
- **`atom-tickets-pp-cli partner get-showtimes-by-ids`** - Get showtimes by IDs
- **`atom-tickets-pp-cli partner get-showtimes-by-venue`** - Get showtimes by venue
- **`atom-tickets-pp-cli partner get-showtimes-for-venues`** - Get showtimes for multiple venues
- **`atom-tickets-pp-cli partner get-venue-by-vendor-id`** - Get a venue by partner vendor ID
- **`atom-tickets-pp-cli partner get-venue-details-by-ids`** - Get venues by IDs
- **`atom-tickets-pp-cli partner get-venue-details-by-location`** - Get venues by location
- **`atom-tickets-pp-cli partner ping`** - Verify Partner API connectivity
- **`atom-tickets-pp-cli partner search-productions-by-name`** - Search productions by name
- **`atom-tickets-pp-cli partner search-venues-by-name`** - Search venues by name


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`atom-tickets-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`atom-tickets-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`atom-tickets-pp-cli learnings list`** - Inspect taught rows
- **`atom-tickets-pp-cli learnings forget <query>`** - Undo a teach
- **`atom-tickets-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`atom-tickets-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`atom-tickets-pp-cli teach-pattern`** - Install a query/resource template up front
- **`atom-tickets-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ATOM_TICKETS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `atom-tickets-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
atom-tickets-pp-cli partner get-production-by-vendor-id mock-value

# JSON for scripting and agents
atom-tickets-pp-cli partner get-production-by-vendor-id mock-value --json

# Filter to specific fields
atom-tickets-pp-cli partner get-production-by-vendor-id mock-value --json --select id,name,status

# Dry run — show the request without sending
atom-tickets-pp-cli partner get-production-by-vendor-id mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
atom-tickets-pp-cli partner get-production-by-vendor-id mock-value --agent
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
atom-tickets-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `atom-tickets-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/atom-tickets-partner-pp-cli/config.toml`; `--home`, `ATOM_TICKETS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ATOM_TICKETS_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `atom-tickets-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `atom-tickets-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ATOM_TICKETS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The Partner API returns unauthorized or forbidden.** — Confirm ATOM_TICKETS_API_KEY is an active approved Partner API key.
- **A showtime has no checkout target.** — Choose an option whose detailed record includes checkoutUrl; the CLI does not fabricate purchase links.
