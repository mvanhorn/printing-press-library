# G2 CLI

**Every G2 Marketplace surface, plus a credit-aware local mirror, full-text review search, and buyer-intent triage no other G2 tool offers.**

g2-pp-cli unifies the v2 OpenAPI surface and the legacy syndication API behind one auth layer, mirrors products, reviews, vendors, categories, and buyer-intent into a local SQLite store, and adds workflows the Anthropic G2 MCP and every scraper miss: credit-burn forecasting (`credits forecast`), Alternatives-signal switching threats (`alt-track`), cross-product weekly diffs (`watch products`), and FTS over synced reviews.

Printed by [@rahulbansal16](https://github.com/rahulbansal16) (Rahul Bansal).

## Install

The recommended path installs both the `g2-pp-cli` binary and the `pp-g2` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install g2
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install g2 --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install g2 --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install g2 --agent claude-code
npx -y @mvanhorn/printing-press-library install g2 --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/cmd/g2-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/g2-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-g2 --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-g2 --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-g2 skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-g2. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/g2-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `G2_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/cmd/g2-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "g2": {
      "command": "g2-pp-mcp",
      "env": {
        "G2_DEFAULTHOST": "<defaultHost>",
        "G2_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

G2 ships two authentication surfaces. The v2 API uses OAuth 2.0 (`G2_CLIENT_ID` + `G2_CLIENT_SECRET`) plus an `AccountAPIToken` header (`G2_ACCOUNT_API_TOKEN`). The legacy syndication API (products, reviews, vendors, product_mappings, hashed_users) uses an `api_token` query parameter (`G2_SYNDICATION_TOKEN`). `g2-pp-cli auth login` walks you through both and stores them in `~/.config/g2-pp-cli/auth.json`. A paid G2 Sell-tier subscription is required for production endpoints; the developer portal at `my.g2.com/developers` provides a 1,000-call/month starter quota.

## Quick Start

```bash
# Validate auth, list granted OAuth scopes, and surface credit balance without burning a call.
g2-pp-cli doctor --dry-run

# Mirror products and categories into the local SQLite store.
g2-pp-cli sync --resources products,categories --since 30d --json

# Sync products then their reviews (hierarchical); FTS index updates automatically.
g2-pp-cli sync --resources products,products_reviews --since 7d --json

# Full-text search across synced reviews.
g2-pp-cli search "rate limit" --type reviews --limit 20 --json

# Daily buyer-intent triage flattened into a CSV ready for the CRM.
g2-pp-cli buyer-intent list --since 24h --min-score 50 --csv

# Project month-end credit spend before scheduling more sync runs.
g2-pp-cli credits forecast --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`watch products`** — Diff star ratings, new review counts, and badge changes across a list of products since a given window.

  _Reach for this when a competitive-intel or PMM workflow asks for a head-to-head delta over a window. Replaces tab-by-tab eyeballing of multiple G2 pages._

  ```bash
  g2-pp-cli watch products --product datadog,new-relic,dynatrace --since 7d --json
  ```
- **`search`** — Full-text search across locally synced reviews, scoped by product list and structured filters (rating, segment).

  _Use when a buyer or competitive analyst wants to grep verbatims across many products' review corpora._

  ```bash
  g2-pp-cli search "rate limit" --type reviews --limit 20 --json
  ```
- **`alt-track`** — Rank companies showing Alternatives-signal buyer intent against your product, weighted by visit count and employee size.

  _Pick this when sales needs a ranked list of accounts evaluating switching away from your product._

  ```bash
  g2-pp-cli alt-track my-product --since 30d --csv
  ```
- **`analytics`** — Per category, surface products in the top growth quartile by 30-day review velocity that aren't yet top quartile by absolute review count.

  _Reach for this when a PMM or competitive-intel analyst wants early signals on momentum players in a market._

  ```bash
  g2-pp-cli analytics --type products --group-by category --limit 20 --json
  ```
- **`reviews list`** — Filter synced reviews for the syndication-eligible flag, scoped by product and a recency window.

  _Pick this when wiring a marketing site or landing page to auto-refresh approved review quotes._

  ```bash
  g2-pp-cli reviews list --product my-product --syndication-eligible --since 7d --json
  ```
- **`watch market-signals`** — Diff locally snapshotted market_signals for a category over a window; emit intent-score and visits-count deltas.

  _Reach for this during a Friday competitive-intel report to spot category-level momentum changes._

  ```bash
  g2-pp-cli watch market-signals --category devops-monitoring --since 7d --json
  ```

### Reachability mitigation
- **`credits forecast`** — Project month-end credit spend from trailing 14-day usage and refuse sync runs that would exceed remaining credits.

  _Pick this before a heavy sync to avoid mid-month throttling on credit-metered endpoints._

  ```bash
  g2-pp-cli credits forecast --json
  ```

### Agent-native plumbing
- **`buyer-intent list`** — Filter buyer-intent rows by recency and minimum score and flatten nested firmographics into a sales-ready CSV.

  _Pick this for a daily cron that pushes high-intent companies into the team's CRM or Slack._

  ```bash
  g2-pp-cli buyer-intent list --since 24h --min-score 50 --csv
  ```
- **`analytics`** — For each product in a list, return the lowest-rated reviews and their cons verbatims as JSON for downstream summarization.

  _Use when competitive intel needs to ground a complaint roundup across competitors; pipe the JSON to a summarizer of choice._

  ```bash
  g2-pp-cli analytics --type reviews --group-by product --limit 10 --json
  ```

## Recipes


### Monday morning competitive snapshot

```bash
g2-pp-cli watch products --product datadog,new-relic,dynatrace --since 7d --json --select "product,star_delta,new_reviews"
```

Joins yesterday's `products` snapshot with new `reviews` rows to produce a flat star-delta-by-competitor view; `--select` keeps the agent context short.

### Daily buyer-intent triage to CSV

```bash
g2-pp-cli buyer-intent list --since 24h --min-score 50 --csv > intent-$(date +%Y%m%d).csv
```

Cron-friendly: emits a flat CSV with company, domain, employees, industry, intent_score, page_type, signal_type.

### Switching-threat list for sales

```bash
g2-pp-cli alt-track my-product --since 30d --csv
```

Surfaces companies showing Alternatives-signal intent against your product, ranked by visit count and employee size. The website paywalls this view.

### Find rate-limit complaints across competitors

```bash
g2-pp-cli search "rate limit" --type reviews --limit 50 --json --select "product_id,title,rating,cons"
```

FTS5 query across all synced reviews; pair with `--select` to keep nested response payloads manageable for downstream agents.

### Pre-sync credit check

```bash
g2-pp-cli credits forecast --json
```

Reads the local `credit_ledger`, projects month-end burn from the trailing 14-day average, and exits non-zero if a planned sync would exceed the remaining quota.

## Usage

Run `g2-pp-cli --help` for the full command reference and flag list.

## Commands

### buyer-intent

Manage buyer intent

- **`g2-pp-cli buyer-intent`** - **Requires `buyer_intent.read` scope.**

Query buyer intent data across one or more products in a single request.
Pass product_id to specify which products, or omit to use your assigned products.
Supports multi-product queries — the data is not scoped to a single subject product.

### categories

Manage categories

- **`g2-pp-cli categories get`** - **No scope required.**
- **`g2-pp-cli categories get-all-category-features`** - List all category product features grouped by category
- **`g2-pp-cli categories get-category`** - Shows a single category

### market-signals

Manage market signals

- **`g2-pp-cli market-signals`** - **Requires `market_signals.read` scope.**

Retrieve buyer intent signals (interactions) for specific categories.

Returns one result per buyer intent interaction within the specified date range.
Each interaction includes the category UUID, company name, and domain.

If no dates are provided, defaults to today for both start and end date.

Results are paginated using cursor-based pagination (default 25, max 100 per page).

### organization

Manage organization


### partners

Manage partners


### product-mappings

Manage product mappings

- **`g2-pp-cli product-mappings create`** - Create a product mapping
- **`g2-pp-cli product-mappings get`** - List of product mappings
- **`g2-pp-cli product-mappings update`** - Update a product mapping

### product-ratings

Manage product ratings

- **`g2-pp-cli product-ratings <product_id>`** - **Requires `products.ratings.read` scope.**

### products

Manage products

- **`g2-pp-cli products get`** - **Requires `products.read` scope.**
- **`g2-pp-cli products get-all-features`** - List all product features grouped by product
- **`g2-pp-cli products get-productid`** - **Requires `products.read` scope.**

### questions

Manage questions

- **`g2-pp-cli questions get`** - **Requires `questions.read` scope.**
- **`g2-pp-cli questions get-id`** - **Requires `questions.read` scope.**

### reports

Manage reports

- **`g2-pp-cli reports <product_id>`** - **Requires `reports.read` scope.**

### sandbox

Manage sandbox

- **`g2-pp-cli sandbox <subject_product_id>`** - **No scope is required to access this sandbox endpoint**

**⚠️ Sandbox API**: This endpoint is a sandbox endpoint and is data limited. Only data between 6 months to 1 year ago will be available.

Query buyer intent data using an analytical query interface.

## Query Language

This endpoint uses an OLAP-style query language that supports:
- **Dimensions**: Group and categorize data (e.g., company_name, company_domain, day)
- **Measures**: Aggregate numerical values (e.g., total_activity, visitor_count)
- **Filters**: Apply conditions to limit results

### Filter Operators
Filters use the format `dimension_filters[name_suffix]` where `name`
is the dimension name and `suffix` indicates the operation:

- `_eq`: Equals (exact match)
- `_not_eq`: Not equals
- `_cont`: Contains (substring match)
- `_not_cont`: Does not contain
- `_gt`: Greater than
- `_gteq`: Greater than or equal
- `_lt`: Less than
- `_lteq`: Less than or equal
- `_present`: Field has a value
- `_empty`: Field is empty

### Time Series vs Aggregated Data
- **Aggregated**: Request dimensions without `day` for company-level summaries
- **Time Series**: Include `day` dimension for daily breakdowns over time

### Sorting Results
Use the `sort` parameter to order results by any dimension or measure:
- **Ascending**: `sort=company_name` or `sort=total_activity`
- **Descending**: `sort=-company_intent_score` (prefix with `-`)
- **Default**: Results are sorted by `-company_intent_score` (highest intent first)

Available sort fields include all dimensions and measures listed above.

### Examples
- Company summary: `dimensions=company_name,company_domain&measures=total_activity`
- Time series: `dimensions=company_name,day&measures=company_intent_score`
- Filtered: `dimension_filters[company_name_cont]=Acme&dimensions=company_name`
- Sorted by activity: `dimensions=company_name&measures=total_activity&sort=-total_activity`

### screenshots

Manage screenshots

- **`g2-pp-cli screenshots get`** - **Requires `screenshots.read` scope.**
- **`g2-pp-cli screenshots get-id`** - **Requires `screenshots.read` scope.**

### user

Manage user

- **`g2-pp-cli user`** - Returns the currently authenticated user.

### users

Manage users

- **`g2-pp-cli users add-product-to-research-board`** - Add a product to a research board
- **`g2-pp-cli users batch-add-products-to-research-board`** - Add multiple products to a research board
- **`g2-pp-cli users batch-remove-products-from-research-board`** - Remove multiple products from a research board
- **`g2-pp-cli users create-research-board`** - Create a research board
- **`g2-pp-cli users delete-research-board`** - Delete a research board
- **`g2-pp-cli users get-current-product-review`** - **⚠️ G2 is currently restricting access to this endpoint.**
This endpoint retrieves the review for a specific product owned by the current user.
- **`g2-pp-cli users get-current-products`** - **⚠️ G2 is currently restricting access to this endpoint.**
This endpoint shows subscription-specific product information, of
products that the current account has ownership of.
- **`g2-pp-cli users get-research-board`** - Get a research board
- **`g2-pp-cli users list-research-board-products`** - List products on a research board
- **`g2-pp-cli users list-research-boards`** - List research boards for current user
- **`g2-pp-cli users remove-product-from-research-board`** - Remove a product from a research board
- **`g2-pp-cli users submit-current-product-review`** - **⚠️ G2 is currently restricting access to this endpoint.**
 This endpoint allows the user to submit or update a review for a specific product.
 The review is based on predefined questions and answers.'
- **`g2-pp-cli users update-research-board`** - Update a research board

### vendors

Manage vendors

- **`g2-pp-cli vendors get`** - **Requires `vendors.read` scope.**
- **`g2-pp-cli vendors get-id`** - **Requires `vendors.read` scope.**


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
g2-pp-cli categories get

# JSON for scripting and agents
g2-pp-cli categories get --json

# Filter to specific fields
g2-pp-cli categories get --json --select id,name,status

# Dry run — show the request without sending
g2-pp-cli categories get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
g2-pp-cli categories get --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `G2_DEFAULT_HOST` resolves `{defaultHost}`

Base URL: `https://{defaultHost}`

## Health Check

```bash
g2-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/g2-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `G2_DEFAULTHOST` | endpoint | Yes |  |
| `G2_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `g2-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $G2_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on v2 endpoints** — Re-run `g2-pp-cli auth login` and confirm the OAuth scopes you registered include the resource you are calling (e.g., `buyer_intent.read`).
- **401 on syndication endpoints (/api/2018-01-01/syndication/...)** — Syndication uses a separate `api_token` query parameter, not OAuth. Set `G2_SYNDICATION_TOKEN` or re-run `g2-pp-cli auth login --syndication`.
- **429 Too Many Requests / credit exhaustion mid-run** — Check `g2-pp-cli credits balance --json` and forecast burn with `g2-pp-cli credits forecast --json`. Add `--budget-check` to future sync calls to abort early.
- **Reviews show empty `reviewer` field** — G2 intentionally hashes reviewer identities. The `hashed_users` payload exposes segment, industry, and country only. This is expected, not a bug.
- **Sync returns 0 rows for products you can see on g2.com** — Your developer-portal app must include `products.read` (and for reviews, `products.reviews.read`). Re-register at `my.g2.com/developers` with the full scope list.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**biegehydra/Advanced-G2-Scraper**](https://github.com/biegehydra/Advanced-G2-Scraper) — JavaScript (3 stars)
- [**Anthropic G2 MCP Server**](https://documentation.g2.com/docs/g2-mcp-server) — TypeScript (closed-source, hosted)
- [**G2 Postman Workspace**](https://www.postman.com/g2-demo) — Postman
- [**G2 Scraper (RapidAPI)**](https://g2scraper.com/) — HTTP service

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
