# Lemon Squeezy CLI

**Every Lemon Squeezy resource, plus a local SQLite mirror that surfaces MRR, churn, license seats, and discount-campaign pace no other LS tool ships.**

Lemon Squeezy's official SDKs are libraries, not CLIs, and the dashboard is a click-walk per store. This CLI mirrors all 19 LS resources to local SQLite with offline FTS5 and cross-entity SQL, then layers transcendence commands — `revenue-snapshot`, `mrr-trend`, `churn-watch`, `dunning-alert`, `license-rollup`, `refund-cascade`, `campaign-watch`, `webhook-audit` — that combine entities no single endpoint returns. Built for indie SaaS founders and license-key sellers who live in the LS state machine.

Created by [@jcastillo725](https://github.com/jcastillo725) (Joseph Alvin Castillo).

## Install

The recommended path installs both the `lemonsqueezy-pp-cli` binary and the `pp-lemonsqueezy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install lemonsqueezy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install lemonsqueezy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install lemonsqueezy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install lemonsqueezy --agent claude-code
npx -y @mvanhorn/printing-press-library install lemonsqueezy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/cmd/lemonsqueezy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lemonsqueezy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install lemonsqueezy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-lemonsqueezy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-lemonsqueezy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install lemonsqueezy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/lemonsqueezy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `LEMONSQUEEZY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/cmd/lemonsqueezy-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "lemonsqueezy": {
      "command": "lemonsqueezy-pp-mcp",
      "env": {
        "LEMONSQUEEZY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Lemon Squeezy uses HTTP Bearer auth. Create an API key at https://app.lemonsqueezy.com/settings/api, then export `LEMONSQUEEZY_API_KEY=<your-key>`. Verify with `lemonsqueezy-pp-cli doctor`.


## Catalog setup and checkout limitations

Lemon Squeezy's public API exposes catalog resources as read-only: `products`, `variants`, and `files` support list/get, not create/update/delete. The CLI must not use private dashboard APIs and must not fake product-create dry-runs. For catalog setup, generate a dashboard handoff packet:

```bash
lemonsqueezy-pp-cli capabilities --resource products --json
lemonsqueezy-pp-cli dashboard handoff product \
  --name "Juno Home Chief of Staff Starter Kit" \
  --slug juno-home-chief-of-staff-starter-kit \
  --sku juno-home-chief-of-staff-starter-kit \
  --price-usd 149 \
  --type "digital download" \
  --redirect-url https://www.littlemight.com/ai-house-manager/thank-you/ \
  --affiliate-percent 25 \
  --affiliate-approval "manual approval" \
  --affiliate-cookie-days 30 \
  --json
```

Once the product/variant/file exist in the dashboard, checkout creation is API-supported via `POST /v1/checkouts`:

```bash
lemonsqueezy-pp-cli checkouts create \
  --store-id <STORE_ID> \
  --variant-id <VARIANT_ID> \
  --redirect-url https://www.littlemight.com/ai-house-manager/thank-you/ \
  --dry-run --json
```

Live checkout creation validates the store and variant first unless `--skip-validate` is set, then prints the checkout ID/URL returned by Lemon Squeezy.

## Quick Start

```bash
# Verify the binary loads and shows expected env vars before hitting the API.
lemonsqueezy-pp-cli doctor --dry-run

# Pull the core surface into local SQLite so the transcendence commands have data to work with.
lemonsqueezy-pp-cli sync --resources stores,products,variants,subscriptions,orders,customers

# Get a one-number rollup of 30-day + lifetime revenue, refund-adjusted from local orders.
lemonsqueezy-pp-cli revenue-snapshot --json

# List subscriptions that flipped into past_due/unpaid/cancelled in the last 7 days with dollar exposure.
lemonsqueezy-pp-cli churn-watch --since 7d --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Revenue + churn signal
- **`revenue-snapshot`** — Point-in-time revenue rollup combining LS's denormalized 30-day/lifetime store counters with refund-adjusted net from local orders.

  _Reach for this when you want one number for 'how is the store doing right now' without walking every order through the API._

  ```bash
  lemonsqueezy-pp-cli revenue-snapshot --agent
  ```
- **`mrr-trend`** — Weekly MRR over a sliding window, classified as new / renewal / refunded with a week-over-week net delta.

  _Pick this over a raw `list-subscriptions` call when you need to show how MRR moved week-over-week, not just where it stands today._

  ```bash
  lemonsqueezy-pp-cli mrr-trend --weeks 12 --json
  ```
- **`churn-watch`** — Lists subscriptions that flipped to past_due/unpaid/cancelled/expired in a window, with customer email and dollar exposure per row.

  _Reach for this on Monday-morning sweeps to see what changed since Friday — beats clicking through the dashboard's status filters._

  ```bash
  lemonsqueezy-pp-cli churn-watch --since 7d --json
  ```
- **`dunning-alert`** — Lists subscription-invoices with status=failed whose parent subscription is still active or past_due — the recoverable window.

  _Use this to find subs whose latest renewal failed but who haven't churned yet — the window where a Slack ping or grace email recovers revenue._

  ```bash
  lemonsqueezy-pp-cli dunning-alert --json
  ```

### License + refund ops
- **`license-rollup`** — Per-variant and per-key activation statistics joined across license-keys, license-key-instances, and variants.

  _Reach for this when you need to surface piracy-shaped seat distributions or just answer 'how many keys are active per variant'._

  ```bash
  lemonsqueezy-pp-cli license-rollup --json --select keys
  ```
- **`refund-cascade`** — Given a refunded order ID, walks order → order-items → license-keys → instances, then optionally disables the keys via --apply.

  _Use this after a refund hits, to make sure the buyer's license keys actually get disabled instead of staying active._

  ```bash
  lemonsqueezy-pp-cli refund-cascade order_3aBc --dry-run --json
  ```

### Campaign + integration ops
- **`campaign-watch`** — Per discount code: redemptions used vs cap, redemption velocity over last 24h, projected sellout time at current pace (needs >=6h of redemption activity to stabilise).

  _Run this during a capped sale (Founding-Member tier, Early Access) to know when a tier will sell out so you can expire the code before overselling._

  ```bash
  lemonsqueezy-pp-cli campaign-watch FOUNDING-LIFETIME FOUNDING-2YR FOUNDING-1YR --json
  ```
- **`webhook-audit`** — Cross-store webhook coverage matrix grouped by URL host, flagging stale destinations (localhost, ngrok, *.test, *.local).

  _Reach for this to catch orphaned webhooks pointing at last week's ngrok URL before they cause a missed event in production._

  ```bash
  lemonsqueezy-pp-cli webhook-audit --json
  ```

## Recipes

### Monday-morning revenue + churn sweep

```bash
lemonsqueezy-pp-cli sync --resources stores,subscriptions,subscription-invoices,orders --since 14d && lemonsqueezy-pp-cli revenue-snapshot --json && lemonsqueezy-pp-cli churn-watch --since 7d --json
```

Refreshes the local mirror, then prints the revenue rollup and the past-week churn delta — the ritual that replaces the dashboard click-walk.

### Find recoverable failed renewals

```bash
lemonsqueezy-pp-cli dunning-alert --json --select rows.customer_email,rows.amount_usd
```

Lists every failed invoice whose subscription is still active or past_due — the dollar-recoverable window before the customer churns.

### Refund cascade for an order

```bash
lemonsqueezy-pp-cli refund-cascade order_3aBc --apply --json
```

Walks the order → order-items → license-keys chain and disables every key tied to the refund. Drop `--apply` for a dry preview.

### Track a capped Founding-Member sale

```bash
lemonsqueezy-pp-cli sync --resources discounts,discount-redemptions && lemonsqueezy-pp-cli campaign-watch FOUNDING-LIFETIME FOUNDING-2YR FOUNDING-1YR --json
```

Live capacity + redemption velocity per tier with sellout projection — run hourly during a launch to expire codes before overselling.

### Webhook coverage audit

```bash
lemonsqueezy-pp-cli sync --resources webhooks && lemonsqueezy-pp-cli webhook-audit --json --select hosts.url,hosts.stale,hosts.event_count
```

Groups every webhook across every store by URL host, flags localhost/ngrok/*.test destinations so orphans don't break production.

## Usage

Run `lemonsqueezy-pp-cli --help` for the full command reference and flag list.

## Commands

### agent-native catalog workflows

- **`lemonsqueezy-pp-cli capabilities`** - Show public API list/get/create/update/delete support by resource
- **`lemonsqueezy-pp-cli dashboard`** - Dashboard-only Lemon Squeezy workflows and handoff packets
- **`lemonsqueezy-pp-cli dashboard handoff`** - Generate dashboard handoff artifacts for unsupported public API writes
- **`lemonsqueezy-pp-cli dashboard handoff product`** - Generate exact dashboard fields for a product/variant/file setup handoff
- **`lemonsqueezy-pp-cli import <resource>`** - Import JSONL records only for API-writable resources; refuses products/variants/files as read-only
- **`lemonsqueezy-pp-cli which [query]`** - Resolve natural-language capability requests, including negative catalog guidance

### affiliates

Manage affiliates

- **`lemonsqueezy-pp-cli affiliates get`** - Lemon Squeezy Retrieve an affiliate
- **`lemonsqueezy-pp-cli affiliates list`** - Lemon Squeezy List all affiliates

### checkouts

Manage checkouts

- **`lemonsqueezy-pp-cli checkouts create`** - Create a checkout URL for an existing store/variant; supports safe JSON:API dry-run and live preflight validation
- **`lemonsqueezy-pp-cli checkouts get`** - Lemon Squeezy Retrieve a checkout
- **`lemonsqueezy-pp-cli checkouts list`** - Lemon Squeezy List all checkouts

### customers

Manage customers

- **`lemonsqueezy-pp-cli customers get`** - Lemon Squeezy Retrieve a customer
- **`lemonsqueezy-pp-cli customers list`** - Lemon Squeezy List all customers

### discount-redemptions

Manage discount redemptions

- **`lemonsqueezy-pp-cli discount-redemptions get`** - Lemon Squeezy Retrieve a discount redemption
- **`lemonsqueezy-pp-cli discount-redemptions list`** - Lemon Squeezy List all discount redemptions

### discounts

Manage discounts

- **`lemonsqueezy-pp-cli discounts create`** - Lemon Squeezy Create a discount
- **`lemonsqueezy-pp-cli discounts delete`** - Lemon Squeezy Delete a discount
- **`lemonsqueezy-pp-cli discounts get`** - Lemon Squeezy Retrieve a discount
- **`lemonsqueezy-pp-cli discounts list`** - Lemon Squeezy List all discounts

### files

Manage files

- **`lemonsqueezy-pp-cli files get`** - Lemon Squeezy Retrieve a file
- **`lemonsqueezy-pp-cli files list`** - Lemon Squeezy List all files

### health

Manage health

- **`lemonsqueezy-pp-cli health`** - Lemon Squeezy Health

### license-key-instances

Manage license key instances

- **`lemonsqueezy-pp-cli license-key-instances get`** - Lemon Squeezy Retrieve a license key instance
- **`lemonsqueezy-pp-cli license-key-instances list`** - Lemon Squeezy List all license key instances

### license-keys

Manage license keys

- **`lemonsqueezy-pp-cli license-keys get`** - Lemon Squeezy Retrieve a license key
- **`lemonsqueezy-pp-cli license-keys list`** - Lemon Squeezy List all license keys

### order-items

Manage order items

- **`lemonsqueezy-pp-cli order-items get`** - Lemon Squeezy Retrieve an order item
- **`lemonsqueezy-pp-cli order-items list`** - Lemon Squeezy List all order items

### orders

Manage orders

- **`lemonsqueezy-pp-cli orders get`** - Lemon Squeezy Retrieve an order
- **`lemonsqueezy-pp-cli orders list`** - Lemon Squeezy List all orders

### prices

Manage prices

- **`lemonsqueezy-pp-cli prices get`** - Lemon Squeezy Retrieve a price
- **`lemonsqueezy-pp-cli prices list`** - Lemon Squeezy List all prices

### products

Manage products

- **`lemonsqueezy-pp-cli products get`** - Lemon Squeezy Retrieve a product
- **`lemonsqueezy-pp-cli products list`** - Lemon Squeezy List all products

### stores

Manage stores

- **`lemonsqueezy-pp-cli stores get`** - Lemon Squeezy Retrieve a store
- **`lemonsqueezy-pp-cli stores list`** - Lemon Squeezy List all stores

### subscription-invoices

Manage subscription invoices

- **`lemonsqueezy-pp-cli subscription-invoices get`** - Lemon Squeezy Retrieve a subscription invoice
- **`lemonsqueezy-pp-cli subscription-invoices list`** - Lemon Squeezy List all subscription invoices

### subscription-items

Manage subscription items

- **`lemonsqueezy-pp-cli subscription-items get`** - Lemon Squeezy Retrieve a subscription item
- **`lemonsqueezy-pp-cli subscription-items list`** - Lemon Squeezy List all subscription items
- **`lemonsqueezy-pp-cli subscription-items update`** - Lemon Squeezy Update a subscription item

### subscriptions

Manage subscriptions

- **`lemonsqueezy-pp-cli subscriptions delete`** - Lemon Squeezy Cancel a Subscription
- **`lemonsqueezy-pp-cli subscriptions get`** - Lemon Squeezy Retrieve a subscription
- **`lemonsqueezy-pp-cli subscriptions list`** - Lemon Squeezy List all subscriptions
- **`lemonsqueezy-pp-cli subscriptions update`** - Lemon Squeezy Update a subscription

### usage-records

Manage usage records

- **`lemonsqueezy-pp-cli usage-records create`** - Lemon Squeezy Create a usage record
- **`lemonsqueezy-pp-cli usage-records get`** - Lemon Squeezy Retrieve a usage-record
- **`lemonsqueezy-pp-cli usage-records list`** - Lemon Squeezy List all usage records

### users

Manage users

- **`lemonsqueezy-pp-cli users`** - Lemon Squeezy Retrieve the authenticated user

### variants

Manage variants

- **`lemonsqueezy-pp-cli variants get`** - Lemon Squeezy Retrieve a variant
- **`lemonsqueezy-pp-cli variants list`** - Lemon Squeezy List all variants

### webhooks

Manage webhooks

- **`lemonsqueezy-pp-cli webhooks create`** - Lemon Squeezy Create a webhook
- **`lemonsqueezy-pp-cli webhooks delete`** - Lemon Squeezy Delete a webhook
- **`lemonsqueezy-pp-cli webhooks get`** - Lemon Squeezy Retrieve a webhook
- **`lemonsqueezy-pp-cli webhooks list`** - Lemon Squeezy List all webhooks
- **`lemonsqueezy-pp-cli webhooks update`** - Lemon Squeezy Update a webhook

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
lemonsqueezy-pp-cli affiliates list

# JSON for scripting and agents
lemonsqueezy-pp-cli affiliates list --json

# Filter to specific fields
lemonsqueezy-pp-cli affiliates list --json --select id,name,status

# Dry run — show the request without sending
lemonsqueezy-pp-cli affiliates list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
lemonsqueezy-pp-cli affiliates list --agent
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

## Health Check

```bash
lemonsqueezy-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/lemonsqueezy-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `LEMONSQUEEZY_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `lemonsqueezy-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `lemonsqueezy-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $LEMONSQUEEZY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports `LEMONSQUEEZY_API_KEY not set`** — Export `LEMONSQUEEZY_API_KEY=<your-key>` in your shell (or persist in ~/.zshenv for non-interactive subprocesses).
- **401 Unauthorized on any command** — The API key is invalid or revoked. Generate a fresh one at app.lemonsqueezy.com/settings/api.
- **429 Too Many Requests** — Lemon Squeezy rate-limits at 300 req/min per IP. Rerun after the `Retry-After` window or narrow `sync --resources` to a smaller set.
- **churn-watch/mrr-trend return empty rows** — Run `sync --resources subscriptions,subscription-invoices --full` first — these commands read the local mirror, not the live API.
- **campaign-watch projects nonsense sellout times** — The projection needs at least 6 hours of redemption history. Resync `discount-redemptions` and rerun once redemptions exist in the window.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**lemonsqueezy.js**](https://github.com/lmsqueezy/lemonsqueezy.js) — TypeScript
- [**lemonsqueezy-go**](https://github.com/NdoleStudio/lemonsqueezy-go) — Go
- [**lemonsqueezy-mcp (YawLabs)**](https://github.com/YawLabs/lemonsqueezy-mcp) — TypeScript
- [**mcp-lemonsqueezy**](https://github.com/atharvagupta2003/mcp-lemonsqueezy) — Python
- [**lemonsqueezy-mcp-server (Intrepid)**](https://github.com/IntrepidServicesLLC/lemonsqueezy-mcp-server) — Python
- [**lemonsqueezy-claude-skills**](https://github.com/adrianwedd/lemonsqueezy-claude-skills) — Markdown
- [**lemonsqueezy-admin**](https://github.com/abakermi/lemonsqueezy-admin) — Markdown
- [**lemonsqueezy-py-api**](https://github.com/wdonofrio/lemonsqueezy-py-api) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
