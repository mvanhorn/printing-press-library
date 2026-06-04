# Blu-ray.com CLI

**The disc-collector's CLI for Blu-ray.com — offline catalog, live deals, and a price-drop watchlist with zero account required.**

Sync the public Blu-ray.com sitemap into a local SQLite + FTS5 index and search ~400,000 releases without a network round-trip. Track prices with a local watchlist that pings you on new historical lows. Pipe everything as JSON.

Learn more at [Blu-ray.com](https://www.blu-ray.com).

Created by [@vinnyp](https://github.com/vinnyp) (Vinny Pasceri).

## Install

The recommended path installs both the `blu-ray-pp-cli` binary and the `pp-blu-ray` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install blu-ray
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install blu-ray --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install blu-ray --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install blu-ray --agent claude-code
npx -y @mvanhorn/printing-press-library install blu-ray --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/cmd/blu-ray-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/blu-ray-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-blu-ray --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-blu-ray --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install blu-ray --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/blu-ray-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/cmd/blu-ray-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "blu-ray": {
      "command": "blu-ray-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account, no API key, no OAuth — Blu-ray.com is read from its published HTML and XML sitemap. The CLI sends a normal browser User-Agent (configurable), throttles itself to stay under the site's per-IP budget (~4,000 pages/day), and never touches robots-disallowed paths.

## Quick Start

```bash
# Pull the public sitemap into a local SQLite + FTS5 index (~400k+ Blu-ray releases). Run weekly.
blu-ray-pp-cli sync

# Offline title search — instant, regex-capable, no round-trip.
blu-ray-pp-cli search 'fight club' --format 4k --json

# Fetch one release in full (specs, ratings, audio, subtitles, packaging) — cached locally.
blu-ray-pp-cli releases get Fight-Club-4K-Blu-ray 406956 --json

# Live 4K UHD deals at 30%+ off, narrowed to the columns an agent cares about.
blu-ray-pp-cli deals --country USA --format 4k --min-discount 30 --json --select title,sale_price,percent_off

# Add a release to the local watchlist, then rescan deals and alert when the target is hit.
blu-ray-pp-cli watch add 406956 --target-price 14.99 && blu-ray-pp-cli watch check

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Instant offline search across every Blu-ray release in Blu-ray.com's published sitemap (~400k+ entries). Faster than the site's JS-rendered search and works without a network round-trip after one sync.

  _Reach for this whenever an agent needs to resolve a title to a release id, list every edition of a film, or filter the catalog by year/format/country. Single round-trip vs. the dozens of network hits the website does._

  ```bash
  blu-ray-pp-cli search 'fight club' --format 4k --year 2025 --json
  ```
- **`watch check`** — Local watchlist of release ids; re-scans Blu-ray.com deals on demand and alerts when any watched disc hits its target price or a new historical low.

  _Use this whenever a user wants notifications about disc prices without polling third-party services. Pairs naturally with cron or a launchd job._

  ```bash
  blu-ray-pp-cli watch add 9929 --target-price 14.99 && blu-ray-pp-cli watch check --agent
  ```
- **`drift`** — Diffs today's sitemap against the last sync and surfaces 'new in catalog this week', 'removed', and 'metadata changed' so collectors don't miss announcements.

  _Use this to catch up after being away from the site for a week, or to build a weekly digest of what dropped, what got delayed, and what got pulled._

  ```bash
  blu-ray-pp-cli drift --since 2026-05-01 --kind bluray --json
  ```

### Decision support
- **`editions`** — Given a movie umbrella id, lists every disc edition (4K UHD, Blu-ray, Steelbook, Director's Cut, country variant) with release date, list price, current price, and Blu-ray.com community rating in a single view.

  _Use this when a user is deciding which edition of a film to buy (4K vs Blu-ray, Criterion vs Arrow, region A vs region B). Surfaces the trade-off at a glance._

  ```bash
  blu-ray-pp-cli editions 9929 --country US --json
  ```
- **`history`** — Shows per-retailer price history for a release id, captured automatically by watch check + deals --record. Optional inline ASCII spark plot.

  _Use this to know whether the current 'deal' is actually historically low, or just a small dip. Distinguishes real bargains from clickbait._

  ```bash
  blu-ray-pp-cli history 9929 --retailer amazon --plot
  ```

### Round-tripping
- **`upc`** — Resolves a CSV of UPC codes (e.g. the comma-separated export Blu-ray.com itself produces) back to local release records, hydrating titles, formats, ratings, and current prices.

  _Use this whenever a user is moving a collection between tools (Blu-ray.com to Trakt, CLZ, Letterboxd) or building a watch-list from a barcode scan._

  ```bash
  blu-ray-pp-cli upc ./my-collection.csv --dry-run --json
  ```

## Cookbook

### Find every 4K UHD release of Fight Club
Resolve a title to local release ids, narrowed to the format a collector actually wants.

```bash
blu-ray-pp-cli search 'fight club' --format 4k --json --select id,title,year,distributor,country
```

### Track a target price
Add a release to the local watchlist, scan current deals, then inspect the stored price curve later.

```bash
blu-ray-pp-cli watch add 9929 --target-price 14.99
blu-ray-pp-cli watch check --agent
blu-ray-pp-cli history 9929 --plot
```

### Import and enrich a UPC export
Turn a UPC-only collection export into structured local release records.

```bash
blu-ray-pp-cli upc ./my-bluray-collection.csv --enrich --json > collection.json
```

### Build a weekly catalog drift count
Use sitemap snapshots to count new releases added since the last weekly sync.

```bash
blu-ray-pp-cli sync --kind bluray --quiet
blu-ray-pp-cli drift --since 2026-05-10 --kind bluray --json | jq '.added | length'
```

### Export a portable catalog slice
Save the local 4K catalog as a file another tool can consume without touching the network.

```bash
blu-ray-pp-cli export --format csv --kind 4k --output 4k-releases.csv
```

## Usage

Run `blu-ray-pp-cli --help` for the full command reference and flag list.

## Commands

### calendar

Release calendar (by year + format + country).

- **`blu-ray-pp-cli calendar digital`** - Digital release calendar (streaming/rental window opens).
- **`blu-ray-pp-cli calendar releases`** - Release calendar page for a given year, optionally filtered by country and format. JS-driven UI; the raw HTML still contains the listing data the page renders.
- **`blu-ray-pp-cli calendar theatrical`** - Theatrical release calendar.

### deals

Live disc deals (sale prices across retailers).

- **`blu-ray-pp-cli deals`** - Current Blu-ray.com deals, filterable by country and format. Each row carries the underlying release id and the affiliate click URL (the latter is not followed by the CLI).

### news

Blu-ray.com news stories.

- **`blu-ray-pp-cli news get`** - Fetch a single news story by id.
- **`blu-ray-pp-cli news index`** - News index page (latest stories on top). Hand-parser extracts headline + posted-date + body link.

### releases

Disc release pages and listings (Blu-ray, 4K, 3D, DVD, digital, iTunes, MA, UV).

- **`blu-ray-pp-cli releases get`** - Fetch the canonical release detail page by URL slug and id. The id is stable; slug is documented for redirect-safe fetching.
- **`blu-ray-pp-cli releases new`** - List recent Blu-ray, 4K, DVD, and digital releases (paginated). Returns release page links from the static template.

### sitemap

Public XML sitemaps. Used by `sync` to enumerate every release id; safe to fetch (allowed by robots.txt).

- **`blu-ray-pp-cli sitemap bluraymovies`** - One of nine gzipped Blu-ray release shards (50,000 URLs each). Pull all nine for the full Blu-ray catalog.
- **`blu-ray-pp-cli sitemap index`** - Sitemap index — points at gzipped sub-sitemaps for main, news, bluraymovies (9 shards), dvdmovies (7), itunesmovies (5), digitalmovies (2), cast (11), ma, games, other.
- **`blu-ray-pp-cli sitemap news`** - Compressed news sitemap — each entry has title + publication_date inline (no per-story fetch needed for enumeration).

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
blu-ray-pp-cli deals

# JSON for scripting and agents
blu-ray-pp-cli deals --json

# Filter to specific fields
blu-ray-pp-cli deals --json --select id,name,status

# Dry run — show the request without sending
blu-ray-pp-cli deals --dry-run

# Agent mode — JSON + compact + no prompts in one flag
blu-ray-pp-cli deals --agent
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
blu-ray-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/blu-ray-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 403 / suddenly empty deals** — You hit Blu-ray.com's per-IP throttle (~4,000 pages/day). Wait 24h, lower concurrency (`--wait 3`), or set BLURAY_PP_PROXY.
- **Garbled accented characters in titles** — Pages are ISO-8859-1; the parser handles this automatically. If you see mojibake, ensure your terminal is UTF-8 (`echo $LANG` should include UTF-8) — the CLI re-encodes before printing.
- **`search` returns nothing** — Run `blu-ray-pp-cli sync` once — the offline index is empty until you've synced the sitemap. After sync, `\bdoctor\b` reports the local catalog size.
- **A movie's URL has changed** — The numeric `id` is stable. `releases get <stale-slug> <id>` follows the 301 to the current slug; or look up the current slug with `search` first.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**TUVIMEN/blu-ray-scraper**](https://github.com/TUVIMEN/blu-ray-scraper) — Python
- [**Flexget plugin (PR #1336)**](https://github.com/Flexget/Flexget/pull/1336) — Python
- [**blu2trakt**](https://xenonnsmb.com/2018/04/22/blu2trakt-import-your-blu-ray.com-library-into-trakt/) — PHP

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
