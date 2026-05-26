# Jinko CLI



Printed by [@dyzsasd](https://github.com/dyzsasd) (Shuai).

## Install

The recommended path installs both the `jinko-pp-cli` binary and the `pp-jinko` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install jinko
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install jinko --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install jinko --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install jinko --agent claude-code
npx -y @mvanhorn/printing-press-library install jinko --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jinko/cmd/jinko-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jinko-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-jinko --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-jinko --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-jinko skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-jinko. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/jinko-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/jinko/cmd/jinko-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "jinko": {
      "command": "jinko-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

```bash
jinko-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
jinko-pp-cli devplatform list
```

## Usage

Run `jinko-pp-cli --help` for the full command reference and flag list.

## Commands

### devplatform

Manage devplatform

- **`jinko-pp-cli devplatform create`** - Unified flight shop endpoint. When offer_token is present the handler re-prices that offer (price-check mode); otherwise it performs a live search by route with optional JIN-187 filter fields (cabin_class, direct_only, max_price, include_carriers, exclude_carriers).
- **`jinko-pp-cli devplatform create-admin`** - Add credits to a user's rate limit balance (admin only)
- **`jinko-pp-cli devplatform create-keys`** - Validate an API key without incrementing rate limit counter (public, no auth required)
- **`jinko-pp-cli devplatform create-keys-2`** - Verify an API key and increment rate limit counter (public, no auth required)
- **`jinko-pp-cli devplatform create-orgs`** - Create a new devplatform organization (admin only)
- **`jinko-pp-cli devplatform create-orgs-2`** - Create a partner API key that can be used to create shadow users (admin only)
- **`jinko-pp-cli devplatform create-orgs-3`** - Create a shadow user that can later be claimed by a real identity (partner key or admin auth)
- **`jinko-pp-cli devplatform create-users`** - Create a new developer platform user (admin only)
- **`jinko-pp-cli devplatform create-users-2`** - Check whether a devplatform user exists and their current status (public, no auth)
- **`jinko-pp-cli devplatform create-users-3`** - Link a real WorkOS identity to an existing shadow user account. Email is taken from JWT, not request body.
- **`jinko-pp-cli devplatform create-users-4`** - Create a new API key for the authenticated user
- **`jinko-pp-cli devplatform create-users-5`** - Self-register a new devplatform account using the authenticated JWT identity
- **`jinko-pp-cli devplatform delete`** - Revoke an API key by ID for the authenticated user
- **`jinko-pp-cli devplatform delete-orgs`** - Revoke an API key for a specific user in an organization (org_admin or admin)
- **`jinko-pp-cli devplatform get`** - Get organization details by ID (org_admin or admin)
- **`jinko-pp-cli devplatform get-orgs`** - List all users belonging to an organization (org_admin or admin)
- **`jinko-pp-cli devplatform get-users`** - Resolve the active API key for a user by WorkOS ID (admin only)
- **`jinko-pp-cli devplatform list`** - List all devplatform organizations (admin only)
- **`jinko-pp-cli devplatform list-users`** - List all users or filter by status (pending, active, suspended). Admin only.
- **`jinko-pp-cli devplatform list-users-2`** - Get the devplatform user profile for the authenticated JWT user
- **`jinko-pp-cli devplatform list-users-3`** - List all API keys for the authenticated user
- **`jinko-pp-cli devplatform list-users-4`** - Get API usage and rate limit statistics for the authenticated user
- **`jinko-pp-cli devplatform update`** - Approve a pending devplatform user by WorkOS ID (admin only)
- **`jinko-pp-cli devplatform update-admin`** - Update the role of a devplatform user (admin only)
- **`jinko-pp-cli devplatform update-orgs`** - Assign a user as admin of an organization by WorkOS ID (admin only)

### flights

Manage flights

- **`jinko-pp-cli flights create`** - Check a flight offer from search results to get confirmed availability and detailed fare options
- **`jinko-pp-cli flights create-destinationsearch`** - Search for destination cities with cheapest flight options based on filter criteria
- **`jinko-pp-cli flights create-refundcheck`** - Check whether a flight booking is eligible for a refund and get the refund amount
- **`jinko-pp-cli flights create-search`** - Search flights (synchronous, filter-based)

### hotels

Manage hotels

- **`jinko-pp-cli hotels create`** - Search hotels with a natural-language query (e.g. "beach
- **`jinko-pp-cli hotels create-shop`** - Search hotels by destination and return available rooms with live pricing

### payment

Manage payment

- **`jinko-pp-cli payment create`** - Create a payment authorization for a quote using Stripe
- **`jinko-pp-cli payment create-authorization`** - Capture an authorized payment (full or partial amount)
- **`jinko-pp-cli payment create-cart`** - Create a payment authorization for a shopping cart quote using Stripe
- **`jinko-pp-cli payment create-stripe`** - Get the Stripe client secret (PaymentIntent flow) or checkout URL (Checkout Session flow)
- **`jinko-pp-cli payment get`** - Retrieve payment authorization information by ID
- **`jinko-pp-cli payment get-authorization`** - Check the current status of a payment from Stripe and update local record
- **`jinko-pp-cli payment get-quote`** - Retrieve payment authorization information by quote ID

### shop

Manage shop

- **`jinko-pp-cli shop create`** - Unified trip management endpoint. Accepts create / add_product / remove_product / set_travelers / remove_travelers actions and returns the resulting trip state.
- **`jinko-pp-cli shop create-fulfillment`** - Verify payment with Stripe API and transition fulfillment from awaiting_payment to processing
- **`jinko-pp-cli shop create-fulfillment-2`** - Retrieve current fulfillment orchestration status and results
- **`jinko-pp-cli shop create-fulfillment-3`** - Schedule fulfillment orchestration for all successfully quoted products
- **`jinko-pp-cli shop create-quote`** - Retrieve current quote orchestration status and results
- **`jinko-pp-cli shop create-quote-2`** - Schedule quote orchestration for all products in a cart
- **`jinko-pp-cli shop create-sync`** - Selects or updates ancillaries (seats, bags, meals, assistance) for a trip item. The body references a trip_id and trip_item_token and provides the desired selection; the handler validates the selection against the item's available ancillary inventory.
- **`jinko-pp-cli shop create-sync-2`** - Starts the checkout flow for a trip (cart), creating a Stripe Checkout Session or PaymentIntent depending on the configured authorisation type.
- **`jinko-pp-cli shop get`** - Retrieve a trip by ID

### shopping-cart

Manage shopping cart

- **`jinko-pp-cli shopping-cart create`** - Create a new shopping cart bound to a session
- **`jinko-pp-cli shopping-cart delete`** - Remove a product from the shopping cart (soft delete)
- **`jinko-pp-cli shopping-cart get`** - Retrieve a shopping cart with its products
- **`jinko-pp-cli shopping-cart get-shoppingcart`** - Retrieve a specific product's details from the cart
- **`jinko-pp-cli shopping-cart update`** - Add a product to an existing shopping cart
- **`jinko-pp-cli shopping-cart update-shoppingcart`** - Update contact info and/or travelers on a shopping cart

### travelers

Manage travelers

- **`jinko-pp-cli travelers create`** - Create a traveler profile
- **`jinko-pp-cli travelers delete`** - Delete a traveler profile
- **`jinko-pp-cli travelers get`** - Retrieve a traveler profile by ID
- **`jinko-pp-cli travelers list`** - List traveler profiles for the authenticated user
- **`jinko-pp-cli travelers update`** - Update a traveler profile


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
jinko-pp-cli devplatform list

# JSON for scripting and agents
jinko-pp-cli devplatform list --json

# Filter to specific fields
jinko-pp-cli devplatform list --json --select id,name,status

# Dry run — show the request without sending
jinko-pp-cli devplatform list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
jinko-pp-cli devplatform list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
jinko-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/api-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
