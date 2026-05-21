---
name: pp-lunch-money
description: "A Go CLI for Lunch Money's official v2 OpenAPI — offline SQLite store, subscription detection, and bulk smart... Trigger phrases: `categorize my transactions`, `find duplicate charges`, `subscription audit`, `what's my budget burn`, `net worth at a date`, `use lunch-money-pp`, `run lunch-money`."
author: "salmonumbrella"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - lunch-money-pp-cli
---

# Lunch Money — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `lunch-money-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install lunch-money --cli-only
   ```
2. Verify: `lunch-money-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/lunch-money/cmd/lunch-money-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Built on Lunch Money's published v2 alpha spec for forward compatibility — every endpoint exposed as a Cobra subcommand with --json, --select dotted-paths, --csv, --dry-run, and typed exit codes. Local SQLite store powers offline search, history queries, and joins (net worth at any date, stale balance audit, duplicate detection, subscription detective) that single API calls can't answer. A companion `lunch-money-pp-mcp` binary ships alongside for Claude Desktop / Claude Code MCP integration.

## When to Use This CLI

Use this CLI when you need to bulk-fix transactions, audit subscriptions or recurring items, reconcile net worth across Plaid + manual + crypto accounts, or expose Lunch Money to agent workflows. Especially valuable for Sunday-morning budgeting rituals (triage, retag, budget burn-down), end-of-month asset snapshots (manual account updates, balance history upserts), and expense-report assembly (transactions list --tag, attachment exports). Avoid it for one-off web-UI exploration — that's what lunchmoney.app is for.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local-join transcendence
- **`transactions subscriptions`** — Find regular-cadence merchant charges that are not yet linked to a recurring item, so you can promote them before subscription creep sneaks past you.

  _When a user asks 'what subscriptions are draining my account?', this surfaces unlabeled recurring charges in one local SQL pass._

  ```bash
  lunch-money-pp-cli transactions subscriptions --suspected-only --json --select 'payee,suggested_cadence,avg_amount,occurrences'
  ```
- **`recurring missing`** — List recurring items whose next expected date falls in the requested month but have no matching transaction yet, so you can catch missed bills.

  _Answers 'which subscriptions haven't hit my account yet?' which is a top-three Maya question with no API equivalent._

  ```bash
  lunch-money-pp-cli recurring missing --month 2026-05 --json
  ```
- **`transactions duplicates`** — Detect near-duplicate transactions in a window (same merchant, amount within tolerance, date within range) so you can group or delete them in bulk.

  _Use after a trip or when reviewing imports to catch the same Uber ride processed twice._

  ```bash
  lunch-money-pp-cli transactions duplicates --window 3 --tolerance 1.00 --json --select 'cluster_id,payee,amount,date,id'
  ```

### Net-worth tools
- **`accounts stale`** — Surface manual accounts and manual crypto whose balance has not been updated in N days, so net worth stays honest.

  _Answers 'what manual balances do I owe an update?' before quarterly NAV updates or Sunday reconciliation._

  ```bash
  lunch-money-pp-cli accounts stale --over 30d --json --select 'account_type,account_name,last_updated,days_stale'
  ```
- **`net-worth on`** — Reconstruct net worth at any historical date by walking balance_history across every account type, with last-known-balance carry-forward where needed.

  _Use for quarterly snapshots, retirement-plan inputs, or 'what was my net worth before I bought the car?' questions._

  ```bash
  lunch-money-pp-cli net-worth on 2026-03-31 --json --select 'as_of_date,total,by_account_type'
  ```

### Bulk transaction tools
- **`transactions retag`** — Match transactions by regex on payee/notes via local FTS, preview the affected set, then bulk-apply tag/category changes via the API.

  _Answers 'fix every Amazon transaction since January' in one command instead of pagination clicks._

  ```bash
  lunch-money-pp-cli transactions retag --match '^AMZN' --category-id 73 --dry-run --json
  ```
- **`triage`** — Pull all unreviewed transactions, compute a suggested category from prior categorizations of the same merchant, and optionally mass-apply the suggestion.

  _Sunday-morning ritual for power-user budgeters; agent-friendly suggested categories let an LLM bulk-approve high-confidence rows._

  ```bash
  lunch-money-pp-cli triage --limit 20 --json --select 'id,payee,amount,suggested_category,confidence'
  ```

### Budget intelligence
- **`budgets burn`** — For the current budget period, project end-of-month spend per category at the current linear rate and flag categories projected to overshoot.

  _Mid-month answer to 'am I on track?' that flows into agent workflows and dashboards._

  ```bash
  lunch-money-pp-cli budgets burn --period 2026-05 --json --select 'category_name,target,spent,projected_end_spend,over_under'
  ```

### Session orientation
- **`changed`** — List everything created or edited in the local store since a cutoff (e.g. --since 8h, --since 2026-05-01). Use at session start to orient before taking action.

  _Agents starting a fresh session can ask 'what changed overnight?' in one call instead of paginating multiple list endpoints._

  ```bash
  lunch-money-pp-cli changed --since 8h --json
  ```

## Command Reference

**balance-history** — View and update historical account balances. Balance history is what drives the [Net Worth](https://my.lunchmoney.app/net-worth) views in the Lunch Money app. Balance history is generated for each account's balance on the first day of each month and can be edited in the Lunch Money app or via the API.

- `lunch-money-pp-cli balance-history delete-entry` — Delete a single monthly balance history entry by its id.
- `lunch-money-pp-cli balance-history delete-for-account` — Delete all historical balance entries for a single manual, Plaid, manual crypto, or deleted account. Crypto synced...
- `lunch-money-pp-cli balance-history delete-for-crypto-synced` — Delete all historical balance entries for a single synced crypto symbol stream.<br><br> The path identifies both the...
- `lunch-money-pp-cli balance-history get` — Retrieve historical balance entries.<br><br> Balance history is monthly. When `start_date` and `end_date` are both...
- `lunch-money-pp-cli balance-history get-for-account` — Retrieve historical balance entries for one manual, Plaid, manual crypto, or deleted account. Crypto synced accounts...
- `lunch-money-pp-cli balance-history get-for-crypto-synced` — Retrieve historical balance entries for a single synced crypto symbol stream.<br><br> Use the `crypto_synced`...
- `lunch-money-pp-cli balance-history update-details` — Update archived metadata for a deleted balance history source.<br><br> Pass the `deleted` source id returned on...
- `lunch-money-pp-cli balance-history upsert-for-account` — Upsert one or more historical balance entries for a single manual, Plaid, manual crypto, or deleted acount. Crypto...
- `lunch-money-pp-cli balance-history upsert-for-crypto-synced` — Upsert one or more historical balance entries for a single synced crypto symbol stream.<br><br> The path identifies...

**budgets** — View settings and modify budget amounts.<p> Use the [/summary](#tag/summary) endpoint to view the budget details performance.

- `lunch-money-pp-cli budgets delete` — Removes the budget for the given category and period. If there already is no budget set for that period, the request...
- `lunch-money-pp-cli budgets get-settings` — Returns the budget-related settings for the user's account.
- `lunch-money-pp-cli budgets upsert` — Create or update a budget for a category and period.<p> If a budget already exists for the specified `start_date`...

**categories** — Work with categories

- `lunch-money-pp-cli categories create-category` — Creates a new category with the given name.<br> If the `is_group` attribute is set to true, a category group is...
- `lunch-money-pp-cli categories delete-category` — Attempts to delete the single category or category group specified on the path. By default, this will only work if...
- `lunch-money-pp-cli categories get-all` — Retrieve a list of all categories associated with the user's account.
- `lunch-money-pp-cli categories get-category-by-id` — Retrieve details of a specific category or category group by its ID.
- `lunch-money-pp-cli categories update-category` — Modifies the properties of an existing category or category group.<br><br> You may submit the response from a `GET...

**crypto** — Manage crypto

- `lunch-money-pp-cli crypto create-manual` — Create a manually managed crypto asset.<br><br> If `display_name` is `null`, clients may derive one from...
- `lunch-money-pp-cli crypto delete-manual` — Delete a single manually managed crypto asset by ID.<p> If this crypto asset has a balance history, and you do not...
- `lunch-money-pp-cli crypto get-all-manual` — Retrieve all manually managed crypto balances associated with the user's account.
- `lunch-money-pp-cli crypto get-all-synced` — Retrieves all synced crypto accounts associated with the user's account.
- `lunch-money-pp-cli crypto get-manual-by-id` — Retrieve a single manually managed crypto balance by ID.
- `lunch-money-pp-cli crypto get-synced-balance-by-symbol` — Retrieves a single balance from the specified synced crypto account using the crypto symbol.
- `lunch-money-pp-cli crypto get-synced-by-id` — Retrieves the synced crypto account and all nested balances for the specified synced crypto account ID.
- `lunch-money-pp-cli crypto refresh-synced` — Trigger a balance refresh for the specified synced crypto account. Returns the refreshed synced crypto account.
- `lunch-money-pp-cli crypto update-manual` — Modify a manually managed crypto balance.<br><br> You may submit the response from `GET /crypto/manual/{id}` as the...

**cryptocurrencies** — Manage cryptocurrencies

- `lunch-money-pp-cli cryptocurrencies create-cryptocurrency` — Adds a new cryptocurrency to the supported manual-crypto list.<br><br> Lunch Money uses...
- `lunch-money-pp-cli cryptocurrencies get-all` — Retrieve the list of cryptocurrencies currently supported for manual tracking.<p> When creating a new manual crypto...

**manual-accounts** — Work with manually managed accounts (formerly called assets)

- `lunch-money-pp-cli manual-accounts create` — Create a new manually-managed account.
- `lunch-money-pp-cli manual-accounts delete` — Deletes the single manual account with the ID specified on the path. If any transactions exist with the...
- `lunch-money-pp-cli manual-accounts get-all` — Retrieve a list of all manually-managed accounts associated with the user's account.
- `lunch-money-pp-cli manual-accounts get-by-id` — Retrieve the details of the manual account with the specified ID.
- `lunch-money-pp-cli manual-accounts update` — Modifies the properties of an existing manual account.<br><br> You may submit the response from a `GET...

**me** — View details and settings for the current user and account

- `lunch-money-pp-cli me` — Get details about the user associated with the supplied authorization token.

**plaid-accounts** — Work with accounts synced through Plaid

- `lunch-money-pp-cli plaid-accounts get-all` — Retrieve a list of all synced accounts associated with the user's account.
- `lunch-money-pp-cli plaid-accounts get-by-id` — Retrieve the details of the plaid account with the specified ID.
- `lunch-money-pp-cli plaid-accounts trigger-fetch` — Use this endpoint to trigger a fetch for latest data from Plaid.<br><br> Eligible accounts are those who last_fetch...

**recurring-items** — Work with recurring items

- `lunch-money-pp-cli recurring-items get-all-recurring` — Retrieve recurring items for a specified time frame.
- `lunch-money-pp-cli recurring-items get-recurring-by-id` — Retrieve the details of a specific recurring item with the specified ID.

**summary** — View a summary of the user's budget

- `lunch-money-pp-cli summary` — Retrieves a summary of the user's budget. Use this endpoint to access budget configuration details and performance...

**tags** — Work with tags

- `lunch-money-pp-cli tags create` — Creates a new tag with the given name
- `lunch-money-pp-cli tags delete` — Deletes the tag with the ID specified on the path.<br> If transactions or rules exist with the tag, a dependents...
- `lunch-money-pp-cli tags get-all` — Retrieve a list of all tags associated with the user's account.
- `lunch-money-pp-cli tags get-by-id` — Retrieve the details of a specific tag with the specified ID.
- `lunch-money-pp-cli tags update` — Updates an existing tag.<br><br> You may submit the response from a `GET /tags/{id}` as the request body; however,...

**transactions** — Work with transactions

- `lunch-money-pp-cli transactions create-new` — Use this endpoint to add transactions to a budget.<p> The request body for this endpoint must include a list of...
- `lunch-money-pp-cli transactions delete` — Deletes the transaction with the IDs specified in the request body.<p> If any of the specified transactions are a...
- `lunch-money-pp-cli transactions delete-attachment` — Deletes a file attachment from a transaction.
- `lunch-money-pp-cli transactions delete-by-id` — Deletes the transaction with the ID specified on the path.<p> If the specified transaction is a split transaction or...
- `lunch-money-pp-cli transactions get-all` — Retrieve a list of all transactions associated with a user's account. <br>If called with no parameters, this...
- `lunch-money-pp-cli transactions get-attachment-url` — Returns a signed url that can be used to download the file attachment.
- `lunch-money-pp-cli transactions get-by-id` — Retrieves the details of a specific transaction by its ID, including the following properties which are not returned...
- `lunch-money-pp-cli transactions group` — Specify a set of existing transaction IDs to group together as a single grouped transaction. The new transaction...
- `lunch-money-pp-cli transactions split` — Splits an existing transaction into a set of smaller child transactions.<br><br> After a transaction has been split,...
- `lunch-money-pp-cli transactions ungroup` — Deletes the transaction group with the ID specified on the path.<br> The transactions within the group are not...
- `lunch-money-pp-cli transactions unsplit` — Deletes the split children of a previously split transactions and restores the parent transactions to the normal...
- `lunch-money-pp-cli transactions update` — Modifies the properties of multiple existing transactions in a single request.<br><br> You may submit complete...
- `lunch-money-pp-cli transactions update-id` — Modifies the properties of an existing transaction.<br><br> You may submit the response from a `GET...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
lunch-money-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning triage: what changed overnight

```bash
lunch-money-pp-cli changed --since 8h --json
```

Show everything created or edited in the local store in the last 8 hours — use at session start to orient before taking action. Pulls from the offline SQLite store (no API call); refresh first with `lunch-money-pp-cli sync` if the cache is stale. Pass `--types transactions` (or `categories,manual-accounts`, etc.) to narrow, `--limit 10` to cap per type, and combine with `--select` to pick specific fields out of the result.

### Sunday triage with agent-applied suggestions

```bash
lunch-money-pp-cli triage --limit 50 --json --select 'id,payee,amount,suggested_category,confidence' | jq '[.[] | select(.confidence > 0.9) | .id] | join(",")'
```

Pipe the high-confidence triage rows to jq for an ID list, then apply them via `transactions retag --ids "$IDS" --category-id <id>` (which calls the bulk PUT /transactions endpoint under the hood).

### Find the asset driving this month's drift

```bash
lunch-money-pp-cli net-worth on 2026-04-30 --json --select 'by_account_type.account_name,by_account_type.balance' && lunch-money-pp-cli net-worth on 2026-05-12 --json --select 'by_account_type.account_name,by_account_type.balance'
```

Run net-worth at two dates and diff the per-account balances — answers Devon's Sunday 'what moved?' question without exporting CSV.

### Promote a stealth subscription to a recurring item

```bash
lunch-money-pp-cli transactions subscriptions --suspected-only --json --select 'payee,avg_amount,suggested_cadence' | jq '.[] | select(.avg_amount > 5)'
```

Surfaces local-detected subscription patterns (≥3 occurrences, monthly cadence) not yet flagged. Filter with jq for subscriptions over a threshold; the v2 API does not yet expose a recurring_items create endpoint, so promote each one in the web UI at https://my.lunchmoney.app/recurring.

### Drill into a budget category with deeply nested occurrences

```bash
lunch-money-pp-cli summary --start-date 2026-01-01 --end-date 2026-05-31 --include-occurrences --json --select 'categories.name,categories.occurrences.amount_spent,categories.occurrences.budget_amount'
```

The summary endpoint returns category objects with nested occurrences arrays. Dotted --select narrows the payload to the three fields needed for a per-month spend-vs-budget table without parsing the rest.

### Bulk retag every Amazon transaction since January

```bash
lunch-money-pp-cli transactions retag --match '^AMZN' --start-date 2026-01-01 --category-id 73 --add-tag amazon --dry-run --json
```

Dry-run shows count + sample rows from the local FTS5 search. Drop --dry-run to apply via the bulk PUT /transactions endpoint.

## Auth Setup

**Public API** (`api.lunchmoney.dev/v2`) — used by every top-level command. Get a bearer token from https://my.lunchmoney.app/developers and set it as `LUNCHMONEY_API_KEY`:

```bash
export LUNCHMONEY_API_KEY="<your-token>"
```

Or save it to the config file:

```bash
lunch-money-pp-cli auth set-token <your-token>
```

The value IS the bearer token — the CLI prepends `Authorization: Bearer ` for you.

**Internal API** (`api.lunchmoney.app`, web-UI backend) — only required for the `internal` subcommand tree (rules, Plaid taxonomy, bulk-edit primitives the public API doesn't expose). Seed once with `lunch-money-pp-cli internal auth set-cookie '<paste your browser Cookie: header>'`, or set `LUNCHMONEY_INTERNAL_COOKIE` for a non-persistent override (CI / one-off shells). See `internal/internalapi/README.md` for the cookie-seeding walkthrough.

Run `lunch-money-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  lunch-money-pp-cli balance-history get --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
lunch-money-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
lunch-money-pp-cli feedback --stdin < notes.txt
lunch-money-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.lunch-money-pp-cli/feedback.jsonl`. They are never POSTed unless `LUNCH_MONEY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LUNCH_MONEY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
lunch-money-pp-cli profile save briefing --json
lunch-money-pp-cli --profile briefing balance-history get
lunch-money-pp-cli profile list --json
lunch-money-pp-cli profile show briefing
lunch-money-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `lunch-money-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add lunch-money-pp-mcp -- lunch-money-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which lunch-money-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   lunch-money-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `lunch-money-pp-cli <command> --help`.
