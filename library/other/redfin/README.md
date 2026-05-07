# Redfin CLI

**Stingray-backed Redfin CLI with the workflows the website can't do — saved-search diff, sold-price trends, $/sqft ranking, and offline SQL.**

Search homes for sale via Redfin's internal Stingray endpoints from the terminal, sync results to a local SQLite store, and run the workflows the website never built: diff a saved search week-over-week with `watch`, rank by $/sqft net of HOA with `rank`, pull sold comps for a subject property with `comps`, surface price drops or stale listings with `drops`, and overlay market trends across multiple cities with `trends`. Every command is `--json` / `--select`-shaped so an agent can pipe the output without burning context.

Learn more at [Redfin](https://www.redfin.com).

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/redfin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/other/redfin/cmd/redfin-pp-cli@latest
```

## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-redfin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-redfin --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-redfin skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-redfin. The skill defines how its required CLI can be installed.
```

## Authentication

No authentication required. The Stingray endpoints are the same ones Redfin's web app uses, served unauthenticated to US IPs. Account-only features (saved homes, alerts, agent dashboard) are intentionally out of scope. Surf with Chrome TLS fingerprint clears AWS-WAF protection at runtime; cliutil.AdaptiveLimiter handles per-IP rate limits with exponential backoff.

## Quick Start

```bash
# Verify connectivity and US-IP geo access (Stingray is US-only).
redfin-pp-cli doctor


# Resolve the canonical region_id you need for every search.
redfin-pp-cli region resolve "Austin, TX" --json


# Search — verifies Stingray endpoints respond and JSON output is well-formed.
redfin-pp-cli homes --region-id 30772 --region-type 6 --beds-min 3 --price-max 600000 --status for-sale --json --limit 10


# Persist that search to the local store under slug 'austin-3br'.
redfin-pp-cli sync-search austin-3br --region-id 30772 --region-type 6 --beds-min 3 --price-max 600000 --status for-sale


# Rank synced listings by net-HOA $/sqft — the metric Redfin's sort never offers.
redfin-pp-cli rank --by price-per-sqft --net-hoa --region-id 30772 --region-type 6 --json --limit 10


# After a few days — diff against the previous sync and emit NEW / REMOVED / PRICE-CHANGED / STATUS-CHANGED.
redfin-pp-cli watch austin-3br --since 7d --json


# One-shot market snapshot: active, pending, sold-90d, medians, % with price drops.
redfin-pp-cli summary 30772 --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Time-series intelligence
- **`watch`** — Re-run a saved gis search and surface what's NEW, REMOVED, PRICE-CHANGED, or STATUS-CHANGED since the last sync.

  _Pick this when an agent is tracking a buyer's shortlist over time and needs a reproducible 'what changed' digest._

  ```bash
  redfin-pp-cli watch austin-3br --since 7d --json
  ```
- **`drops`** — List active listings whose price dropped by N% in a window, OR whose days-on-market exceed a threshold.

  _Pick this when timing the market or surfacing lowball candidates before tour scheduling._

  ```bash
  redfin-pp-cli drops --region-id 30772 --region-type 6 --since 7d --min-pct 3 --dom-min 30 --json
  ```

### Local-store math
- **`rank`** — Rank synced listings by price-per-sqft, with optional HOA-fee subtraction over a 5-year horizon.

  _Pick this when value-per-dollar is the goal and HOA-heavy condos must compete fairly against single-family._

  ```bash
  redfin-pp-cli rank --by price-per-sqft --net-hoa --region-id 30772 --region-type 6 --json --limit 25
  ```

### Shortlist workflows
- **`compare`** — Pull 2-8 listings through the combined Stingray detail endpoint and emit aligned columnar output (price, $/sqft, beds, baths, lot, year, schools, AVM delta, last sale, taxes).

  _Pick this when narrowing a shortlist; the wide table makes school-rating and AVM-delta differences obvious._

  ```bash
  redfin-pp-cli compare <your-listing-url> <another-listing-url> --json
  ```
- **`comps`** — For a subject listing, derive a circular polygon from --radius, run a sold-status search, filter by --sqft-tol and --bed-match, return the ranked comp set.

  _Pick this when an agent needs to pull comparable sales for a buyer offer; collapses 20 minutes of polygon-clicking into one command._

  ```bash
  redfin-pp-cli comps <your-listing-url> --radius 0.5 --sqft-tol 15 --months 6 --bed-match --json
  ```

### Cross-market joins
- **`rank`** — Union synced listings across multiple region slugs and rank across the entire set, deduped by listing URL.

  _Pick this when an agent needs a single ranked feed across multiple metros without writing a fan-out loop._

  ```bash
  redfin-pp-cli rank --regions 30772,30773,30774 --by price-per-sqft --beds-min 3 --price-max 600000 --json --limit 25
  ```
- **`trends`** — Pull aggregate-trends for N regions and emit one tidy long table (region × month × metric) over a window.

  _Pick this when a relocator is comparing cities and needs the medians overlaid on the same axis._

  ```bash
  redfin-pp-cli trends --regions 30743,18028,30739 --metric median-sale --period 24 --json
  ```

### Bulk extraction
- **`export`** — Slice the price space into bands, page-walk gis-csv per band until each returns under 350 rows, dedupe on listing URL, emit one CSV/JSON.

  _Pick this when you need every comp for a year, not the first 350 sorted by relevance._

  ```bash
  redfin-pp-cli export --region-slug "city/30772/TX/Austin" --status sold --year 2024 --csv > austin-sold-2024.csv
  ```

### Aggregations
- **`summary`** — Single command: active count, pending count, sold-90d count, median list, median sold, median DOM, median $/sqft, % with price drops, plus a trends snapshot.

  _Pick this when an agent needs the one-shot snapshot of a market for a buyer brief._

  ```bash
  redfin-pp-cli summary 30772:6 --json
  ```
- **`appreciation`** — For all child neighborhoods under a parent metro, call aggregate-trends and rank by YoY median-sale % change.

  _Pick this when a relocator or investor needs the 'where in this metro is hottest' answer._

  ```bash
  redfin-pp-cli appreciation --parent "city/30772/TX/Austin" --period 12 --json --limit 10
  ```

## Usage

Run `redfin-pp-cli --help` for the full command reference and flag list.

## Commands

### homes

Search Redfin homes for sale via the internal Stingray /api/gis JSON endpoint.

- **`redfin-pp-cli homes list`** - Run a Stingray gis search and return parsed listing rows from the JSON map payload. Strip the {}&& CSRF prefix before decoding.

### listing

Fetch full listing detail by combining initialInfo, aboveTheFold, and belowTheFold Stingray calls.

- **`redfin-pp-cli listing initial`** - First Stingray call for a listing — returns the canonical listingId and propertyId from the URL path.

### market

Aggregate market trends for a region (median sale price, days on market, supply, list-to-sale ratio) over a window.

- **`redfin-pp-cli market trends`** - Fetch aggregate-trends JSON for one region and period (months).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
redfin-pp-cli homes

# JSON for scripting and agents
redfin-pp-cli homes --json

# Filter to specific fields
redfin-pp-cli homes --json --select id,name,status

# Dry run — show the request without sending
redfin-pp-cli homes --dry-run

# Agent mode — JSON + compact + no prompts in one flag
redfin-pp-cli homes --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `REDFIN_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `redfin-pp-cli homes`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-redfin -g
```

Then invoke `/pp-redfin <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/redfin/cmd/redfin-pp-mcp@latest
```

Then register it:

```bash
claude mcp add redfin redfin-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/redfin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/redfin/cmd/redfin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "redfin": {
      "command": "redfin-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
redfin-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/redfin-pp-cli/config.toml`

Environment variables:

- `REDFIN_CONFIG` — override the config file path
- `REDFIN_BASE_URL` — override the base API URL (default `https://www.redfin.com`)
- `REDFIN_NO_AUTO_REFRESH=1` — disable the pre-read freshness hook (read commands won't auto-refresh stale local data)
- `REDFIN_FEEDBACK_ENDPOINT` — when set, `feedback` entries can be POSTed to this URL
- `REDFIN_FEEDBACK_AUTO_SEND=true` — auto-send feedback entries when the endpoint is configured
- `NO_COLOR` — disable colored output (also responds to `--no-color`)

The local SQLite store lives at `~/.local/share/redfin-pp-cli/data.db`.

## Cookbook

```bash
# Watch a saved search weekly — agent-friendly digest
redfin-pp-cli sync-search austin-3br --region-id 30772 --region-type 6 --beds-min 3 --price-max 600000 --status for-sale
redfin-pp-cli watch austin-3br --since 7d --json

# Sold comps for a subject listing within 0.5mi, ±15% sqft, last 6 months
redfin-pp-cli comps /TX/Austin/123-Main/home/12345 --radius 0.5 --sqft-tol 15 --months 6 --bed-match --json

# Rank by net-HOA $/sqft across multiple regions
redfin-pp-cli rank --regions 30772:6,30773:6,30774:6 --by price-per-sqft --net-hoa --beds-min 3 --json --limit 25

# Surface listings with a >=3% price drop or >30 days on market
redfin-pp-cli drops --region-id 30772 --region-type 6 --since 7d --min-pct 3 --dom-min 30 --json

# Side-by-side compare 2-8 listings (price, $/sqft, schools, AVM delta)
redfin-pp-cli compare \
  /TX/Austin/123-Main/home/12345 \
  /TX/Austin/456-Elm/home/67890 \
  --json

# Cross-metro trends overlay (period is months as integer)
redfin-pp-cli trends --regions 30743:6,18028:6,30739:6 --metric median-sale --period 24 --json

# Hottest neighborhoods within a metro by YoY median-sale appreciation
redfin-pp-cli appreciation --parent "city/30772/TX/Austin" --period 12 --json

# One-shot market snapshot for a region
redfin-pp-cli summary 30772 --json

# Bulk export all sold listings for a year (price-banded, dedup'd, CSV-shaped)
redfin-pp-cli export --region-slug "city/30772/TX/Austin" --status sold --year 2024 --format csv > austin-sold-2024.csv

# Pipe ranked output into jq for further filtering
redfin-pp-cli rank --by price-per-sqft --region-id 30772 --region-type 6 --json --limit 100 \
  | jq '[.[] | select(.beds >= 4 and .price_per_sqft < 350)]'

# Use a saved profile to bake in repeating flags (region, filters)
redfin-pp-cli profile save austin-3br --region-id 30772 --region-type 6 --beds-min 3
redfin-pp-cli homes --profile austin-3br --price-max 600000 --json
```

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **homes / region returns HTTP 403 or 429** — Surf cleared the homepage but Redfin rate-limits per IP. Run `redfin-pp-cli doctor` and wait 30-60 seconds before retrying; cliutil.AdaptiveLimiter will back off automatically.
- **All commands return 403 from non-US IPs** — Stingray is geo-restricted to US IPs. Run from a US-based machine, or use a US VPN.
- **watch reports no changes when you know listings changed** — Confirm a previous `sync-search` exists. Check with: `sqlite3 ~/.local/share/redfin-pp-cli/data.db "SELECT MAX(observed_at) FROM listing_snapshots WHERE saved_search = 'austin-3br'"`. If empty, run sync first.
- **rank or summary returns empty** — These read from the local store. Run `sync-search` first. Check with: `sqlite3 ~/.local/share/redfin-pp-cli/data.db "SELECT COUNT(*) FROM listing"`.
- **export hits the 350-row cap on a single price band** — The bulk exporter slices the price space automatically. If a band is still saturated, narrow `--status` or split by `--year`.
- **JSON parse error: unexpected token at offset 0** — Stingray wraps responses in `{}&&{...}`. The CLI strips the prefix automatically; if you're calling the API directly, drop the first 4 bytes before parsing.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.redfin.com
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: browser_http (70% confidence)
- Protocols: stingray-json-api (95% confidence)
- Protection signals: aws-cloudfront-waf (85% confidence)
- Generation hints: Strip the {}&& CSRF prefix from every Stingray JSON response before decoding, Use Surf with Chrome TLS fingerprint at runtime (UsesBrowserHTTPTransport), Conservative rate limit: 1 req/s default with adaptive backoff on 429, Stingray is geo-restricted to US IPs; doctor command should warn non-US users, Region IDs are visible in redfin.com URL paths (e.g., /city/30772/TX/Austin); region type 6=city, 1=zip, 11=neighborhood
- Candidate command ideas: homes — Stingray gis search is the primary entry point; listing — Listing detail composes initialInfo + aboveTheFold + belowTheFold; market — aggregate-trends endpoint exposes neighborhood medians

Warnings from discovery:
- csrf-prefix: Stingray JSON responses are prefixed with the literal bytes '{}&&' as CSRF prevention. Generated client must strip them before json.Unmarshal.
- geo-restricted: Stingray endpoints are US-only. Non-US callers will get 403 regardless of TLS fingerprint.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**reteps/redfin**](https://github.com/reteps/redfin) — Python
- [**dreed47/redfin**](https://github.com/dreed47/redfin) — Python
- [**wang-ye/redfin-scraper**](https://github.com/wang-ye/redfin-scraper) — Python
- [**alientechsw/RedfinPlus**](https://github.com/alientechsw/RedfinPlus) — Documentation

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
