# Swiggy CLI

Created by Som Samantray (@SomSamantray).
Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

**Every Swiggy Builders Club tool, one persistent OAuth session, and a local order history no Swiggy surface shows you.**

Wraps all 51 official Swiggy MCP tools across Food, Instamart, and Dineout into one CLI that respects Swiggy's own session-reuse rules instead of tripping rate limits. Adds cross-domain spend history, a safe-retry guard for non-idempotent order placement, and a UPI payment-wait command on top.

MCP orchestration sends each operation in a `tools/call` envelope. Verification mode can run the 24 explicitly read-only operations and keeps all other operations behind the mutation guard.

On Windows, credential-file checks reject a NULL access-control list even when the security descriptor also contains protection or inheritance flags.

## Install

The recommended path installs both the `swiggy-pp-cli` binary and the `pp-swiggy` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install swiggy
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install swiggy --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install swiggy --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install swiggy --agent claude-code
npx -y @mvanhorn/printing-press-library install swiggy --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/swiggy/cmd/swiggy-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/swiggy-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install swiggy --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-swiggy --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-swiggy --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install swiggy --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local OAuth tokens — authenticate first if you haven't:

```bash
swiggy-pp-cli auth login
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/swiggy-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SWIGGY_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/food-and-dining/swiggy/cmd/swiggy-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "swiggy": {
      "command": "swiggy-pp-mcp",
      "env": {
        "SWIGGY_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Swiggy Builders Club uses OAuth 2.1 with PKCE and Dynamic Client Registration — no API key. Run `swiggy-pp-cli auth login` to open a browser, log in with your phone number and OTP on Swiggy's own consent screen, and the CLI stores the resulting 5-day access token. There is no refresh token in v1.0; re-run `auth login` when it expires.

## Quick Start

```bash
# Health check that works without auth — confirms the binary and config are set up before logging in.
swiggy-pp-cli doctor --dry-run

# Complete the OAuth 2.1 PKCE browser login (phone + OTP) once; the CLI persists the session.
swiggy-pp-cli auth login

# First real call — confirms the session works and gives you an addressId for search.
swiggy-pp-cli food get-addresses

# Search restaurants near a saved address.
swiggy-pp-cli food search-restaurants --address-id addr_01HXYZ --query biryani

# See cross-domain (Food + Instamart) spend once you have some order history.
swiggy-pp-cli history --address-id addr_01HXYZ --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`history`** — See spend and order counts across Food and Instamart in one view, even though Swiggy keeps them completely separate. Instamart totals are a sample of at most 20 orders and are labeled partial when that cap is hit. `--since` is reserved and unused. Dineout has no orders-list tool and is not included.

  _Reach for this when the user asks about overall Swiggy spend or activity rather than a single domain's orders._

  ```bash
  swiggy-pp-cli history --address-id addr_01HXYZ --agent
  ```

### Agent-native plumbing
- **`pay wait`** — Block until a UPI payment resolves instead of hand-rolling a polling loop yourself. Not for COD orders — check payment status once and proceed instead.

  _Use this immediately after any UPI place-order/checkout/book-table call; do not poll check-payment-status manually._

  ```bash
  swiggy-pp-cli pay wait --paas-id paas_123 --order-id ord_01HXYZ --domain food --max-wait 5s
  ```
- **`status`** — Check local credential presence and stored token expiry. A token with no stored expiry is reported as unknown, not fully authenticated. This is not a live Swiggy validation.

  _Check this before starting a multi-step order flow to avoid a mid-flow 401._

  ```bash
  swiggy-pp-cli status
  ```

### Reachability mitigation
- **`order verify-before-retry`** — Check whether a food or grocery order actually went through before retrying a failed placement. Amount/restaurant matches must fall inside `--within` (default 30m); orders without timestamps fail closed.

  _Use this before ever re-issuing place-food-order or checkout after a 5xx or timeout._

  ```bash
  swiggy-pp-cli order verify-before-retry --domain food --address-id addr_01HXYZ --amount 450
  ```

## Recipes

### Order food end-to-end

```bash
swiggy-pp-cli food search-restaurants --address-id addr_01HXYZ --query biryani && swiggy-pp-cli food get-restaurant-menu --address-id addr_01HXYZ --restaurant-id r_123 --select restaurantId,items.id,items.name,items.price
```

Search then narrow a large menu payload down to just the fields needed to build a cart.

### Book a table

```bash
swiggy-pp-cli dineout get-available-slots --restaurant-id r_456 --date 2026-09-20 --latitude 28.4595 --longitude 77.0266
```

Check slot availability before booking.

### Safe retry after a failed order

```bash
swiggy-pp-cli order verify-before-retry --domain food --address-id addr_01HXYZ --restaurant-id r_123 --amount 450
```

Confirms whether a non-idempotent place-food-order call actually succeeded before retrying.

### Check spend across Food and Instamart

```bash
swiggy-pp-cli history --address-id addr_01HXYZ --agent --select total_spend,order_count,by_domain
```

Cross-domain aggregation (Food + Instamart; Dineout has no orders-list tool) with a narrowed agent-friendly field selection. --address-id is required to include Food.

## Usage

Run `swiggy-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SWIGGY_CONFIG_DIR`, `SWIGGY_DATA_DIR`, `SWIGGY_STATE_DIR`, or `SWIGGY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SWIGGY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SWIGGY_HOME=/srv/swiggy
swiggy-pp-cli doctor
```

Under `SWIGGY_HOME=/srv/swiggy`, the four dirs resolve to `/srv/swiggy/config`, `/srv/swiggy/data`, `/srv/swiggy/state`, and `/srv/swiggy/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "swiggy": {
      "command": "swiggy-pp-mcp",
      "env": {
        "SWIGGY_HOME": "/srv/swiggy"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SWIGGY_DATA_DIR` overrides an explicit `--home` for that kind. Use `SWIGGY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SWIGGY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `swiggy-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### dineout

Swiggy Dineout MCP tools

- **`swiggy-pp-cli dineout book-table`** - Swiggy Dineout (Reservations): Book a table at a restaurant for a specific time slot
- **`swiggy-pp-cli dineout cancel-booking`** - Swiggy Dineout (Reservations): Cancel an existing table reservation by orderId
- **`swiggy-pp-cli dineout check-payment-status`** - Check one payment-status iteration for an in-flight UPI payment
- **`swiggy-pp-cli dineout confirm-order`** - Complete an order after payment succeeds
- **`swiggy-pp-cli dineout create-cart`** - Swiggy Dineout (Reservations): Create a booking cart
- **`swiggy-pp-cli dineout get-available-slots`** - Swiggy Dineout (Reservations): Check available time slots for TABLE BOOKING at a restaurant
- **`swiggy-pp-cli dineout get-booking-status`** - Swiggy Dineout (Reservations): Get booking status and details for a dineout reservation
- **`swiggy-pp-cli dineout get-payment-options`** - Fetch the live payment methods currently available for the cart
- **`swiggy-pp-cli dineout get-restaurant-details`** - Swiggy Dineout (Reservations): Get details about a specific restaurant for TABLE BOOKING
- **`swiggy-pp-cli dineout get-saved-locations`** - Swiggy Dineout (Reservations): Get user's saved addresses for restaurant search
- **`swiggy-pp-cli dineout report-error`** - Generate an error report to share with the Swiggy MCP team
- **`swiggy-pp-cli dineout search-restaurants-dineout`** - Swiggy Dineout (Reservations): find restaurants to BOOK A TABLE at

### food

Swiggy Food MCP tools

- **`swiggy-pp-cli food apply-food-coupon`** - Apply coupon code or discount to food delivery order
- **`swiggy-pp-cli food check-payment-status`** - Check one payment-status iteration for an in-flight UPI payment
- **`swiggy-pp-cli food confirm-order`** - Complete an order after payment succeeds
- **`swiggy-pp-cli food create-address`** - Swiggy (Instamart/Food): Create a new delivery address for the authenticated user.
- **`swiggy-pp-cli food delete-address`** - Swiggy (Instamart/Food): Delete a saved delivery address for the authenticated user.
- **`swiggy-pp-cli food fetch-food-coupons`** - Get available coupons and offers for food delivery order
- **`swiggy-pp-cli food flush-food-cart`** - Clear or empty the food delivery cart
- **`swiggy-pp-cli food get-addresses`** - Swiggy (Instamart/Food): Get saved delivery addresses for the authenticated Swiggy user, sorted by last order date (most recent first)
- **`swiggy-pp-cli food get-food-cart`** - Get current food delivery cart with all items
- **`swiggy-pp-cli food get-food-delivery-status`** - Get the latest delivery ETA and terminal delivery state for a Food order
- **`swiggy-pp-cli food get-food-order-details`** - Get detailed information about a specific food delivery order
- **`swiggy-pp-cli food get-food-orders`** - Swiggy Food order history - Use this to fetch ORDER HISTORY, past orders, or active orders
- **`swiggy-pp-cli food get-payment-options`** - Fetch the live payment methods currently available for the cart
- **`swiggy-pp-cli food get-restaurant-menu`** - Browse a restaurant's complete menu as a flat, deduplicated list of dishes
- **`swiggy-pp-cli food place-food-order`** - Place food delivery order and confirm order placement
- **`swiggy-pp-cli food report-error`** - Generate an error report to share with the Swiggy MCP team
- **`swiggy-pp-cli food search-menu`** - Search for dishes and menu items to order for food delivery
- **`swiggy-pp-cli food search-restaurants`** - Search and order food from restaurants for delivery
- **`swiggy-pp-cli food track-food-order`** - Track food delivery order status and delivery progress
- **`swiggy-pp-cli food update-food-cart`** - Add items to food delivery cart or update cart contents

### instamart

Swiggy Instamart MCP tools

- **`swiggy-pp-cli instamart apply-coupon`** - Swiggy Instamart (Grocery): Apply a coupon code to the current Instamart cart
- **`swiggy-pp-cli instamart check-payment-status`** - Check one payment-status iteration for an in-flight UPI payment
- **`swiggy-pp-cli instamart checkout`** - Swiggy Instamart (Grocery): Place and confirm Swiggy Instamart grocery order
- **`swiggy-pp-cli instamart clear-cart`** - Clear (remove all items from) the Instamart cart
- **`swiggy-pp-cli instamart confirm-order`** - Complete an order after payment succeeds
- **`swiggy-pp-cli instamart create-address`** - Swiggy (Instamart/Food): Create a new delivery address for the authenticated user.
- **`swiggy-pp-cli instamart delete-address`** - Swiggy (Instamart/Food): Delete a saved delivery address for the authenticated user.
- **`swiggy-pp-cli instamart get-addresses`** - Swiggy (Instamart/Food): Get saved delivery addresses for the authenticated Swiggy user, sorted by last order date (most recent first)
- **`swiggy-pp-cli instamart get-cart`** - Swiggy Instamart (Grocery): Get current Swiggy Instamart grocery cart with all items and bill breakdown
- **`swiggy-pp-cli instamart get-delivery-status`** - ## Usage notes  - Use for structured delivery ETA refreshes after an Instamart order is placed. - Do not call in a tight loop
- **`swiggy-pp-cli instamart get-order-details`** - Get detailed information for a specific Swiggy Instamart order by order ID
- **`swiggy-pp-cli instamart get-orders`** - Swiggy Instamart order history - Use this to fetch ORDER HISTORY, past orders, or order preferences
- **`swiggy-pp-cli instamart get-payment-options`** - Fetch the live payment methods currently available for the cart
- **`swiggy-pp-cli instamart list-coupons`** - Swiggy Instamart (Grocery): List available coupons for the current cart
- **`swiggy-pp-cli instamart report-error`** - Generate an error report to share with the Swiggy MCP team
- **`swiggy-pp-cli instamart search-products`** - Search for products available at the selected address
- **`swiggy-pp-cli instamart track-order`** - Track Swiggy Instamart order status in real-time
- **`swiggy-pp-cli instamart update-cart`** - Swiggy Instamart (Grocery): Update Swiggy Instamart grocery cart with items
- **`swiggy-pp-cli instamart your-go-to-items`** - Fetch the user's Your Go To Items (frequently or recently ordered items) for the selected delivery address


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
swiggy-pp-cli dineout get-available-slots --restaurant-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
swiggy-pp-cli dineout get-available-slots --restaurant-id 550e8400-e29b-41d4-a716-446655440000 --json
# Filter to specific fields by name
swiggy-pp-cli dineout get-available-slots --restaurant-id 550e8400-e29b-41d4-a716-446655440000 --json --select <field>[,<field>...]

# Dry run — show the request without sending
swiggy-pp-cli dineout get-available-slots --restaurant-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
swiggy-pp-cli dineout get-available-slots --restaurant-id 550e8400-e29b-41d4-a716-446655440000 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Health Check

```bash
swiggy-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `swiggy-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `SWIGGY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SWIGGY_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `swiggy-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `swiggy-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SWIGGY_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 on every call** — Run `swiggy-pp-cli auth login` again — access tokens expire after 5 days and v1.0 has no refresh token.
- **place-food-order failed with a 5xx and you're unsure if it placed** — Run `swiggy-pp-cli order verify-before-retry` before retrying — place-food-order is not idempotent.
- **429 / rate limited** — Stop retrying immediately; the CLI reuses one persistent session per domain by design, but concurrent multi-domain commands can still add up — space out calls across food/instamart/dineout.
- **UPI payment stuck in PENDING_PAYMENT** — Run `swiggy-pp-cli pay wait --paas-id paas_123 --order-id ord_01HXYZ --domain food` (swap --domain for instamart/dineout as needed) to long-poll to a terminal state instead of polling manually.
