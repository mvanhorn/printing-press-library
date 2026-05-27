# Reno Goat CLI

Search, compare, price-watch, and project-track renovation and interior products across 11 retailers from one CLI. Reno Goat routes each query to the sources that carry that product type — fixtures and appliances go to Ferguson, furniture to West Elm and Article, hardware and lighting to Rejuvenation, and five Shopify DTC stores (Schoolhouse, Blu Dot, Gus Modern, Floyd, Lulu & Georgia) cover furniture and decor. Standalone utilities provide Lowe's autocomplete and store locator. No API keys required.

## What It Does

- **Fan-out search** — `product-search "pendant light"` hits every active source in parallel and returns normalized results. `--room kitchen` narrows to foundational + appliances + decor sources; `--category furniture` targets West Elm, Article, and Shopify DTC.
- **Price watch** — track any product's price in a local SQLite database, poll on a schedule, get alerts when it drops past a threshold.
- **Project tracker** — group products into named renovation projects with quantities and cross-store budget totals.
- **Product comparison** — side-by-side normalized view of any two or more products, across different retailers.
- **Saved products + stale detection** — save products locally; `--check-stale` re-fetches to catch discontinuation, out-of-stock, or price drift.
- **Spec sheet export** — pull normalized product specifications from unstructured retailer pages.
- **Brand cross-reference** — see which retailers carry a brand and compare price points.
- **Store locator** — find Lowe's, West Elm, and Rejuvenation stores near a ZIP code.
- **Delivery checks** — shipping options and availability by postal code (West Elm, Rejuvenation).
- **Deals and promotions** — active sales, discount percentages, promo codes.
- **Reviews** — product reviews from Ferguson and Article, including ratings and UGC media.
- **Autocomplete** — typeahead suggestions from Lowe's, West Elm, and Rejuvenation.

## Source Registry

| Source | Transport | Categories | Status |
|--------|-----------|------------|--------|
| Ferguson | GraphQL | foundational, appliances | **active** |
| West Elm | Constructor.io | furniture, decor | **active** |
| Rejuvenation | Constructor.io | foundational, decor | **active** |
| Article | APQ GraphQL | furniture, decor | **active** |
| Shopify DTC (5 stores) | Storefront API | furniture, decor | **active** |
| Lowe's | standard HTTP (partial) | foundational, appliances | stub (autocomplete + stores only) |
| Home Depot | SSR only | foundational, appliances | stub |
| Wayfair | PerimeterX clearance | foundational, appliances, furniture | stub |
| AllModern | PerimeterX clearance | appliances, furniture | stub |
| Restoration Hardware | DataDome | foundational, furniture | stub |
| IKEA | unknown | furniture, decor, foundational | stub |

Active sources are queried by the fan-out. Stubs are registered for visibility and future activation.

**Lowe's** was assessed via the [Printing Press](https://github.com/mvanhorn/cli-printing-press) printer protocol (`probe-reachability` + `browser-sniff` on a Firefox HAR capture). Autocomplete and store locator are standard HTTP (no auth, User-Agent header only). Product search and the recommendation engine (`pythia-recs-svc`, 14 endpoints) require browser session cookies and are stubbed.

**Home Depot** was assessed via the same protocol. Every endpoint tested — autocomplete (`/complete/search/`), store finder (`/StoreFinderServices/v2/`), GraphQL search (`/federation-gateway/graphql`) — returned 403 from both stdlib and headered HTTP. A Firefox HAR captured zero requests to `www.homedepot.com`; Home Depot renders all product and store data server-side with no XHR/fetch API surface. Stubbed with no viable activation path short of an HTML scraping adapter.

## Category Routing

Queries are routed to sources by product category:

| Category | Active Sources |
|----------|---------------|
| foundational (faucets, fixtures, hardware, lighting) | Ferguson, Rejuvenation |
| appliances | Ferguson |
| furniture | West Elm, Article, Shopify DTC |
| decor | West Elm, Rejuvenation, Shopify DTC |

Room shortcuts expand to categories: `--room kitchen` = foundational + appliances + decor. `--room bathroom` = foundational + furniture + decor.

## Install

```bash
npx -y @mvanhorn/printing-press-library install reno-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install reno-goat --cli-only
```

For skill only:

```bash
npx -y @mvanhorn/printing-press-library install reno-goat --skill-only
```

To constrain the skill install to specific agents:

```bash
npx -y @mvanhorn/printing-press-library install reno-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install reno-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/reno-goat/cmd/reno-goat-pp-cli@latest
```

### Pre-built binary

Download from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/reno-goat-current). On macOS: `xattr -d com.apple.quarantine <binary>`. On Unix: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-reno-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-reno-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-reno-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-reno-goat. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle for one-click MCP installs.

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/reno-goat-current).
2. Double-click the `.mcpb` file.

Requires Claude Desktop 1.0.0 or later. Ships for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`).

<details>
<summary>Manual JSON config (advanced)</summary>

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/reno-goat/cmd/reno-goat-pp-mcp@latest
```

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "reno-goat": {
      "command": "reno-goat-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify setup
reno-goat-pp-cli doctor

# Fan-out search across all active sources
reno-goat-pp-cli product-search all "pendant light"

# Search by room (expands to relevant categories + sources)
reno-goat-pp-cli product-search all "faucet" --room kitchen

# Search a single source
reno-goat-pp-cli product-search ferguson-search --query "rainfall showerhead"

# Lowe's autocomplete
reno-goat-pp-cli suggest lowes-suggest "bathroom vanity"

# Find Lowe's stores near you
reno-goat-pp-cli stores lowes-stores 66101

# Get full product details
reno-goat-pp-cli product ferguson-product --url https://www.fergusonhome.com/product/...

# Compare products across retailers
reno-goat-pp-cli compare https://www.westelm.com/... https://www.article.com/...

# Watch a product's price
reno-goat-pp-cli watch add https://www.westelm.com/... --threshold 15

# Start a renovation project with budget tracking
reno-goat-pp-cli project create "kitchen reno"
reno-goat-pp-cli project add "kitchen reno" https://www.fergusonhome.com/... --qty 1
reno-goat-pp-cli project budget "kitchen reno"
```

## Commands

### product-search

Fan-out search across sources, routed by category.

- `product-search all <query>` — fan-out to all active sources
- `product-search all <query> --room kitchen` — category-routed fan-out
- `product-search ferguson-search` — Ferguson (fixtures, appliances)
- `product-search westelm-search` — West Elm (furniture, decor)
- `product-search rejuvenation-search` — Rejuvenation (hardware, lighting)
- `product-search article-search` — Article (furniture, decor)
- `product-search shopify-search` — Shopify DTC stores

### product

Full product details from any source.

- `product ferguson-product` — Ferguson via GraphQL
- `product westelm-product` — West Elm
- `product rejuvenation-product` — Rejuvenation
- `product article-product` — Article via APQ (50+ fields)
- `product shopify-product` — Shopify DTC via Storefront API

### watch

Track product prices over time in local SQLite.

- `watch add <url> [--threshold 15]` — start watching
- `watch list` — list watches
- `watch check` — poll prices, flag drops
- `watch history <url>` — price history

### project

Group products into renovation projects with budget tracking.

- `project create <name>` — create a project
- `project add <name> <url> [--qty N]` — add a product
- `project budget <name>` — budget totals
- `project list` — list projects

### saved

Save products and detect staleness.

- `saved add <url>` — save a product
- `saved list` — list saved
- `saved --check-stale` — detect discontinued, OOS, or price changes

### compare

- `compare <url1> <url2> [url3...]` — side-by-side comparison

### suggest

Autocomplete suggestions.

- `suggest lowes-suggest <query>` — Lowe's autocomplete with category facets
- `suggest westelm-suggest <query>` — West Elm via Constructor.io
- `suggest rejuvenation-suggest <query>` — Rejuvenation via Constructor.io

### stores

Find physical stores near a location.

- `stores lowes-stores <zip>` — Lowe's stores (address, hours, lat/lng, phone, features)
- `stores westelm-stores <zip>` — West Elm stores
- `stores rejuvenation-stores <zip>` — Rejuvenation stores

### brands

- `brands ferguson-brands` — Ferguson brand facets
- `brands westelm-brands` — West Elm brands
- `brands rejuvenation-brands` — Rejuvenation brands

### delivery

- `delivery westelm-delivery` — West Elm delivery by ZIP
- `delivery rejuvenation-delivery` — Rejuvenation delivery by ZIP

### deals

- `deals` — active promotions and promo codes

### reviews

- `reviews ferguson-reviews` — Ferguson reviews
- `reviews article-reviews` — Article reviews + UGC media

### sources

- `sources` — list all sources with status, transport, and categories

## Output Formats

```bash
# Table (default in terminal, JSON when piped)
reno-goat-pp-cli product-search all "pendant light"

# JSON
reno-goat-pp-cli product-search all "pendant light" --json

# Filter fields
reno-goat-pp-cli product-search all "pendant light" --json --select name,price,source

# Dry run
reno-goat-pp-cli product-search all "pendant light" --dry-run

# Agent mode (JSON + compact + no prompts)
reno-goat-pp-cli product-search all "pendant light" --agent
```

## Agent Usage

Designed for AI agent consumption:

- **Non-interactive** — never prompts, every input is a flag
- **Pipeable** — `--json` to stdout, errors to stderr
- **Filterable** — `--select name,price` returns only the fields you need
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — local SQLite store for watch/project/saved data

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Configuration

Config file: `~/.config/reno-goat-pp-cli/config.toml`

```bash
reno-goat-pp-cli doctor    # verify configuration and connectivity
```

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
