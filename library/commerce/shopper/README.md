# Shopper CLI

**The first CLI for Shopper — every catalog, cart, and delivery surface plus a local price/basket history the web app throws away each cycle.**

Shopper's recurring basket, fixed charge-7-days-before clock, and drifting prices produce a time series the official app discards every cycle. This CLI keeps it in a local SQLite store, unlocking charge-calendar, basket diff, price-watch, restock prediction, catalog-drift detection, and cashback optimization — none of which any Shopper interface offers.

Learn more at [Shopper](https://shopper.com.br).

Created by [@educrvz](https://github.com/educrvz) (educrvz).

## Install

The recommended path installs both the `shopper-pp-cli` binary and the `pp-shopper` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install shopper
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install shopper --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install shopper --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install shopper --agent claude-code
npx -y @mvanhorn/printing-press-library install shopper --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopper-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-shopper --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-shopper --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-shopper skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-shopper. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopper-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SHOPPER_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "shopper": {
      "command": "shopper-pp-mcp",
      "env": {
        "SHOPPER_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Shopper uses a Bearer JWT. Sign in at shopper.com.br in your browser, copy the token from the Authorization header of any siteapi.shopper.com.br request (DevTools > Network), and set SHOPPER_TOKEN. The CLI also sends app-os-x-version and your store context automatically.

## Quick Start

```bash
# Health check; verifies token + store context without a live call.
shopper doctor --dry-run

# Search the catalog — the most common first action.
shopper catalog search arroz --agent

# See your current recurring basket total and item count.
shopper cart summary --agent

# See upcoming charge dates and edit deadlines.
shopper charge-calendar --weeks 8 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Time & Money Clock
- **`charge-calendar`** — Every upcoming cycle's charge date, edit-lock deadline, and delivery date in one timeline so you never miss an edit window or get surprised by a charge.

  _Reach for this before any basket edit to answer 'can I still change the order / when does money leave the account'._

  ```bash
  shopper charge-calendar --weeks 8 --locking-soon --agent
  ```

### Basket Intelligence
- **`basket diff`** — Compares your current recurring basket against a previous cycle's snapshot to show exactly what was added, dropped, or re-quantified before the template locks.

  _Reach for this to audit what changed before confirming a cycle, or to explain why this charge differs from the last._

  ```bash
  shopper basket diff --from last-cycle --to current --agent --select added,removed,price_changed
  ```
- **`restock predict`** — Predicts when you'll run out of each staple from your historical buying cadence and suggests what to add to the upcoming basket.

  _Reach for this to proactively fill the next basket so the household doesn't run out of cafe/arroz/fralda mid-cycle._

  ```bash
  shopper restock predict --horizon 14d --suggest-adds --agent
  ```

### Price & Catalog Drift
- **`price-watch`** — Tracks the price history of the SKUs you actually buy and alerts when one rises or drops meaningfully versus your own purchase baseline.

  _Reach for this before confirming a cycle to catch staples that quietly got pricier, or find real drops worth stocking up on._

  ```bash
  shopper price-watch --threshold 8% --since 60d --only-basket --agent
  ```
- **`catalog drift`** — Flags products you buy that were discontinued, silently swapped, or kept their price while shrinking the pack, surfacing the real R$/kg or R$/L change.

  _Reach for this when the user asks 'why is my bill the same but I have less', or to auto-find replacements for staples that vanished._

  ```bash
  shopper catalog drift --metric per-unit --kind shrinkflation,discontinued --since 90d --agent
  ```

### Cashback Optimization
- **`cashback optimize`** — Computes the cheapest set of items to add (or whether to wait) to cross the next cashback tier, favoring things you'll need anyway.

  _Reach for this near the edit deadline to decide whether topping up the basket earns net-positive cashback without buying junk._

  ```bash
  shopper cashback optimize --tier 2399 --reward 100 --prefer-restock --agent
  ```

## Recipes


### Find a staple and check your price baseline

```bash
shopper catalog search 'cafe pilao' --agent --select results.name,results.price && shopper price-watch --only-basket --threshold 5%
```

Search the catalog, then see whether your tracked staples drifted in price.

### Pre-deadline cycle review

```bash
shopper charge-calendar --locking-soon --agent && shopper basket diff --from last-cycle --to current --agent
```

Check which cycle locks soon and audit what changed before it does.

### Hit the cashback tier efficiently

```bash
shopper cashback optimize --tier 2399 --reward 100 --prefer-restock --agent
```

Decide the cheapest useful top-up to earn cashback.

## Usage

Run `shopper-pp-cli --help` for the full command reference and flag list.

## Commands

### address

Operations on address

- **`shopper-pp-cli address`** - GET /address/

### cart

Operations on summary

- **`shopper-pp-cli cart add`** - Add a product to the cart or increase its quantity
- **`shopper-pp-cli cart summary`** - GET /cart/summary
- **`shopper-pp-cli cart remove`** - Remove a product from the cart or decrease its quantity

### catalog

Operations on departments

- **`shopper-pp-cli catalog count`** - POST /catalog/search/count
- **`shopper-pp-cli catalog filters`** - POST /catalog/search/filters
- **`shopper-pp-cli catalog search`** - POST /catalog/search
- **`shopper-pp-cli catalog banner-view`** - GET /catalog/banners/{banner_id}/view
- **`shopper-pp-cli catalog banners`** - GET /catalog/banners
- **`shopper-pp-cli catalog departments`** - GET /catalog/departments
- **`shopper-pp-cli catalog news`** - GET /catalog/products/news
- **`shopper-pp-cli catalog suggest`** - GET /catalog/search/suggest

### delivery

Operations on summary

- **`shopper-pp-cli delivery calendar`** - GET /delivery/v2/calendar
- **`shopper-pp-cli delivery summary`** - GET /delivery/summary

### features

Operations on toggle

- **`shopper-pp-cli features select`** - POST /features/stores/select
- **`shopper-pp-cli features start`** - POST /features/timer/start
- **`shopper-pp-cli features view`** - POST /features/toggle/view
- **`shopper-pp-cli features stores`** - GET /features/stores
- **`shopper-pp-cli features tick`** - GET /features/timer/tick
- **`shopper-pp-cli features toggle`** - GET /features/toggle

### session

Session and social-login validation

- **`shopper-pp-cli session`** - GET /auth/validation/social


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
shopper-pp-cli address

# JSON for scripting and agents
shopper-pp-cli address --json

# Filter to specific fields
shopper-pp-cli address --json --select id,name,status

# Dry run — show the request without sending
shopper-pp-cli address --dry-run

# Agent mode — JSON + compact + no prompts in one flag
shopper-pp-cli address --agent
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

## Health Check

```bash
shopper-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/shopper-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SHOPPER_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `shopper-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `shopper-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SHOPPER_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 AUTH008 No Token** — Set SHOPPER_TOKEN to a fresh Bearer JWT from a logged-in browser session (DevTools > Network > any siteapi request).
- **Empty catalog/cart results** — Select a store first: shopper features select <id>; the store context (x-store-id) is required for catalog and cart.
