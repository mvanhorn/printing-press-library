# Home Goat CLI

Every home furnishing query in one CLI — product search, price comparison, store locator, delivery checks, reviews, and deal finding across Ferguson, West Elm, Rejuvenation, Article, and Shopify DTC stores (Schoolhouse, Blu Dot, Gus Modern, Floyd, Lulu & Georgia), with category-based routing that sends queries to the sources that carry each product type.

Learn more at [Home Goat](https://www.fergusonhome.com).

Printed by [@H179922](https://github.com/H179922) (H179922).

## Install

The recommended path installs both the `home-goat-pp-cli` binary and the `pp-home-goat` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install home-goat
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install home-goat --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install home-goat --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install home-goat --agent claude-code
npx -y @mvanhorn/printing-press-library install home-goat --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/home-goat/cmd/home-goat-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/home-goat-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-home-goat --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-home-goat --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-home-goat skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-home-goat. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/home-goat-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/home-goat/cmd/home-goat-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "home-goat": {
      "command": "home-goat-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
home-goat-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
home-goat-pp-cli brands ferguson-brands
```

## Usage

Run `home-goat-pp-cli --help` for the full command reference and flag list.

## Commands

### brands

List brands available at a source and cross-reference brand availability across retailers. Shows which retailers carry a given brand and at what price points.

- **`home-goat-pp-cli brands ferguson-brands`** - List brands available at Ferguson by searching with a brand filter. Returns brand facets from search results.
- **`home-goat-pp-cli brands rejuvenation-brands`** - List brands available at Rejuvenation via Constructor.io brand facets.
- **`home-goat-pp-cli brands westelm-brands`** - List brands available at West Elm via Constructor.io brand facets.

### categories

Browse product category trees at a source. Currently supported for Rejuvenation via their catalog API.

- **`home-goat-pp-cli categories`** - Browse the Rejuvenation product category tree. Returns subcategories, product counts, and navigation paths.

### compare

Side-by-side product comparison across any sources. Fetches full details for each product URL and renders a normalized comparison table of price, specs, ratings, and availability.

- **`home-goat-pp-cli compare <urls>`** - Compare 2+ products side-by-side. Each URL is resolved to its source, fetched in parallel, and rendered in a comparison table. Works across different sources.

### deals

Find active promotions, sales, and eligible discounts. Queries promotion eligibility APIs for current deals.

- **`home-goat-pp-cli deals`** - Check eligible promotions for a West Elm product or category. Returns active sales, discount percentages, and promo codes.

### delivery

Check delivery availability and options by postal code. Supported for West Elm and Rejuvenation via their delivery information APIs.

- **`home-goat-pp-cli delivery rejuvenation-delivery`** - Check delivery availability and options from Rejuvenation by postal code.
- **`home-goat-pp-cli delivery westelm-delivery`** - Check delivery availability and options from West Elm by postal code.

### find-related

Find cross-sell and complementary products. Currently supported for Article via the CROSS_SELL APQ query.

- **`home-goat-pp-cli find-related <product_url>`** - Find cross-sell/related products from Article via APQ CROSS_SELL query. Returns complementary items for the given product.

### find-similar

Find similar products at the same retailer. Currently supported for Article via the SIMILAR_PRODUCTS APQ query.

- **`home-goat-pp-cli find-similar <product_url>`** - Find similar products from Article via APQ SIMILAR_PRODUCTS query. Returns visually and categorically similar items.

### product

Get full product details from any source. Resolves the source from the product URL and fetches complete product data including specs, images, variants, and pricing.

- **`home-goat-pp-cli product article-product`** - Get full product details from Article via APQ PRODUCT query. Returns 50+ fields including specs, reviews summary, delivery estimates, and customization options.
- **`home-goat-pp-cli product ferguson-product`** - Get full product details from Ferguson via GraphQL ProductDetail query.
- **`home-goat-pp-cli product rejuvenation-product`** - Get full product details from Rejuvenation.
- **`home-goat-pp-cli product shopify-product`** - Get full product details from a Shopify DTC store via Storefront API. Resolves store from URL.
- **`home-goat-pp-cli product westelm-product`** - Get full product details from West Elm.

### product-search

Fan-out product search across all Tier 1 sources. Category-based routing sends queries to relevant sources. Returns normalized products with unified price, rating, and brand fields.

- **`home-goat-pp-cli product-search article-search`** - Search products via Article APQ GraphQL. Uses SEARCH_PRODUCTS persisted query hash. Returns paginated product results with pricing and ratings.
- **`home-goat-pp-cli product-search ferguson-search`** - Search products via Ferguson GraphQL. Returns ProductSearchResult (count + products) or SearchRedirect. Primary source for foundational fixtures and appliances.
- **`home-goat-pp-cli product-search rejuvenation-search`** - Search products via Rejuvenation Constructor.io API. Same API shape as West Elm with different key. Primary source for foundational hardware and decor.
- **`home-goat-pp-cli product-search shopify-search`** - Search products across Shopify DTC stores via Storefront API GraphQL. Fans out to all configured stores (Schoolhouse, Blu Dot, Gus Modern, Floyd, Lulu & Georgia).
- **`home-goat-pp-cli product-search westelm-search`** - Search products via West Elm Constructor.io API. Returns faceted results with product data, prices, and images.

### reviews

Get product reviews from a single source. Supports Ferguson (via GraphQL) and Article (via APQ queries for reviews and UGC media).

- **`home-goat-pp-cli reviews article-reviews`** - Get product reviews from Article via APQ getProductReviewsByProductId query. Includes ratings, review text, and UGC media.
- **`home-goat-pp-cli reviews ferguson-reviews`** - Get product reviews from Ferguson via the ProductDetail GraphQL query (reviews are embedded in product detail response).

### stores

Find physical retail stores near a location. Supported for West Elm and Rejuvenation via their store locator APIs.

- **`home-goat-pp-cli stores rejuvenation-stores`** - Find Rejuvenation stores near a location.
- **`home-goat-pp-cli stores westelm-stores`** - Find West Elm stores near a location.

### suggest

Autocomplete and typeahead suggestions from Constructor.io sources (West Elm and Rejuvenation). Returns search suggestions and optionally product previews.

- **`home-goat-pp-cli suggest rejuvenation-suggest`** - Autocomplete suggestions from Rejuvenation via Constructor.io.
- **`home-goat-pp-cli suggest westelm-suggest`** - Autocomplete suggestions from West Elm via Constructor.io.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
home-goat-pp-cli brands ferguson-brands

# JSON for scripting and agents
home-goat-pp-cli brands ferguson-brands --json

# Filter to specific fields
home-goat-pp-cli brands ferguson-brands --json --select id,name,status

# Dry run — show the request without sending
home-goat-pp-cli brands ferguson-brands --dry-run

# Agent mode — JSON + compact + no prompts in one flag
home-goat-pp-cli brands ferguson-brands --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
home-goat-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/home-goat-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
