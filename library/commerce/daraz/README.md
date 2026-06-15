# Daraz.pk CLI

**Every Daraz product search, plus a local price database, real-deal ranking, and drop alerts no Daraz scraper has.**

daraz-pp-cli searches Daraz.pk listings, reads reviews, and extracts product detail like any scraper — then keeps it in a local SQLite store so it can do what one-shot scrapers cannot: track price history (price-history), rank genuine deals over fake discounts (deals, value), diff a saved search over time (since), and vet sellers (seller stats). Read-only, no login required.

## Install

The recommended path installs both the `daraz-pp-cli` binary and the `pp-daraz` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install daraz
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install daraz --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install daraz --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install daraz --agent claude-code
npx -y @mvanhorn/printing-press-library install daraz --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/daraz/cmd/daraz-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/daraz-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install daraz --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-daraz --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-daraz --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install daraz --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/daraz-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/daraz/cmd/daraz-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "daraz": {
      "command": "daraz-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check — confirms daraz.pk is reachable; needs no credentials.
daraz-pp-cli doctor --dry-run

# Core search with a PKR price range, cheapest first.
daraz-pp-cli products --query "gaming laptop" --price 50000-150000 --sort priceasc

# Read recent reviews for a product by its item ID.
daraz-pp-cli reviews --item-id 599201597 --page-size 5

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local price intelligence
- **`price-history`** — See a product's recorded price trend and its lowest-ever price from your local store.

  _Reach for this to judge whether a current price is actually low for that item, not just discounted on paper._

  ```bash
  daraz-pp-cli price-history 599201597 --agent
  ```
- **`watch`** — Record current prices for every item matching a search into the local store so price history builds over time.

  _Run periodically on a query you care about; then price-history shows the trend for any item it captured._

  ```bash
  daraz-pp-cli watch "gaming laptop"
  ```
- **`since`** — Diff a saved search against its last local snapshot to show new listings and price moves since you last checked.

  _Use to monitor a market over time instead of re-reading a full result list each run._

  ```bash
  daraz-pp-cli since "gaming mouse" --agent
  ```

### Smart ranking
- **`deals`** — Rank a search by a composite of discount, rating, and units sold to surface genuinely good deals.

  _Use when the goal is the best-value item for a query, not raw listing order._

  ```bash
  daraz-pp-cli deals "laptop" --agent --select name,price,discountPct,rating
  ```
- **`value`** — Flag inflated original-price discounts by comparing each item's claimed original price to the local median for that query.

  _Use to avoid misleading discounts; it is the inverse of deals._

  ```bash
  daraz-pp-cli value "power bank" --agent
  ```
- **`compare`** — Find the same item across sellers and show the cheapest and best-rated side by side.

  _Use to pick the best seller/listing for an item before buying._

  ```bash
  daraz-pp-cli compare "airpods pro" --agent
  ```

### Seller trust
- **`seller stats`** — Aggregate a seller's catalog from the local store: average rating, price range, listing count, discount pattern.

  _Use to vet a seller's track record before trusting a listing._

  ```bash
  daraz-pp-cli seller stats 1066739 --agent
  ```

## Recipes


### Agent-friendly search with field selection

```bash
daraz-pp-cli deals "wireless earbuds" --agent --select name,price,discountPct,rating,url
```

deals returns a flat, ranked array of listings, so --select trims each row to just the fields an agent needs and keeps payloads small.

### Find the best-value items, not the biggest fake discount

```bash
daraz-pp-cli deals "power bank" --agent --select name,price,discountPct,rating
```

Ranks by a discount-times-rating-times-sales composite computed locally.

### Start tracking a product's price

```bash
daraz-pp-cli watch 599201597
```

Records the current price so price-history builds a real trend over time.

### See what changed in a market since last check

```bash
daraz-pp-cli since "gaming mouse" --agent
```

Diffs the current search against the last local snapshot for new listings and price moves.

## Usage

Run `daraz-pp-cli --help` for the full command reference and flag list.

## Commands

### products

Search Daraz product listings

- **`daraz-pp-cli products`** - Search products by keyword with sort and price filtering

### reviews

Read product reviews and rating distribution

- **`daraz-pp-cli reviews`** - List reviews for a product by its numeric item ID


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
daraz-pp-cli products --query example-value

# JSON for scripting and agents
daraz-pp-cli products --query example-value --json

# Filter to specific fields
daraz-pp-cli products --query example-value --json --select id,name,status

# Dry run — show the request without sending
daraz-pp-cli products --query example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
daraz-pp-cli products --query example-value --agent
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
daraz-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/daraz-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Empty results for a query that has products on the site** — Drop filters and retry: daraz-pp-cli products --query "<keyword>" — then re-add --price.
- **price-history or since shows nothing** — Populate the local store first: daraz-pp-cli watch <itemId> (for price-history) or run the same search once before since.
- **reviews returns nothing** — Confirm the numeric item ID from a product URL (the digits after -i in .../products/...-i<itemId>.html).

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**oxylabs/lazada-scraper**](https://github.com/oxylabs/lazada-scraper) — Python
- [**zohaibbashir/Daraz.pk-Web-Scraper**](https://github.com/zohaibbashir/Daraz.pk-Web-Scraper) — Python
- [**kazalnsl/daraz-scraper**](https://github.com/kazalnsl/daraz-scraper) — Python
- [**Yeasir-Hossain/daraz-scrapper**](https://github.com/Yeasir-Hossain/daraz-scrapper) — JavaScript
- [**abdulalikhan/DarazPK-API**](https://github.com/abdulalikhan/DarazPK-API) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
