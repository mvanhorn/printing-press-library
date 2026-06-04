# Rappi (Mexico) CLI

**The first agent-native CLI for Rappi Mexico — read-only catalog browsing with offline SQLite snapshots, cross-city coverage, and a transcendence set (newcomers diff, top-rated with review-count floor, neighborhood pivots, geo-radius) that Rappi's own UI can't do.**

Browse restaurants and stores across every Mexican city Rappi serves, snapshot the catalog to a local SQLite database, then ask questions the Rappi UI cannot answer — like "which sushi spots opened in Roma Norte this month," "top-rated burgers with 100+ reviews," or "pharmacies within 1km of a supermarket." All read-only and proxy-free.

Learn more at [Rappi (Mexico)](https://www.rappi.com.mx).

Created by [@bobeglz](https://github.com/bobeglz) (bobe).

## Install

The recommended path installs both the `rappi-pp-cli` binary and the `pp-rappi` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install rappi
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install rappi --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install rappi --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install rappi --agent claude-code
npx -y @mvanhorn/printing-press-library install rappi --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/cmd/rappi-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rappi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-rappi --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-rappi --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install rappi --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/rappi-current).
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
    "rappi": {
      "command": "rappi-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No login or API key required. The CLI reads only public catalog pages (restaurants, stores, categories, promotions). Cart, orders, and account features are out of scope by design and not exposed in this CLI.

## Quick Start

```bash
# Confirm the CLI can reach rappi.com.mx and write to its config path.
rappi-pp-cli doctor

# Snapshot the CDMX restaurant + store catalog into the local SQLite store. Takes about a minute.
rappi-pp-cli sync

# The listicle-grade filter Rappi UI hides — pick top-rated burgers with a real review-count floor.
rappi-pp-cli restaurants top --city ciudad-de-mexico --category hamburguesas --min-rating 4.5 --min-reviews 100 --limit 10 --agent

# Cross-city coverage matrix across all five store types — the analyst view Rappi has no UI for.
rappi-pp-cli stores coverage --cities ciudad-de-mexico,guadalajara,monterrey --agent

# FTS5 search across the synced catalog with field selection for narrow agent output.
rappi-pp-cli search "sushi roma" --agent --select id,name,rating,address

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`restaurants diff`** — See newcomers and closures in any city + cuisine between two snapshots — answers "what's new in Roma Norte sushi this week" in one command.

  _Pick this over a raw list when the user asks 'what's new' — the diff is the answer; you don't have to compute it._

  ```bash
  rappi-pp-cli restaurants diff --city ciudad-de-mexico --category sushi --since 2026-04-01 --agent
  ```
- **`restaurants top`** — Top-rated restaurants with both a minimum rating AND a minimum review-count floor — the listicle-grade filter Rappi UI hides.

  _Reach for this when a user asks 'best burgers in Polanco' — it weeds out new restaurants with three perfect ratings._

  ```bash
  rappi-pp-cli restaurants top --city ciudad-de-mexico --category hamburguesas --min-rating 4.5 --min-reviews 100 --limit 10 --agent
  ```
- **`stores coverage`** — Cross-city, cross-store-type coverage matrix (CDMX × markets × pharmacies × liquor × express, all in one table) for the cities you sync.

  _Best command for retail analysts asking 'where is Rappi expanding'; one query gives the whole MX picture._

  ```bash
  rappi-pp-cli stores coverage --cities ciudad-de-mexico,guadalajara,monterrey --agent
  ```
- **`stores coverage-diff`** — Delta-vs-last-snapshot of the (city, store_type) coverage matrix — see store openings and store-type expansions over time.

  _Pair with weekly sync to spot expansion trends; agents can flag 'new pharmacy zone added in GDL'._

  ```bash
  rappi-pp-cli stores coverage-diff --since 2026-04-01 --agent
  ```
- **`restaurants by-neighborhood`** — Group restaurants by neighborhood within a city (Polanco vs Condesa vs Roma Norte) and rank by count or top-rated per neighborhood.

  _When the question is 'which neighborhood has the most sushi options' or 'top-rated pizza per neighborhood' — this is the answer._

  ```bash
  rappi-pp-cli restaurants by-neighborhood --city ciudad-de-mexico --category pizza --agent
  ```
- **`restaurants multi-category`** — Restaurants listed under two or more cuisine categories — surfaces fusion places and mis-categorized spots in one query.

  _Pick this when a user wants 'fusion sushi-mexicana' or to disambiguate a chain with multiple cuisine listings._

  ```bash
  rappi-pp-cli restaurants multi-category --city ciudad-de-mexico --agent
  ```
- **`restaurants brand`** — Find every city × category where a restaurant brand (e.g., "Sushi Itto") appears in the synced catalog.

  _Reach for this on chain-coverage questions and multi-city expansion analysis._

  ```bash
  rappi-pp-cli restaurants brand --name "Sushi Itto" --agent
  ```

### Agent-native plumbing
- **`restaurants open`** — Restaurants open at an arbitrary local time (e.g., "23:30 on Sunday") parsed from schema.org openingHours — beyond Rappi's "open now" view.

  _Use this for late-night-eat queries and Sunday-morning planning where the live Rappi view is misleading._

  ```bash
  rappi-pp-cli restaurants open --city ciudad-de-mexico --at "23:30" --category sushi --agent
  ```
- **`restaurants near`** — Restaurants within a Haversine radius of a lat/lng with optional category filter — sorted by distance.

  _Best for proximity questions when the user has coordinates (address geocoded externally) and needs a precise radius._

  ```bash
  rappi-pp-cli restaurants near --lat 19.4216 --lng -99.1700 --radius-km 2 --category tacos --agent
  ```
- **`stores adjacency`** — Stores of type A within a Haversine radius of stores of type B (e.g., pharmacies within 1km of supermarkets) — for concierge-style "one-stop trip" planning.

  _Concierge agents picking a single trip route should reach for this over two independent radius queries. Requires `--fetch-detail` because list pages do not include store coordinates._

  ```bash
  rappi-pp-cli stores adjacency --type farmatodo --within-km 1 --of-type market --city ciudad-de-mexico --fetch-detail --agent
  ```

## Usage

Run `rappi-pp-cli --help` for the full command reference and flag list.

## Commands

### catalog

Alphabetized product catalog index

- **`rappi-pp-cli catalog`** - Browse the product catalog indexed by initial letter and page number

### promotions

Public promotions and active campaigns

- **`rappi-pp-cli promotions`** - Public promotions landing page

### restaurants

Restaurant catalog browsing via SSR list and detail pages

- **`rappi-pp-cli restaurants get`** - Fetch the restaurant detail page (name, cuisine, address, hours, geo, rating)
- **`rappi-pp-cli restaurants list-category`** - List restaurants in a city filtered by cuisine category (hamburguesas, pizza, sushi, tacos, etc.)
- **`rappi-pp-cli restaurants list-city`** - List restaurants in a Mexican city (e.g. ciudad-de-mexico, guadalajara, monterrey)

### stores

Supermarket, pharmacy, liquor, and convenience store catalog

- **`rappi-pp-cli stores get`** - Fetch a store detail page (name, type, address, branding)
- **`rappi-pp-cli stores list-by-type`** - List stores by type (market for supermarkets, farmatodo for pharmacy, liquor, express, rappimall-parent)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
rappi-pp-cli promotions

# JSON for scripting and agents
rappi-pp-cli promotions --json

# Filter to specific fields
rappi-pp-cli promotions --json --select id,name,status

# Dry run — show the request without sending
rappi-pp-cli promotions --dry-run

# Agent mode — JSON + compact + no prompts in one flag
rappi-pp-cli promotions --agent
```

## Cookbook

Common Rappi-Mexico recipes. Every flag below comes from `<cmd> --help`; copy and adapt.

```bash
# 1. Snapshot a city before exploring it
rappi-pp-cli sync
rappi-pp-cli doctor
```

```bash
# 2. Listicle-grade "best burgers in CDMX" with a real review-count floor
rappi-pp-cli restaurants top \
  --city ciudad-de-mexico \
  --category hamburguesas \
  --min-rating 4.5 \
  --min-reviews 100 \
  --limit 10 \
  --agent
```

```bash
# 3. What's new in Roma Norte sushi since the start of the month
rappi-pp-cli restaurants diff \
  --city ciudad-de-mexico \
  --category sushi \
  --since 2026-04-01 \
  --agent
```

```bash
# 4. Late-night search — open at 23:30 anywhere in CDMX
rappi-pp-cli restaurants open \
  --city ciudad-de-mexico \
  --at "23:30" \
  --category sushi \
  --agent
```

```bash
# 5. Proximity ranking around any lat/lng (city centroid fallback)
rappi-pp-cli restaurants near \
  --lat 19.4216 \
  --lng -99.1700 \
  --radius-km 2 \
  --category tacos \
  --agent
```

```bash
# 6. Track a brand's footprint across every synced city
rappi-pp-cli restaurants brand \
  --name "Sushi Itto" \
  --agent
```

```bash
# 7. Cross-city, cross-store-type coverage matrix
rappi-pp-cli stores coverage \
  --cities ciudad-de-mexico,guadalajara,monterrey \
  --agent
```

```bash
# 8. Find pharmacies within 1km of supermarkets — one-stop trip planning
rappi-pp-cli stores adjacency \
  --type farmatodo \
  --of-type market \
  --within-km 1 \
  --city ciudad-de-mexico \
  --fetch-detail \
  --agent
```

```bash
# 9. Full-text search across everything you've synced, narrow fields
rappi-pp-cli search "sushi roma" \
  --agent \
  --select id,name,rating,address
```

```bash
# 10. Force local-only search (skip the network entirely)
rappi-pp-cli search "tacos" --data-source local --agent
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
rappi-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/rappi-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **All commands return empty results** — Run `rappi-pp-cli sync --city <city>` first — read commands query the local SQLite store populated by sync, not the live API.
- **HTTP 429 from rappi.com.mx** — Rate limit hit. Wait 60 seconds and re-run; the CLI uses a real-Chrome User-Agent and Spanish Accept-Language to minimize blocking.
- **"menu items" not in the data** — Menu items + prices are loaded by Rappi's web UI via XHR after page hydration and aren't reachable from this CLI. Menu CATEGORY names (Tacos, Postres) ARE returned in restaurant detail.
- **Sync skips a city** — Check the city slug against `rappi-pp-cli cities list` — common values are ciudad-de-mexico, guadalajara, monterrey, naucalpan.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.rappi.com.mx/
- Capture coverage: 10 API entries from 11 total network entries
- Reachability: standard_http (95% confidence)

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**parseforge/rappi-scraper**](https://apify.com/parseforge/rappi-scraper) — JavaScript
- [**yasmany.casanova/rappi-restaurant-scraper**](https://apify.com/yasmany.casanova/rappi-restaurant-scraper) — JavaScript
- [**luminati-io/rappi-price-tracker**](https://github.com/luminati-io/rappi-price-tracker) — JavaScript
- [**ingmpesca/rappi-webScraping**](https://github.com/ingmpesca/rappi-webScraping) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
