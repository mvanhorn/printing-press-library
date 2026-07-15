# Squarespace CLI

Squarespace Commerce API coverage plus browser-backed account dashboard reads for domains, DNS records, email forwarding, Google Workspace pricing, and domain billing metadata.

Created by [@zaydiscold](https://github.com/zaydiscold) (Zayd).

## Install

The recommended path installs both the `squarespace-pp-cli` binary and the `pp-squarespace` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install squarespace
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install squarespace --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install squarespace --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install squarespace --agent claude-code
npx -y @mvanhorn/printing-press-library install squarespace --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/squarespace/cmd/squarespace-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/squarespace-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install squarespace --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-squarespace --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-squarespace --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install squarespace --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/squarespace-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `COMMERCE_AUTHORIZATION` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "squarespace": {
      "command": "squarespace-pp-mcp",
      "env": {
        "COMMERCE_AUTHORIZATION": "<your-key>"
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

For Commerce API commands, get your access token from your API provider's developer portal, then store it:

```bash
squarespace-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export COMMERCE_AUTHORIZATION="your-token-here"
```

For account dashboard commands, provide a browser Cookie header from an authenticated `account.squarespace.com` session:

```bash
export SQUARESPACE_ACCOUNT_COOKIE_FILE="$HOME/.config/squarespace/account-cookie.txt"
```

The account commands dynamically resolve domain names through `account domain get --name <domain>` before calling domain-id, website-id, or contract-id endpoints. Do not hardcode internal Squarespace IDs into scripts.

### 3. Verify Setup

```bash
squarespace-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
squarespace-pp-cli contacts get --pagination-parameters example-value
```

## Usage

Run `squarespace-pp-cli --help` for the full command reference and flag list.

## Account Dashboard Commands

These commands use the logged-in account dashboard API. They are read-only and are useful for domains that are not covered by the public Commerce API.

```bash
squarespace-pp-cli account domain-summaries --page-size 50 --json
squarespace-pp-cli account domain get --name example.com --json
squarespace-pp-cli account domain custom-records --name example.com --json
squarespace-pp-cli account domain email-forwarding --name example.com --json
squarespace-pp-cli account domain email-mx-conflicts --name example.com --json
squarespace-pp-cli account domain billing-eligibility --name example.com --json
squarespace-pp-cli account domain billing-valid-terms --name example.com --json
squarespace-pp-cli account domain google-workspace-pricing --country-code US --json
```

The resolver first calls `/api/account/1/domains/byName/<domain>`, then uses the returned `id`, `websiteId`, and `subscriptionId` for the follow-on endpoint. That keeps the CLI domain-agnostic across different accounts.

## Commands

### 1-0

Manage 1 0

- **`squarespace-pp-cli 1-0 adjust-inventory-stock-levels`** - Adjusts stock quantities for product variants. Stock quantities can be added or subtracted, set with a given number, or marked as "unlimited". All quantity information is stored in InventoryItems.
- **`squarespace-pp-cli 1-0 create-order`** - Creates an order using information from a third-party sales channel. A successful request creates an Order resource.
- **`squarespace-pp-cli 1-0 create-webhook-subscription`** - Creates a webhook subscription. A successful request returns the created Webhook Subscription resource.
- **`squarespace-pp-cli 1-0 delete-webhook-subscription`** - Deletes a webhook subscription.
- **`squarespace-pp-cli 1-0 fulfill-order`** - Updates the status of a specific order to fulfilled, with options to include shipment information and send a customer notification.
- **`squarespace-pp-cli 1-0 get-documents-by-id`** - Retrieves information for specific transaction Documents. The response contains up to 50 Documents. Multiple Documents can be retrieved by providing a comma-separated list of Document ids.
- **`squarespace-pp-cli 1-0 get-documents-by-updated-on`** - Retrieves all financial transactions for orders and donations. Per order and donation, the response groups transactions into a Document, contains up to 50 Documents ordered by their modification date (modifiedOn), and supports dynamic cursors for pagination.
- **`squarespace-pp-cli 1-0 get-inventory-items`** - Retrieves real-time stock information for all product variants. Stock information is stored in an InventoryItem for each product variant. The response contains up to 50 InventoryItems and supports dynamic cursors for pagination.
- **`squarespace-pp-cli 1-0 get-member-profile`** - Retrieves basic details about the Squarespace member who owns the provided OAuth token
- **`squarespace-pp-cli 1-0 get-order`** - Retrieves information for a specific order. The response contains order information in an Order.
- **`squarespace-pp-cli 1-0 get-orders`** - Retrieves information about all orders. Orders can be filtered by Customer ID and date ranges. The response contains order information in an Order, up to 50 Orders ordered by their modification date (modifiedOn), and supports dynamic cursors for pagination.
- **`squarespace-pp-cli 1-0 get-profiles`** - Retrieves all profiles; profiles can be filtered and sorted. The response contains up to 50 Profiles and supports dynamic cursors for pagination.
- **`squarespace-pp-cli 1-0 get-specific-inventory-items`** - Retrieves real-time stock information for specific product variants. Stock information is stored in an InventoryItem and up to 50 InventoryItems can be retrieved per request.
- **`squarespace-pp-cli 1-0 get-specific-profiles`** - Retrieves information for specific profiles. The response contains a list of up to 50 Profiles. Multiple Profiles can be retrieved by providing a comma-separated list of profile ids.
- **`squarespace-pp-cli 1-0 get-store-pages`** - Retrieves information for all store pages on the website. The response supports dynamic cursors for pagination.
- **`squarespace-pp-cli 1-0 get-webhook-subscription`** - Retrieves information for a specific webhook subscription.
- **`squarespace-pp-cli 1-0 get-webhook-subscriptions`** - Retrieves information for all webhook subscriptions. The response contains up to 25 Webhook Subscriptions.
- **`squarespace-pp-cli 1-0 get-website-profile`** - Retrieves basic details about the website that owns the provided API key or OAuth token
- **`squarespace-pp-cli 1-0 rotate-subscription-secret`** - Rotates a webhook subscription's secret. The previous secret for a subscription is no longer valid after a new one is generated.
- **`squarespace-pp-cli 1-0 send-test-notification-for-webhook-subscription`** - Sends a notification to a subscribed webhook endpoint for testing purposes. This is a one-time notification, and will not be retried if it fails or times out.
- **`squarespace-pp-cli 1-0 update-webhook-subscription`** - Updates information for a webhook subscription. A successful request returns the updated Webhook Subscription resource.

### commerce

Manage commerce

- **`squarespace-pp-cli commerce associate-product-variant-image`** - Assigns a product image to a product variant. Specifying imageId of null will delete the association.
- **`squarespace-pp-cli commerce create-product`** - Creates a product with appropriate subresources based on product type.
- **`squarespace-pp-cli commerce create-product-variant`** - Creates a variant of a product. Creating a variant for a physical or service product requires that the product already has at least one attribute.
- **`squarespace-pp-cli commerce delete-product`** - Delete specified product
- **`squarespace-pp-cli commerce delete-product-image`** - Delete one product image
- **`squarespace-pp-cli commerce delete-product-variant`** - Delete one product variant
- **`squarespace-pp-cli commerce get-product-image-processing-status`** - Retrieves the processing status for an uploaded product image. A successful request indicates that a ProductImage is processing, ready, or in an error state.
- **`squarespace-pp-cli commerce get-products`** - Retrieves information for products; products can be filtered within a date range and by product type. The response contains product information in a Product, up to 50 Products ordered by their modification date (modifiedOn), and supports dynamic cursors for pagination.
- **`squarespace-pp-cli commerce get-specific-products`** - Retrieves information for specific products, including information for any variants or images. Up to 50 products can be retrieved per request.
- **`squarespace-pp-cli commerce update-product`** - Updates information for a product. The endpoint supports partial updates.
- **`squarespace-pp-cli commerce update-product-image`** - Updates information for a product image. Currently, the endpoint only supports updates to the alt text for an image.
- **`squarespace-pp-cli commerce update-product-image-order`** - Updates the ordering of a product image on the product details page.
- **`squarespace-pp-cli commerce update-product-variant`** - Updates a variant of a product. The endpoint supports partial updates.
- **`squarespace-pp-cli commerce upload-product-image`** - Uploads an image for a product. Uploading an image doesn't set the product's featured image. A successful request creates a ProductImage subresource of the Product.

### commerce-analytics

Manage commerce analytics

- **`squarespace-pp-cli commerce-analytics get-transactions-summaries`** - Retrieve transaction summaries grouped by contact

### contacts

Manage customer contacts and address book entries for a website: create, read, update, delete, and query contacts; maintain addresses for shipping and fulfillment.

- **`squarespace-pp-cli contacts create`** - Creates a new contact. Requires OAuth website scope WEBSITE_CONTACTS or WEBSITE_PROFILES (read/write).
- **`squarespace-pp-cli contacts delete`** - Deletes the contact for the given contact ID. Requires OAuth website scope WEBSITE_CONTACTS or WEBSITE_PROFILES (read/write).
- **`squarespace-pp-cli contacts get`** - Returns a paginated list of contacts for the website. Requires OAuth website scope WEBSITE_CONTACTS_READ, WEBSITE_CONTACTS, WEBSITE_PROFILES_READ, or WEBSITE_PROFILES.
- **`squarespace-pp-cli contacts get-contactid`** - Returns the contact for the given contact ID. Requires OAuth website scope WEBSITE_CONTACTS_READ, WEBSITE_CONTACTS, WEBSITE_PROFILES_READ, or WEBSITE_PROFILES.
- **`squarespace-pp-cli contacts patch`** - Updates a contact using JSON merge patch. Requires OAuth website scope WEBSITE_CONTACTS or WEBSITE_PROFILES (read/write).
- **`squarespace-pp-cli contacts query`** - Returns a paginated list of contacts matching filters and sort options in the request body. Requires OAuth website scope WEBSITE_CONTACTS_READ, WEBSITE_CONTACTS, WEBSITE_PROFILES_READ, or WEBSITE_PROFILES.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
squarespace-pp-cli contacts get --pagination-parameters example-value

# JSON for scripting and agents
squarespace-pp-cli contacts get --pagination-parameters example-value --json

# Filter to specific fields
squarespace-pp-cli contacts get --pagination-parameters example-value --json --select id,name,status

# Dry run — show the request without sending
squarespace-pp-cli contacts get --pagination-parameters example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
squarespace-pp-cli contacts get --pagination-parameters example-value --agent
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
squarespace-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/commerce-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `COMMERCE_AUTHORIZATION` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `squarespace-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $COMMERCE_AUTHORIZATION`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
