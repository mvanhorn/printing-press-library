# Monarch Money CLI

**A single Go binary for Monarch Money with offline SQLite, agent-native JSON, and local analytics no other Monarch tool ships.**

monarch-pp-cli wraps the Monarch GraphQL surface as agent-friendly Cobra commands and adds eleven novel commands — budget burn, net-worth delta attribution, subscription drift detection, cashflow forecasting, agent context bundles, and more — that no Monarch competitor offers. Authenticate by capturing a session token from your Monarch web app and saving it via `auth set-token`.

## Install

The recommended path installs both the `monarch-pp-cli` binary and the `pp-monarch` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install monarch
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install monarch --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/monarch-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-monarch --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-monarch --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-monarch skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-monarch. The skill defines how its required CLI can be installed.
```

## Authentication

Monarch has no public API. Capture a session token from your logged-in Monarch web app (Chrome DevTools → Application → Cookies → look for the Authorization Token cookie/header) and pass it to `monarch-pp-cli auth set-token <token>`. Or set `MONARCH_TOKEN` in your environment. Tokens last several months. Interactive `auth login --chrome` cookie import and `--email --password --mfa` headless login are tracked as v0.2 work; until then the env-var / set-token path is the supported flow.

## Quick Start

```bash
# Save your captured Monarch session token to the local config (one-time).
monarch-pp-cli auth set-token YOUR_MONARCH_TOKEN


# Verify auth and reachability before issuing real commands.
monarch-pp-cli doctor


# Emit one JSON blob covering net worth, cashflow, budgets, and upcoming bills — pipe to Claude.
monarch-pp-cli snapshot --json


# Show which budgets are projected to overshoot before month-end.
monarch-pp-cli budgets burn --json


# Find subscriptions whose price quietly changed.
monarch-pp-cli recurring drift --threshold 5pct


# Filtered + projected list — agent-shaped output.
monarch-pp-cli transactions list --search 'whole foods' --start-date 2026-04-01 --json --select date,amount,merchant.name,category.name

```

## Known Gaps

These limitations were observed during Phase 5 live acceptance and are tracked for v0.2:

- **Three mutations are wired with best-guess GraphQL strings but not live-verified.** `goals create`, `goals delete`, and `transactions splits-set` will dry-run cleanly but may return HTTP 400 from input-shape mismatch. Their input objects (`CreateGoalV2MutationInput`, `UpdateTransactionSplitMutationInput`) couldn't be discovered through external probing because Monarch's GraphQL has introspection disabled and its error messages don't name missing fields. Capturing one HAR per mutation from `app.monarch.com`'s DevTools Network tab will resolve each.
- **`reports configurations`, `reports data`, and `import` still POST `{}` to `/graphql`** and return HTTP 400. Their schema shape is opaque without introspection and they are niche commands not exercised by the novel-feature workflows. Use the typed `cashflow summary`, `cashflow by-category`, and `transactions summary` commands instead for reporting needs.
- **No interactive `auth login` flow.** Cookie import (`auth login --chrome`) and headless `--email --password --mfa` login are tracked as v0.2. Until then, capture a session token from your logged-in Monarch web app and use `monarch-pp-cli auth set-token <token>` or set `MONARCH_TOKEN` in your environment.

## Unique Features

These capabilities aren't available in any other tool for this API.

### Categorize at scale
- **`transactions categorize-bulk`** — Match transactions by merchant pattern + amount, apply a category, optionally promote the predicate to a server-side rule, and backfill historical matches in one call.

  _When a user says 'all my Whole Foods runs under $40 are groceries', this is the one command that fixes 80 weeks of past misfires AND prevents future ones._

  ```bash
  monarch-pp-cli transactions categorize-bulk --merchant 'Whole Foods' --amount-lt 40 --category Groceries --create-rule --backfill 90d --dry-run
  ```
- **`transactions categorize-next`** — Emit the oldest Uncategorized transaction with a deterministic top-1 category suggestion derived from the local merchant×category histogram. No LLM.

  _Turns the weekly categorize ritual into a tight pipe of suggestion → confirm → next._

  ```bash
  monarch-pp-cli transactions categorize-next --json
  ```

### Local cross-table joins
- **`networth explain`** — Decompose the net-worth change over a window into income, spending, market gain/loss, and transfers — the four numbers Monarch's chart hides behind a single line.

  _When the user asks 'why did my net worth jump $4,200 this week?', the answer is one command instead of an hour of spreadsheet work._

  ```bash
  monarch-pp-cli networth explain --since 7d --json
  ```
- **`budgets burn`** — For each active budget, compute spend-per-day vs allocation-per-day, project month-end position, and flag categories on track to overshoot.

  _Tells you on day 18 that groceries will overrun by $120, while there's still time to course-correct._

  ```bash
  monarch-pp-cli budgets burn --json --select category,projected_overshoot,days_remaining
  ```
- **`categories leaks`** — Find merchants whose transactions split across 3+ categories (likely miscategorized) and Uncategorized merchants with prior labeled history.

  _Surfaces the cleanup work that compound over years of inconsistent tagging._

  ```bash
  monarch-pp-cli categories leaks --json
  ```

### Local time-series analytics
- **`recurring drift`** — Detect recurring streams whose latest charge deviates from the trailing 6-month median by more than a threshold (default 5%).

  _Catches the silent Netflix-going-from-$15-to-$22 bump months before the user would notice scrolling through statements._

  ```bash
  monarch-pp-cli recurring drift --threshold 5pct --json
  ```
- **`cashflow forecast`** — Walk current balances forward N days through scheduled recurring streams and a trailing-90-day average of non-recurring outflow; emit per-day projected balance.

  _Answers Wednesday's 'will Friday's rent clear?' without manual spreadsheet math._

  ```bash
  monarch-pp-cli cashflow forecast --days 30 --json
  ```
- **`goals pace`** — Compute trailing-90-day contribution velocity per goal and project completion date vs target.

  _Tells the FIRE-tracker whether their savings rate is actually trending toward 2032 or 2035._

  ```bash
  monarch-pp-cli goals pace --json
  ```

### Reachability mitigation
- **`accounts stale`** — Surface accounts whose last_synced is past a threshold, grouped by institution, with severity sorted by financial impact (big-balance accounts first).

  _Tells you before you trust a net-worth number which accounts are reporting yesterday's data and which are still 2024._

  ```bash
  monarch-pp-cli accounts stale --threshold 24h --json
  ```

### Agent-native plumbing
- **`snapshot`** — Emit one JSON blob composed of net-worth-by-type, MTD cashflow, top-10 categories, active budget burn, next-7-day recurring, and last-7-day transactions — shaped for LLM consumption.

  _One pipe to claude lets an agent reason over the user's full financial state without losing structure to CSV exports._

  ```bash
  monarch-pp-cli snapshot --json
  ```
- **`cashflow monthly-memo`** — Aggregate one month's transactions, budgets, net-worth-snapshots, and recurring data into a structured packet (top categories, MoM deltas, budget hits/misses, biggest charges, NW delta) ready for an LLM narrative draft.

  _Turns 'write me my monthly money summary' from a 30-minute spreadsheet pull into a single command._

  ```bash
  monarch-pp-cli cashflow monthly-memo --month 2026-04 --json
  ```

## Usage

Run `monarch-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

All linked and manual accounts (cash, credit, investment, loans, real estate)

- **`monarch-pp-cli accounts balance_history`** - Recent daily balances per account for the dashboard chart
- **`monarch-pp-cli accounts list`** - List every account with current balance, type, and institution
- **`monarch-pp-cli accounts networth_snapshots`** - Net-worth aggregate snapshots over time (asset/liability breakdown)
- **`monarch-pp-cli accounts refresh`** - Trigger a force-refresh of synced accounts
- **`monarch-pp-cli accounts refresh_status`** - Check the status of the latest force-refresh request
- **`monarch-pp-cli accounts types`** - List account type and subtype catalog used by Monarch

### budgets

Per-category and flex budgets, with actuals and variance

- **`monarch-pp-cli budgets list`** - List active budgets with budgeted amounts
- **`monarch-pp-cli budgets set_amount`** - Set the budgeted amount for a category
- **`monarch-pp-cli budgets status`** - Get budget-versus-actual for the active period

### cashflow

Income vs expenses summaries and category-level cashflow

- **`monarch-pp-cli cashflow by_category`** - Cashflow grouped by category
- **`monarch-pp-cli cashflow summary`** - Income vs expenses for a date window with savings rate

### categories

Transaction categories and category groups

- **`monarch-pp-cli categories create`** - Create a custom category
- **`monarch-pp-cli categories delete`** - Delete a category
- **`monarch-pp-cli categories groups`** - List category groups (top-level groupings)
- **`monarch-pp-cli categories list`** - List all categories (system + custom)

### credit

Credit score and report (powered by Spinwheel)

- **`monarch-pp-cli credit report`** - Get the current credit-score and report summary

### goals

Savings, debt-paydown, and investment goals

- **`monarch-pp-cli goals create`** - Create a savings/debt goal
- **`monarch-pp-cli goals delete`** - Delete a goal
- **`monarch-pp-cli goals list`** - List goals with progress and target amounts

### holdings

Investment holdings within brokerage accounts

- **`monarch-pp-cli holdings list`** - List investment holdings with quantity, price, and market value

### institutions

Connected financial institutions

- **`monarch-pp-cli institutions list`** - List connected institutions and their connection health

### me

Current user and household profile

- **`monarch-pp-cli me get`** - Get the authenticated user and household preferences

### recurring

Recurring merchants and bills (subscriptions, rent, paychecks)

- **`monarch-pp-cli recurring list`** - List active recurring streams with cadence and next-occurrence
- **`monarch-pp-cli recurring search`** - Search merchants for adding new recurring streams

### reports

Aggregated reports (spending, income, net worth) over time

- **`monarch-pp-cli reports configurations`** - List saved report configurations
- **`monarch-pp-cli reports data`** - Get aggregated reports data with grouping (category, merchant, date)

### subscription

Monarch subscription details for the household

- **`monarch-pp-cli subscription get`** - Get current Monarch plan, billing cycle, and trial status

### tags

Transaction tags (with usage counts)

- **`monarch-pp-cli tags create`** - Create a transaction tag
- **`monarch-pp-cli tags list`** - List tags with their transaction counts

### transactions

Bank, credit, and investment transactions with splits, tags, and categorization

- **`monarch-pp-cli transactions create`** - Create a manual transaction
- **`monarch-pp-cli transactions delete`** - Delete a transaction
- **`monarch-pp-cli transactions get`** - Get a transaction with its splits, tags, and rule provenance
- **`monarch-pp-cli transactions list`** - List transactions with filters (date, account, category, merchant, amount, tag)
- **`monarch-pp-cli transactions splits_get`** - Get the splits for a transaction
- **`monarch-pp-cli transactions splits_set`** - Replace the splits for a transaction
- **`monarch-pp-cli transactions summary`** - Aggregated transaction summary card (counts, totals) for a date window
- **`monarch-pp-cli transactions tags_set`** - Set the tags on a transaction (replaces existing)
- **`monarch-pp-cli transactions update`** - Update a transaction's category, notes, merchant, or tags


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
monarch-pp-cli accounts list

# JSON for scripting and agents
monarch-pp-cli accounts list --json

# Filter to specific fields
monarch-pp-cli accounts list --json --select id,name,status

# Dry run — show the request without sending
monarch-pp-cli accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
monarch-pp-cli accounts list --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-monarch -g
```

Then invoke `/pp-monarch <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add monarch monarch-pp-mcp -e MONARCH_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/monarch-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MONARCH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "monarch": {
      "command": "monarch-pp-mcp",
      "env": {
        "MONARCH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
monarch-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/monarch-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MONARCH_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `monarch-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MONARCH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every command** — Token expired or session stale: capture a fresh token from your Monarch web app and re-run `monarch-pp-cli auth set-token <token>`, or update `MONARCH_TOKEN` in your environment.
- **Empty results after sync** — Run `monarch-pp-cli accounts stale --threshold 24h` — if institutions are stale, run `monarch-pp-cli accounts refresh --wait` then re-sync.
- **GraphQL operation returned null data** — Monarch occasionally renames operations. Run `monarch-pp-cli doctor` to validate endpoints; update the binary with `go install github.com/<owner>/monarch-pp-cli@latest`.
- **Search returns no results** — FTS index rebuilds after each sync. If `search` is empty after sync, run `monarch-pp-cli sync --rebuild-index`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**theFong/mmoney-cli**](https://github.com/theFong/mmoney-cli) — Python (26 stars)
- [**eshaffer321/monarchmoney-go**](https://github.com/eshaffer321/monarchmoney-go) — Go (1 stars)
- [**keithah/monarchmoney-ts-mcp**](https://github.com/keithah/monarchmoney-ts-mcp) — TypeScript
- [**robcerda/monarch-mcp-server**](https://github.com/robcerda/monarch-mcp-server) — Python
- [**keithah/monarchmoney-enhanced**](https://github.com/keithah/monarchmoney-enhanced) — Python
- [**Maninae/monarch-money-cli**](https://github.com/Maninae/monarch-money-cli) — Python
- [**hammem/monarchmoney**](https://github.com/hammem/monarchmoney) — Python
- [**colvint/monarch-money-mcp**](https://github.com/colvint/monarch-money-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
