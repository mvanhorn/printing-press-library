---
name: pp-home-goat
description: "Multi-source home furnishing CLI combining 5 Tier 1 APIs: search, compare, stores, delivery, reviews"
author: "H179922"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - home-goat-pp-cli
    install:
      - kind: go
        bins: [home-goat-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/home-goat/cmd/home-goat-pp-cli
---

# Home Goat — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `home-goat-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install home-goat --cli-only
   ```
2. Verify: `home-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/home-goat/cmd/home-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every home furnishing query in one CLI — product search, price comparison, store locator, delivery checks, reviews, and deal finding across Ferguson, West Elm, Rejuvenation, Article, and Shopify DTC stores (Schoolhouse, Blu Dot, Gus Modern, Floyd, Lulu & Georgia), with category-based routing that sends queries to the sources that carry each product type.

## Command Reference

**brands** — List brands available at a source and cross-reference brand availability across retailers. Shows which retailers carry a given brand and at what price points.

- `home-goat-pp-cli brands ferguson-brands` — List brands available at Ferguson by searching with a brand filter. Returns brand facets from search results.
- `home-goat-pp-cli brands rejuvenation-brands` — List brands available at Rejuvenation via Constructor.io brand facets.
- `home-goat-pp-cli brands westelm-brands` — List brands available at West Elm via Constructor.io brand facets.

**categories** — Browse product category trees at a source. Currently supported for Rejuvenation via their catalog API.

- `home-goat-pp-cli categories` — Browse the Rejuvenation product category tree. Returns subcategories, product counts, and navigation paths.

**compare** — Side-by-side product comparison across any sources. Fetches full details for each product URL and renders a normalized comparison table of price, specs, ratings, and availability.

- `home-goat-pp-cli compare <urls>` — Compare 2+ products side-by-side.

**deals** — Find active promotions, sales, and eligible discounts. Queries promotion eligibility APIs for current deals.

- `home-goat-pp-cli deals` — Check eligible promotions for a West Elm product or category.

**delivery** — Check delivery availability and options by postal code. Supported for West Elm and Rejuvenation via their delivery information APIs.

- `home-goat-pp-cli delivery rejuvenation-delivery` — Check delivery availability and options from Rejuvenation by postal code.
- `home-goat-pp-cli delivery westelm-delivery` — Check delivery availability and options from West Elm by postal code.

**find-related** — Find cross-sell and complementary products. Currently supported for Article via the CROSS_SELL APQ query.

- `home-goat-pp-cli find-related <product_url>` — Find cross-sell/related products from Article via APQ CROSS_SELL query.

**find-similar** — Find similar products at the same retailer. Currently supported for Article via the SIMILAR_PRODUCTS APQ query.

- `home-goat-pp-cli find-similar <product_url>` — Find similar products from Article via APQ SIMILAR_PRODUCTS query. Returns visually and categorically similar items.

**product** — Get full product details from any source. Resolves the source from the product URL and fetches complete product data including specs, images, variants, and pricing.

- `home-goat-pp-cli product article-product` — Get full product details from Article via APQ PRODUCT query.
- `home-goat-pp-cli product ferguson-product` — Get full product details from Ferguson via GraphQL ProductDetail query.
- `home-goat-pp-cli product rejuvenation-product` — Get full product details from Rejuvenation.
- `home-goat-pp-cli product shopify-product` — Get full product details from a Shopify DTC store via Storefront API. Resolves store from URL.
- `home-goat-pp-cli product westelm-product` — Get full product details from West Elm.

**product-search** — Fan-out product search across all Tier 1 sources. Category-based routing sends queries to relevant sources. Returns normalized products with unified price, rating, and brand fields.

- `home-goat-pp-cli product-search article-search` — Search products via Article APQ GraphQL. Uses SEARCH_PRODUCTS persisted query hash.
- `home-goat-pp-cli product-search ferguson-search` — Search products via Ferguson GraphQL. Returns ProductSearchResult (count + products) or SearchRedirect.
- `home-goat-pp-cli product-search rejuvenation-search` — Search products via Rejuvenation Constructor.io API. Same API shape as West Elm with different key.
- `home-goat-pp-cli product-search shopify-search` — Search products across Shopify DTC stores via Storefront API GraphQL.
- `home-goat-pp-cli product-search westelm-search` — Search products via West Elm Constructor.io API. Returns faceted results with product data, prices, and images.

**reviews** — Get product reviews from a single source. Supports Ferguson (via GraphQL) and Article (via APQ queries for reviews and UGC media).

- `home-goat-pp-cli reviews article-reviews` — Get product reviews from Article via APQ getProductReviewsByProductId query.
- `home-goat-pp-cli reviews ferguson-reviews` — Get product reviews from Ferguson via the ProductDetail GraphQL query (reviews are embedded in product detail response).

**stores** — Find physical retail stores near a location. Supported for West Elm and Rejuvenation via their store locator APIs.

- `home-goat-pp-cli stores rejuvenation-stores` — Find Rejuvenation stores near a location.
- `home-goat-pp-cli stores westelm-stores` — Find West Elm stores near a location.

**suggest** — Autocomplete and typeahead suggestions from Constructor.io sources (West Elm and Rejuvenation). Returns search suggestions and optionally product previews.

- `home-goat-pp-cli suggest rejuvenation-suggest` — Autocomplete suggestions from Rejuvenation via Constructor.io.
- `home-goat-pp-cli suggest westelm-suggest` — Autocomplete suggestions from West Elm via Constructor.io.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
home-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `home-goat-pp-cli watch <product-url> [--threshold <percent>]` — Price watch system.
- `home-goat-pp-cli project <subcommand> [args]` — Project tracker for grouping saved products into named renovation/design projects with running budget totals.
- `home-goat-pp-cli saved [--check-stale]` — Manage saved/bookmarked products. `saved` lists all saved products.
- `home-goat-pp-cli config` — View and edit CLI configuration (default category routing, preferred sources, output format, SQLite database path).
- `home-goat-pp-cli sources` — List all upstream API sources, their status (active/stub/deferred), categories served, rate limits
- `home-goat-pp-cli history` — Show past search queries with timestamps, result counts, and sources queried.
- `home-goat-pp-cli save <product-url>` — Bookmark a product URL for later. Stores the product snapshot (title, price, source, URL) in local SQLite.
- `home-goat-pp-cli version` — Print CLI version, build info, configured sources, and SQLite database path.

## Auth Setup

No authentication required.

Run `home-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  home-goat-pp-cli brands ferguson-brands --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
home-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
home-goat-pp-cli feedback --stdin < notes.txt
home-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/home-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `HOME_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HOME_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
home-goat-pp-cli profile save briefing --json
home-goat-pp-cli --profile briefing brands ferguson-brands
home-goat-pp-cli profile list --json
home-goat-pp-cli profile show briefing
home-goat-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `home-goat-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/home-goat/cmd/home-goat-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add home-goat-pp-mcp -- home-goat-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which home-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   home-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `home-goat-pp-cli <command> --help`.
