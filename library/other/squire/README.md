# Squire CLI

**Every barbershop on Squire, queryable from the terminal — cross-shop price compare, soonest-available barber, and review rankings the website never built.**

squire-pp-cli mirrors Squire's barbershop discovery API into a local SQLite store so you can do what getsquire.com cannot: compare named shops side by side, find the cheapest haircut in a city, rank shops by rating confidence, and watch a shop for price or staff drift. All read-only, no account required.

Learn more at [Squire](https://api.getsquire.com).

Created by [@devbasu](https://github.com/devbasu) (Dev Basu).

## Install

The recommended path installs both the `squire-pp-cli` binary and the `pp-squire` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install squire
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install squire --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install squire --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install squire --agent claude-code
npx -y @mvanhorn/printing-press-library install squire --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/squire/cmd/squire-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/squire-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install squire --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-squire --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-squire --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install squire --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/squire-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/squire/cmd/squire-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "squire": {
      "command": "squire-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Confirm the CLI is wired up and the API is reachable (no auth needed).
squire-pp-cli doctor --dry-run

# Put two shops head to head on price, rating, and staff (live; no sync needed).
squire-pp-cli compare barber-theory-toronto cadmen-barbershop-toronto-toronto --json

# Rank a shop's barbers by their next open slot.
squire-pp-cli soonest barber-theory-toronto --json

# Find the cheapest haircut-category service across the given shops.
squire-pp-cli cheapest Haircut --near barber-theory-toronto --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-shop discovery
- **`soonest`** — Find the barber who can cut your hair soonest across several shops, ranked by next open slot.

  _Reach for this when the user wants the earliest appointment and doesn't care which shop — the website cannot answer this._

  ```bash
  squire-pp-cli soonest --near barber-theory-toronto --service Haircut --agent
  ```
- **`compare`** — Put two or more named shops side by side on average price, rating, review count, and staff size.

  _Use when the user has specific shops in mind and wants a head-to-head; not for ranking an unknown set._

  ```bash
  squire-pp-cli compare barber-theory-toronto another-shop-route --json
  ```
- **`roster`** — Rank the best shops in a city by rating weighted by review volume, with Squire's AI review summary attached.

  _Use when relocating or exploring a new area and you want quality-ranked shops in one view._

  ```bash
  squire-pp-cli roster --city-id 66e194c2-9cc3-4859-b2cf-c3da22df3582 --lat 21.3069 --lon -157.8583 --min-reviews 25 --limit 10 --agent
  ```

### Price intelligence
- **`cheapest`** — Rank shops by the lowest price for one service category (e.g. Haircut) across a city or near a shop.

  _Use when price is the deciding factor for a single service the user wants._

  ```bash
  squire-pp-cli cheapest Haircut --near toronto --limit 10 --json
  ```
- **`watch`** — Snapshot a shop's prices, staff, and rating; on re-run, diff against the last snapshot and show what changed.

  _Use to detect cents-level price moves, added/removed barbers, or rating shifts at one shop over time._

  ```bash
  squire-pp-cli watch barber-theory-toronto --json
  ```

## Recipes


### Find the soonest haircut near a shop

```bash
squire-pp-cli soonest --near barber-theory-toronto --service Haircut --agent
```

Ranks barbers across nearby shops by their next open slot.

### Cheapest Haircut & Beard in a city

```bash
squire-pp-cli cheapest "Haircut & Beard" --near toronto --limit 10 --json
```

Sorts shops by the lowest price for that service category.

### Triage shops in a new city

```bash
squire-pp-cli roster --city-id 66e194c2-9cc3-4859-b2cf-c3da22df3582 --lat 21.3069 --lon -157.8583 --min-reviews 25 --agent --select ranked.name,ranked.rating,ranked.num_ratings
```

Ranks shops by rating confidence and narrows the agent payload to the fields that matter.

### Detect price changes at your usual shop

```bash
squire-pp-cli watch barber-theory-toronto --json
```

Diffs the current snapshot against the last run and reports cents-level changes.

## Usage

Run `squire-pp-cli --help` for the full command reference and flag list.

## Commands

### directory

Operations on public

- **`squire-pp-cli directory`** - GET /v1/search/public

### discover

Operations on shops

- **`squire-pp-cli discover list-city`** - GET /discover/api/city
- **`squire-pp-cli discover list-shops`** - GET /discover/api/shops

### reviews

Operations on shop

- **`squire-pp-cli reviews <shop_id>`** - GET /v1/reviews/shop/{shop_id}

### shop

Operations on details

- **`squire-pp-cli shop get-details`** - GET /v1/shop/{shop_id}/details
- **`squire-pp-cli shop get-next-available-time`** - GET /v1/shop/{shop_id}/barber/{barber_id}/next-available-time
- **`squire-pp-cli shop get-professional`** - GET /v1/shop/{shop_id}/details/professional
- **`squire-pp-cli shop get-service`** - GET /v2/shop/{shop_id}/service
- **`squire-pp-cli shop get-service-2`** - GET /v2/shop/{shop_id}/barber/{barber_id}/service


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
squire-pp-cli directory

# JSON for scripting and agents
squire-pp-cli directory --json

# Filter to specific fields
squire-pp-cli directory --json --select id,name,status

# Dry run — show the request without sending
squire-pp-cli directory --dry-run

# Agent mode — JSON + compact + no prompts in one flag
squire-pp-cli directory --agent
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
squire-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/squire-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A services command returns 500 'invalid input syntax for type uuid'** — Pass the shop UUID, not the slug, to service commands; run 'squire-pp-cli resolve <slug>' to get the UUID (or use the slug-aware compound commands which resolve it for you).
- **Compound commands return empty results** — The compound commands (compare/soonest/cheapest/roster) are live and need no sync; ensure the shop slug or UUID is correct (try 'squire-pp-cli directory list-public --term <name>').
- **Reviews command returns only 5 results** — Reviews paginate via --limit and --skip; raise --limit or page with --skip.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://api.getsquire.com/v1/search/public
- Capture coverage: 9 API entries from 9 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Candidate command ideas: get_details — Derived from observed GET /v1/shop/{shop_id}/details traffic.; get_next_available_time — Derived from observed GET /v1/shop/{shop_id}/barber/{barber_id}/next-available-time traffic.; get_professional — Derived from observed GET /v1/shop/{shop_id}/details/professional traffic.; get_service — Derived from observed GET /v2/shop/{shop_id}/barber/{barber_id}/service traffic.; get_shop — Derived from observed GET /v1/reviews/shop/{shop_id} traffic.; list_city — Derived from observed GET /discover/api/city traffic.; list_public — Derived from observed GET /v1/search/public traffic.; list_shops — Derived from observed GET /discover/api/shops traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
