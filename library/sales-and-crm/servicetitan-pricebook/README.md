# ServiceTitan Pricebook CLI

**A focused per-module ServiceTitan CLI for the Pricebook — sync once, then audit markup, costs, vendor parts, and warranties the ST UI cannot.**

This CLI (servicetitan-pricebook-pp-cli) mirrors every ServiceTitan Pricebook v2 endpoint (categories, client-specific pricing, discounts & fees, equipment, materials, materials markup, bulk operations, images, services, export feeds) and adds twelve novel commands that join across them in a local SQLite store. It snapshots cost and price history on every sync, so markup-audit, cost-drift, and reprice become one-shot. It is part of a per-module ST CLI family (servicetitan-crm, servicetitan-dispatch, servicetitan-inventory, servicetitan-jpm, servicetitan-pricebook) designed to replace the heavy 600+-tool general ServiceTitan MCP with focused, agent-native binaries.

## Install

The recommended path installs both the `servicetitan-pricebook-pp-cli` binary and the `pp-servicetitan-pricebook` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install servicetitan-pricebook
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install servicetitan-pricebook --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/servicetitan-pricebook-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-servicetitan-pricebook --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-servicetitan-pricebook --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-servicetitan-pricebook skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-servicetitan-pricebook. The skill defines how its required CLI can be installed.
```

## Authentication

ServiceTitan uses composed auth: a static ST-App-Key header (set ST_APP_KEY) plus an OAuth2 client-credentials bearer token (set ST_CLIENT_ID and ST_CLIENT_SECRET). Run servicetitan-pricebook-pp-cli auth login to walk the credential setup, then set the three credentials plus ST_TENANT_ID (your numeric tenant ID) in your environment. The client mints and refreshes the bearer token automatically and attaches both headers to every call. Whitespace is stripped defensively from credentials — a known JKA gotcha where a trailing newline in an env file produced opaque invalid_client 400s.

## Quick Start

```bash
# Confirm ST_APP_KEY, bearer token, ST_TENANT_ID, and base URL before anything else.
servicetitan-pricebook-pp-cli doctor


# Pull materials, equipment, services, categories, markup tiers, and rate sheets into the local store and snapshot cost/price history.
servicetitan-pricebook-pp-cli sync


# See markup-drift, vendor-part-gap, warranty, cost-drift, and orphan counts in one rollup.
servicetitan-pricebook-pp-cli health --agent


# Drill into the SKUs whose markup has drifted off the tier ladder.
servicetitan-pricebook-pp-cli markup-audit --tolerance 5


# Diff a vendor quote against current pricebook costs before writing anything back.
servicetitan-pricebook-pp-cli quote-reconcile ./2m-quote.csv

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Margin discipline
- **`markup-audit`** — Find every material and equipment SKU whose actual markup has drifted off the materials-markup tier ladder.

  _Reach for this after any vendor cost change to catch SKUs that are now mispriced for their tier before margin erodes._

  ```bash
  servicetitan-pricebook-pp-cli markup-audit --tolerance 5 --agent
  ```
- **`cost-drift`** — Show every SKU whose cost moved since a given date, with old and new cost, old and new price, and whether the price followed.

  _Reach for this to see whether recent vendor cost increases were actually passed through to price._

  ```bash
  servicetitan-pricebook-pp-cli cost-drift --since 2026-04-01 --agent
  ```
- **`reprice`** — Compute the tier-correct price for markup-drifted SKUs and emit the exact update payloads; preview by default, --apply to write.

  _Reach for this after markup-audit to apply hold-markup pricing in one pass instead of editing each SKU by hand._

  ```bash
  servicetitan-pricebook-pp-cli reprice --tolerance 5 --agent
  ```

### Pricebook hygiene
- **`vendor-part-gaps`** — List materials and equipment whose primary vendor part number is empty — the missing 2M Part # sweep.

  _Reach for this to enforce the 2M Part # discipline before a vendor-quote reconcile, which matches on vendor part._

  ```bash
  servicetitan-pricebook-pp-cli vendor-part-gaps --kind material --agent
  ```
- **`warranty-lint`** — Flag equipment whose warranty text is not prefixed Manufacturer's, or that is missing JKA's 1-year parts & labor line.

  _Reach for this to keep warranty attribution honest — manufacturer warranties clearly labeled, JKA's own offering listed alongside._

  ```bash
  servicetitan-pricebook-pp-cli warranty-lint --agent
  ```
- **`orphan-skus`** — List materials, equipment, and services assigned to inactive or non-existent categories.

  _Reach for this during category taxonomy cleanup to find SKUs that fell out of the visible tree._

  ```bash
  servicetitan-pricebook-pp-cli orphan-skus --agent
  ```
- **`dedupe`** — Cluster near-duplicate materials and equipment so excess pricebook growth can be collapsed.

  _Reach for this to find redundant parts before they multiply — duplicate SKUs scatter estimates and inventory across entries that should be one._

  ```bash
  servicetitan-pricebook-pp-cli dedupe --kind material --min-score 0.8 --agent
  ```
- **`copy-audit`** — Flag SKUs whose display name or description is empty, too short, ALL-CAPS, a bare part number, or otherwise not customer-facing.

  _Reach for this to find pricebook entries that read like internal shorthand instead of customer-facing sales copy._

  ```bash
  servicetitan-pricebook-pp-cli copy-audit --kind equipment --agent
  ```

### Vendor quote workflow
- **`quote-reconcile`** — Match a vendor cost file against synced SKUs by vendor part number and print a no-write diff of proposed cost changes; accepts CSV or Claude-extracted JSON from a quote, order confirmation, or invoice PDF.

  _Reach for this when a vendor quote, order confirmation, or invoice lands; have Claude extract it to JSON, then reconcile against current costs before touching the pricebook._

  ```bash
  servicetitan-pricebook-pp-cli quote-reconcile ./2m-quote-2026-05.csv --agent
  ```
- **`bulk-plan`** — Turn a reviewed cost or copy change file into a single pricebook bulk-update payload instead of N individual update calls.

  _Reach for this to apply a batch of reconciled changes in one rate-limit-friendly call._

  ```bash
  servicetitan-pricebook-pp-cli bulk-plan ./reviewed-changes.csv --agent
  ```

### Agent-native plumbing
- **`health`** — One compact rollup of markup-drift, vendor-part-gap, warranty-lint, cost-drift, orphan-SKU, duplicate, and weak-copy counts.

  _Reach for this first in any pricebook session to see what needs attention before drilling into a specific audit._

  ```bash
  servicetitan-pricebook-pp-cli health --agent
  ```
- **`find`** — Forgiving ranked search over synced SKUs tuned for describing a part you do not know the code for.

  _Reach for this when a tech describes a part in plain words — it returns suggested SKUs with the code, price, vendor part, and category they need to pick one._

  ```bash
  servicetitan-pricebook-pp-cli find "1 hp submersible pump motor" --agent
  ```

## Usage

Run `servicetitan-pricebook-pp-cli --help` for the full command reference and flag list.

## Commands

### categories

Manage categories

- **`servicetitan-pricebook-pp-cli categories create`** - Post to add a new category to your pricebook
- **`servicetitan-pricebook-pp-cli categories delete`** - Deletes an existing category from your pricebook
- **`servicetitan-pricebook-pp-cli categories get`** - Gets category details
- **`servicetitan-pricebook-pp-cli categories get-list`** - GET the categories in your pricebook
- **`servicetitan-pricebook-pp-cli categories update`** - Edits an existing category in your pricebook

### clientspecificpricing

Manage clientspecificpricing

- **`servicetitan-pricebook-pp-cli clientspecificpricing client-specific-pricing-get-all-rate-sheets`** - Client specific pricing_get all rate sheets
- **`servicetitan-pricebook-pp-cli clientspecificpricing client-specific-pricing-update-rate-sheet`** - Client specific pricing_update rate sheet

### discounts-and-fees

Manage discounts and fees

- **`servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-create`** - Post to add a new discount or fee to your pricebook
- **`servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-delete`** - Deletes a discount or fee from your pricebook
- **`servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-get`** - Get details of a discount or fee in the pricebook.
- **`servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-get-list`** - Get data on all of the discounts or fees in the pricebook. Supports optional search filtering.
- **`servicetitan-pricebook-pp-cli discounts-and-fees discount-and-fees-update`** - Edit an existing item in your pricebook

### equipment

Manage equipment

- **`servicetitan-pricebook-pp-cli equipment create`** - Post to add new equipment to your pricebook
- **`servicetitan-pricebook-pp-cli equipment delete`** - Deletes equipment from your pricebook
- **`servicetitan-pricebook-pp-cli equipment get`** - Get details of equipment in the pricebook.
- **`servicetitan-pricebook-pp-cli equipment get-list`** - Get data on all the equipment in the pricebook. Supports optional search filtering.
- **`servicetitan-pricebook-pp-cli equipment update`** - Edit an existing item in your pricebook

### images

Manage images

- **`servicetitan-pricebook-pp-cli images get`** - Downloads a specified pricebook image.
- **`servicetitan-pricebook-pp-cli images post`** - Uploads a specified image to temporary storage.
To associate the image with a pricebook item, send a separate request to update that item.

### materials

Manage materials

- **`servicetitan-pricebook-pp-cli materials create`** - Add a new Materials to your pricebook
- **`servicetitan-pricebook-pp-cli materials delete`** - Deletes a material from your pricebook
- **`servicetitan-pricebook-pp-cli materials get`** - Get details on a material in the pricebook.
- **`servicetitan-pricebook-pp-cli materials get-cost-types`** - Get details on materials in the pricebook.
- **`servicetitan-pricebook-pp-cli materials get-list`** - Get details on materials in the pricebook. Supports optional search filtering.
- **`servicetitan-pricebook-pp-cli materials update`** - Edit an existing item in your pricebook

### materialsmarkup

Manage materialsmarkup

- **`servicetitan-pricebook-pp-cli materialsmarkup materials-markup-create`** - Create materials markup item
- **`servicetitan-pricebook-pp-cli materialsmarkup materials-markup-get`** - Get materials markup item
- **`servicetitan-pricebook-pp-cli materialsmarkup materials-markup-get-list`** - Get materials markup collection
- **`servicetitan-pricebook-pp-cli materialsmarkup materials-markup-update`** - Update materials markup item

### pricebook

Manage pricebook

- **`servicetitan-pricebook-pp-cli pricebook bulk-create`** - Pricebook bulk_create
- **`servicetitan-pricebook-pp-cli pricebook bulk-update`** - Pricebook bulk_update

### pricebook-export

Manage pricebook export

- **`servicetitan-pricebook-pp-cli pricebook-export categories`** - Provides export feed for categories
- **`servicetitan-pricebook-pp-cli pricebook-export equipment`** - Provides export feed for equipment
- **`servicetitan-pricebook-pp-cli pricebook-export materials`** - Provides export feed for materials
- **`servicetitan-pricebook-pp-cli pricebook-export services`** - Provides export feed for services

### services

Manage services

- **`servicetitan-pricebook-pp-cli services create`** - Post to add a new service to your pricebook
- **`servicetitan-pricebook-pp-cli services delete`** - Deletes a service from your pricebook
- **`servicetitan-pricebook-pp-cli services get`** - Get details a service in the pricebook.
- **`servicetitan-pricebook-pp-cli services get-list`** - Get data on all of the services in the pricebook. Supports optional search filtering.
- **`servicetitan-pricebook-pp-cli services update`** - Edit an existing item in your pricebook


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
servicetitan-pricebook-pp-cli categories get mock-value mock-value

# JSON for scripting and agents
servicetitan-pricebook-pp-cli categories get mock-value mock-value --json

# Filter to specific fields
servicetitan-pricebook-pp-cli categories get mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
servicetitan-pricebook-pp-cli categories get mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
servicetitan-pricebook-pp-cli categories get mock-value mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-servicetitan-pricebook -g
```

Then invoke `/pp-servicetitan-pricebook <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add servicetitan-pricebook servicetitan-pricebook-pp-mcp -e ST_CLIENT_ID=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/servicetitan-pricebook-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ST_CLIENT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "servicetitan-pricebook": {
      "command": "servicetitan-pricebook-pp-mcp",
      "env": {
        "ST_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
servicetitan-pricebook-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/servicetitan-pricebook-pp-cli/config.toml` (override with `SERVICETITAN_PRICEBOOK_CONFIG`).

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ST_APP_KEY` | per_call | Yes | Static ServiceTitan App Key, sent as the `ST-App-Key` header on every call. |
| `ST_CLIENT_ID` | auth_flow_input | Yes | OAuth2 client_id; exchanged for a bearer token. |
| `ST_CLIENT_SECRET` | auth_flow_input | Yes | OAuth2 client_secret paired with `ST_CLIENT_ID`. |
| `ST_TENANT_ID` | per_call | Yes | Numeric ServiceTitan tenant ID; every Pricebook path is tenant-scoped. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `servicetitan-pricebook-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ST_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth fails with invalid_client 400** — Check for trailing whitespace or newlines in ST_CLIENT_ID / ST_CLIENT_SECRET; the client strips them, but verify the env var was set cleanly.
- **every call returns 401 JWT not present** — Run doctor — if the bearer token is missing, confirm ST_CLIENT_ID and ST_CLIENT_SECRET are set; the client mints the token on first call.
- **sync returns 0 records** — Confirm ST_TENANT_ID is set to your numeric tenant ID; every pricebook path is tenant-scoped and an unset tenant silently yields no data.
- **markup-audit or cost-drift returns nothing** — Run sync first — these commands read the local store and the sku_cost_history snapshot table, not the live API.
- **429 rate-limited during sync** — ServiceTitan allows ~7,000 req/hr per app key per environment; the client backs off automatically — re-run sync or use --resources to scope it.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
