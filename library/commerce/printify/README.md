# Printify CLI

**Create and audit Printify products with manifest-driven image uploads, personalization checks, and local proofing.**

This CLI covers Printify's official product, upload, catalog, order, shop, and webhook API surface, then adds workflow commands for product creation confidence. Agents can compile personalized manifests, upload images, validate placement, compare drift, and inspect fulfillment risk without parsing raw nested API payloads.

Created by [@horknfbr](https://github.com/horknfbr) (horknfbr).

## Install

The recommended path installs both the `printify-pp-cli` binary and the `pp-printify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install printify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install printify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install printify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install printify --agent claude-code
npx -y @mvanhorn/printing-press-library install printify --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/printify/cmd/printify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/printify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install printify --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-printify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-printify --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install printify --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/printify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PRINTIFY_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/printify/cmd/printify-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "printify": {
      "command": "printify-pp-mcp",
      "env": {
        "PRINTIFY_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Printify uses a personal access token as a bearer token. Set `PRINTIFY_API_TOKEN` in your environment or `.env`; do not pass the token as a command argument.

## Quick Start

```bash
# Find the shop ID to use for product and order work while keeping the response small.
printify-pp-cli shops-json --agent --select id,title

# Choose a blueprint before selecting a print provider or variants.
printify-pp-cli catalog retrieves-list-of-blueprints-in-the --agent --select id,title

# Upload product artwork before composing the product manifest.
printify-pp-cli uploads an-image --body-json '{"file_name":"front.png","contents":"data:image/png;base64,iVBORw0KGgo="}' --agent

# Create a draft product from a prepared manifest.
printify-pp-cli shops products-json create-anew-product 123456 --title Sample --blueprint-id 384 --print-provider-id 1 --variants '[]' --print-areas '[]' --agent

# Compare the intended manifest with the product that now exists in Printify.
printify-pp-cli product-drift --product-file ./examples/current-product.json --manifest ./examples/sample-product.json --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Product proofing
- **`personalization-audit`** — Audit a product for documented personalization placeholder fields, missing text/image setup, and unsupported browser-only gaps.

  _Use this before publishing a personalized product or before handing a manifest to another agent._

  ```bash
  printify-pp-cli personalization-audit --product-file ./examples/sample-product.json --agent
  ```
- **`placement-matrix`** — Show variant, print-area, placeholder, uploaded image, x/y/scale/angle, and missing-placement rows for a product.

  _Use this when an agent must prove artwork placement is consistent across variants._

  ```bash
  printify-pp-cli placement-matrix --product-file ./examples/sample-product.json --uploads-file ./examples/sample-uploads.json --agent
  ```
- **`product-drift`** — Compare an intended product manifest against the current Printify product payload after create or update.

  _Use this after automation creates a product and before trusting the resulting listing._

  ```bash
  printify-pp-cli product-drift --product-file ./examples/current-product.json --manifest ./examples/sample-product.json --agent
  ```

### Catalog decisions
- **`catalog-margin-matrix`** — Join catalog variants and shipping data to estimate per-variant cost and margin at a target retail price.

  _Use this before creating a batch when product economics matter more than raw catalog browsing._

  ```bash
  printify-pp-cli catalog-margin-matrix --variants-file ./examples/sample-variants.json --shipping-file ./examples/sample-shipping.json --target-price 24.99 --agent
  ```

### Batch automation
- **`personalization-batch`** — Expand a reusable product manifest and CSV rows into per-product manifests using documented image and text placeholder fields.

  _Use this when an agent needs repeatable personalized product drafts before making API writes._

  ```bash
  printify-pp-cli personalization-batch --template ./examples/template-product.json --csv ./examples/personalization.csv --out ./examples/generated-manifests --agent
  ```

### Operational audits
- **`asset-reuse`** — List uploaded images, where each is used, unused uploads, and products sharing the same artwork.

  _Use this to clean upload libraries or verify that a product batch reuses the expected artwork._

  ```bash
  printify-pp-cli asset-reuse --products-file ./examples/sample-products.json --uploads-file ./examples/sample-uploads.json --agent
  ```
- **`fulfillment-risk`** — Flag open orders tied to risky product, variant, publish, or shipment states.

  _Use this when an agent needs to find fulfillment problems before customer-service work starts._

  ```bash
  printify-pp-cli fulfillment-risk --orders-file ./examples/sample-orders.json --products-file ./examples/sample-products.json --agent
  ```

## Recipes

### Find a shop with narrow output

```bash
printify-pp-cli shops-json --agent --select id,title
```

Use this first so follow-up product and order commands target the right shop without dumping the full shop payload.

### Compile personalized product manifests

```bash
printify-pp-cli personalization-batch --template ./examples/template-product.json --csv ./examples/personalization.csv --out ./examples/generated-manifests --agent
```

Turn a reusable template and row data into deterministic product manifests before any API writes.

### Proof artwork placement

```bash
printify-pp-cli placement-matrix --product-file ./examples/sample-product.json --uploads-file ./examples/sample-uploads.json --agent --select variant_id,print_area,image_id,x,y,scale,angle
```

Inspect only the placement columns an agent needs before publishing.

### Check product drift after create

```bash
printify-pp-cli product-drift --product-file ./examples/current-product.json --manifest ./examples/sample-product.json --agent
```

Confirm the remote product still matches the manifest the automation intended.

### Scan fulfillment risk

```bash
printify-pp-cli fulfillment-risk --orders-file ./examples/sample-orders.json --products-file ./examples/sample-products.json --agent
```

Join orders, variants, product state, and shipment state into a single operational risk view.

## Usage

Run `printify-pp-cli --help` for the full command reference and flag list.

## Commands

### catalog

Browse the Printify catalog including blueprints, print providers, product variants, and shipping information. Explore available products and their customization options.

- **`printify-pp-cli catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint`** - Retrieve a list of all print providers that fulfill orders for a specific blueprint
- **`printify-pp-cli catalog retrieve-alist-of-available-print-providers`** - Retrieves the list of blueprints in the catalog to explore from
- **`printify-pp-cli catalog retrieve-alist-of-variants-of-ablueprint-from-aspecific-print-provider`** - Retrieves the list of of variants options for the Print Provider and Blueprint.
    Those form the set of options available for customization Product (Blueprint)
    on particular manufacturer (Print Provider).
- **`printify-pp-cli catalog retrieve-aspecific-blueprint`** - Retrieves the list of blueprints in the catalog to explore from
- **`printify-pp-cli catalog retrieve-aspecific-print-provider`** - Retrieves the list of blueprints in the catalog to explore from
- **`printify-pp-cli catalog retrieve-available-shipping-list-information`** - Retrieves the list of print providers avilable for the Blueprint
- **`printify-pp-cli catalog retrieve-economy-shipping-method-information`** - Retrieves the list of print providers available for the Blueprint
- **`printify-pp-cli catalog retrieve-express-shipping-method-information`** - Retrieves the list of print providers available for the Blueprint
- **`printify-pp-cli catalog retrieve-priority-shipping-method-information`** - Retrieves the list of print providers available for the Blueprint
- **`printify-pp-cli catalog retrieve-shipping-information`** - Retrieves the list of print providers avilable for the Blueprint
- **`printify-pp-cli catalog retrieve-specific-shipping-method-information`** - Retrieves the list of print providers avilable for the Blueprint
- **`printify-pp-cli catalog retrieves-list-of-blueprints-in-the`** - Retrieves the list of blueprints in the catalog to explore from

### shops

Manage Printify shops and shop connections. Retrieve shop information and disconnect shops from your account.

### shops-json

Manage shops json

- **`printify-pp-cli shops-json`** - This will return the list of available merchant shops (IDs and titles)

### uploads

Upload and manage images and assets. Upload images from URLs or base64-encoded content, retrieve upload information, and archive uploaded images.

- **`printify-pp-cli uploads an-image`** - Upload an image
- **`printify-pp-cli uploads retrieve-an-uploaded-image-by-id`** - Retrieve an uploaded image by id

### uploads-json

Manage uploads json

- **`printify-pp-cli uploads-json`** - Retrieve a list of uploaded images

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
printify-pp-cli catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint mock-value

# JSON for scripting and agents
printify-pp-cli catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint mock-value --json

# Filter to specific fields
printify-pp-cli catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint mock-value --json --select id,name,status

# Dry run — show the request without sending
printify-pp-cli catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
printify-pp-cli catalog retrieve-alist-of-all-print-providers-that-fulfill-orders-for-aspecific-blueprint mock-value --agent
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
printify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/printify-public-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PRINTIFY_BEARER_AUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `printify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PRINTIFY_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 or unauthorized responses** — Confirm `PRINTIFY_API_TOKEN` is set from `.env` or your shell, then rerun `printify-pp-cli health --agent`.
- **Product creation fails with missing variants or print areas** — Run catalog blueprint/provider commands first, then regenerate the manifest with valid `blueprint_id`, `print_provider_id`, `variants`, and `print_areas`.
- **Personalization looks incomplete** — Run `printify-pp-cli personalization-audit --product-file ./examples/sample-product.json --agent`; the CLI reports documented fields separately from unsupported browser-only controls.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**TSavo/printify-mcp**](https://github.com/TSavo/printify-mcp) — JavaScript/TypeScript (27 stars)
- [**lawrencemq/printipy**](https://github.com/lawrencemq/printipy) — Python (17 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
