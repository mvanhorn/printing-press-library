# Odoo18cli CLI

Generic Odoo 18 CLI covering the main business models: sales, purchases,
products, partners, manufacturing, inventory, and accounting.

Connects to the Odoo XML-RPC external API at /xmlrpc/2/common (auth)
and /xmlrpc/2/object (ORM: search_read, read, create, write).

Supports multiple Odoo instances via profiles:
  odoo18cli --profile edarredo partners list --limit 10

Auth: set ODOO_URL, ODOO_DB, ODOO_USER, ODOO_API_KEY environment variables.
Generate an API key in Odoo via Settings → Users → your user → Account Security.

Printed by [@andreampiovesana](https://github.com/andreampiovesana) (Andrea M. Piovesana).

## Install

The recommended path installs both the `odoo18cli-pp-cli` binary and the `pp-odoo18cli` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install odoo18cli
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install odoo18cli --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/odoo18cli-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-odoo18cli --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-odoo18cli --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-odoo18cli skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-odoo18cli. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export ODOO_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/odoo-pp-cli/config.toml`.

### 3. Verify Setup

```bash
odoo18cli-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
odoo18cli-pp-cli xmlrpc authenticate
```

## Usage

Run `odoo18cli-pp-cli --help` for the full command reference and flag list.

## Commands

### xmlrpc

Manage xmlrpc

- **`odoo18cli-pp-cli xmlrpc authenticate`** - Calls common.authenticate(db, username, api_key, {}) via XML-RPC.
Returns integer UID used in all subsequent object calls.
- **`odoo18cli-pp-cli xmlrpc confirm-sales-order`** - Confirm a quotation into a sales order
- **`odoo18cli-pp-cli xmlrpc create-manufacturing-order`** - Create a new manufacturing order
- **`odoo18cli-pp-cli xmlrpc create-partner`** - Create a new partner
- **`odoo18cli-pp-cli xmlrpc create-product`** - Create a new product template
- **`odoo18cli-pp-cli xmlrpc create-purchase-order`** - Create a new purchase order (RFQ)
- **`odoo18cli-pp-cli xmlrpc create-sales-order`** - Create a new sales order
- **`odoo18cli-pp-cli xmlrpc get-bom`** - Get a single BOM by ID
- **`odoo18cli-pp-cli xmlrpc get-invoice`** - Get a single invoice or bill by ID
- **`odoo18cli-pp-cli xmlrpc get-manufacturing-order`** - Get a single manufacturing order by ID
- **`odoo18cli-pp-cli xmlrpc get-partner`** - Get a single partner by ID
- **`odoo18cli-pp-cli xmlrpc get-pricelist`** - Get a single pricelist by ID
- **`odoo18cli-pp-cli xmlrpc get-product`** - Get a single product template by ID
- **`odoo18cli-pp-cli xmlrpc get-purchase-order`** - Get a single purchase order by ID
- **`odoo18cli-pp-cli xmlrpc get-sales-order`** - Get a single sales order by ID
- **`odoo18cli-pp-cli xmlrpc get-transfer`** - Get a single transfer by ID
- **`odoo18cli-pp-cli xmlrpc list-accounts`** - List chart of accounts
- **`odoo18cli-pp-cli xmlrpc list-boms`** - List bills of materials
- **`odoo18cli-pp-cli xmlrpc list-inventory`** - List stock quantities (on-hand inventory)
- **`odoo18cli-pp-cli xmlrpc list-invoices`** - Calls execute_kw on account.move/search_read. Use domain to filter
by move_type (out_invoice, in_invoice, out_refund, in_refund).
- **`odoo18cli-pp-cli xmlrpc list-journal-entries`** - List journal entry lines
- **`odoo18cli-pp-cli xmlrpc list-manufacturing-orders`** - List manufacturing orders
- **`odoo18cli-pp-cli xmlrpc list-partners`** - List partners (customers and/or vendors)
- **`odoo18cli-pp-cli xmlrpc list-pricelists`** - List customer pricelists
- **`odoo18cli-pp-cli xmlrpc list-products`** - List product templates
- **`odoo18cli-pp-cli xmlrpc list-purchase-orders`** - List purchase orders
- **`odoo18cli-pp-cli xmlrpc list-sales-orders`** - Calls execute_kw on sale.order/search_read. Returns sales orders
matching the optional domain filter.
- **`odoo18cli-pp-cli xmlrpc list-stock-moves`** - List stock moves
- **`odoo18cli-pp-cli xmlrpc list-supplier-prices`** - List supplier price rules
- **`odoo18cli-pp-cli xmlrpc list-transfers`** - List stock transfers (pickings)
- **`odoo18cli-pp-cli xmlrpc list-workcenters`** - List work centers
- **`odoo18cli-pp-cli xmlrpc list-workorders`** - List work orders
- **`odoo18cli-pp-cli xmlrpc update-product`** - Update product template fields


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
odoo18cli-pp-cli xmlrpc authenticate

# JSON for scripting and agents
odoo18cli-pp-cli xmlrpc authenticate --json

# Filter to specific fields
odoo18cli-pp-cli xmlrpc authenticate --json --select id,name,status

# Dry run — show the request without sending
odoo18cli-pp-cli xmlrpc authenticate --dry-run

# Agent mode — JSON + compact + no prompts in one flag
odoo18cli-pp-cli xmlrpc authenticate --agent
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
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-odoo18cli -g
```

Then invoke `/pp-odoo18cli <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add odoo18cli odoo18cli-pp-mcp -e ODOO_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/odoo18cli-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ODOO_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "odoo18cli": {
      "command": "odoo18cli-pp-mcp",
      "env": {
        "ODOO_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
odoo18cli-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/odoo-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ODOO_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `odoo18cli-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ODOO_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
