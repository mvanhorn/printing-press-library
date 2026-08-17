# Reno Goat CLI

Search, compare, price-watch, and project-track renovation selections across 33 active sources plus 5 tracked stubs from one CLI. Reno Goat focuses on the squishy middle between commodity builder supply and home decor: homeowner-visible products that often need a GC, plumber, electrician, HVAC tech, cabinet installer, or tile/flooring trade to install. It routes plumbing fixtures and bath showroom rows to Ferguson, Floor & Decor, PlumbersStock, FaucetDepot, FaucetList, PlumbTile, Modern Bathroom, KBAuthority, Vintage Tub, Signature Hardware, and QualityBath, HVAC equipment selections to Pioneer Mini Split, Sylvane, IWAe, and The Hardware Hut, appliances to Ferguson, IKEA, GE Appliances, Bray & Scarff, PC Richard, Appliance Factory, Best Buy, Abt, and Homewise Appliance, electrical/lighting selections to Super Bright LEDs, PROLIGHTING, 1000Bulbs, Bees Lighting, Lighting New York, and Lightology, cabinet/door/bath hardware to Rejuvenation, IKEA, and The Hardware Hut, and flooring/tile/surface selections to Floor & Decor, The Hardware Hut, and IKEA. Standalone utilities provide Lowe's autocomplete and store locator. No API keys required.

## What It Does

- **Fan-out search** — `product-search "pendant light"` hits every active source in parallel and returns normalized results. `--room kitchen` expands to plumbing + electrical + flooring + hardware + materials + appliances + decor; `--category electrical` targets Super Bright LEDs, PROLIGHTING, 1000Bulbs, Bees Lighting, Lighting New York, and Lightology.
- **Model intelligence** — `model-intel "36 induction cooktop"` discovers model/SKU candidates, enriches returned rows from product pages for spec/install PDFs, and probes predictable model pages where that makes sense. Auto mode infers installed-selection categories from queries such as `mini split heat pump`, `thermostat`, `floor register`, `linear drain`, `shower niche`, `shower door`, `shower head`, `shower panel`, `medicine cabinet`, `bidet seat`, `pot filler`, `cabinet pull`, `door hinge`, `grab bar`, `robe hook`, `toilet paper holder`, `towel bar`, `towel ring`, `soap dispenser`, `towel warmer`, `lighted mirror`, `vanity light`, `picture light`, `ceiling fan`, `floor warming`, `recessed light`, or `bathroom faucet`, so the same compound lookup works beyond appliances without a manual category flag. Readable search-result pages can also contribute labeled category model candidates and exact-model price evidence when they expose product-price fields.
- **Source probes** — `source-probe --candidate all` checks appliance and bath showroom routes with the current plain HTTP transport and reports readable pages, WAF interstitials, and redirects as explicit evidence before a source is promoted.
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
| IKEA | SIK search | foundational, appliances, furniture, decor | **active** |
| GE Appliances | Searchspring | appliances | **active** |
| Bray & Scarff | NMG Hasura GraphQL | appliances | **active** |
| PC Richard | Demandware embedded JSON | appliances | **active** |
| Appliance Factory | AVB REST over HTTP/1.1 | appliances | **active** |
| Best Buy | Next SSR product cards | appliances | **active** |
| Abt | HTTP/1.1 search and product schema | appliances | **active** |
| Homewise Appliance | Bloomreach API | appliances | **active** |
| Floor & Decor | Algolia | foundational | **active** |
| Super Bright LEDs | Algolia | foundational, electrical | **active** |
| PROLIGHTING | Klevu | foundational, electrical | **active** |
| 1000Bulbs | HTML product cards | foundational, electrical | **active** |
| Bees Lighting | Shopify suggest | electrical | **active** |
| Lighting New York | Demandware embedded JSON | electrical, decor | **active** |
| Lightology | HTML GTM product cards | electrical, decor | **active** |
| PlumbersStock | Next SSR product cards | plumbing | **active** |
| FaucetDepot | Algolia | foundational, plumbing | **active** |
| FaucetList | Shopify suggest | plumbing | **active** |
| PlumbTile | Shopify suggest | plumbing | **active** |
| Modern Bathroom | Shopify suggest | plumbing | **active** |
| KBAuthority | Searchspring autocomplete HTML | plumbing, decor | **active** |
| Vintage Tub | Searchspring | plumbing, decor | **active** |
| Signature Hardware | Demandware suggestions HTML | plumbing, decor | **active** |
| Pioneer Mini Split | Shopify suggest | hvac | **active** |
| Sylvane | Shopify suggest | hvac | **active** |
| IWAe | Hyva product cards | hvac | **active** |
| The Hardware Hut | embedded JSON | foundational, hardware, materials | **active** |
| Lowe's | standard HTTP (partial) | foundational, appliances | stub (autocomplete + stores only) |
| Home Depot | SSR only | foundational, appliances | stub |
| Wayfair | PerimeterX clearance | foundational, appliances, furniture | stub |
| AllModern | PerimeterX clearance | appliances, furniture | stub |
| Restoration Hardware | DataDome | foundational, furniture | stub |

Active sources are queried by the fan-out. Stubs are registered for visibility and future activation.

**Lowe's** was assessed via the [Printing Press](https://github.com/mvanhorn/cli-printing-press) printer protocol (`probe-reachability` + `browser-sniff` on a Firefox HAR capture). Autocomplete and store locator are standard HTTP (no auth, User-Agent header only). Product search and the recommendation engine (`pythia-recs-svc`, 14 endpoints) require browser session cookies and are stubbed.

**Home Depot** was assessed via the same protocol. Every endpoint tested — autocomplete (`/complete/search/`), store finder (`/StoreFinderServices/v2/`), GraphQL search (`/federation-gateway/graphql`) — returned 403 from both stdlib and headered HTTP. A Firefox HAR captured zero requests to `www.homedepot.com`; Home Depot renders all product and store data server-side with no XHR/fetch API surface. Stubbed with no viable activation path short of an HTML scraping adapter.

## Category Routing

Queries are routed to sources by product category:

| Category | Active Sources |
|----------|---------------|
| foundational (broad fallback) | Ferguson, Rejuvenation, IKEA, Floor & Decor, Super Bright LEDs, PROLIGHTING, 1000Bulbs, FaucetDepot, The Hardware Hut |
| plumbing | Ferguson, Floor & Decor, PlumbersStock, FaucetDepot, FaucetList, PlumbTile, Modern Bathroom, KBAuthority, Vintage Tub |
| electrical | Super Bright LEDs, PROLIGHTING, 1000Bulbs, Bees Lighting, Lighting New York, Lightology |
| hvac | Pioneer Mini Split, Sylvane, IWAe, The Hardware Hut |
| flooring | Floor & Decor |
| hardware | The Hardware Hut, Rejuvenation, IKEA |
| materials | Floor & Decor, The Hardware Hut, IKEA |
| appliances | Ferguson, IKEA, GE Appliances, Bray & Scarff, PC Richard, Appliance Factory, Best Buy, Abt, Homewise Appliance |
| furniture | West Elm, Article, Shopify DTC, IKEA |
| decor | West Elm, Rejuvenation, Shopify DTC, IKEA, Lighting New York, Lightology, KBAuthority, Vintage Tub |

Room shortcuts expand to categories: `--room kitchen` = plumbing + electrical + flooring + hardware + materials + appliances + decor. `--room bathroom` = plumbing + electrical + flooring + hardware + materials + decor.

Reno Goat intentionally deprioritizes commodity builder supplies such as lumber, generic pipe/fittings, P-traps, wire nuts, raw service parts, and bulk fasteners unless they are part of a homeowner-selected installed system.

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

# Compound model lookup with spec docs and model-page probes
reno-goat-pp-cli model-intel "36 induction cooktop" --json

# Probe blocked/readable showroom routes
reno-goat-pp-cli source-probe --candidate all --json

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

### model-intel

Compound model lookup for installed-selection decisions.

- `model-intel <category query>` — discover product rows, extract model/SKU candidates, collect price/spec/install docs from source rows and returned product pages, and probe predictable model pages
- `model-intel <model>` — probe exact model-page candidates and report whether each source is readable, blocked, or missing
- `model-intel <query>` — auto-infer installed-selection categories such as HVAC, electrical, plumbing, flooring, hardware, materials, appliances, furniture, or decor from the query
- `model-intel <query> --category hvac --sources routed` — force model/SKU extraction against the active source set for a category
- `model-intel <query> --room kitchen --sources routed` — expand a room into categories, then search the routed active sources
- `model-intel <query> --sources active` — run the extraction pass across every active source
- `model-intel <query> --search-offers=false` — skip search-result model and price fallback evidence

Default `--sources auto` first infers installed-selection categories from the query and routes through Reno Goat's category source map. If no category is inferred, it falls back to the appliance-oriented GE Appliances and Bosch discovery path. Routed and active modes still allow explicit control, so HVAC, electrical, plumbing, flooring, hardware, and finish-material rows can contribute priced model/SKU candidates. For finish selections that do not use appliance-style model numbers, `model-intel` preserves source product IDs or URL SKU tokens as selection identifiers. Returned rows and search-result offer URLs are enriched from their product pages for linked PDFs; this captures spec/install/dimmer/warranty documents when the retailer exposes them, and filters sibling-model or promo PDFs when a model number is known. AJ Madison model URLs are probed for exact-model or appliance-like records and reported as blocked when PerimeterX/CAPTCHA prevents direct CLI replay.

With `--search-offers=true`, `model-intel` also queries readable Brave Search result pages. Category queries can add labeled `brave-search:<host>` model candidates from product-result fields, while exact-model lookups record product-price fields as fallback offers filtered to exact-model title/URL anchors. The CLI then tries a bounded offer-page enrichment pass; readable offer pages can add model-specific PDFs, while blocked pages are reported in `probe_status`. These are clearly labeled search-result evidence and are used as fallback discovery when direct retailer pages are WAF-blocked or not yet implemented.

Priced rows can also be enriched from manufacturer product pages when the retailer already supplies a sourceable model row. For Moen shower-valve and trim rows returned by active plumbing sources such as KBAuthority or FaucetDepot, `model-intel` probes the matching `shop.moen.com/products/<model>` page, adds the Moen product-page offer where available, and captures canonical `assets.moen.com` product-specification, instruction-sheet, and exploded-parts PDFs. For Leviton dimmer/control rows returned by active electrical sources such as 1000Bulbs, `model-intel` probes `leviton.com/products/<model>` and captures the manufacturer product bulletin plus English instruction-sheet PDF. These are bounded manufacturer enrichment steps, not standalone manufacturer sources.

Manufacturer catalogs that do not expose pricing are used only as `model-intel` discovery sources, not as active retail sources. Broan-NuTone is the first explicit example: bath-fan and range-hood queries can pull Broan/NuTone model numbers and spec/install PDFs from readable manufacturer pages, then `--search-offers=true` tries a deterministic bounded batch of exact-model retailer pricing probes so manufacturer-only rows are useful only when paired with sourceable offer evidence.

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

### source-probe

Probe candidate retailer/showroom routes with Reno Goat's current plain HTTP transport. Each probe reads up to 256KB of the response body and reports `body_truncated` when the page is larger than the diagnostic window.

- `source-probe --candidate appliance-showrooms` — probe appliance showroom seed routes, including AJ Madison, Abt, Bray & Scarff, Ferguson Home, Grand Appliance, and Plesser's
- `source-probe --candidate appliance-priority-gaps` — probe local showroom and big-box priority routes: ABW, ADU catalog, AJ Madison, Abt, Best Buy, Costco, PC Richard, Appliance Factory, Homewise Appliance, Spencer's, Grand Appliance, and Warners' Stellian
- `source-probe --candidate bath-showrooms` — probe bath showroom seed routes, including Build.com, FaucetDirect, QualityBath, Signature Hardware, and Supply.com
- `source-probe --candidate bath-priority-gaps` — probe bath-showroom priority routes: QualityBath, Vintage Tub, HomePerfect, DecorPlanet, Build.com, and Signature Hardware
- `source-probe --candidate all` — run every seed and priority-gap group
- `source-probe --candidate none --url <url>` — probe a custom route

The result labels routes as `readable`, `blocked`, or `unreachable` and records known blockers such as Cloudflare and PerimeterX where the captured response body exposes them. This is a triage surface; a readable root page still needs product/API extraction before it becomes an active source.

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
