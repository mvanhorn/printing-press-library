# Gumroad CLI

**Gumroad's seller API as an agent-ready CLI and MCP server.**

This tool wraps Gumroad's documented OAuth API for products, files, covers, variants, offer codes, custom fields, resource subscriptions, sales, subscribers, licenses, payouts, tax forms, and earnings. It also adds the Printing Press local sync/search/analytics layer so agents can answer seller questions from a consistent local snapshot instead of repeatedly walking paginated endpoints.

Printed by [@bheemreddy181](https://github.com/bheemreddy181) (Bheem Reddy).

## Install

The recommended path installs both the `gumroad-pp-cli` binary and the `pp-gumroad` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install gumroad
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install gumroad --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gumroad-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gumroad --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gumroad --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-gumroad skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-gumroad. The skill defines how its required CLI can be installed.
```

## Authentication

Create a Gumroad OAuth application, authorize it with the scopes needed for your workflow, and provide the resulting access token as GUMROAD_ACCESS_TOKEN. The MCP bundle exposes this as a sensitive user configuration value.

## Quick Start

```bash
# Confirm the token is present and the API is reachable.
gumroad-pp-cli doctor


# List products in agent-friendly JSON.
gumroad-pp-cli products list --agent


# Sync seller data for local search and analytics.
gumroad-pp-cli sync --resources products,sales,subscribers,payouts --latest-only --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local seller intelligence

- **`sync --resources products,sales,subscribers,payouts --latest-only --json`** — Refresh a local SQLite snapshot of the seller-facing Gumroad resources that agents inspect most often.

  _Use this before research or reporting tasks so later searches and analytics are grounded in one consistent seller snapshot._

  ```bash
  gumroad-pp-cli sync --resources products,sales,subscribers,payouts --latest-only --json
  ```
- **`search`** — Search locally synced products, sales, subscribers, payouts, and tax records through one command.

  _Use this when the user remembers a buyer, product, license, or payout clue but not the exact Gumroad object ID._

  ```bash
  gumroad-pp-cli search "annual plan" --data-source local --json --limit 20
  ```
- **`analytics`** — Group and count synced Gumroad records locally for fast summaries without additional API traffic.

  _Use this for lightweight revenue, subscriber, product, and payout triage after a sync._

  ```bash
  gumroad-pp-cli analytics --type sales --group-by product_id --limit 10 --json
  ```

### Operational monitoring

- **`tail`** — Poll selected Gumroad resources and emit NDJSON changes for scripts or agent monitors.

  _Use this for one-off checks after a launch, refund, payout, or product update._

  ```bash
  gumroad-pp-cli tail --resource sales --interval 30s --json
  ```

## Usage

Run `gumroad-pp-cli --help` for the full command reference and flag list.

## Commands

### earnings

Manage earnings

- **`gumroad-pp-cli earnings get`** - Retrieve an annual earnings breakdown for the authenticated user. Requires view_tax_data scope.

### files

Manage files

- **`gumroad-pp-cli files abort`** - Cancel a multipart upload started by /files/presign. Requires edit_products scope.
- **`gumroad-pp-cli files complete`** - Finalize a multipart upload started by /files/presign. Requires edit_products scope.
- **`gumroad-pp-cli files presign`** - Start a multipart upload and return presigned URLs for each part. Requires edit_products scope.

### licenses

Manage licenses

- **`gumroad-pp-cli licenses decrement-uses-count`** - Decrement the uses count of a license. Requires edit_products scope.
- **`gumroad-pp-cli licenses disable`** - Disable a license. Requires edit_products scope.
- **`gumroad-pp-cli licenses enable`** - Enable a license. Requires edit_products scope.
- **`gumroad-pp-cli licenses rotate`** - Rotate a license key. The old key will no longer be valid. Requires edit_products scope.
- **`gumroad-pp-cli licenses verify`** - Verify a license key.

### payouts

Manage payouts

- **`gumroad-pp-cli payouts get`** - Retrieve details of a payout. Requires view_payouts scope.
- **`gumroad-pp-cli payouts get-upcoming`** - Retrieve upcoming payouts. Requires view_payouts scope.
- **`gumroad-pp-cli payouts list`** - Retrieve payouts for the authenticated user. Requires view_payouts scope.

### products

Manage products

- **`gumroad-pp-cli products create`** - Create a new product as a draft. Requires edit_products or account scope.
- **`gumroad-pp-cli products delete`** - Permanently delete a product.
- **`gumroad-pp-cli products get`** - Retrieve details of a product.
- **`gumroad-pp-cli products list`** - Retrieve all existing products for the authenticated user.
- **`gumroad-pp-cli products update`** - Update an existing product. Collection fields such as files, tags, and rich_content replace the full collection.

### resource-subscriptions

Manage resource subscriptions

- **`gumroad-pp-cli resource-subscriptions create`** - Subscribe to a resource webhook. Requires view_sales scope.
- **`gumroad-pp-cli resource-subscriptions delete`** - Unsubscribe from a resource.
- **`gumroad-pp-cli resource-subscriptions list`** - Show active subscriptions for the input resource. Requires view_sales scope.

### sales

Manage sales

- **`gumroad-pp-cli sales get`** - Retrieve details of a sale. Requires view_sales scope.
- **`gumroad-pp-cli sales list`** - Retrieve successful sales by the authenticated user. Requires view_sales scope.

### subscribers

Manage subscribers

- **`gumroad-pp-cli subscribers get`** - Retrieve details of a subscriber. Requires view_sales scope.

### tax-forms

Manage tax forms

- **`gumroad-pp-cli tax-forms list`** - Retrieve 1099 tax forms for the authenticated user. Requires view_tax_data scope.

### user

Manage user

- **`gumroad-pp-cli user get`** - Retrieve the authenticated user's data.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gumroad-pp-cli earnings --year 42

# JSON for scripting and agents
gumroad-pp-cli earnings --year 42 --json

# Filter to specific fields
gumroad-pp-cli earnings --year 42 --json --select id,name,status

# Dry run — show the request without sending
gumroad-pp-cli earnings --year 42 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gumroad-pp-cli earnings --year 42 --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-gumroad -g
```

Then invoke `/pp-gumroad <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add gumroad gumroad-pp-mcp -e GUMROAD_ACCESS_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gumroad-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GUMROAD_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gumroad": {
      "command": "gumroad-pp-mcp",
      "env": {
        "GUMROAD_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
gumroad-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gumroad-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GUMROAD_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gumroad-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GUMROAD_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Gumroad returns a scope error.** — Re-authorize the OAuth application with the endpoint scope shown in the command help, such as view_sales, edit_sales, edit_products, view_payouts, or view_tax_data.
- **File upload completion fails after a retry.** — Start a new /files/presign flow; Gumroad upload IDs are one-use after /files/complete.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Gumroad API documentation**](https://gumroad.com/api) — documentation
- [**Gumroad open-source app**](https://github.com/antiwork/gumroad) — Ruby/TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
