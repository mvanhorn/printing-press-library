# Redfin CLI

**Every Redfin property, market stat, and price history — synced locally, queryable offline, and built for agents.**

The only Redfin CLI that accumulates sync history into a local SQLite store, so you can track price trends over time, diff what changed since last Tuesday, and rank deals by price drop × days on market — without re-fetching from the API every time. Beats the inactive Python library and the $5/1000-event Apify MCP servers on every dimension: free, fast, composable, and agent-native.

## Install

The recommended path installs both the `redfin-pp-cli` binary and the `pp-redfin` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install redfin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install redfin --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/redfin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-redfin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-redfin --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-redfin skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-redfin. The skill defines how its required CLI can be installed.
```

## Quick Start

```bash
# Verify Redfin endpoints are reachable
redfin-pp-cli doctor


# Search for 2BR listings under $900k
redfin-pp-cli search "San Francisco, CA" --beds 2 --max-price 900000 --json


# Sync listings and market stats for two zips into local SQLite
redfin-pp-cli sync --zip 94110 --zip 94112


# See how prices moved over 8 weeks
redfin-pp-cli price-trend --zip 94110 --weeks 8 --json


# Rank sold comps for a property by price-per-sqft similarity
redfin-pp-cli comp-score YOUR_PROPERTY --months 6 --json


# Check what changed in a saved search since last look
redfin-pp-cli watchlist check buyers-list --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`diff`** — See exactly what changed in a saved search or region since your last sync — new listings, price drops, and status changes as structured JSON.

  _Use when you need to know what changed in a market without re-running a full search; the answer is instant from local data._

  ```bash
  redfin-pp-cli diff --region 94110 --since 2026-05-05 --json
  ```
- **`watchlist check`** — Save named search criteria and check for new listings, price drops, or status changes since your last look.

  _Use when monitoring specific markets for a buyer or seller; produces actionable deltas instead of full result sets._

  ```bash
  redfin-pp-cli watchlist check sf-condos --json --select address,price,price_delta,status
  ```
- **`price-trend`** — Pull median price, days on market, and inventory as a time series for any zip code across your accumulated sync history.

  _Use when an agent or analyst needs historical market trajectory; Redfin's UI shows only a three-month sparkline with no exportable data._

  ```bash
  redfin-pp-cli price-trend --zip 94110 --weeks 12 --field median_price --json
  ```
- **`market-heat`** — Rank all synced zip codes and neighborhoods from hottest to coldest by price velocity, inventory compression, and DOM delta.

  _Use when comparing markets for investment or relocation decisions; the ranking only exists after syncing multiple regions._

  ```bash
  redfin-pp-cli market-heat --weeks 8 --sort price_velocity --top 10 --json --agent
  ```
- **`matrix`** — Compare median price, DOM, and inventory across a grid of zip codes and property types in a single pivot table -- the ITA Matrix for real estate.

  _Use when comparing markets across multiple property types simultaneously; surfaces the cross-dimensional pattern that normally requires dozens of separate searches._

  ```bash
  redfin-pp-cli matrix --zips 94110,94112,94103 --types condo,sfr --field median_price --json --agent --select zip,property_type,median_price,dom,inventory
  ```

### Agent-native plumbing

- **`comp-score`** — Rank recently sold comparables for a property by price-per-sqft similarity and recency, outputting a scored JSON list agents can act on.

  _Use when valuing a property; replaces a three-tool manual workflow (browser + spreadsheet + Python) with a single composable command._

  ```bash
  redfin-pp-cli comp-score YOUR_PROPERTY --months 6 --beds 2 --baths 2 --json
  ```
- **`deal-score`** — Score and rank active listings in a region by combining recent price drops with days-on-market to surface the most motivated-seller opportunities.

  _Use when an investor or buyer wants to surface underpriced listings without manual spreadsheet analysis._

  ```bash
  redfin-pp-cli deal-score --region 94110 --max-price 900000 --min-dom 30 --min-drop-pct 3 --json --agent
  ```
- **`seller-pulse`** — Get a seller-oriented market snapshot: inventory trend, DOM trend, list-to-prior-sale ratio, and percentage of listings with price drops.

  _Use when an agent or seller needs to know whether market conditions favor listing now or waiting._

  ```bash
  redfin-pp-cli seller-pulse --zip 94110 --weeks 4 --json
  ```
- **`dom-distribution`** — Show the days-on-market distribution for active listings in a zip code — what percentage are fresh (0-7d), recent (8-30d), stale (31-90d), or old (90+d).

  _Use when assessing whether a market has fresh inventory (competitive) or stale listings (buyer's market); key signal for offer strategy._

  ```bash
  redfin-pp-cli dom-distribution --zip 94110 --json
  ```

## Usage

Run `redfin-pp-cli --help` for the full command reference and flag list.

## Commands

### comparables

Comparable active and sold properties

- **`redfin-pp-cli comparables nearby_homes`** - Properties in the immediate neighborhood
- **`redfin-pp-cli comparables similar_listings`** - Currently active similar listings
- **`redfin-pp-cli comparables similar_sold`** - Recently sold comparable properties

### listings

Property search and location autocomplete

- **`redfin-pp-cli listings autocomplete`** - Autocomplete a location string to get region_id and region_type
- **`redfin-pp-cli listings csv`** - Download search results as CSV
- **`redfin-pp-cli listings list`** - Search properties by geographic filters

### market

Regional market statistics

- **`redfin-pp-cli market region_stats`** - Market-level statistics for a region

### neighborhood

Neighborhood data, schools, commute, and lifestyle scores

- **`redfin-pp-cli neighborhood commute`** - Commute time estimates for drive, transit, and bike
- **`redfin-pp-cli neighborhood schools`** - Nearby schools, parks, shopping, and amenities
- **`redfin-pp-cli neighborhood stats`** - Walk Score, Bike Score, and Transit Score for a property location

### properties

Property details and history

- **`redfin-pp-cli properties above_fold`** - Primary property details including price, beds, baths, photos
- **`redfin-pp-cli properties activity`** - Listing status history and activity changes
- **`redfin-pp-cli properties below_fold`** - Full property details including MLS data, price history, and amenities
- **`redfin-pp-cli properties building`** - Condo building details and HOA information
- **`redfin-pp-cli properties comments`** - Public comments on a property listing
- **`redfin-pp-cli properties cost_ownership`** - Monthly cost breakdown including mortgage, taxes, insurance, HOA
- **`redfin-pp-cli properties floor_plans`** - Floor plans for rental properties
- **`redfin-pp-cli properties hood_photos`** - Neighborhood street photos
- **`redfin-pp-cli properties info_panel`** - Compact property summary panel
- **`redfin-pp-cli properties initial_info`** - Get listing ID and basic property data from a Redfin URL path
- **`redfin-pp-cli properties page_tags`** - Page metadata tags for a property
- **`redfin-pp-cli properties parcel`** - Property parcel and lot information
- **`redfin-pp-cli properties primary_region`** - Primary region context for a property
- **`redfin-pp-cli properties seller_data`** - Seller information for claimed homes
- **`redfin-pp-cli properties tour_dates`** - Available tour dates and times

### valuation

Automated valuation and price history

- **`redfin-pp-cli valuation avm`** - Current automated valuation model (AVM) estimate
- **`redfin-pp-cli valuation avm_history`** - Historical AVM price trend data
- **`redfin-pp-cli valuation owner_estimate`** - Owner-provided or derived valuation estimate
- **`redfin-pp-cli valuation rental_estimate`** - Estimated rental value for a property


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
redfin-pp-cli listings list

# JSON for scripting and agents
redfin-pp-cli listings list --json

# Filter to specific fields
redfin-pp-cli listings list --json --select id,name,status

# Dry run — show the request without sending
redfin-pp-cli listings list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
redfin-pp-cli listings list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-redfin -g
```

Then invoke `/pp-redfin <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

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


Install the MCP binary from this CLI's published public-library entry or pre-built release.

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

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **search returns empty results** — Run 'redfin-pp-cli doctor' to confirm endpoints are reachable; try a broader location string like a city name rather than a zip
- **price-trend or market-heat shows only one data point** — Run 'redfin-pp-cli sync' on multiple days — time series requires accumulated sync history in the local store
- **comp-score returns no results** — Run 'redfin-pp-cli sync --zip <zip>' first to populate the similar_sold table; or increase --months and --radius
- **429 rate limit responses** — Reduce request frequency; the CLI uses adaptive rate limiting by default; add --delay flag to increase pause between calls

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**reteps/redfin**](https://github.com/reteps/redfin) — Python
- [**alientechsw/RedfinPlus**](https://github.com/alientechsw/RedfinPlus) — Python
- [**brojonat/gredfin**](https://github.com/brojonat/gredfin) — Go
- [**timendez/go-redfin-archiver**](https://github.com/timendez/go-redfin-archiver) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
