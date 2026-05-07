# Amazon Seller CLI

Read FBA inventory, orders, sales reports, listings, and catalog data for an Amazon seller account.

Learn more at [Amazon Seller](https://developer-docs.amazon.com/sp-api/).

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amazon-seller-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-seller/cmd/amazon-seller-pp-cli@latest
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Self-authorize your private application in the provider console, export the OAuth client ID, OAuth client secret, and refresh token, then run doctor:

```bash
export SP_API_LWA_CLIENT_ID="<client-id>"
export SP_API_LWA_CLIENT_SECRET="<client-secret>"
export SP_API_REFRESH_TOKEN="<refresh-token>"
amazon-seller-pp-cli doctor
```

The CLI exchanges the refresh token for an access token on the first live request and caches the access token locally.

### 3. Verify Setup

```bash
amazon-seller-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
amazon-seller-pp-cli sellers marketplaces
amazon-seller-pp-cli fba-inventory list --granularity-type Marketplace --granularity-id ATVPDKIKX0DER --marketplace-ids ATVPDKIKX0DER
```

## Usage

Run `amazon-seller-pp-cli --help` for the full command reference and flag list.

## Commands

### catalog

Read Catalog Items API item data.

- **`amazon-seller-pp-cli catalog get`** - Get one catalog item by ASIN.
- **`amazon-seller-pp-cli catalog search`** - Search catalog items. Provide marketplaceIds plus one valid search mode such as keywords or identifiers with identifiersType.

### fba-inventory

Inspect Fulfillment by Amazon inventory summaries.

- **`amazon-seller-pp-cli fba-inventory list`** - List FBA inventory summaries. For North America marketplace-level inventory, pass granularityType=Marketplace, granularityId=ATVPDKIKX0DER, and marketplaceIds=ATVPDKIKX0DER.

### listings

Read Listings Items API data for seller SKUs.

- **`amazon-seller-pp-cli listings get`** - Get one listing item by seller ID and SKU.
- **`amazon-seller-pp-cli listings search`** - Search listing items for a seller.

### orders

Search and inspect Orders API v2026-01-01 order records.

- **`amazon-seller-pp-cli orders get`** - Get one Orders API v2026-01-01 order.
- **`amazon-seller-pp-cli orders search`** - Search orders. Provide exactly one of createdAfter or lastUpdatedAfter; Amazon returns 400 for invalid combinations.

### reports

Create reports, poll report status, and inspect report document metadata.

- **`amazon-seller-pp-cli reports create`** - Create a report request. Prefer --stdin for JSON bodies so marketplaceIds remains a JSON array and reportOptions remains a JSON object.
- **`amazon-seller-pp-cli reports document`** - Get report document metadata and the presigned download URL. This command does not download or open the document.
- **`amazon-seller-pp-cli reports get`** - Get one report by report ID. This is the manual polling endpoint for report processing status.
- **`amazon-seller-pp-cli reports list`** - List reports. If nextToken is set, Amazon requires it to be the only query parameter; pass no other filters with nextToken.

### sellers

Verify seller authorization and list marketplace participations.

- **`amazon-seller-pp-cli sellers marketplaces`** - List marketplace participations for the authorized seller account.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
amazon-seller-pp-cli sellers marketplaces

# JSON for scripting and agents
amazon-seller-pp-cli orders search --created-after 2026-04-01T00:00:00Z --marketplace-ids ATVPDKIKX0DER --max-results-per-page 5 --json

# Filter to specific fields
amazon-seller-pp-cli catalog get <asin> --marketplace-ids ATVPDKIKX0DER --json --select asin,attributes,summaries

# Dry run — show the request without sending
amazon-seller-pp-cli reports list --report-types GET_FLAT_FILE_ALL_ORDERS_DATA_BY_ORDER_DATE_GENERAL --marketplace-ids ATVPDKIKX0DER --dry-run

# Agent mode — JSON + compact + no prompts in one flag
amazon-seller-pp-cli listings search <seller-id> --marketplace-ids ATVPDKIKX0DER --page-size 3 --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-amazon-seller -g
```

Then invoke `/pp-amazon-seller <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-seller/cmd/amazon-seller-pp-mcp@latest
```

Then register it:

```bash
# Self-authorize your private application, then provide the OAuth setup env vars:
claude mcp add amazon-seller amazon-seller-pp-mcp -e SP_API_LWA_CLIENT_ID=<client-id> -e SP_API_LWA_CLIENT_SECRET=<client-secret> -e SP_API_REFRESH_TOKEN=<refresh-token>
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-amazon-seller --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-amazon-seller --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-amazon-seller skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-amazon-seller. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle uses OAuth setup environment variables from your MCP host. Self-authorize your private application, provide the OAuth client ID, OAuth client secret, and refresh token, then run doctor.

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/amazon-seller-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SP_API_LWA_CLIENT_ID`, `SP_API_LWA_CLIENT_SECRET`, and `SP_API_REFRESH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/amazon-seller/cmd/amazon-seller-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "amazon-seller": {
      "command": "amazon-seller-pp-mcp",
      "env": {
        "SP_API_LWA_CLIENT_ID": "<client-id>",
        "SP_API_LWA_CLIENT_SECRET": "<client-secret>",
        "SP_API_REFRESH_TOKEN": "<refresh-token>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
amazon-seller-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/amazon-seller-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SP_API_LWA_CLIENT_ID` | auth_flow_input | Yes | Set during initial auth setup. |
| `SP_API_LWA_CLIENT_SECRET` | auth_flow_input | Yes | Set during initial auth setup. |
| `SP_API_REFRESH_TOKEN` | auth_flow_input | Yes | Set during initial auth setup. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `amazon-seller-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
