# Shopify CLI

**Operate a Shopify store from the terminal with curated Admin GraphQL commands, local sync, analytics, and bulk exports.**

Endpoint mirrors for orders, products, customers, inventory items, fulfillment orders, and abandoned checkouts. A local SQLite store for offline reads, full-text search, and compound analytics (revenue, funnel, growth, ops risk, merchandising). MCP server with both stdio and HTTP transport so agents (OpenAI Codex CLI, hosted clients) consume the same surface without learning GraphQL.

Learn more at [Shopify](https://shopify.dev/docs/api/admin-graphql).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).
Contributors: [@benjaminn8](https://github.com/benjaminn8) (Benjamin).

## Install

The recommended path installs both the `shopify-pp-cli` binary and the `pp-shopify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install shopify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install shopify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install shopify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install shopify --agent claude-code
npx -y @mvanhorn/printing-press-library install shopify --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopify/cmd/shopify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install shopify --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-shopify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-shopify --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install shopify --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/shopify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SHOPIFY_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopify/cmd/shopify-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "shopify": {
      "command": "shopify-pp-mcp",
      "env": {
        "SHOPIFY_API_VERSION": "2026-04",
        "SHOPIFY_SHOP": "<your-store>.myshopify.com",
        "SHOPIFY_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set SHOPIFY_ACCESS_TOKEN to a custom-app token with the read scopes you need (read_orders, read_products, read_customers, read_inventory, read_fulfillments). Tag mutations (orders tag, customers tag) additionally need write_orders/write_customers.

## Quick Start

```bash
# Verify-safe health check; works without auth.
shopify-pp-cli doctor --dry-run

# Local archive state for all resources before any sync.
shopify-pp-cli workflow status --json

# List recent orders with agent-native JSON.
shopify-pp-cli orders list --first 10 --json

# One-call support summary: order + fulfillment + inventory availability.
shopify-pp-cli orders brief gid://shopify/Order/1234 --json

# Block until the current bulk export reaches a terminal state.
shopify-pp-cli bulk-operations wait --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native composites
- **`orders brief`** — See a single support-ready summary of an order with fulfillment status and inventory availability, without stitching three calls together yourself.

  _Reach for this instead of separate orders/fulfillment-orders/inventory-items calls when answering a support 'where is my order' question._

  ```bash
  shopify-pp-cli orders brief gid://shopify/Order/1234 --agent --json
  ```
- **`bulk-operations wait`** — Block until the current Shopify bulk export finishes instead of hand-polling in a loop.

  _Use after bulk-operations run-query when you need the terminal result rather than a point-in-time snapshot._

  ```bash
  shopify-pp-cli bulk-operations wait --max-wait 10m --json
  ```

### Reachability mitigation
- **`doctor throttle`** — Check remaining GraphQL query-cost budget before a heavy batch or agent session.

  _Run this before a big sync or bulk export to avoid hitting THROTTLED mid-task._

  ```bash
  shopify-pp-cli doctor throttle --json
  ```

### Local state that compounds
- **`store diff`** — See which rows changed content since the last sync, per resource.

  _Use for a weekly 'what changed' review across synced resources instead of re-scanning everything._

  ```bash
  shopify-pp-cli store diff --since 24h --json
  ```

## Recipes

### Confirm a clean install

```bash
shopify-pp-cli doctor --json
```

JSON health output suitable for piping into agent context.

### Support ticket one-shot

```bash
shopify-pp-cli orders brief gid://shopify/Order/1234 --json
```

Order + fulfillment + inventory availability in one call instead of three.

### Narrow a verbose response for agents

```bash
shopify-pp-cli orders list --first 5 --agent --json --select edges.node.id,edges.node.name,edges.node.displayFinancialStatus
```

Pairs --agent with --select dotted paths so agents do not burn context on fields they did not ask for.

### Block on a bulk export

```bash
shopify-pp-cli bulk-operations wait --max-wait 10m --json
```

Blocks until the current bulk operation reaches a terminal state instead of hand-polling.

### Pre-flight budget check

```bash
shopify-pp-cli doctor throttle --json
```

Check remaining GraphQL query-cost budget before a heavy sync or agent session.

## Usage

Run `shopify-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SHOPIFY_CONFIG_DIR`, `SHOPIFY_DATA_DIR`, `SHOPIFY_STATE_DIR`, or `SHOPIFY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SHOPIFY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SHOPIFY_HOME=/srv/shopify
shopify-pp-cli doctor
```

Under `SHOPIFY_HOME=/srv/shopify`, the four dirs resolve to `/srv/shopify/config`, `/srv/shopify/data`, `/srv/shopify/state`, and `/srv/shopify/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "shopify": {
      "command": "shopify-pp-mcp",
      "env": {
        "SHOPIFY_HOME": "/srv/shopify"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SHOPIFY_DATA_DIR` overrides an explicit `--home` for that kind. Use `SHOPIFY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SHOPIFY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `shopify-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### abandoned-checkouts

Shopify abandoned checkouts for recovery campaigns and lost-cart analysis.

- **`shopify-pp-cli abandoned-checkouts <id>`** - Get one Shopify abandoned checkout by GraphQL ID.
- **`shopify-pp-cli abandoned-checkouts`** - List abandoned checkouts from the Shopify Admin GraphQL API.

### customers

Shopify customers with lifetime order count, lifetime spend, and contact fields.

- **`shopify-pp-cli customers <id>`** - Get one Shopify customer by GraphQL ID.
- **`shopify-pp-cli customers`** - List customers from the Shopify Admin GraphQL API.

### fulfillment-orders

Shopify fulfillment orders for lag, routing, and fulfillment-state analysis.

- **`shopify-pp-cli fulfillment-orders <id>`** - Get one Shopify fulfillment order by GraphQL ID.
- **`shopify-pp-cli fulfillment-orders`** - List fulfillment orders from the Shopify Admin GraphQL API.

### inventory-items

Shopify inventory items with tracked status and available quantities by location.

- **`shopify-pp-cli inventory-items <id>`** - Get one Shopify inventory item by GraphQL ID.
- **`shopify-pp-cli inventory-items`** - List inventory items from the Shopify Admin GraphQL API.

### orders

Shopify orders with money totals, financial state, and fulfillment state.

- **`shopify-pp-cli orders <id>`** - Get one Shopify order by GraphQL ID.
- **`shopify-pp-cli orders`** - List orders from the Shopify Admin GraphQL API.

### products

Shopify products with product status, catalog metadata, and a compact variant inventory projection.

- **`shopify-pp-cli products <id>`** - Get one Shopify product by GraphQL ID.
- **`shopify-pp-cli products`** - List products from the Shopify Admin GraphQL API.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`shopify-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`shopify-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`shopify-pp-cli learnings list`** - Inspect taught rows
- **`shopify-pp-cli learnings forget <query>`** - Undo a teach
- **`shopify-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`shopify-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`shopify-pp-cli teach-pattern`** - Install a query/resource template up front
- **`shopify-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SHOPIFY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `shopify-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
shopify-pp-cli abandoned-checkouts list

# JSON for scripting and agents
shopify-pp-cli abandoned-checkouts list --json

# Filter to specific fields
shopify-pp-cli abandoned-checkouts list --json --select id,name,status

# Dry run — show the request without sending
shopify-pp-cli abandoned-checkouts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
shopify-pp-cli abandoned-checkouts list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `SHOPIFY_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `shopify-pp-cli abandoned-checkouts`
- `shopify-pp-cli abandoned-checkouts get`
- `shopify-pp-cli abandoned-checkouts list`
- `shopify-pp-cli abandoned-checkouts search`
- `shopify-pp-cli customers`
- `shopify-pp-cli customers get`
- `shopify-pp-cli customers list`
- `shopify-pp-cli customers search`
- `shopify-pp-cli fulfillment-orders`
- `shopify-pp-cli fulfillment-orders get`
- `shopify-pp-cli fulfillment-orders list`
- `shopify-pp-cli fulfillment-orders search`
- `shopify-pp-cli inventory-items`
- `shopify-pp-cli inventory-items get`
- `shopify-pp-cli inventory-items list`
- `shopify-pp-cli inventory-items search`
- `shopify-pp-cli orders`
- `shopify-pp-cli orders get`
- `shopify-pp-cli orders list`
- `shopify-pp-cli orders search`
- `shopify-pp-cli products`
- `shopify-pp-cli products get`
- `shopify-pp-cli products list`
- `shopify-pp-cli products search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `SHOPIFY_API_VERSION` resolves `{api_version}`
- `SHOPIFY_SHOP` resolves `{shop}`

Base URL: `https://{shop}`

GraphQL path: `/admin/api/{api_version}/graphql.json`

## Health Check

```bash
shopify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `shopify-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/shopify-pp-cli/config.toml`; `--home`, `SHOPIFY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SHOPIFY_API_VERSION` | endpoint | Yes |  |
| `SHOPIFY_SHOP` | endpoint | Yes |  |
| `SHOPIFY_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `shopify-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `shopify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SHOPIFY_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Verify SHOPIFY_ACCESS_TOKEN is an Admin API token, not a Storefront API token. Re-issue under custom-app settings.
- **Empty results from local search or analytics** — Run shopify-pp-cli sync --full first; analytics and search read from the local SQLite store.
- **THROTTLED error mid-task** — Run shopify-pp-cli doctor throttle --json before a heavy batch or agent session to check remaining query-cost budget.
