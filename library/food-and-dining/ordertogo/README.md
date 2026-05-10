# OrderToGo CLI

**Browse, cart, and place orders at any OrderToGo.com restaurant from the terminal — pure-Go agent-native client with a hard budget gate and headless Braintree drive.**

OrderToGo.com is a multi-tenant pickup ordering platform that powers small chains across multiple US metros. There is no CLI, MCP, or SDK presence anywhere for it. This CLI gives an agent every read endpoint, a safe `order plan` that composes a cart locally and validates against a `--max` cap, and an `order place` that drives a headless Chrome through Braintree DropIn to actually submit. Everything is offline-cacheable, agent-callable, and verify-env safe by construction.

Learn more at [OrderToGo](https://www.ordertogo.com).

## Install

The recommended path installs both the `ordertogo-pp-cli` binary and the `pp-ordertogo` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install ordertogo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install ordertogo --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ordertogo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ordertogo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ordertogo --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-ordertogo skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-ordertogo. The skill defines how its required CLI can be installed.
```

## Authentication

OrderToGo authenticates via Firebase phone-OTP on the web. The CLI does not implement phone OTP — instead, `auth login --chrome` imports your existing OrderToGo session cookies from your local Chrome profile (default user-data-dir, default profile). After import, every command travels with the cookie; refresh by re-running `auth login --chrome` if your session expires.

## Quick Start

```bash
# Import OrderToGo session cookies from Chrome - no password, no OTP from the CLI
ordertogo-pp-cli auth login --chrome


# Pull full order history once so 'usual' detection and analytics work offline
ordertogo-pp-cli sync --resources orders


# Optional - by default the CLI uses your most-frequent restaurant from the last 30 days
ordertogo-pp-cli config set default_restaurant mixsushibarlin


# Confirm the most recent order - --reuse-last will recompose this exact cart
ordertogo-pp-cli last-order --json


# Recompose, hit /m/api/orders for tax, assert total <= cap before any payment surface opens
ordertogo-pp-cli orders plan --reuse-last --max 30 --json


# Drive headless Chrome through Braintree with your saved card; only fires after the budget gate has already passed
ordertogo-pp-cli orders place --reuse-last --confirm --max 30

# Verification-safe checkout rehearsal for agents and CI
PRINTING_PRESS_VERIFY=1 ordertogo-pp-cli orders place --reuse-last --confirm --max 30

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Safe agent-driven ordering
- **`order plan`** — Recompose your previous order locally, validate against tax, and refuse to fire if total exceeds your --max cap.

  _Reach for this when an agent needs to know 'will this fit my budget' before committing — answer comes from one structured call with no browser involvement._

  ```bash
  ordertogo-pp-cli order plan --reuse-last --max 30 --json
  ```
- **`order place`** — Drive a headless Chrome via chromedp to complete the Braintree DropIn flow with your saved card, with the budget cap enforced before the browser opens.

  _Use when an agent has a budget-validated plan and the user has confirmed; this is the one command that actually moves money._

  ```bash
  ordertogo-pp-cli order place --reuse-last --confirm --max 30
  ```

### Local state that compounds
- **`usual`** — Cluster your historical orders by item-set similarity and surface the recurring set that defines 'your usual' at any restaurant you order from.

  _Reach for this when a user says 'order my usual' and the agent needs to decide whether one obvious pattern exists or whether to ask which usual._

  ```bash
  ordertogo-pp-cli usual --restaurant <slug> --json
  ```
- **`spending`** — Total spent, average order, days since last order, top items, and weekly cadence — all from local order history with one SQL query.

  _Use when an agent needs to answer 'how much have I spent here' or 'how often do I order' without re-fetching history._

  ```bash
  ordertogo-pp-cli spending --since 90d --json
  ```
- **`order plan`** — Pass --tip auto and the CLI applies your average tip percentage from history at this restaurant.

  _Reach for this when an agent doesn't want to make the user pick a tip percentage — go with what's habitual._

  ```bash
  ordertogo-pp-cli order plan --reuse-last --tip auto --json
  ```

### Reachability mitigation
- **`order plan`** — Refuse to fire if the restaurant is not currently open at the requested pickup time, using cached hours.

  _Catches a class of agent failures before the payment flow opens._

  ```bash
  ordertogo-pp-cli order plan --reuse-last --pickup-at '7:00 PM'
  ```

### Agent-native plumbing
- **`agent-context`** — Single-call structured dump: account, default restaurant, your usual, last-order summary, budget hint, days-since-last.

  _Reach for this when an agent enters a session and needs a complete picture before suggesting any action._

  ```bash
  ordertogo-pp-cli agent-context --json
  ```
- **`order place`** — All side-effect commands short-circuit when PRINTING_PRESS_VERIFY=1 is set, printing 'would place: <plan>' instead of submitting.

  _Use when an agent or test harness wants to exercise the place path without real-world consequences._

  ```bash
  PRINTING_PRESS_VERIFY=1 ordertogo-pp-cli order place --reuse-last --confirm --max 30
  ```

## Usage

Run `ordertogo-pp-cli --help` for the full command reference and flag list.

## Commands

### coupons

Promotional coupons available to your account

- **`ordertogo-pp-cli coupons list`** - List active coupons for your account (endpoint shape inferred from web 'My Coupons' panel)
- **`ordertogo-pp-cli coupons mark_used`** - Mark a promotion code as used after applying it to an order

### giftcards

Giftcard balances and history per restaurant

- **`ordertogo-pp-cli giftcards list`** - List your giftcards across all restaurants (endpoint shape inferred from web 'My Giftcards' panel)

### notifications

Notification badge for orders, rewards, and platform messages

- **`ordertogo-pp-cli notifications unread_count`** - Count of unread notifications for your account

### orders

Order history, detail, validation, and tracking - the core ordering data path

- **`ordertogo-pp-cli orders cancel`** - Cancel your own order within the void window (typically before preparing-state)
- **`ordertogo-pp-cli orders list`** - List your order history across all restaurants (returns latest N orders, server-paginated)
- **`ordertogo-pp-cli orders show`** - Get order detail by orderid - items, options, totals, payment method, points earned, status timeline
- **`ordertogo-pp-cli orders track`** - HTML order tracking page (received → preparing → ready → picked up). Parsed for status by `order track`.
- **`ordertogo-pp-cli orders validate`** - Pre-validate a cart - returns an order token plus tax computation, used by `order plan` before any payment surface opens
- **`ordertogo-pp-cli orders plan`** - Reuse a previous order or item list, validate tax and tip, and save the active cart behind a budget gate
- **`ordertogo-pp-cli orders place`** - Drive Chrome through checkout for the active cart after explicit confirmation and max-budget validation

### payment

Braintree client token for payment-method nonce generation (used internally by chromedp headless flow)

- **`ordertogo-pp-cli payment braintree_token`** - Returns a Braintree client token used by the DropIn UI to mint a single-use payment nonce. Hand-driven by chromedp during `order place`.
- **`ordertogo-pp-cli payment checkout`** - Submit order with payment nonce and customer details. Body must include `nonce` from Braintree client. The CLI uses chromedp to obtain the nonce; this endpoint is not directly callable from a Go CLI without driving the Braintree client SDK.

### restaurants

Restaurants on the OrderToGo platform - filter by location, view detail, list multi-location chains

- **`ordertogo-pp-cli restaurants list`** - List restaurants in a location code (e.g. sto for Seattle area)
- **`ordertogo-pp-cli restaurants menu`** - Full menu for a restaurant with categories, items, and modifier options
- **`ordertogo-pp-cli restaurants show`** - Show restaurant detail - hours, address, phone, location code, multi-location chain mapping

### rewards

Reward points balances per restaurant

- **`ordertogo-pp-cli rewards list`** - List your reward points across all restaurants (endpoint shape inferred from web 'My Rewards' panel)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ordertogo-pp-cli coupons list

# JSON for scripting and agents
ordertogo-pp-cli coupons list --json

# Filter to specific fields
ordertogo-pp-cli coupons list --json --select id,name,status

# Dry run — show the request without sending
ordertogo-pp-cli coupons list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ordertogo-pp-cli coupons list --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-ordertogo -g
```

Then invoke `/pp-ordertogo <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
# Some tools work without auth. For full access, set up auth first:
ordertogo-pp-cli auth login --chrome

claude mcp add ordertogo ordertogo-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
ordertogo-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ordertogo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ordertogo": {
      "command": "ordertogo-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
ordertogo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ordertogo-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ordertogo-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth login --chrome can't find cookies** — Pass --chrome-profile 'Profile 1' or --user-data-dir <path> if you use a non-default Chrome profile
- **order place fails inside Braintree DropIn (saved card not selected)** — Re-run with --visible to drop into a visible Chrome window and tap Place Order yourself; the CLI still confirms via order-history polling
- **Restaurant returns HTTP 498 closed** — Run `restaurants show <slug> --closed-check` first; the CLI refuses to plan against a closed restaurant after a sync of hours
- **order place returns 'budget exceeded'** — Either raise --max or pass --confirm-over-budget; the CLI never silently exceeds the cap
- **Stale menu items in usual** — Run `sync --menu <slug>` to refresh the menu cache before composing
- **No restaurants show up in `restaurants list`** — Pass --location <code> (e.g. sto for Seattle area, det for Detroit) — the CLI doesn't assume a metro and reads location codes from your synced order history

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
