# Shopping CLI

**Every LemmeBuyIt retail endpoint, plus a local store that compares one UPC across 70+ retailers, ranks weekly price drops, and computes Amazon FBA arbitrage margin — compound queries the per-retailer API can't answer.**

Shopping wraps the LemmeBuyIt aggregated-retail API (115M+ products across 70+ retailers) and adds a local SQLite layer no other tool has. `index` syncs products and price history; `compare` finds the cheapest retailer for a shared identifier; `deals` runs a compound discount/price/rating/stock query across stores; `arbitrage` ranks Amazon FBA resale margin; `price-drops` surfaces weekly price drops and `leaderboard` ranks discount buckets. Agent-native --json/--select, offline search, typed exit codes.

Learn more at [Shopping](https://www.lemmebuyit.com/developer).

Created by [@NicholasSpisak](https://github.com/NicholasSpisak) (NicholasSpisak).

## Install

The recommended path installs both the `shopping-pp-cli` binary and the `pp-shopping` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install shopping
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install shopping --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install shopping --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install shopping --agent claude-code
npx -y @mvanhorn/printing-press-library install shopping --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopping/cmd/shopping-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopping-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install shopping --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-shopping --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-shopping --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install shopping --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopping-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SHOPPING_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopping/cmd/shopping-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "shopping": {
      "command": "shopping-pp-mcp",
      "env": {
        "SHOPPING_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authenticate with a single LemmeBuyIt API key sent as the `X-API-Key` header on every request (set `SHOPPING_API_KEY`, or run `shopping-pp-cli auth set-token <key>`). Free keys reach the `/shopping/` product surface; paid keys additionally unlock full product search, weekly price history, Amazon ASIN history, and the Amazon profitability data that powers `arbitrage`. Get a key at https://www.lemmebuyit.com/developer.

## Quick Start

```bash
# Verify config and connectivity before pulling data.
shopping-pp-cli doctor --dry-run

# See which retailers your key can reach.
shopping-pp-cli retailers list --json

# Build the local store from one retailer's on-sale catalog.
shopping-pp-cli index --retailer walmart --on-sale --limit 200

# Run a compound deal query over the synced store.
shopping-pp-cli deals --min-discount 30 --max-price 100 --in-stock --json

# Find the cheapest retailer for a shared identifier.
shopping-pp-cli compare 012345678905 --lookup-type upc --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`index`** — Pull products (and optional price history) from one or more retailers into a fast local SQLite store so every other query runs instantly and offline.

  _Reach for this first: the compare/deals/arbitrage/analytics commands all read what index populates._

  ```bash
  shopping-pp-cli index --retailer walmart --retailer target --on-sale --limit 500
  ```
- **`watch status`** — Pin items you care about and, after each index refresh, see which ones moved, by how much, and which hit your target price.

  _Use it for change-since-last-observation on a curated set instead of re-fetching and eyeballing prices._

  ```bash
  shopping-pp-cli watch status --json
  ```

### Cross-retailer intelligence
- **`compare`** — See which synced retailer sells the exact same item (by UPC/EAN/GTIN/ASIN) cheapest right now, ranked, with the savings spread.

  _Use it to answer 'who has this cheapest' in one call instead of N per-retailer lookups._

  ```bash
  shopping-pp-cli compare 012345678905 --lookup-type upc --in-stock --json
  ```
- **`deals`** — Surface deals that clear several bars at once — deep discount, under a price ceiling, well-reviewed, in stock — across all tracked retailers in one ranked list.

  _Pick this when an agent needs the best deals worth a human's time, not a raw product dump._

  ```bash
  shopping-pp-cli deals --min-discount 30 --max-price 100 --min-rating 4 --in-stock --sort discount --json
  ```

### Profitability
- **`arbitrage`** — Rank synced products by what you would net reselling them on Amazon after referral and FBA fees, filtered by ROI and buy price.

  _Use it to filter clearance products down to the ones that actually clear a resale ROI bar._

  ```bash
  shopping-pp-cli arbitrage --retailer walmart --min-roi 30 --max-buy-price 50 --in-stock --json
  ```

### Time-series analytics
- **`price-drops`** — Rank the products that fell the most over the last week (or N weeks) across everything synced, by both dollar and percent drop.

  _Reach for it to catch fresh markdowns first instead of re-pulling each product's history by hand._

  ```bash
  shopping-pp-cli price-drops --weeks 1 --min-drop-pct 15 --limit 25 --json
  ```
- **`leaderboard`** — Show which retailers and categories are consistently the deepest-discount buckets in your synced data, by average discount, on-sale count, or average price.

  _Use it to decide which retailer/category to scan first when hunting deals._

  ```bash
  shopping-pp-cli leaderboard --by avg-discount --limit 15 --json
  ```

## Recipes


### Cheapest retailer for a UPC

```bash
shopping-pp-cli compare 012345678905 --lookup-type upc --in-stock --json --select results.retailer_id,results.current_price
```

Cross-retailer compare narrowed to the two fields an agent needs to act on.

### Best in-stock deals

```bash
shopping-pp-cli deals --min-discount 40 --max-price 75 --in-stock --sort discount --limit 20 --json
```

Compound discount/price/stock query ranked by discount over the synced store.

### FBA arbitrage shortlist

```bash
shopping-pp-cli arbitrage --retailer walmart --min-roi 30 --max-buy-price 50 --json --select results.product_name,results.roi,results.net_margin
```

Rank resale candidates by ROI, selecting only the decision fields.

### This week's biggest drops

```bash
shopping-pp-cli price-drops --weeks 1 --min-drop-pct 20 --limit 25 --json
```

Weekly time-series delta ranking across every synced product.

## Usage

Run `shopping-pp-cli --help` for the full command reference and flag list.

## Commands

### amazon

Manage amazon

- **`shopping-pp-cli amazon <asin>`** - Returns chart-ready weekly Amazon-side time-series data for a
specific ASIN — direct access to amazon_price_history without
going through a retailer's product_id. Useful when the consumer
already knows the ASIN (e.g., charting the historical-backfill
data without a Walmart UPC mapping in the path).

The returned series carries Amazon retail / 3P-new / Buy Box
prices plus monthly_sold per week, sorted ascending by `ts`.
Weeks with no observed samples are omitted.

For Walmart-side or merged history, use
`/v1/retailers/walmart/products/{product_id}/price-history` instead.

### retailers

Operations related to retailers and their products.

- **`shopping-pp-cli retailers`** - Retrieves a list of retailers that the authenticated API key has access to.

**Currently Supported Retailers (73 total):**
This rollout is configured in `configs/retailers.yaml` and may vary by API key permissions.
Use this endpoint to fetch the current retailer list available to the authenticated key.

### status

API health and status checks.

- **`shopping-pp-cli status`** - Provides the current operational status of the API service, including database connectivity.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
shopping-pp-cli retailers

# JSON for scripting and agents
shopping-pp-cli retailers --json

# Filter to specific fields
shopping-pp-cli retailers --json --select id,name,status

# Dry run — show the request without sending
shopping-pp-cli retailers --dry-run

# Agent mode — JSON + compact + no prompts in one flag
shopping-pp-cli retailers --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
shopping-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/shopping-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SHOPPING_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `shopping-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `shopping-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SHOPPING_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 "API key required"** — Set SHOPPING_API_KEY (or run `shopping-pp-cli auth set-token <key>`); every endpoint needs the X-API-Key header. Get a key at lemmebuyit.com/developer.
- **403 / 'not in your plan' on full products, price-history, or arbitrage** — Those are paid-tier endpoints; a free key only reaches the `/shopping/` surface. Use a paid key or the `retailers shopping products` commands.
- **compare / deals / arbitrage return nothing** — Run `shopping-pp-cli index --retailer <id>` first; these commands read the local store, which is empty until you sync.
