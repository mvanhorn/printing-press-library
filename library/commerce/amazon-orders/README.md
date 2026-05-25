# Amazon Orders CLI

**Walk your Amazon order history offline — every order, item, shipment, and dollar in a local SQLite store no other tool gives you.**

Sync once and ask cross-cutting questions forever. Where is my stuff right now, what did I spend last quarter, which deliveries are slipping, when did I order that thing — answered in milliseconds without re-hitting the live site or burning agent context on full HTML pages.

Learn more at [Amazon Orders](https://www.amazon.com).

## Install

The recommended path installs both the `amazon-orders-pp-cli` binary and the `pp-amazon-orders` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install amazon-orders
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install amazon-orders --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install amazon-orders --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install amazon-orders --agent claude-code
npx -y @mvanhorn/printing-press-library install amazon-orders --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/cmd/amazon-orders-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amazon-orders-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-amazon-orders --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-amazon-orders --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-amazon-orders skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-amazon-orders. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
amazon-orders-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amazon-orders-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/cmd/amazon-orders-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "amazon-orders": {
      "command": "amazon-orders-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Amazon publishes no buyer API. The CLI imports cookies from your logged-in Chrome / Firefox / Safari / Brave session via `auth login --chrome`. Those cookies persist locally, refresh automatically, and authenticate every subsequent fetch — no API key, no OAuth, no resident browser at runtime.

### Headless agents (1Password / Vault / Bitwarden)

For CI, dev containers, and remote hosts where `auth login --chrome` is not viable, capture the session once on a logged-in machine and inject it on every other host via your secrets manager. The cookie value never enters an LLM's context window because the bytes flow `op → stdin → CLI` without a shell variable in the middle.

```bash
# Stash once (logged-in machine):
amazon-orders-pp-cli auth export | op document create - --title amazon-orders-session --vault Agent

# Inject on any other machine:
op read "op://Agent/amazon-orders-session/file" | amazon-orders-pp-cli auth import --stdin
```

The exported JSON shape is `amazon-orders-session/v1`. `auth import` also accepts `--input <file>`, the `AMAZON_COOKIES` env var, or a raw `"k=v; k=v"` cookie string with `--raw-cookies`. See `SKILL.md` ("Headless agent setup with 1Password") for the refresh recipe and substitutes for `vault kv get`, `aws secretsmanager`, `pass`, and `bw`.

## Quick Start

```bash
# Import cookies from your logged-in browser session — required for any authenticated fetch.
amazon-orders-pp-cli auth login --chrome

# Walk the last 3 months of orders into the local store, including per-order item detail.
amazon-orders-pp-cli sync --since 90d --concurrency 1

# See every in-flight package with current status and ETA.
amazon-orders-pp-cli where-is-my-stuff --json

# Roll up your 2026 Amazon spending by month.
amazon-orders-pp-cli spend --by month --year 2026 --json

# Find every order containing 'usb-c cable' — FTS5-backed, instant, offline.
amazon-orders-pp-cli find 'usb-c cable' --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`where-is-my-stuff`** — All in-flight Amazon shipments with their current status and ETA in one view.

  _When an agent needs to answer 'is my package coming today' across many orders, this is the one-shot view._

  ```bash
  amazon-orders-pp-cli where-is-my-stuff --json --select orderId,status,etaDate,carrier
  ```
- **`delivery-slips`** — Orders whose actual delivery date slipped more than N days from the original estimate.

  _Surfaces unreliable carriers and sellers without manually scrolling through every order._

  ```bash
  amazon-orders-pp-cli delivery-slips --days 3 --since 2025-01-01 --json
  ```
- **`spend`** — Spending broken down by month, year, category, seller, or payment method.

  _Gives agents a one-shot answer for budgeting, expense reporting, and trend analysis._

  ```bash
  amazon-orders-pp-cli spend --by month --year 2025 --json
  ```
- **`top-items`** — Most-ordered items by frequency or by total spend, ASIN-grouped across all history.

  _Helps an agent reason about what the user actually consumes vs one-off purchases._

  ```bash
  amazon-orders-pp-cli top-items --by total-spend --limit 20 --json
  ```
- **`subscribe-and-save`** — Recurring purchases inferred from order history (same ASIN ordered on a regular cadence).

  _Surfaces candidates for actual S&S enrollment and detects de-facto subscriptions the user may not realize they have._

  ```bash
  amazon-orders-pp-cli subscribe-and-save --min-occurrences 3 --json
  ```
- **`arriving-soon`** — Shipments arriving in the next N days, sorted by ETA.

  _Lets an agent plan around incoming deliveries (e.g. 'is my router arriving before the meeting on Friday?')._

  ```bash
  amazon-orders-pp-cli arriving-soon --days 7 --json
  ```
- **`late`** — Active shipments past their original estimated delivery date.

  _Surfaces carrier delays the moment they happen, no manual review._

  ```bash
  amazon-orders-pp-cli late --json
  ```

### Agent-native plumbing
- **`find`** — FTS5 search across orders, items, sellers, and tracking notes.

  _Direct answer to 'when did I order that thing' without scrolling through years of order history._

  ```bash
  amazon-orders-pp-cli find 'usb-c cable' --json --select orderId,placedDate,total
  ```

## Usage

Run `amazon-orders-pp-cli --help` for the full command reference and flag list.

## Commands

### gift_cards

Gift card balance and activity history.

- **`amazon-orders-pp-cli gift_cards balance`** - Current gift card balance plus the activity log: amounts, kinds (added/applied/refund), dates, linked order IDs.

### orders

Your buyer-side order history listings and per-order detail pages.

- **`amazon-orders-pp-cli orders get`** - Full detail for a single order: items, ASINs, prices, shipments, payment method, ship-to address, totals.
- **`amazon-orders-pp-cli orders invoice`** - Printable invoice for an order (HTML), useful for VAT/expense reconciliation.
- **`amazon-orders-pp-cli orders list`** - Fetch one page of your order history. Use timeFilter (year-2026, last30days, months-3) and startIndex to paginate.

### shipments

Per-package tracking details.

- **`amazon-orders-pp-cli shipments track`** - Tracking detail for a single shipment: carrier, tracking number, status, ETA, delivery confirmation.

### transactions

Charges and refunds across all orders, recurring services, and Prime.

- **`amazon-orders-pp-cli transactions list`** - First page of your transactions list, grouped by date. Each row has payment method, last-4, signed amount, and (when applicable) the linked order ID.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
amazon-orders-pp-cli orders list

# JSON for scripting and agents
amazon-orders-pp-cli orders list --json

# Filter to specific fields
amazon-orders-pp-cli orders list --json --select id,name,status

# Dry run — show the request without sending
amazon-orders-pp-cli orders list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
amazon-orders-pp-cli orders list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `AMAZON_ORDERS_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `amazon-orders-pp-cli orders`
- `amazon-orders-pp-cli orders get`
- `amazon-orders-pp-cli orders invoice`
- `amazon-orders-pp-cli orders list`
- `amazon-orders-pp-cli transactions`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
amazon-orders-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/amazon-orders-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AMAZON_COOKIES` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `amazon-orders-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AMAZON_COOKIES`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`auth status` reports `unauthenticated` after `auth login --chrome`** — Make sure you're logged in to amazon.com in Chrome, then re-run `auth login --chrome --domain amazon.com`.
- **`sync` fails with `RateLimitError` after ~10 orders** — Pass `--rate 0.5` to slow the per-order detail fetch, or omit `--full-details` to fetch only the listing pages.
- **Order detail returns 401 even when logged in** — Amazon rotated your session-id; re-run `auth login --chrome` to refresh cookies.
- **`track <id>` returns empty when the order has multiple shipments** — Pass `--shipment-id <SID>` (visible in `orders get <id> --json`) to disambiguate.
- **Foreign-locale orders parse with garbled dates** — v1 supports US (.com) only. Multi-region shipping in v2; track issue.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.amazon.com/gp/your-account/order-history
- Capture coverage: 6 API entries from 6 total network entries
- Reachability: standard_http (95% confidence)
- Protocols: html_cookie (85% confidence)

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**amazon-orders**](https://github.com/alexdlaird/amazon-orders) — Python
- [**amazon-order-history-csv-download-mcp**](https://github.com/marcusquinn/amazon-order-history-csv-download-mcp) — TypeScript
- [**azad**](https://github.com/philipmulcahy/azad) — TypeScript
- [**Amazon-Order-History**](https://github.com/MaX-Lo/Amazon-Order-History) — Python
- [**amazon_order_history_scraper**](https://github.com/drewdaemon/amazon_order_history_scraper) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
