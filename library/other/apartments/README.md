# Apartments.com CLI

**The apartment-hunt CLI that actually works in 2026 — Surf-cleared bot protection plus a local SQLite store the website itself doesn't have.**

Search every Apartments.com listing path-slug from the terminal, sync results to a local SQLite store, and run the workflows the website never built: diff a saved search week-over-week with `watch`, rank by $/sqft net of pet fees with `value`, compare a shortlist side-by-side with `compare`, and surface price drops or phantom listings with `drops`, `stale`, and `phantoms`. Every command is `--json`/`--select`-shaped so an agent can pipe the output without burning context.

Learn more at [Apartments.com](https://www.apartments.com).

## Install

The recommended path installs both the `apartments-pp-cli` binary and the `pp-apartments` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install apartments
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install apartments --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/apartments/cmd/apartments-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apartments-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

## Authentication

No authentication required. Apartments.com's anonymous search and listing pages are the entire API surface this CLI uses; saved-search login (cookie session) is intentionally out of scope. Surf with Chrome TLS fingerprint clears the Akamai-style protection at runtime — no clearance cookie capture, no resident browser.

## Quick Start

```bash
# First search — verifies Surf transport clears protection and JSON output is well-formed.
apartments-pp-cli rentals --city austin --state TX --beds 2 --price-max 2500 --pets dog --json


# Persist that search to the local store under the slug 'austin-2br' so transcendence commands can read it.
apartments-pp-cli sync-search austin-2br --city austin --state TX --beds 2 --price-max 2500 --pets dog


# Rank the synced listings by $/sqft — the ratio metric apartments.com's sort omits.
apartments-pp-cli rank --by sqft --beds 2 --price-max 2500 --json --limit 10


# After a few days, run this — `watch` diffs against the previous sync and emits NEW / REMOVED / PRICE-CHANGED sets.
apartments-pp-cli watch austin-2br --since 7d --json


# The Monday-morning digest: new + removed + price drops + top-by-$/sqft + stale + phantoms in one structured output.
apartments-pp-cli digest --saved-search austin-2br --since 7d --format md

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Time-series intelligence
- **`watch`** — Re-run a stored search and surface what's NEW, REMOVED, or PRICE-CHANGED since the last sync.

  _Pick this when an agent is tracking a relocation over time and needs a reproducible 'what changed since last week' digest, not a fresh search._

  ```bash
  apartments-pp-cli watch austin-2br --json --since 7d
  ```
- **`drops`** — List listings whose max-rent dropped by ≥N% within a time window.

  _Pick this when timing the market or watching for distressed listings._

  ```bash
  apartments-pp-cli drops --since 14d --min-pct 5 --json
  ```
- **`stale`** — Flag listings whose price and availability haven't changed in N days — often phantom or stuck.

  _Pick this when a listing seems too good to be true; stale ones often are._

  ```bash
  apartments-pp-cli stale --days 30 --json --select url,maxrent,unchanged_days
  ```
- **`phantoms`** — Surface listings flagged by a three-signal join: 404 on re-fetch, dropped from saved-search results, or stale ≥45 days.

  _Pick this when prepping a shortlist for tour scheduling — phantoms waste tour slots._

  ```bash
  apartments-pp-cli phantoms --json
  ```
- **`history`** — Time-series of every observation of one listing — rent, availability, status.

  _Pick this when reasoning about a single listing's price trajectory._

  ```bash
  apartments-pp-cli history https://www.apartments.com/example-property-1234 --json
  ```

### Cross-market joins
- **`nearby`** — Fan out a search across multiple cities, zips, or neighborhoods and return one ranked, deduped list.

  _Pick this when an agent needs a single ranked feed across multiple search slugs without writing a fan-out loop._

  ```bash
  apartments-pp-cli nearby austin-tx round-rock-tx pflugerville-tx --beds 2 --price-max 2500 --rank sqft --agent
  ```

### Local-store math
- **`value`** — Rank synced listings by 12-month total cost (rent + pet rent + pet deposit + pet fee), filtered to your hard budget.

  _Pick this when budget is binding and pet fees might push a listing over the line._

  ```bash
  apartments-pp-cli value --budget 2800 --pet dog --months 12 --json --select rank,url,total_cost
  ```
- **`rank`** — Rank synced listings by ratio metrics — price per square foot or price per bedroom.

  _Pick this when value-per-dollar is the goal, not 'best match' or 'lowest price'._

  ```bash
  apartments-pp-cli rank --by sqft --beds 2 --price-max 2500 --json --limit 10
  ```
- **`floorplans`** — Rank per-floor-plan rent/sqft across synced listings — same building can yield 4 plans at different ratios.

  _Pick this when a building has multiple floor plans and you want the cheap one specifically._

  ```bash
  apartments-pp-cli floorplans --rank price-per-sqft --beds 2 --json --limit 10
  ```
- **`must-have`** — Filter synced listings to those whose amenities array contains ALL listed terms via FTS5.

  _Pick this when the must-haves are free-text, not in apartments.com's amenity dropdown._

  ```bash
  apartments-pp-cli must-have "in-unit washer" "covered parking" "dishwasher" --json
  ```

### Shortlist workflows
- **`compare`** — Pivot 2–8 listings into a wide table — one column per listing — with computed $/sqft and amenity overlap.

  _Pick this when narrowing a shortlist; the wide table makes amenity-overlap deltas obvious._

  ```bash
  apartments-pp-cli compare austin-arboretum-1 austin-arboretum-2 austin-arboretum-3 --json
  ```
- **`digest`** — Single-shot composer: new + removed + price-drops + top-5 by $/sqft + stale + phantom flags for one saved search over N days.

  _Pick this when an agent needs a Monday-morning summary in one call._

  ```bash
  apartments-pp-cli digest --saved-search austin-2br --since 7d --format md
  ```
- **`shortlist`** — Tag-based local shortlist table; add/show/remove listings with notes and tags.

  _Pick this when an agent or user is curating a shortlist; downstream commands like `compare` read from it._

  ```bash
  apartments-pp-cli shortlist add https://www.apartments.com/example-1234 --tag austin --note "liked the kitchen"
  ```

### Aggregations
- **`market`** — Median, p10, p90 of rent and rent/sqft, pet-friendly share, by city/state and bed count.

  _Pick this when an agent needs to anchor 'is this a fair price' against the local distribution._

  ```bash
  apartments-pp-cli market austin-tx --beds 2 --json
  ```

## Usage

Run `apartments-pp-cli --help` for the full command reference and flag list.

## Commands

### Search & Fetch (live)

| Command | What it does |
|---|---|
| `rentals` | Path-slug search by city/state/zip, beds, price, pets, type. Returns parsed placards. |
| `listing <url-or-id>` | Fetch one detail page; falls back to the most recent placard snapshot when the live fetch is rate-gated. |
| `nearby <slug...>` | Fan out across multiple city-state slugs; returns one ranked, deduped list. |

### Persist (local SQLite store)

| Command | What it does |
|---|---|
| `sync-search <slug>` | Run a saved search and append placards to `listing_snapshots` under the slug. |
| `sync` | Generic sync helper for the synced-data layer. |
| `import` / `export` | Round-trip the local store via JSONL/JSON for backup or migration. |

### Time-series & change detection

| Command | What it does |
|---|---|
| `watch <slug>` | Diff the latest two syncs of a saved search: NEW / REMOVED / PRICE_CHANGED. |
| `drops` | Listings whose max-rent dropped by ≥N% within a `--since` window. |
| `stale` | Listings whose price and availability have not changed in N days. |
| `phantoms` | Listings flagged by 404 on re-fetch, dropped from saved-search results, or stale ≥`--days`. |
| `history <url-or-id>` | Time-series of every observation of one listing — rent, availability, status. |
| `digest` | One-shot composer: new + removed + price drops + top-by-$/sqft + stale + phantoms for one saved search. |

### Ranking & analysis (on synced data)

| Command | What it does |
|---|---|
| `rank` | Rank by ratio metrics: `--by sqft\|bed\|rent`. |
| `value` | Rank by 12-month total cost (rent + pet fees), filtered by `--budget`. |
| `floorplans` | Rank per-floor-plan rent/sqft — same building can yield 4 plans at different ratios. |
| `market <city-state>` | Median, p10, p90 of rent and rent/sqft, pet-friendly share, by bed count. |
| `must-have <term...>` | Filter to listings whose amenities array contains ALL listed terms. |
| `compare <id...>` | Pivot 2–8 listings into a wide table — one column per listing — with computed $/sqft and amenity overlap. |

### Shortlist & profiles

| Command | What it does |
|---|---|
| `shortlist add\|show\|remove` | Tag-based local shortlist with notes; downstream commands like `compare` read from it. |
| `profile` | Save / list / apply named flag sets for reuse. |

### Utilities

| Command | What it does |
|---|---|
| `doctor` | Verify config, transport, and connectivity. |
| `agent-context` | Emit structured JSON describing this CLI for agents. |
| `which` | Find the command that implements a capability. |
| `api` | Browse all API endpoints by interface name. |
| `workflow` | Compound workflows that combine multiple operations. |
| `feedback` | Record feedback about this CLI (local by default; upstream opt-in). |
| `version` | Print version. |


## Cookbook

Recipes use verified flag names and the local store. Run `apartments-pp-cli sync-search <slug> --city <city> --state <st>` once before any "synced data" recipe.

```bash
# 1. Cheapest 2BRs in Austin under $2,500 with dog policy, ranked by $/sqft.
apartments-pp-cli rentals --city austin --state tx --beds 2 --price-max 2500 --pets dog --json \
  | jq '.[] | select(.sqft > 0) | {url, rent: .max_rent, sqft, ppsqft: (.max_rent / .sqft)}'

# 2. Persist a saved-search so transcendence commands can read it.
apartments-pp-cli sync-search austin-2br --city austin --state tx --beds 2 --price-max 2500 --pets dog

# 3. Weekly diff on the saved-search — what's new, removed, or repriced since the last sync.
apartments-pp-cli watch austin-2br --since 7d --json

# 4. Monday-morning digest as Markdown for an email or PR description.
apartments-pp-cli digest --saved-search austin-2br --since 7d --format md

# 5. Rank synced listings by 12-month total cost net of pet fees, capped at $2,800/mo.
apartments-pp-cli value --budget 2800 --pet dog --months 12 --json --select rank,url,total_cost

# 6. Ranked $/sqft across multiple metro slugs — one feed.
apartments-pp-cli nearby austin-tx round-rock-tx pflugerville-tx --beds 2 --price-max 2500 --rank sqft --agent

# 7. Surface listings whose price has dropped 10%+ in the last 14 days.
apartments-pp-cli drops --since 14d --min-pct 10 --json --limit 50

# 8. Flag stuck or phantom listings before booking tours.
apartments-pp-cli phantoms --days 45 --json
apartments-pp-cli stale --days 30 --json --limit 25

# 9. Rank floor plans within the same building (same building, different price ratios).
apartments-pp-cli floorplans --rank price-per-sqft --beds 2 --json --limit 10

# 10. Anchor "is this a fair price" against the local distribution.
apartments-pp-cli market austin-tx --beds 2 --json

# 11. Filter to listings that have ALL of these amenities (FTS5 intersect).
apartments-pp-cli must-have "in-unit washer" "covered parking" "dishwasher" --json

# 12. Side-by-side compare a shortlist of 2–8 listings.
apartments-pp-cli compare the-domain-austin-tx the-grove-austin-tx austin-arboretum --json

# 13. Build a tagged shortlist as you research.
apartments-pp-cli shortlist add https://www.apartments.com/the-domain-austin-tx/abc123/ --tag favorite --note "rooftop pool"
apartments-pp-cli shortlist show --tag favorite --json

# 14. Time-series of one listing's rent and availability.
apartments-pp-cli history https://www.apartments.com/the-domain-austin-tx/abc123/ --json

# 15. Save common output flags as a reusable profile, then apply them to any command.
apartments-pp-cli profile save agent-defaults --json --compact --no-color
apartments-pp-cli rentals --profile agent-defaults --city austin --state tx --beds 2
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
apartments-pp-cli listing example-property

# JSON for scripting and agents
apartments-pp-cli listing example-property --json

# Filter to specific fields
apartments-pp-cli listing example-property --json --select id,name,status

# Dry run — show the request without sending
apartments-pp-cli listing example-property --dry-run

# Agent mode — JSON + compact + no prompts in one flag
apartments-pp-cli listing example-property --agent
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

Set `APARTMENTS_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-apartments -g
```

Then invoke `/pp-apartments <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/apartments/cmd/apartments-pp-mcp@latest
```

Then register it:

```bash
claude mcp add apartments apartments-pp-mcp
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-apartments --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-apartments --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-apartments skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-apartments. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/apartments-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/apartments/cmd/apartments-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "apartments": {
      "command": "apartments-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
apartments-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/apartments-pp-cli/config.toml`

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **search returns 0 listings or HTTP 403** — Run `apartments-pp-cli doctor` to verify Surf transport is active. If 403 persists, the Chrome TLS fingerprint may need refresh — file an issue with the doctor output.
- **watch reports no changes when you know listings changed** — Confirm a previous `sync` exists: `apartments-pp-cli sql "SELECT MAX(observed_at) FROM listing_snapshots WHERE saved_search = 'austin-2br'"`. If empty, run sync first.
- **value or rank command returns empty** — These read from the local store. Run `sync` first. Check with `apartments-pp-cli sql "SELECT COUNT(*) FROM listings"`.
- **amenity must-have intersect returns no rows** — FTS5 needs amenities populated; some listings have empty amenity arrays. Try `apartments-pp-cli sql "SELECT url, length(amenities) FROM listings ORDER BY length(amenities) DESC LIMIT 5"` to confirm.
- **rate limited or repeated 429s during sync** — Sync uses adaptive pacing automatically. If you hit a wall, pause for 30 seconds and re-run; cliutil.AdaptiveLimiter will back off.

## Known Gaps

- **`listing <url>` live fetch is rate-gated.** Apartments.com applies stricter Akamai bot protection on individual listing detail pages (`/<property-slug>/`) than on search-results pages (`/<city-state>/`). Surf with Chrome TLS fingerprint clears the search pages reliably (`probe-reachability` reports `mode: browser_http`), but most listing detail pages return 403 even via Surf (`mode: browser_clearance_http` — would require clearance-cookie import or full browser). The `listing` command **falls back to the most-recent snapshot from `rentals` / `sync-search`** when the live fetch returns 403, so placard data (URL, beds, baths, max rent, search-slug provenance) remains available. Detail-only fields (`amenities`, `floor_plans`, `pet_policy.fees`, `available_at`, `phone`) require either a future clearance-cookie path or a manual HAR; they are populated only for listings whose live fetch succeeds.
- **Saved-search login (cookie session) is intentionally out of scope for v1.** All shipped commands work anonymously.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.apartments.com
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: browser_http (85% confidence)
- Protocols: html-ssr (95% confidence)
- Protection signals: akamai-bot-manager (85% confidence)
- Generation hints: use Surf with Chrome TLS fingerprint at runtime (UsesBrowserHTTPTransport), all responses are HTML/SSR — extract via html_extract mode: page, no clearance cookie capture; no resident browser sidecar, schema.org microdata (meta itemprop=streetAddress|addressLocality|addressRegion|postalCode) plus data-beds / data-baths / data-maxrent attributes are the primary extraction targets
- Candidate command ideas: search — Path-slug search is the primary entry point at apartments.com; get — Listing detail page extracts schema.org microdata

Warnings from discovery:
- protection-active: Apartments.com (CoStar) employs Akamai-style bot detection. stdlib HTTP returns 403; Surf with Chrome TLS fingerprint clears it. Watch for protection escalation that might require Chrome-clearance cookie import or full-browser fallback in future versions.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**johnludwigm/PyApartments**](https://github.com/johnludwigm/PyApartments) — Python (12 stars)
- [**adinutzyc21/apartments-scraper**](https://github.com/adinutzyc21/apartments-scraper) — Python
- [**shilongdai/Apartment_Scraper**](https://github.com/shilongdai/Apartment_Scraper) — Python
- [**cccdenhart/apartments-scraper**](https://github.com/cccdenhart/apartments-scraper) — Python
- [**davidhuang620/Apartments.com-web-Scrapping**](https://github.com/davidhuang620/Apartments.com-web-Scrapping) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
