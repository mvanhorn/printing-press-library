# Shopify CLI

Ecommerce orders, products, customers, inventory, fulfillment orders, and bulk operations via the Shopify Admin GraphQL API.

Learn more at [Shopify](https://shopify.dev/docs/api/admin-graphql).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `shopify-pp-cli` binary and the `pp-shopify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install shopify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install shopify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install shopify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install shopify --agent claude-code
npx -y @mvanhorn/printing-press-library install shopify --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopify/cmd/shopify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-shopify --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-shopify --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install shopify --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SHOPIFY_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopify/cmd/shopify-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "shopify": {
      "command": "shopify-pp-mcp",
      "env": {
        "SHOPIFY_SHOP": "<your-store>.myshopify.com",
        "SHOPIFY_API_VERSION": "2026-04",
        "SHOPIFY_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials
This CLI talks to the Shopify GraphQL API at `https://{shop}/admin/api/{api_version}/graphql.json`.

Set the endpoint variables for the tenant, workspace, or API version you want this CLI to use:

```bash
export SHOPIFY_SHOP="<your-store>.myshopify.com"
export SHOPIFY_API_VERSION="2026-04"
```

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export SHOPIFY_ACCESS_TOKEN="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/shopify-pp-cli/config.toml`.

### 3. Verify Setup

```bash
shopify-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
shopify-pp-cli abandoned-checkouts list
```

## Usage

Run `shopify-pp-cli --help` for the full command reference and flag list.

## Commands

### abandoned-checkouts

Shopify abandoned checkouts for recovery campaigns and lost-cart analysis.

- **`shopify-pp-cli abandoned-checkouts get`** - Get one Shopify abandoned checkout by GraphQL ID.
- **`shopify-pp-cli abandoned-checkouts list`** - List abandoned checkouts from the Shopify Admin GraphQL API.

### customers

Shopify customers with lifetime order count, lifetime spend, and contact fields.

- **`shopify-pp-cli customers get`** - Get one Shopify customer by GraphQL ID.
- **`shopify-pp-cli customers list`** - List customers from the Shopify Admin GraphQL API.

### fulfillment-orders

Shopify fulfillment orders for lag, routing, and fulfillment-state analysis.

- **`shopify-pp-cli fulfillment-orders get`** - Get one Shopify fulfillment order by GraphQL ID.
- **`shopify-pp-cli fulfillment-orders list`** - List fulfillment orders from the Shopify Admin GraphQL API.

### inventory-items

Shopify inventory items with tracked status and available quantities by location.

- **`shopify-pp-cli inventory-items get`** - Get one Shopify inventory item by GraphQL ID.
- **`shopify-pp-cli inventory-items list`** - List inventory items from the Shopify Admin GraphQL API.

### orders

Shopify orders with money totals, financial state, and fulfillment state.

- **`shopify-pp-cli orders get`** - Get one Shopify order by GraphQL ID.
- **`shopify-pp-cli orders list`** - List orders from the Shopify Admin GraphQL API.

### products

Shopify products with product status, catalog metadata, and a compact variant inventory projection.

- **`shopify-pp-cli products get`** - Get one Shopify product by GraphQL ID.
- **`shopify-pp-cli products list`** - List products from the Shopify Admin GraphQL API.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
shopify-pp-cli abandoned-checkouts list

# JSON for scripting and agents
shopify-pp-cli abandoned-checkouts list --json

# Filter to specific fields
shopify-pp-cli abandoned-checkouts list --json --select id,name,status

# Dry run — show the request without sending
shopify-pp-cli abandoned-checkouts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
shopify-pp-cli abandoned-checkouts list --agent
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

Set `SHOPIFY_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `shopify-pp-cli abandoned-checkouts`
- `shopify-pp-cli abandoned-checkouts get`
- `shopify-pp-cli abandoned-checkouts list`
- `shopify-pp-cli abandoned-checkouts search`
- `shopify-pp-cli customers`
- `shopify-pp-cli customers get`
- `shopify-pp-cli customers list`
- `shopify-pp-cli customers search`
- `shopify-pp-cli fulfillment-orders`
- `shopify-pp-cli fulfillment-orders get`
- `shopify-pp-cli fulfillment-orders list`
- `shopify-pp-cli fulfillment-orders search`
- `shopify-pp-cli inventory-items`
- `shopify-pp-cli inventory-items get`
- `shopify-pp-cli inventory-items list`
- `shopify-pp-cli inventory-items search`
- `shopify-pp-cli orders`
- `shopify-pp-cli orders get`
- `shopify-pp-cli orders list`
- `shopify-pp-cli orders search`
- `shopify-pp-cli products`
- `shopify-pp-cli products get`
- `shopify-pp-cli products list`
- `shopify-pp-cli products search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `SHOPIFY_SHOP` resolves `{shop}`
- `SHOPIFY_API_VERSION` resolves `{api_version}`

Base URL: `https://{shop}`

GraphQL path: `/admin/api/{api_version}/graphql.json`

## Health Check

```bash
shopify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/shopify-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SHOPIFY_SHOP` | endpoint | Yes |  |
| `SHOPIFY_API_VERSION` | endpoint | Yes |  |
| `SHOPIFY_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `shopify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SHOPIFY_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
