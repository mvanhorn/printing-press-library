---
name: pp-redfin
description: "Every Redfin property, market stat, and price history — synced locally, queryable offline, and built for agents. Trigger phrases: `search redfin listings`, `check redfin prices`, `pull comps for this property`, `what changed in my saved search`, `is the market hot in this zip`, `track price trends for a neighborhood`, `use redfin`, `run redfin`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - redfin-pp-cli
---

# Redfin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `redfin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install redfin --cli-only
   ```
2. Verify: `redfin-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The only Redfin CLI that accumulates sync history into a local SQLite store, so you can track price trends over time, diff what changed since last Tuesday, and rank deals by price drop × days on market — without re-fetching from the API every time. Beats the inactive Python library and the $5/1000-event Apify MCP servers on every dimension: free, fast, composable, and agent-native.

## When to Use This CLI

Use this CLI when you need to track Redfin property data over time, compare markets, or compute insights that require accumulating multiple data points (price trends, DOM distributions, deal scoring). The local SQLite store makes it especially useful for agents that need to answer 'what changed?' or 'which market is hottest?' without hitting the API repeatedly. For one-off lookups of a single property, the live search and property-detail commands work without syncing first.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

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

## Command Reference

**comparables** — Comparable active and sold properties

- `redfin-pp-cli comparables nearby_homes` — Properties in the immediate neighborhood
- `redfin-pp-cli comparables similar_listings` — Currently active similar listings
- `redfin-pp-cli comparables similar_sold` — Recently sold comparable properties

**listings** — Property search and location autocomplete

- `redfin-pp-cli listings autocomplete` — Autocomplete a location string to get region_id and region_type
- `redfin-pp-cli listings csv` — Download search results as CSV
- `redfin-pp-cli listings list` — Search properties by geographic filters

**market** — Regional market statistics

- `redfin-pp-cli market` — Market-level statistics for a region

**neighborhood** — Neighborhood data, schools, commute, and lifestyle scores

- `redfin-pp-cli neighborhood commute` — Commute time estimates for drive, transit, and bike
- `redfin-pp-cli neighborhood schools` — Nearby schools, parks, shopping, and amenities
- `redfin-pp-cli neighborhood stats` — Walk Score, Bike Score, and Transit Score for a property location

**properties** — Property details and history

- `redfin-pp-cli properties above_fold` — Primary property details including price, beds, baths, photos
- `redfin-pp-cli properties activity` — Listing status history and activity changes
- `redfin-pp-cli properties below_fold` — Full property details including MLS data, price history, and amenities
- `redfin-pp-cli properties building` — Condo building details and HOA information
- `redfin-pp-cli properties comments` — Public comments on a property listing
- `redfin-pp-cli properties cost_ownership` — Monthly cost breakdown including mortgage, taxes, insurance, HOA
- `redfin-pp-cli properties floor_plans` — Floor plans for rental properties
- `redfin-pp-cli properties hood_photos` — Neighborhood street photos
- `redfin-pp-cli properties info_panel` — Compact property summary panel
- `redfin-pp-cli properties initial_info` — Get listing ID and basic property data from a Redfin URL path
- `redfin-pp-cli properties page_tags` — Page metadata tags for a property
- `redfin-pp-cli properties parcel` — Property parcel and lot information
- `redfin-pp-cli properties primary_region` — Primary region context for a property
- `redfin-pp-cli properties seller_data` — Seller information for claimed homes
- `redfin-pp-cli properties tour_dates` — Available tour dates and times

**valuation** — Automated valuation and price history

- `redfin-pp-cli valuation avm` — Current automated valuation model (AVM) estimate
- `redfin-pp-cli valuation avm_history` — Historical AVM price trend data
- `redfin-pp-cli valuation owner_estimate` — Owner-provided or derived valuation estimate
- `redfin-pp-cli valuation rental_estimate` — Estimated rental value for a property


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
redfin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find price-dropped deals in a neighborhood

```bash
redfin-pp-cli deal-score --region 94110 --min-dom 30 --min-drop-pct 3 --max-price 900000 --json --agent --select address,price,dom,price_drop_pct,deal_rank
```

Surfaces listings where price dropped 3%+ AND have been sitting 30+ days — both signals for motivated sellers.

### Compare market conditions across zip codes

```bash
redfin-pp-cli market-heat --weeks 8 --sort price_velocity --top 10 --json --agent --select zip,median_price,price_velocity,dom_delta,inventory_compression
```

Ranks synced markets from hottest to coolest by price velocity; requires at least two sync runs per region.

### Check what changed in a watchlist since yesterday

```bash
redfin-pp-cli watchlist check sf-condos --since 24h --json --select address,price,price_delta,status,days_on_market
```

Returns only listings that are new, dropped in price, or went pending since the last check — no full result set to parse.

### Run comps on a property before an offer

```bash
redfin-pp-cli comp-score YOUR_PROPERTY --months 6 --beds 2 --baths 1 --json --agent --select address,sold_price,price_per_sqft,ppsqft_delta,days_since_sold,similarity_rank
```

Ranks nearby sold comps by combined recency and price-per-sqft match; the --select flag narrows the output for agent context budget.

### Seller timing check for a zip code

```bash
redfin-pp-cli seller-pulse --zip 94110 --weeks 4 --json --agent
```

Returns inventory_delta, dom_trend, pct_with_price_drop, and list_to_prior_sale_ratio as a seller-angle snapshot.

## Auth Setup

No authentication required.

Run `redfin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  redfin-pp-cli listings list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
redfin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
redfin-pp-cli feedback --stdin < notes.txt
redfin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.redfin-pp-cli/feedback.jsonl`. They are never POSTed unless `REDFIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `REDFIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
redfin-pp-cli profile save briefing --json
redfin-pp-cli --profile briefing listings list
redfin-pp-cli profile list --json
redfin-pp-cli profile show briefing
redfin-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `redfin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add redfin-pp-mcp -- redfin-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which redfin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   redfin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `redfin-pp-cli <command> --help`.
