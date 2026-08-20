# Shopper CLI

**Every Shopper storefront in one CLI — catalog, cart, delivery schedule, charge calendar, and spend analytics no web UI surfaces.**

shopper-pp-cli covers all six Shopper storefronts (Compra Programada, Fresh, Pet, Compra Única, Now, Now Bebidas) with correct store/cluster scoping, full siteapi REST surface, browser-deep-link helpers for subscription mutations, and a local SQLite layer for offline product search, basket diffs, price tracking, and cross-store spend rollup.

Learn more at [Shopper](https://siteapi.shopper.com.br).

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

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopper-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install shopper --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-shopper --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-shopper --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install shopper --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

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

Set SHOPPER_TOKEN to your Shopper JWT (from the siteapi Authorization header in browser DevTools). Run 'shopper-pp-cli doctor' to verify. The token is long-lived for the subscription cycle. Never store raw card numbers or CPF in config; card management always opens the browser.

## Quick Start

```bash
# Verify CLI is installed and config path is set before auth
shopper-pp-cli doctor --dry-run

# Discover all six storefronts with IDs and payment parameters
shopper-pp-cli stores

# See current basket total, cashback tier, and minimum-order status
shopper-pp-cli cart list-summary --store programada

# See charge date, edit-lock deadline, and delivery date for the next cycle
shopper-pp-cli charge-calendar --store programada

# Month-over-month actual spend rolled up across all storefronts
shopper-pp-cli orders spend

# Full pre-checkout summary before opening browser to confirm payment
shopper-pp-cli checkout preview --store programada

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Subscription intelligence
- **`charge-calendar`** — Every upcoming cycle's charge date, edit-lock deadline, and delivery date in one timeline so you never miss an edit window or get surprised by a charge.

  _Use when an agent needs to know whether the edit window is still open before modifying a recurring basket._

  ```bash
  shopper-pp-cli charge-calendar --store programada --agent
  ```
- **`basket diff`** — Compares your current recurring basket against a previous cycle's snapshot to show exactly what was added, dropped, or re-quantified before the template locks.

  _Use when an agent needs to verify what changed in the basket since the last confirmed delivery cycle._

  ```bash
  shopper-pp-cli basket diff --store programada --agent
  ```
- **`cashback optimize`** — Computes the cheapest set of items to add (or whether to wait) to cross the next cashback tier, favoring things you'll need anyway.

  _Use when an agent is finalising a basket and wants to maximise cashback return before the edit window closes._

  ```bash
  shopper-pp-cli cashback optimize --tier 2399 --store programada --agent
  ```

### Local state that compounds
- **`price-watch`** — Tracks the price history of the SKUs you actually buy and alerts when one rises or drops meaningfully versus your own purchase baseline.

  _Use when an agent needs to know if a subscribed product has changed price significantly since the last order._

  ```bash
  shopper-pp-cli price-watch --store programada --agent
  ```
- **`restock predict`** — Predicts when you'll run out of each staple from your historical buying cadence and suggests what to add to the upcoming basket.

  _Use when an agent needs to pre-populate a recurring basket with items likely running low before the next cycle._

  ```bash
  shopper-pp-cli restock predict --store programada --agent
  ```
- **`catalog drift`** — Flags products you buy that were discontinued, silently swapped, or kept their price while shrinking the pack, surfacing the real R$/kg or R$/L change.

  _Use when an agent needs to audit whether the recurring basket still contains the same products as originally added._

  ```bash
  shopper-pp-cli catalog drift --store programada --agent
  ```

### Customer journey plumbing
- **`checkout preview`** — Aggregates cart totals, next delivery date, charge date, minimum-order status, and accepted payment types into one pre-checkout view before you open the browser.

  _Use when an agent needs to confirm basket readiness and charge schedule before directing the user to open the browser for final payment._

  ```bash
  shopper-pp-cli checkout preview --store programada --agent
  ```

## Recipes

### Check if the edit window is still open

```bash
shopper-pp-cli charge-calendar --store programada --agent --select next_delivery_date,edit_lock_date,charge_date
```

Returns the key dates for the next cycle; compare edit_lock_date to today to know if the basket can still be changed.

### Find items to add for next cashback tier

```bash
shopper-pp-cli cashback optimize --store programada --agent
```

Computes the cheapest catalog additions to cross the next cashback threshold using your live cart and synced product data.

### Cross-store spend rollup for last 12 months

```bash
shopper-pp-cli orders spend --agent --select store,month,total
```

Queries all 6 storefronts and returns a month-by-store spend matrix for budgeting analysis.

### Detect if a subscribed product changed price

```bash
shopper-pp-cli price-watch --store programada --agent --select product_id,name,price_now,price_baseline,change_pct
```

Scans synced price history for products in your basket and flags meaningful price movements.

### Pre-checkout summary with charge date

```bash
shopper-pp-cli checkout preview --store programada --agent --select total_amount,delivery_date,charge_date,min_order_met,payment_methods
```

Aggregates cart/delivery/payment data so an agent can confirm basket readiness before directing the user to open the checkout browser.

## Usage

Run `shopper-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SHOPPER_CONFIG_DIR`, `SHOPPER_DATA_DIR`, `SHOPPER_STATE_DIR`, or `SHOPPER_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SHOPPER_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SHOPPER_HOME=/srv/shopper
shopper-pp-cli doctor
```

Under `SHOPPER_HOME=/srv/shopper`, the four dirs resolve to `/srv/shopper/config`, `/srv/shopper/data`, `/srv/shopper/state`, and `/srv/shopper/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "shopper": {
      "command": "shopper-pp-mcp",
      "env": {
        "SHOPPER_HOME": "/srv/shopper"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SHOPPER_DATA_DIR` overrides an explicit `--home` for that kind. Use `SHOPPER_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SHOPPER_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `shopper-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### address

Saved delivery addresses with per-address available-store information

- **`shopper-pp-cli address`** - List delivery addresses and which stores are available at each address

### cart

Cart: view summary, add products, remove products

- **`shopper-pp-cli cart add`** - Add a product to the cart or increase its quantity
- **`shopper-pp-cli cart list-summary`** - Show current basket: items, quantities, totals, cashback, and minimum-order status
- **`shopper-pp-cli cart remove`** - Remove a product from the cart or decrease its quantity

### catalog

Product catalog: search, departments, banners, suggestions

- **`shopper-pp-cli catalog create-count`** - Count products matching a search query and optional filters
- **`shopper-pp-cli catalog create-filters`** - Get available filter options for a search query
- **`shopper-pp-cli catalog create-search`** - Search the product catalog by query with optional brand/type/metadata filters
- **`shopper-pp-cli catalog get-view`** - Get details for a specific catalog banner
- **`shopper-pp-cli catalog list-banners`** - List promotional banners for the current store
- **`shopper-pp-cli catalog list-departments`** - List product departments/categories for the current store
- **`shopper-pp-cli catalog list-news`** - List new product arrivals for the current store
- **`shopper-pp-cli catalog list-suggest`** - Get search suggestions for a query prefix

### delivery

Delivery schedule: upcoming delivery date, edit-lock window, and reschedule calendar

- **`shopper-pp-cli delivery list-calendar`** - Get delivery reschedule calendar — allowed date range and disabled days
- **`shopper-pp-cli delivery list-summary`** - Show scheduled delivery date, current delivery status, and store message

### features

Storefront configuration, feature toggles, and timer state

- **`shopper-pp-cli features create-select`** - Select active store (sets session context; no-op for header-scoped reads)
- **`shopper-pp-cli features create-start`** - Start a named feature timer
- **`shopper-pp-cli features create-view`** - Mark a feature toggle as viewed
- **`shopper-pp-cli features list-stores`** - List all available storefronts with store IDs, cluster IDs, payment parameters, and feature flags
- **`shopper-pp-cli features list-tick`** - Get current timer state
- **`shopper-pp-cli features list-toggle`** - Get active feature toggles for the current store

### orders

Purchase history and spend — reads from GET /orders/orders (web 'Histórico de compras')

- **`shopper-pp-cli orders`** - List past orders for the active store (newest-first, paginated by size)

### session

Session and social-login validation

- **`shopper-pp-cli session`** - Validate social-login session status


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`shopper-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`shopper-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`shopper-pp-cli learnings list`** - Inspect taught rows
- **`shopper-pp-cli learnings forget <query>`** - Undo a teach
- **`shopper-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`shopper-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`shopper-pp-cli teach-pattern`** - Install a query/resource template up front
- **`shopper-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SHOPPER_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `shopper-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SHOPPER_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `shopper-pp-cli address`
- `shopper-pp-cli address get`
- `shopper-pp-cli address list`
- `shopper-pp-cli address search`
- `shopper-pp-cli cart`
- `shopper-pp-cli cart get`
- `shopper-pp-cli cart list`
- `shopper-pp-cli cart search`
- `shopper-pp-cli catalog`
- `shopper-pp-cli catalog get`
- `shopper-pp-cli catalog list`
- `shopper-pp-cli catalog search`
- `shopper-pp-cli catalog-departments`
- `shopper-pp-cli catalog-departments get`
- `shopper-pp-cli catalog-departments list`
- `shopper-pp-cli catalog-departments search`
- `shopper-pp-cli catalog-products-news`
- `shopper-pp-cli catalog-products-news get`
- `shopper-pp-cli catalog-products-news list`
- `shopper-pp-cli catalog-products-news search`
- `shopper-pp-cli catalog-search-suggest`
- `shopper-pp-cli catalog-search-suggest get`
- `shopper-pp-cli catalog-search-suggest list`
- `shopper-pp-cli catalog-search-suggest search`
- `shopper-pp-cli delivery`
- `shopper-pp-cli delivery get`
- `shopper-pp-cli delivery list`
- `shopper-pp-cli delivery search`
- `shopper-pp-cli delivery-v2-calendar`
- `shopper-pp-cli delivery-v2-calendar get`
- `shopper-pp-cli delivery-v2-calendar list`
- `shopper-pp-cli delivery-v2-calendar search`
- `shopper-pp-cli features`
- `shopper-pp-cli features get`
- `shopper-pp-cli features list`
- `shopper-pp-cli features search`
- `shopper-pp-cli features-timer-tick`
- `shopper-pp-cli features-timer-tick get`
- `shopper-pp-cli features-timer-tick list`
- `shopper-pp-cli features-timer-tick search`
- `shopper-pp-cli features-toggle`
- `shopper-pp-cli features-toggle get`
- `shopper-pp-cli features-toggle list`
- `shopper-pp-cli features-toggle search`
- `shopper-pp-cli orders`
- `shopper-pp-cli orders get`
- `shopper-pp-cli orders list`
- `shopper-pp-cli orders search`
- `shopper-pp-cli session`
- `shopper-pp-cli session get`
- `shopper-pp-cli session list`
- `shopper-pp-cli session search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
shopper-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `shopper-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/shopper-pp-cli/config.toml`; `--home`, `SHOPPER_HOME`, and per-kind env vars can relocate it.

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
- **401 Unauthorized on any command** — Run 'shopper-pp-cli auth set-token <token>' with a fresh JWT from browser DevTools → Network → Authorization header
- **cart list-summary returns wrong store data** — Pass --store explicitly: programada, fresh, unica, pet, now, or now-bebidas. The default is programada.
- **unica/pet orders missing from 'orders spend'** — orders spend queries all stores by default; if a store shows no data, your account has no orders there
- **delivery calendar shows no available dates** — Run --store with a subscription store (programada/fresh/pet). now/now-bebidas use a different ultra-fast delivery flow.
- **checkout open or delivery reschedule shows wrong store URL** — Pass --store explicitly to get the correct storefront URL (e.g. --store fresh opens fresh.shopper.com.br)
