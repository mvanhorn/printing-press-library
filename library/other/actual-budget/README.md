# Actual Budget CLI

**Your Actual Budget ledger in one static binary: search, SQL, reports and cleanup audits on a local copy pulled straight from your own Actual server, no Node required.**

mirror pull downloads your budget from your actual-server into a local read-only mirror, so ledger, report, audit, templates and changes run offline without the Node API or a version-matched sidecar. Cleanup audits (audit payees, ledger duplicates, audit rules, categorize suggest) surface work the app does not list for you, and the full actual-http-api surface (accounts, payees, transactions, months, ...) plus import-csv is available when you run that sidecar.

Learn more at [Actual Budget](https://actualbudget.org).

Created by [@blujaxfan](https://github.com/blujaxfan) (Matt Anderson).

## Install

The recommended path installs both the `actual-budget-pp-cli` binary and the `pp-actual-budget` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install actual-budget
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install actual-budget --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install actual-budget --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install actual-budget --agent claude-code
npx -y @mvanhorn/printing-press-library install actual-budget --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/actual-budget/cmd/actual-budget-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/actual-budget-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine ./actual-budget-pp-cli`. On Unix, mark it executable: `chmod +x ./actual-budget-pp-cli`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install actual-budget --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-actual-budget --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-actual-budget --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install actual-budget --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/actual-budget-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ACTUAL_SYNC_ID` (required; get it from `actual-budget-pp-cli budgets-remote`) and, if you run the sidecar, `ACTUAL_HTTP_API_KEY` when Claude Desktop prompts you.

The mirror-backed tools (ledger, report, audit, templates, changes, categorize suggest) read the local mirror. Either run `actual-budget-pp-cli mirror pull` with the CLI first (same data dir as the MCP server), or put `ACTUAL_SERVER_URL`, `ACTUAL_PASSWORD` and `ACTUAL_SYNC_ID` in the MCP server env so it can pull. The generated endpoint tools (accounts, payees, transactions, months, ...) need the actual-http-api sidecar.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/actual-budget/cmd/actual-budget-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "actual-budget": {
      "command": "actual-budget-pp-mcp",
      "env": {
        "ACTUAL_SERVER_URL": "http://localhost:5006",
        "ACTUAL_PASSWORD": "your-password",
        "ACTUAL_SYNC_ID": "your-sync-id",
        "ACTUAL_BUDGET_BASE_URL": "http://localhost:5007/v1",
        "ACTUAL_HTTP_API_KEY": "your-sidecar-api-key"
      }
    }
  }
}
```

`ACTUAL_BUDGET_BASE_URL` and `ACTUAL_HTTP_API_KEY` are optional; set them only if you run the actual-http-api sidecar.

</details>

## Setup

Reads (required): set ACTUAL_SERVER_URL (e.g. http://localhost:5006), ACTUAL_PASSWORD (or ACTUAL_SESSION_TOKEN), and ACTUAL_SYNC_ID (Settings → Show advanced settings → Sync ID, or run budgets-remote), plus ACTUAL_ENCRYPTION_PASSWORD if end-to-end encryption is on. Each also accepts a _FILE variant pointing at a secret file. Then run mirror pull. Writes and generated endpoint commands (optional): run the actual-http-api sidecar (https://github.com/jhonderson/actual-http-api) and set ACTUAL_BUDGET_BASE_URL (default http://localhost:5007/v1) and ACTUAL_HTTP_API_KEY to the API_KEY you configured on it.

### Reads (required)

Ledger, report, audit, templates, changes and `categorize suggest` read a local mirror downloaded from your Actual sync server.

```bash
export ACTUAL_SERVER_URL=http://localhost:5006
export ACTUAL_PASSWORD=your-password          # or ACTUAL_SESSION_TOKEN
actual-budget-pp-cli budgets-remote           # lists budgets and their sync ids
export ACTUAL_SYNC_ID=your-sync-id            # or pass --budget-sync-id
export ACTUAL_ENCRYPTION_PASSWORD=...         # only for end-to-end encrypted budgets
actual-budget-pp-cli mirror pull              # --keep-zip also saves a timestamped backup zip
actual-budget-pp-cli doctor
```

Each of these variables also accepts a `_FILE` variant (for example `ACTUAL_PASSWORD_FILE=/run/secrets/actual_password`) holding the value in a file.

### Writes and endpoint commands (optional sidecar)

The generated endpoint commands (accounts, payees, transactions, months, rules, ...), `import-csv`, `categorize apply`, and `sync`/`search`/`analytics`/`export`/`import`/`tail` call the [actual-http-api](https://github.com/jhonderson/actual-http-api) sidecar:

```bash
docker run -d -p 5007:5007 -e ACTUAL_SERVER_URL=http://host.docker.internal:5006 -e ACTUAL_SERVER_PASSWORD=your-password -e API_KEY=your-sidecar-api-key jhonderson/actual-http-api
export ACTUAL_BUDGET_BASE_URL=http://localhost:5007/v1   # default
export ACTUAL_HTTP_API_KEY=your-sidecar-api-key           # the API_KEY set on the sidecar
```

See the sidecar repository for its full configuration.

## Authentication

Reads (required): set ACTUAL_SERVER_URL (e.g. http://localhost:5006), ACTUAL_PASSWORD (or ACTUAL_SESSION_TOKEN), and ACTUAL_SYNC_ID (Settings → Show advanced settings → Sync ID, or run budgets-remote), plus ACTUAL_ENCRYPTION_PASSWORD if end-to-end encryption is on. Each also accepts a _FILE variant pointing at a secret file. Then run mirror pull. Writes and generated endpoint commands (optional): run the actual-http-api sidecar (https://github.com/jhonderson/actual-http-api) and set ACTUAL_BUDGET_BASE_URL (default http://localhost:5007/v1) and ACTUAL_HTTP_API_KEY to the API_KEY you configured on it.

## Quick Start

```bash
# Confirm the binary runs (no network); run plain doctor after exporting credentials
actual-budget-pp-cli doctor --dry-run

# After exporting ACTUAL_SERVER_URL and ACTUAL_PASSWORD: list budgets and their sync ids
actual-budget-pp-cli budgets-remote

# With ACTUAL_SYNC_ID set: download the budget into the local mirror
actual-budget-pp-cli mirror pull

# See where the money went this month
actual-budget-pp-cli report spending --month 2026-09

# Find transactions still needing a category
actual-budget-pp-cli ledger uncategorized --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Ledger cleanup
- **`audit payees`** — Find payee spelling variants (AMZN Mktp vs Amazon Marketplace) and get a ready-to-run payees merge plan (merging runs through the sidecar).

  _Reach for this after an import when payee lists look messy; it returns merge pairs an agent can apply via payees merge._

  ```bash
  actual-budget-pp-cli audit payees --min-similarity 0.85 --agent
  ```
- **`ledger duplicates`** — Catch transactions that were imported or entered twice, ranked by confidence.

  _Use after overlapping CSV imports to find double-counted spending before it skews reports._

  ```bash
  actual-budget-pp-cli ledger duplicates --days 3 --agent
  ```
- **`categorize suggest`** — Get a category suggestion for every uncategorized transaction based on how you categorized that payee before; categorize apply writes them through the actual-http-api sidecar.

  _Pick this to clear the uncategorized queue in bulk instead of one PATCH per transaction._

  ```bash
  actual-budget-pp-cli categorize suggest --min-confidence 0.7 --agent
  ```
- **`audit rules`** — Find rules that never match, are shadowed by another rule, or point at deleted payees and categories.

  _Use when the rules list has grown large and imports categorize unpredictably._

  ```bash
  actual-budget-pp-cli audit rules --months 6 --agent
  ```

### Offline budget intelligence
- **`ledger sql`** — Run read-only SQL against your real budget database without Node or a running engine.

  _Use for any ad-hoc question the canned reports do not answer._

  ```bash
  actual-budget-pp-cli ledger sql "SELECT name, offbudget FROM accounts WHERE tombstone = 0" --agent
  ```
- **`changes since`** — See what was edited, deleted, or restored in the budget since a point in time, with names resolved.

  _Use for a household check-in on what a partner changed, or to audit an import._

  ```bash
  actual-budget-pp-cli changes since 7d --agent
  ```

### Envelope budgeting
- **`audit schedules`** — List bills that are overdue with nothing posted and schedules whose amounts drifted.

  _Use in a weekly bills check to spot missed or changed recurring payments._

  ```bash
  actual-budget-pp-cli audit schedules --tolerance 5 --agent
  ```
- **`templates status`** — See which envelopes are under- or over-funded against the #template and #goal targets in category notes.

  _Use at the start of a month before funding envelopes._

  ```bash
  actual-budget-pp-cli templates status --month 2026-09 --agent
  ```

## Recipes

### Post-import cleanup

```bash
actual-budget-pp-cli ledger duplicates --days 3 --agent
```

Find double-imported transactions after loading overlapping CSV exports (local mirror).

### Narrow the account list

```bash
actual-budget-pp-cli ledger sql "SELECT id, name, offbudget, closed FROM accounts WHERE tombstone = 0" --agent --select id,name
```

Query accounts from the local mirror and keep only the fields an agent needs.

### Merge payee variants

```bash
actual-budget-pp-cli audit payees --agent
```

Get merge pairs from the mirror; run each merge_command with the sidecar running.

### Monthly spending by payee

```bash
actual-budget-pp-cli report spending --month 2026-09 --by payee --agent
```

Outflows grouped by payee for one month from the local mirror.

## Usage

Run `actual-budget-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `ACTUAL_BUDGET_CONFIG_DIR`, `ACTUAL_BUDGET_DATA_DIR`, `ACTUAL_BUDGET_STATE_DIR`, or `ACTUAL_BUDGET_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `ACTUAL_BUDGET_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export ACTUAL_BUDGET_HOME=/srv/actual-budget
actual-budget-pp-cli doctor
```

Under `ACTUAL_BUDGET_HOME=/srv/actual-budget`, the four dirs resolve to `/srv/actual-budget/config`, `/srv/actual-budget/data`, `/srv/actual-budget/state`, and `/srv/actual-budget/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "actual-budget": {
      "command": "actual-budget-pp-mcp",
      "env": {
        "ACTUAL_BUDGET_HOME": "/srv/actual-budget"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `ACTUAL_BUDGET_DATA_DIR` overrides an explicit `--home` for that kind. Use `ACTUAL_BUDGET_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `ACTUAL_BUDGET_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `actual-budget-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### Mirror-backed commands (read the local mirror)

These need the native read setup above and a `mirror pull`; without a mirror they return `[]` with a "run mirror pull" hint.

- **`actual-budget-pp-cli budgets-remote`** - List the budgets stored on your Actual server, with the sync id each one needs
- **`actual-budget-pp-cli mirror pull`** - Download (and decrypt) the budget from the Actual server into the local mirror
- **`actual-budget-pp-cli mirror status`** - Show which budget is mirrored locally, when it was pulled, and its row counts
- **`actual-budget-pp-cli ledger search`** - Find transactions by payee, imported text, notes, category or account name
- **`actual-budget-pp-cli ledger uncategorized`** - List on-budget transactions that still need a category
- **`actual-budget-pp-cli ledger duplicates`** - Catch transactions that were imported or entered twice, ranked by confidence
- **`actual-budget-pp-cli ledger sql`** - Run read-only SQL against the budget database
- **`actual-budget-pp-cli report spending`** - Where the money went in a period, by category, payee or category group
- **`actual-budget-pp-cli report cashflow`** - Income, spending and net per month
- **`actual-budget-pp-cli report budget-vs-actual`** - Budgeted vs spent per category for a month, flagging overspending
- **`actual-budget-pp-cli report net-worth`** - Net worth at each month end, across every account including off-budget
- **`actual-budget-pp-cli report trends`** - Per-category spending month by month, with average and latest-vs-average change
- **`actual-budget-pp-cli report recurring`** - Detect subscriptions and bills: payees charging similar amounts month after month
- **`actual-budget-pp-cli audit payees`** - Find payee spelling variants and get a merge plan (running the merges requires the sidecar)
- **`actual-budget-pp-cli audit rules`** - Find rules that never match, are shadowed by another rule, or point at deleted payees and categories
- **`actual-budget-pp-cli audit schedules`** - List overdue bills with nothing posted and schedules whose amounts drifted
- **`actual-budget-pp-cli templates status`** - Check envelope funding against #template/#goal targets from category notes
- **`actual-budget-pp-cli changes since`** - See what was created, edited, or deleted in the budget since a point in time
- **`actual-budget-pp-cli categorize suggest`** - Suggest categories for uncategorized transactions from your history (read-only)
- **`actual-budget-pp-cli categorize apply`** - Write those suggestions through the sidecar; `--dry-run` previews the PATCH requests

### Sidecar commands

- **`actual-budget-pp-cli import-csv`** - Import a bank CSV export into an account through the actual-http-api sidecar, with reconciliation (requires sidecar)

### Generated endpoint commands

Requires the actual-http-api sidecar.

### accountgroups

Manage accountgroups

- **`actual-budget-pp-cli accountgroups create`** - Creates an account group
- **`actual-budget-pp-cli accountgroups delete`** - Deletes an account group. Accounts belonging to the group are kept and become ungrouped
- **`actual-budget-pp-cli accountgroups get`** - Returns an account group
- **`actual-budget-pp-cli accountgroups list`** - Returns list of account groups
- **`actual-budget-pp-cli accountgroups update`** - Updates an account group

### accounts

List, create, update, close, reopen and delete accounts; balances, balance history, bank sync

- **`actual-budget-pp-cli accounts balance balance`** - Gets the balance for an account; with a cutoff, the balance as of that date
- **`actual-budget-pp-cli accounts balance-history`** - (🔧 Extended) Gets the balance history for an account, from start to end date, with daily granularity. Until date is optional, defaults to today.
- **`actual-budget-pp-cli accounts bank-sync`** - Triggers a bank sync for a specific account
- **`actual-budget-pp-cli accounts bank-sync-all`** - Triggers a bank sync
- **`actual-budget-pp-cli accounts close close <accountId>`** - Closes an account; transferAccountId and transferCategoryId are optional if the balance is 0
- **`actual-budget-pp-cli accounts create`** - Creates an account
- **`actual-budget-pp-cli accounts delete`** - Deletes an account
- **`actual-budget-pp-cli accounts get`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli accounts list`** - Returns list of accounts for the budget associated with the sync id specified
- **`actual-budget-pp-cli accounts reopen reopen <accountId>`** - Reopens a closed account
- **`actual-budget-pp-cli accounts transactions list`** - Returns list of transactions for an account
- **`actual-budget-pp-cli accounts transactions create`** - Adds transactions to an account; returns the created ids
- **`actual-budget-pp-cli accounts transactions create-batch`** - Adds a batch of transactions to an account; returns the created ids
- **`actual-budget-pp-cli accounts transactions import`** - Imports a list of transactions
- **`actual-budget-pp-cli accounts update`** - Updates an account

### actualhttpapiversion

Manage actualhttpapiversion

- **`actual-budget-pp-cli actualhttpapiversion`** - Returns the version of the Actual HTTP API.

### actualserverversion

Manage actualserverversion

- **`actual-budget-pp-cli actualserverversion`** - Returns the version of the Actual server.

### budgets

Manage budgets

- **`actual-budget-pp-cli budgets import`** - 🔧 Extended: Uses the official importBudget API, returning the sync id of the imported budget. Note that importing a budget that already exists does not restore it in place: the Actual library clears the file's server identity on import, so the budget is uploaded as a new file and receives a NEW sync id. The original budget and its sync id are left untouched.
- **`actual-budget-pp-cli budgets list`** - Returns a list of all budget files either locally cached or on the remote server. Remote files have a state field and local files have an id field.

### budgets-export

Manage budgets export

- **`actual-budget-pp-cli budgets-export`** - 🔧 Extended: Uses the official exportBudget API, adding a dated file name derived from the budget name.

### categories

List, create, update and delete budget categories

- **`actual-budget-pp-cli categories create`** - Creates a category
- **`actual-budget-pp-cli categories delete`** - Deletes a category
- **`actual-budget-pp-cli categories get`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli categories list`** - Returns list of categories
- **`actual-budget-pp-cli categories update`** - Updates a category

### categorygroups

Manage categorygroups

- **`actual-budget-pp-cli categorygroups create`** - Creates a category group
- **`actual-budget-pp-cli categorygroups delete`** - Deletes a category group
- **`actual-budget-pp-cli categorygroups list`** - Returns list of category groups
- **`actual-budget-pp-cli categorygroups update`** - Updates a category group

### id-by-name

Manage id by name

- **`actual-budget-pp-cli id-by-name`** - Get the ID of an entity by its name

### months

Manage months

- **`actual-budget-pp-cli months categories get`** - Get one category's budget data for a month
- **`actual-budget-pp-cli months categories list`** - List category budget data for a month
- **`actual-budget-pp-cli months categories update`** - Update a category's budget for a month
- **`actual-budget-pp-cli months categorygroups get`** - Get one category group's budget data for a month
- **`actual-budget-pp-cli months categorygroups list`** - List category group budget data for a month
- **`actual-budget-pp-cli months get`** - Returns the budget information for the month specified
- **`actual-budget-pp-cli months hold`** - Put a budget amount on hold for next month
- **`actual-budget-pp-cli months list`** - Returns list of months for the budget associated with the sync id specified
- **`actual-budget-pp-cli months reset-hold`** - Reset budget hold
- **`actual-budget-pp-cli months transfer`** - Moves money from one category to another for one specific month.<br> If the source category is not specified the money will come from available to budget.<br> If the destination is not specified the money will go to available to budget.<br> 🔧 Extended: Uses official library APIs with additional business logic or transformations.

### notes

Read and write notes on categories, accounts and budget months

- **`actual-budget-pp-cli notes delete-account`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli notes delete-category`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli notes delete-month`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli notes get-account`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli notes get-category`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli notes get-month`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli notes set-account`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli notes set-category`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli notes set-month`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.

### payees

List, create, update, merge and delete payees

- **`actual-budget-pp-cli payees create`** - Creates a payee
- **`actual-budget-pp-cli payees delete`** - Deletes a payee
- **`actual-budget-pp-cli payees get`** - Returns a payee
- **`actual-budget-pp-cli payees list`** - Returns list of payees
- **`actual-budget-pp-cli payees merge`** - Merges payees
- **`actual-budget-pp-cli payees rules list`** - Returns list of rules for a payee
- **`actual-budget-pp-cli payees update`** - Updates a payee

### payees-common

Manage payees common

- **`actual-budget-pp-cli payees-common`** - Returns common payees that appear frequently in transactions, including transfer payees

### preferences

Manage preferences

- **`actual-budget-pp-cli preferences`** - Returns the budget's synced preferences, such as its date, number and currency formats

### rules

List, create, update and delete transaction rules

- **`actual-budget-pp-cli rules create`** - Creates a rule
- **`actual-budget-pp-cli rules delete`** - Deletes a rule
- **`actual-budget-pp-cli rules get`** - Returns a rule
- **`actual-budget-pp-cli rules list`** - Returns list of rules for the budget associated with the sync id specified
- **`actual-budget-pp-cli rules update`** - Updates a rule

### run-query

Manage run query

- **`actual-budget-pp-cli run-query`** - Executes an ActualQL query against the budget data using the aqlQuery method. Note: previously used runQuery which has been deprecated upstream and will be removed in a future release.

### schedules

List, create, update and delete scheduled transactions

- **`actual-budget-pp-cli schedules create`** - Creates a schedule
- **`actual-budget-pp-cli schedules delete`** - Deletes a schedule
- **`actual-budget-pp-cli schedules get`** - 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- **`actual-budget-pp-cli schedules list`** - Returns list of schedules for the budget associated with the sync id specified
- **`actual-budget-pp-cli schedules update`** - Updates a schedule

### tags

List, create, update and delete transaction tags

- **`actual-budget-pp-cli tags create`** - Creates a tag
- **`actual-budget-pp-cli tags delete`** - Deletes a tag
- **`actual-budget-pp-cli tags get`** - Returns a tag
- **`actual-budget-pp-cli tags list`** - Returns list of tags
- **`actual-budget-pp-cli tags update`** - Updates a tag

### transactions

Add, import, update and delete transactions

- **`actual-budget-pp-cli transactions delete`** - Deletes a transaction
- **`actual-budget-pp-cli transactions delete-batch`** - Deletes a set of transactions using the transaction ids
- **`actual-budget-pp-cli transactions update`** - Updates a transaction


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`actual-budget-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`actual-budget-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`actual-budget-pp-cli learnings list`** - Inspect taught rows
- **`actual-budget-pp-cli learnings forget <query>`** - Undo a teach
- **`actual-budget-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`actual-budget-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`actual-budget-pp-cli teach-pattern`** - Install a query/resource template up front
- **`actual-budget-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `ACTUAL_BUDGET_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `actual-budget-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

The `accountgroups list` examples below require the sidecar; the same flags work on mirror-backed commands such as `ledger uncategorized`.

```bash
# Human-readable table (default in terminal, JSON when piped)
actual-budget-pp-cli accountgroups list

# JSON for scripting and agents
actual-budget-pp-cli accountgroups list --json
# Filter to specific fields
actual-budget-pp-cli accountgroups list --json --select id,name

# Dry run — show the request without sending
actual-budget-pp-cli accountgroups list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
actual-budget-pp-cli accountgroups list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - ledger, report, audit, templates and changes read the local mirror from `mirror pull`; `sync`/`search`/`analytics` cache sidecar responses separately and need the sidecar.
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `6` partial failure, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `ACTUAL_SYNC_ID` resolves `{budgetSyncId}`
- `ACTUAL_BUDGET_BASE_URL` overrides the sidecar base URL below
- `ACTUAL_SERVER_URL` is the Actual sync server used by `budgets-remote` and `mirror pull` (not the sidecar)

Base URL: `http://localhost:5007/v1` (actual-http-api sidecar)

## Health Check

```bash
actual-budget-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `actual-budget-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/actual-budget-pp-cli/config.toml`; `--home`, `ACTUAL_BUDGET_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ACTUAL_SERVER_URL` | reads | Yes (reads) | Actual sync server URL, e.g. `http://localhost:5006`. Unset gives exit 10. |
| `ACTUAL_PASSWORD` | reads | Yes (reads, or session token) | Actual server password. |
| `ACTUAL_SESSION_TOKEN` | reads | Alternative | Actual session token, used instead of `ACTUAL_PASSWORD`. |
| `ACTUAL_SYNC_ID` | endpoint | Yes | Budget sync id (from `budgets-remote`); `--budget-sync-id` overrides. Used by reads and sidecar calls. |
| `ACTUAL_ENCRYPTION_PASSWORD` | reads | Only for E2E-encrypted budgets | Budget encryption password. |
| `ACTUAL_BUDGET_DATA_DIR` | path | No | Overrides the data dir; the mirror lives at `<data dir>/mirrors/<sync id>/db.sqlite`. |
| `ACTUAL_BUDGET_BASE_URL` | sidecar | No | actual-http-api sidecar URL (default `http://localhost:5007/v1`). |
| `ACTUAL_HTTP_API_KEY` | sidecar | Yes (sidecar) | The `API_KEY` configured on the sidecar. |

`ACTUAL_SERVER_URL`, `ACTUAL_PASSWORD`, `ACTUAL_SESSION_TOKEN`, `ACTUAL_SYNC_ID` and `ACTUAL_ENCRYPTION_PASSWORD` each also accept a `_FILE` variant holding the path to a file with the value.

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `actual-budget-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `actual-budget-pp-cli doctor` to check credentials
- Native reads (`budgets-remote`, `mirror pull`): check `ACTUAL_PASSWORD` or `ACTUAL_SESSION_TOKEN`; for encrypted budgets, check `ACTUAL_ENCRYPTION_PASSWORD`
- Sidecar commands: verify `echo $ACTUAL_HTTP_API_KEY` matches the sidecar's `API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **exit 10: ACTUAL_SERVER_URL is not set** — export ACTUAL_SERVER_URL=http://localhost:5006 (your Actual server)
- **exit 2: no budget selected** — export ACTUAL_SYNC_ID to a sync id from budgets-remote, or pass --budget-sync-id
- **exit 4: unauthorized or invalid-password from the Actual server** — Check ACTUAL_PASSWORD (or ACTUAL_SESSION_TOKEN), then rerun mirror pull
- **missing-key or decrypt-failure** — Set ACTUAL_ENCRYPTION_PASSWORD to the budget encryption password
- **offline commands return [] with a no local mirror hint** — Run mirror pull
- **generated commands (accounts list, payees merge, ...) fail with connection refused** — Start the actual-http-api sidecar and set ACTUAL_BUDGET_BASE_URL and ACTUAL_HTTP_API_KEY
- **sidecar errors like no such column after an Actual upgrade** — Run doctor to compare sidecar and server versions; upgrade the sidecar image to match

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**actual-http-api**](https://github.com/jhonderson/actual-http-api) — JavaScript (256 stars)
- [**actual-mcp**](https://github.com/s-stefanov/actual-mcp) — TypeScript (242 stars)
- [**actualpy**](https://github.com/bvanelli/actualpy) — Python (109 stars)
- [**actual-mcp-server**](https://github.com/agigante80/actual-mcp-server) — JavaScript (61 stars)
- [**actual-budget-cli**](https://github.com/wirhabenzeit/actual-budget-cli) — JavaScript (22 stars)
- [**abctl**](https://github.com/tekumara/abctl) — JavaScript (1 stars)
- [**Actual official CLI (@actual-app/cli)**](https://github.com/actualbudget/actual/tree/master/packages/cli) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
