# Hotelist CLI

**Every Hotelist feature, plus a local SQLite mirror that ranks rating-per-dollar across whole countries, compares chains head-to-head, and tracks how a city's hotels change over time.**

Hotelist.com (by @levelsio) rates hotels with AI: it reads real traveler reviews, scores actual room photos, and photo-verifies claimed amenities so you can filter on a gym with real weights or a bathtub that actually exists. This CLI puts that data in your terminal and in agent context with --json/--select output, an offline cache, and cross-location commands the single-map website can't express: rank-country, chain-compare, corridor, and a watch/diff drift tracker. Data is scraped from Hotelist (community/AI-rated, not an official API).

## Install

The recommended path installs both the `hotelist-pp-cli` binary and the `pp-hotelist` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install hotelist
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install hotelist --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install hotelist --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install hotelist --agent claude-code
npx -y @mvanhorn/printing-press-library install hotelist --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotelist/cmd/hotelist-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hotelist-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install hotelist --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hotelist --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hotelist --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install hotelist --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hotelist-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotelist/cmd/hotelist-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hotelist": {
      "command": "hotelist-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check: confirms Hotelist.com is reachable before you search.
hotelist-pp-cli doctor --dry-run

# AI-rated hotels in a city, sorted by Hotelist Score.
hotelist-pp-cli search bangkok --json

# Photo-verified amenities: a real weightlifting gym and a pool.
hotelist-pp-cli filter lisbon --gym-weights --pool --json

# Best rating-per-dollar in a location.
hotelist-pp-cli value tulum --json

# National value leaderboard the website's map UI can't produce.
hotelist-pp-cli rank-country portugal --min-rating 8 --max-price 150 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-location value the map UI can't express
- **`rank-country`** — Rank the best hotels across an entire country by Hotelist rating-per-dollar, with compound amenity and price filters.

  _When an agent needs the single best-value hotel in a whole country under hard constraints, reach for this instead of paging city-by-city search._

  ```bash
  hotelist-pp-cli rank-country thailand --min-rating 8 --max-price 150 --amenities pool,coworking --json
  ```
- **`chain-compare`** — Compare hotel chains head-to-head on mean rating, median price, and rating-per-dollar in a country.

  _Answers 'which chain is actually worth it here?' in one call instead of a dozen manual map filters._

  ```bash
  hotelist-pp-cli chain-compare --chains marriott,hilton,hyatt --country japan --metric best-value --json
  ```
- **`corridor`** — Find the best hotel in each stop of a multi-city route in one pass, with shared filters.

  _Plans a nomad's annual route in one command; pick this over N separate searches when the user names several cities._

  ```bash
  hotelist-pp-cli corridor --cities "Chiang Mai,Lisbon,Medellin" --min-rating 7.5 --max-price 120 --amenities coworking --json
  ```

### Local state that compounds
- **`watch`** — Snapshot a saved location over time and diff which hotels improved, declined, or changed price since you last checked.

  _The only way to answer 'did this city's hotels get better or more expensive since last time?' — pick it for any change-over-time question._

  ```bash
  hotelist-pp-cli watch diff lisbon --since 2026-01-01 --metric both --json
  ```
- **`chain-consistency`** — Compute mean, median, and spread of a single chain's ratings across a country to see if the brand is reliably good or full of outliers.

  _Use before trusting a loyalty brand in an unfamiliar region; surfaces hidden outlier risk a single listing can't show._

  ```bash
  hotelist-pp-cli chain-consistency --chain marriott --country thailand --json
  ```
- **`price-cliff`** — Find the price point in a city where rating-per-extra-dollar collapses — the cheapest hotel that's still legitimately good.

  _Turns 'spend the least without sacrificing quality' into one number plus the hotels just below the cliff._

  ```bash
  hotelist-pp-cli price-cliff bangkok --min-rating 7 --json
  ```

## Recipes


### Best-value hotel with a real gym in a city

```bash
hotelist-pp-cli filter lisbon --gym-weights --json --select hotels.name,hotels.hotellist_rating,hotels.price
```

Photo-verified weightlifting gym, narrowed to just name/rating/price so an agent doesn't parse the full payload.

### National value leaderboard

```bash
hotelist-pp-cli rank-country thailand --min-rating 8 --max-price 150 --top 10 --json
```

Top 10 hotels by rating-per-dollar across the whole country in one call.

### Compare loyalty chains before committing

```bash
hotelist-pp-cli chain-compare --chains marriott,hyatt --country japan --metric best-value --json
```

Head-to-head mean rating, median price, and value per chain.

### Plan a nomad route

```bash
hotelist-pp-cli corridor --cities "Chiang Mai,Lisbon,Tbilisi" --min-rating 7.5 --max-price 120 --json
```

Best hotel per stop that clears all filters, in one pass.

### Deep-dive one hotel's AI breakdown

```bash
hotelist-pp-cli show KYLCGAVE --json
```

Full Hotelist Score, AI photo and review ratings, consensus, verified amenities, pros and cons.

## Usage

Run `hotelist-pp-cli --help` for the full command reference and flag list.

## Commands

### hotel

A single AI-rated hotel from Hotelist.

- **`hotelist-pp-cli hotel <hotel_id>`** - Fetch the raw detail-modal HTML for one hotel (Hotelist Score, verified amenities, AI rating breakdown). The 'show' command parses this into structured fields.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hotelist-pp-cli hotel mock-value

# JSON for scripting and agents
hotelist-pp-cli hotel mock-value --json

# Filter to specific fields
hotelist-pp-cli hotel mock-value --json --select id,name,status

# Dry run — show the request without sending
hotelist-pp-cli hotel mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hotelist-pp-cli hotel mock-value --agent
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
hotelist-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/hotelist/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search returns an empty list for a city** — Run 'hotelist-pp-cli sync cities' to refresh the city-to-geohash table, then retry; the city name must match a Hotelist city.
- **prices look off or undated** — Hotelist prices are AI-estimated nightly figures, not live date-specific quotes; --checkin/--checkout are display context only, not a backend filter.
- **stale results** — Add --fresh to bypass the local cache and re-fetch from Hotelist; default cache is fresh-on-read with a domain-appropriate stale window.
- **HTTP 429 or slow responses** — The CLI rate-limits politely by design; lower concurrency with --max-scan-pages on multi-location commands or wait and retry.
