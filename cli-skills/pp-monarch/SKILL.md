---
name: pp-monarch
description: "A single Go binary for Monarch Money with offline SQLite, agent-native JSON, and local analytics no other Monarch... Trigger phrases: `what's my budget burn`, `did any subscription prices change`, `explain my net worth change`, `draft my monthly money memo`, `categorize my Monarch transactions`, `use monarch-pp-cli`, `run monarch-pp-cli`."
author: "Kyle Kirkland"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - monarch-pp-cli
---

# Monarch Money — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `monarch-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install monarch --cli-only
   ```
2. Verify: `monarch-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/monarch/cmd/monarch-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The `monarch-pp-cli` binary wraps the Monarch GraphQL surface as agent-friendly Cobra commands and adds eleven novel commands — budget burn, net-worth delta attribution, subscription drift detection, cashflow forecasting, agent context bundles, and more — that no Monarch competitor offers. Authenticate by capturing a session token from your Monarch web app and saving it via `auth set-token`.

## When to Use This CLI

Reach for monarch-pp-cli when an agent needs typed access to a user's Monarch Money data — for monthly reconciliation memos, budget burn analysis, subscription drift detection, or composing cashflow forecasts. The novel-feature commands compose multiple GraphQL reads into single agent-shaped JSON outputs that no single Monarch API call answers. Set `MONARCH_TOKEN` from a captured session token for both interactive and CI use.

## Unique Capabilities

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

## Command Reference

**accounts** — All linked and manual accounts (cash, credit, investment, loans, real estate)

- `monarch-pp-cli accounts balance_history` — Recent daily balances per account for the dashboard chart
- `monarch-pp-cli accounts list` — List every account with current balance, type, and institution
- `monarch-pp-cli accounts networth_snapshots` — Net-worth aggregate snapshots over time (asset/liability breakdown)
- `monarch-pp-cli accounts refresh` — Trigger a force-refresh of synced accounts
- `monarch-pp-cli accounts refresh_status` — Check the status of the latest force-refresh request
- `monarch-pp-cli accounts types` — List account type and subtype catalog used by Monarch

**budgets** — Per-category and flex budgets, with actuals and variance

- `monarch-pp-cli budgets list` — List active budgets with budgeted amounts
- `monarch-pp-cli budgets set_amount` — Set the budgeted amount for a category
- `monarch-pp-cli budgets status` — Get budget-versus-actual for the active period

**cashflow** — Income vs expenses summaries and category-level cashflow

- `monarch-pp-cli cashflow by_category` — Cashflow grouped by category
- `monarch-pp-cli cashflow summary` — Income vs expenses for a date window with savings rate

**categories** — Transaction categories and category groups

- `monarch-pp-cli categories create` — Create a custom category
- `monarch-pp-cli categories delete` — Delete a category
- `monarch-pp-cli categories groups` — List category groups (top-level groupings)
- `monarch-pp-cli categories list` — List all categories (system + custom)

**credit** — Credit score and report (powered by Spinwheel)

- `monarch-pp-cli credit` — Get the current credit-score and report summary

**goals** — Savings, debt-paydown, and investment goals

- `monarch-pp-cli goals create` — Create a savings/debt goal
- `monarch-pp-cli goals delete` — Delete a goal
- `monarch-pp-cli goals list` — List goals with progress and target amounts

**holdings** — Investment holdings within brokerage accounts

- `monarch-pp-cli holdings` — List investment holdings with quantity, price, and market value

**institutions** — Connected financial institutions

- `monarch-pp-cli institutions` — List connected institutions and their connection health

**me** — Current user and household profile

- `monarch-pp-cli me` — Get the authenticated user and household preferences

**recurring** — Recurring merchants and bills (subscriptions, rent, paychecks)

- `monarch-pp-cli recurring list` — List active recurring streams with cadence and next-occurrence
- `monarch-pp-cli recurring search` — Search merchants for adding new recurring streams

**reports** — Aggregated reports (spending, income, net worth) over time

- `monarch-pp-cli reports configurations` — List saved report configurations
- `monarch-pp-cli reports data` — Get aggregated reports data with grouping (category, merchant, date)

**subscription** — Monarch subscription details for the household

- `monarch-pp-cli subscription` — Get current Monarch plan, billing cycle, and trial status

**tags** — Transaction tags (with usage counts)

- `monarch-pp-cli tags create` — Create a transaction tag
- `monarch-pp-cli tags list` — List tags with their transaction counts

**transactions** — Bank, credit, and investment transactions with splits, tags, and categorization

- `monarch-pp-cli transactions create` — Create a manual transaction
- `monarch-pp-cli transactions delete` — Delete a transaction
- `monarch-pp-cli transactions get` — Get a transaction with its splits, tags, and rule provenance
- `monarch-pp-cli transactions list` — List transactions with filters (date, account, category, merchant, amount, tag)
- `monarch-pp-cli transactions splits_get` — Get the splits for a transaction
- `monarch-pp-cli transactions splits_set` — Replace the splits for a transaction
- `monarch-pp-cli transactions summary` — Aggregated transaction summary card (counts, totals) for a date window
- `monarch-pp-cli transactions tags_set` — Set the tags on a transaction (replaces existing)
- `monarch-pp-cli transactions update` — Update a transaction's category, notes, merchant, or tags


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
monarch-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Categorize a backlog with a single rule

```bash
monarch-pp-cli transactions categorize-bulk --merchant 'Whole Foods' --amount-lt 40 --category Groceries --create-rule --backfill 90d
```

Match historical transactions, apply the category, save the rule, and backfill the last 90 days in one call.

### Decompose this week's net-worth change

```bash
monarch-pp-cli networth explain --since 7d --json
```

Returns the income/spending/market/transfers breakdown so you know whether the bump was earned or lent.

### Surface stale connections before trusting numbers

```bash
monarch-pp-cli accounts stale --threshold 24h --json --select institution.name,last_synced,balance
```

Lists silently broken syncs sorted by financial impact.

### Drill into uncategorized transactions with select

```bash
monarch-pp-cli transactions list --category Uncategorized --start-date 2026-04-01 --json --select id,date,amount,merchant.name,description
```

Narrows a verbose row to the four fields an agent needs to suggest a category.

### Compose a monthly memo for an LLM

```bash
monarch-pp-cli cashflow monthly-memo --month 2026-04 --json
```

Returns a structured packet ready for `| claude` to draft the prose.

## Auth Setup

Monarch has no public API. Capture a session token from your logged-in Monarch web app (Chrome DevTools → Application → Cookies → look for the Authorization Token cookie/header) and pass it to `monarch-pp-cli auth set-token <token>`. Or set `MONARCH_TOKEN` in your environment. Tokens last several months. Interactive `auth login --chrome` cookie import and `--email --password --mfa` headless login are tracked as v0.2 work; until then the env-var / set-token path is the supported flow.

Run `monarch-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  monarch-pp-cli accounts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
monarch-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
monarch-pp-cli feedback --stdin < notes.txt
monarch-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.monarch-pp-cli/feedback.jsonl`. They are never POSTed unless `MONARCH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MONARCH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
monarch-pp-cli profile save briefing --json
monarch-pp-cli --profile briefing accounts list
monarch-pp-cli profile list --json
monarch-pp-cli profile show briefing
monarch-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `monarch-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add monarch-pp-mcp -- monarch-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which monarch-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   monarch-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `monarch-pp-cli <command> --help`.
