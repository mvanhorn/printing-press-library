---
name: pp-actual-budget
description: "Your Actual Budget ledger in one static binary: search, SQL, reports and cleanup audits on a local copy pulled straight from your own Actual server, no Node required. Trigger phrases: `how much did I spend on groceries`, `find duplicate transactions in Actual`, `clean up my Actual Budget payees`, `what changed in my budget this week`, `use actual budget`, `run actual-budget-pp-cli`."
author: "Matt Anderson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - actual-budget-pp-cli
    install:
      - kind: go
        bins: [actual-budget-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/actual-budget/cmd/actual-budget-pp-cli
---

# Actual Budget — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `actual-budget-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install actual-budget --cli-only
   ```
2. Verify: `actual-budget-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/actual-budget/cmd/actual-budget-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

mirror pull downloads your budget from your actual-server into a local read-only mirror, so ledger, report, audit, templates and changes run offline without the Node API or a version-matched sidecar. Cleanup audits (audit payees, ledger duplicates, audit rules, categorize suggest) surface work the app does not list for you, and the full actual-http-api surface (accounts, payees, transactions, months, ...) plus import-csv is available when you run that sidecar.

## When to Use This CLI

Use this CLI to answer questions about an Actual Budget ledger (spending, balances, uncategorized or duplicate transactions, rule and schedule health, template funding, recent changes) from the terminal or an agent; these read a local mirror pulled with mirror pull. With the actual-http-api sidecar running it also scripts writes: bank CSV imports (import-csv), bulk categorization (categorize apply), payee merges and the full accounts, transactions and budget-month API.

## Anti-triggers

Do not use this CLI for:
- Do not use it to edit budgets in real time alongside an open web UI session expecting instant reflection; offline commands read a snapshot until mirror pull runs again
- Do not use it for bank connection setup (GoCardless/SimpleFIN linking); do that in the Actual web app
- Do not use it for YNAB; use a YNAB tool

## Before you start

Run `actual-budget-pp-cli mirror pull` first (credentials in Auth Setup below); mirror-backed commands return `[]` with a "run mirror pull" hint until a mirror exists. Commands that need the actual-http-api sidecar are labeled "requires sidecar" or listed under "Generated endpoint commands".

## Unique Capabilities

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

## Command Reference

### Mirror-backed commands

- `actual-budget-pp-cli budgets-remote` — List the budgets stored on your Actual server, with the sync id each one needs
- `actual-budget-pp-cli mirror pull` — Download (and decrypt) the budget from the Actual server into the local mirror (`--keep-zip` also saves a backup zip)
- `actual-budget-pp-cli mirror status` — Show which budget is mirrored locally, when it was pulled, and its row counts
- `actual-budget-pp-cli ledger search` — Find transactions by payee, imported text, notes, category or account name
- `actual-budget-pp-cli ledger uncategorized` — List on-budget transactions that still need a category
- `actual-budget-pp-cli ledger duplicates` — Catch transactions that were imported or entered twice, ranked by confidence
- `actual-budget-pp-cli ledger sql` — Run read-only SQL against the budget database
- `actual-budget-pp-cli report spending` — Where the money went in a period, by category, payee or category group
- `actual-budget-pp-cli report cashflow` — Income, spending and net per month
- `actual-budget-pp-cli report budget-vs-actual` — Budgeted vs spent per category for a month, flagging overspending
- `actual-budget-pp-cli report net-worth` — Net worth at each month end, across every account including off-budget
- `actual-budget-pp-cli report trends` — Per-category spending month by month, with average and latest-vs-average change
- `actual-budget-pp-cli report recurring` — Detect subscriptions and bills: payees charging similar amounts month after month
- `actual-budget-pp-cli audit payees` — Find payee spelling variants and get a merge plan (merges require sidecar)
- `actual-budget-pp-cli audit rules` — Find rules that never match, are shadowed, or point at deleted payees and categories
- `actual-budget-pp-cli audit schedules` — List overdue bills with nothing posted and schedules whose amounts drifted
- `actual-budget-pp-cli templates status` — Check envelope funding against #template/#goal targets from category notes
- `actual-budget-pp-cli changes since` — See what was created, edited, or deleted in the budget since a point in time
- `actual-budget-pp-cli categorize suggest` — Suggest categories for uncategorized transactions (read-only)
- `actual-budget-pp-cli categorize apply` — Write the suggested categories (requires sidecar; `--dry-run` previews)

### Sidecar commands

- `actual-budget-pp-cli import-csv` — Import a bank CSV export into an account through the actual-http-api sidecar, with reconciliation (requires sidecar)

### Generated endpoint commands

Requires the actual-http-api sidecar.

**accountgroups** — Manage accountgroups

- `actual-budget-pp-cli accountgroups create` — Creates an account group
- `actual-budget-pp-cli accountgroups delete` — Deletes an account group. Accounts belonging to the group are kept and become ungrouped
- `actual-budget-pp-cli accountgroups get` — Returns an account group
- `actual-budget-pp-cli accountgroups list` — Returns list of account groups
- `actual-budget-pp-cli accountgroups update` — Updates an account group

**accounts** — List, create, update, close, reopen and delete accounts; balances, balance history, bank sync

- `actual-budget-pp-cli accounts balance balance` — Gets the balance for an account; with a cutoff, the balance as of that date
- `actual-budget-pp-cli accounts balance-history` — (🔧 Extended) Gets the balance history for an account, from start to end date, with daily granularity.
- `actual-budget-pp-cli accounts bank-sync` — Triggers a bank sync for a specific account
- `actual-budget-pp-cli accounts bank-sync-all` — Triggers a bank sync
- `actual-budget-pp-cli accounts close close <accountId>` — Closes an account; transferAccountId and transferCategoryId are optional if the balance is 0
- `actual-budget-pp-cli accounts create` — Creates an account
- `actual-budget-pp-cli accounts delete` — Deletes an account
- `actual-budget-pp-cli accounts get` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli accounts list` — Returns list of accounts for the budget associated with the sync id specified
- `actual-budget-pp-cli accounts reopen reopen <accountId>` — Reopens a closed account
- `actual-budget-pp-cli accounts transactions list` — Returns list of transactions for an account
- `actual-budget-pp-cli accounts transactions create` — Adds transactions to an account; returns the created ids
- `actual-budget-pp-cli accounts transactions create-batch` — Adds a batch of transactions to an account; returns the created ids
- `actual-budget-pp-cli accounts transactions import` — Imports a list of transactions
- `actual-budget-pp-cli accounts update` — Updates an account

**actualhttpapiversion** — Manage actualhttpapiversion

- `actual-budget-pp-cli actualhttpapiversion` — Returns the version of the Actual HTTP API.

**actualserverversion** — Manage actualserverversion

- `actual-budget-pp-cli actualserverversion` — Returns the version of the Actual server.

**budgets** — Manage budgets

- `actual-budget-pp-cli budgets import` — 🔧 Extended: Uses the official importBudget API, returning the sync id of the imported budget.
- `actual-budget-pp-cli budgets list` — Returns a list of all budget files either locally cached or on the remote server.

**budgets-export** — Manage budgets export

- `actual-budget-pp-cli budgets-export` — 🔧 Extended: Uses the official exportBudget API, adding a dated file name derived from the budget name.

**categories** — List, create, update and delete budget categories

- `actual-budget-pp-cli categories create` — Creates a category
- `actual-budget-pp-cli categories delete` — Deletes a category
- `actual-budget-pp-cli categories get` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli categories list` — Returns list of categories
- `actual-budget-pp-cli categories update` — Updates a category

**categorygroups** — Manage categorygroups

- `actual-budget-pp-cli categorygroups create` — Creates a category group
- `actual-budget-pp-cli categorygroups delete` — Deletes a category group
- `actual-budget-pp-cli categorygroups list` — Returns list of category groups
- `actual-budget-pp-cli categorygroups update` — Updates a category group

**id-by-name** — Manage id by name

- `actual-budget-pp-cli id-by-name` — Get the ID of an entity by its name

**months** — Manage months

- `actual-budget-pp-cli months categories get` — Get one category's budget data for a month
- `actual-budget-pp-cli months categories list` — List category budget data for a month
- `actual-budget-pp-cli months categories update` — Update a category's budget for a month
- `actual-budget-pp-cli months categorygroups get` — Get one category group's budget data for a month
- `actual-budget-pp-cli months categorygroups list` — List category group budget data for a month
- `actual-budget-pp-cli months get` — Returns the budget information for the month specified
- `actual-budget-pp-cli months hold` — Put a budget amount on hold for next month
- `actual-budget-pp-cli months list` — Returns list of months for the budget associated with the sync id specified
- `actual-budget-pp-cli months reset-hold` — Reset budget hold
- `actual-budget-pp-cli months transfer` — Moves money from one category to another for one specific month.

**notes** — Read and write notes on categories, accounts and budget months

- `actual-budget-pp-cli notes delete-account` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli notes delete-category` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli notes delete-month` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli notes get-account` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli notes get-category` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli notes get-month` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli notes set-account` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli notes set-category` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli notes set-month` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.

**payees** — List, create, update, merge and delete payees

- `actual-budget-pp-cli payees create` — Creates a payee
- `actual-budget-pp-cli payees delete` — Deletes a payee
- `actual-budget-pp-cli payees get` — Returns a payee
- `actual-budget-pp-cli payees list` — Returns list of payees
- `actual-budget-pp-cli payees merge` — Merges payees
- `actual-budget-pp-cli payees rules list` — Returns list of rules for a payee
- `actual-budget-pp-cli payees update` — Updates a payee

**payees-common** — Manage payees common

- `actual-budget-pp-cli payees-common` — Returns common payees that appear frequently in transactions, including transfer payees

**preferences** — Manage preferences

- `actual-budget-pp-cli preferences` — Returns the budget's synced preferences, such as its date, number and currency formats

**rules** — List, create, update and delete transaction rules

- `actual-budget-pp-cli rules create` — Creates a rule
- `actual-budget-pp-cli rules delete` — Deletes a rule
- `actual-budget-pp-cli rules get` — Returns a rule
- `actual-budget-pp-cli rules list` — Returns list of rules for the budget associated with the sync id specified
- `actual-budget-pp-cli rules update` — Updates a rule

**run-query** — Manage run query

- `actual-budget-pp-cli run-query` — Executes an ActualQL query against the budget data using the aqlQuery method.

**schedules** — List, create, update and delete scheduled transactions

- `actual-budget-pp-cli schedules create` — Creates a schedule
- `actual-budget-pp-cli schedules delete` — Deletes a schedule
- `actual-budget-pp-cli schedules get` — 🔧 Extended: Uses official library APIs with additional business logic or transformations.
- `actual-budget-pp-cli schedules list` — Returns list of schedules for the budget associated with the sync id specified
- `actual-budget-pp-cli schedules update` — Updates a schedule

**tags** — List, create, update and delete transaction tags

- `actual-budget-pp-cli tags create` — Creates a tag
- `actual-budget-pp-cli tags delete` — Deletes a tag
- `actual-budget-pp-cli tags get` — Returns a tag
- `actual-budget-pp-cli tags list` — Returns list of tags
- `actual-budget-pp-cli tags update` — Updates a tag

**transactions** — Add, import, update and delete transactions

- `actual-budget-pp-cli transactions delete` — Deletes a transaction
- `actual-budget-pp-cli transactions delete-batch` — Deletes a set of transactions using the transaction ids
- `actual-budget-pp-cli transactions update` — Updates a transaction


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
actual-budget-pp-cli which "find duplicate transactions" --json
```

Template: replace the quoted text with the capability in your own words.

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

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

## Auth Setup

Reads (required): set ACTUAL_SERVER_URL (e.g. http://localhost:5006), ACTUAL_PASSWORD (or ACTUAL_SESSION_TOKEN), and ACTUAL_SYNC_ID (Settings → Show advanced settings → Sync ID, or run budgets-remote), plus ACTUAL_ENCRYPTION_PASSWORD if end-to-end encryption is on. Each also accepts a _FILE variant pointing at a secret file. Then run mirror pull. Writes and generated endpoint commands (optional): run the actual-http-api sidecar (https://github.com/jhonderson/actual-http-api) and set ACTUAL_BUDGET_BASE_URL (default http://localhost:5007/v1) and ACTUAL_HTTP_API_KEY to the API_KEY you configured on it.

Run `actual-budget-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  actual-budget-pp-cli accountgroups list --agent --select id,name   # requires sidecar
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — ledger, report, audit, templates and changes read the local mirror from `mirror pull`; `sync`/`search`/`analytics` cache sidecar responses separately and need the sidecar.
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `ACTUAL_BUDGET_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ACTUAL_BUDGET_CONFIG_DIR`, `ACTUAL_BUDGET_DATA_DIR`, `ACTUAL_BUDGET_STATE_DIR`, `ACTUAL_BUDGET_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ACTUAL_BUDGET_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `actual-budget-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ACTUAL_BUDGET_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ACTUAL_BUDGET_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
actual-budget-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "actual-budget-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `actual-budget-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `actual-budget-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `actual-budget-pp-cli sync` (requires the actual-http-api sidecar) to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
actual-budget-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
actual-budget-pp-cli teach \
  --query "$QUERY" \
  --resource-type payees \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
actual-budget-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
actual-budget-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`actual-budget-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `ACTUAL_BUDGET_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
actual-budget-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
actual-budget-pp-cli feedback --stdin < notes.txt
actual-budget-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ACTUAL_BUDGET_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ACTUAL_BUDGET_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
actual-budget-pp-cli profile save briefing --json
actual-budget-pp-cli --profile briefing ledger uncategorized
actual-budget-pp-cli profile list --json
actual-budget-pp-cli profile show briefing
actual-budget-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments, or no budget selected: set `ACTUAL_SYNC_ID`) |
| 3 | Resource not found |
| 4 | Authentication failed: bad `ACTUAL_PASSWORD`/`ACTUAL_SESSION_TOKEN`, missing or wrong `ACTUAL_ENCRYPTION_PASSWORD`, or bad sidecar `ACTUAL_HTTP_API_KEY` |
| 5 | API error (upstream issue) |
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error (e.g. `ACTUAL_SERVER_URL` unset) |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `actual-budget-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/actual-budget/cmd/actual-budget-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add actual-budget-pp-mcp -- actual-budget-pp-mcp
   ```
3. Verify: `claude mcp list`

The mirror-backed tools need a mirror: run `actual-budget-pp-cli mirror pull` first, or pass `ACTUAL_SERVER_URL`, `ACTUAL_PASSWORD` and `ACTUAL_SYNC_ID` in the MCP server env (e.g. `claude mcp add actual-budget-pp-mcp -e ACTUAL_SERVER_URL=http://localhost:5006 -e ACTUAL_PASSWORD=your-password -e ACTUAL_SYNC_ID=your-sync-id -- actual-budget-pp-mcp`). Generated endpoint tools also need `ACTUAL_BUDGET_BASE_URL` and `ACTUAL_HTTP_API_KEY` for the sidecar.

## Direct Use

1. Check if installed: `which actual-budget-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   actual-budget-pp-cli report spending --month 2026-09 --agent
   ```
   (Template: substitute the matched command and its arguments.)
4. If ambiguous, drill into subcommand help, e.g. `actual-budget-pp-cli audit payees --help`.
