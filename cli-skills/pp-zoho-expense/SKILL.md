---
name: pp-zoho-expense
description: "Upload receipts, auto-tag expenses from learned merchant memory Trigger phrases: `ingest invoices into zoho`, `upload my expenses to zoho`, `tag this zoho expense`, `close my zoho expenses for the month`, `submit my zoho expense report`, `use zoho-expense-pp-cli`, `run zoho expense`."
author: "Amitav Khandelwal"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zoho-expense-pp-cli
    install:
      - kind: go
        bins: [zoho-expense-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/cmd/zoho-expense-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/zoho-expense/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Zoho Expense — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zoho-expense-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install zoho-expense --cli-only
   ```
2. Verify: `zoho-expense-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/cmd/zoho-expense-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI when an AI agent needs to ingest invoices from email (or any source) and post them to Zoho Expense on a recurring cadence. It's also the right tool for monthly close automation — bundling expenses into a report and submitting in one command — and for India-specific GST splitting that the Zoho web UI doesn't expose. Not the right tool for occasional one-off expense editing through the UI (Zoho's web app is fine for that).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Hermes-first ingestion
- **`invoice ingest`** — Batch upload a folder of invoices to Zoho Expense, SHA256-dedup against the local store, poll autoscan in parallel, and auto-tag from learned merchant memory — the headline workflow for AI agents that ingest invoices from email.

  _Agents that ingest invoices on a cadence need one command that handles dedup, autoscan, and tagging — not three._

  ```bash
  zoho-expense-pp-cli invoice ingest ~/Downloads/october-invoices --auto-tag --agent
  ```
- **`receipt upload`** — SHA256-hash incoming receipts at upload, refuse duplicates (use --force to override).

  _Idempotent receipt upload — safe to re-run on a folder agents have already processed._

  ```bash
  zoho-expense-pp-cli receipt upload ~/Downloads/uber-receipt.pdf --auto-tag
  ```
- **`expense-untagged`** — List expenses missing category_id or project_id; `--auto-fix` applies the merchant→tag memory map and PUTs back to Zoho.

  _One command turns 30 untagged scanned receipts into 30 tagged ones, using the user's own historical mapping._

  ```bash
  zoho-expense-pp-cli expense-untagged --auto-fix --agent
  ```

### Local intelligence
- **`merchant list`** — Build a local merchant→category/project/tag mapping from sync history; pre-fill tags on first sight of a new merchant.

  _Solves Zoho's 'first-time merchant is always uncategorized' limitation with zero API calls._

  ```bash
  zoho-expense-pp-cli merchant list --agent --select merchant_name,category_name,seen_count
  ```
- **`merchant map`** — Train the local merchant→category mapping for future auto-tag (one merchant at a time, or bulk from a CSV).

  _Lets the user (or agent) explicitly seed the auto-tag map without waiting for sync history to learn it._

  ```bash
  zoho-expense-pp-cli merchant map 'AWS' --category Software --project Engineering
  ```

### Monthly automation
- **`close`** — Bundle a month's unreported expenses into an expense report in one shot — flags still-processing autoscans, flags untagged items, creates the report, attaches, optionally submits.

  _The natural end of every monthly agent workflow — turn raw expenses into a submitted report without leaving the terminal._

  ```bash
  zoho-expense-pp-cli close --month 2026-10 --auto-submit --agent
  ```

### India tax workflow
- **`gst-split`** — India-specific: parse an expense's tax_id and line items, compute CGST/SGST (intra-state) or IGST (inter-state) shares, emit the breakdown or update the expense.

  _Turns a monthly export into something a CA can directly file, without the user re-doing the math by hand._

  ```bash
  zoho-expense-pp-cli gst-split exp_1234567890 --emit-csv
  ```

## Command Reference

**currencies** — Currencies and exchange rates

- `zoho-expense-pp-cli currencies create` — Add a currency to the org
- `zoho-expense-pp-cli currencies delete` — Delete a currency
- `zoho-expense-pp-cli currencies get` — Get a currency
- `zoho-expense-pp-cli currencies list` — List currencies configured in the org
- `zoho-expense-pp-cli currencies update` — Update a currency

**customers** — Customers (contacts) expenses can be billed to

- `zoho-expense-pp-cli customers create` — Create a customer
- `zoho-expense-pp-cli customers delete` — Delete a customer
- `zoho-expense-pp-cli customers get` — Get a customer
- `zoho-expense-pp-cli customers list` — List customers
- `zoho-expense-pp-cli customers update` — Update a customer

**expense_categories** — Expense categories used to classify expenses

- `zoho-expense-pp-cli expense_categories create` — Create an expense category
- `zoho-expense-pp-cli expense_categories delete` — Delete an expense category
- `zoho-expense-pp-cli expense_categories disable` — Disable a category
- `zoho-expense-pp-cli expense_categories enable` — Enable a category
- `zoho-expense-pp-cli expense_categories get` — Get an expense category
- `zoho-expense-pp-cli expense_categories list` — List expense categories
- `zoho-expense-pp-cli expense_categories update` — Update an expense category

**expense_reports** — Expense reports — bundles of expenses submitted for approval and reimbursement

- `zoho-expense-pp-cli expense_reports approval-history` — View the approval history of a report
- `zoho-expense-pp-cli expense_reports approve` — Approve an expense report
- `zoho-expense-pp-cli expense_reports create` — Create an expense report
- `zoho-expense-pp-cli expense_reports get` — Get an expense report (includes attached expenses)
- `zoho-expense-pp-cli expense_reports list` — List expense reports
- `zoho-expense-pp-cli expense_reports reimburse` — Mark an expense report as reimbursed
- `zoho-expense-pp-cli expense_reports reject` — Reject an expense report
- `zoho-expense-pp-cli expense_reports update` — Update an expense report — used to attach more expenses

**expenses** — Expenses — the primary entity for an expense management CLI

- `zoho-expense-pp-cli expenses create` — Create an expense (JSON body — for receipt upload use 'receipt upload')
- `zoho-expense-pp-cli expenses get` — Get a single expense
- `zoho-expense-pp-cli expenses list` — List expenses with rich filters (date range, status, user, category, project)
- `zoho-expense-pp-cli expenses merge` — Merge multiple expenses (used to dedupe a scanned-receipt expense with a manual one)
- `zoho-expense-pp-cli expenses update` — Update an expense — used to add category/project/tags after autoscan

**organizations** — Zoho Expense organizations you have access to

- `zoho-expense-pp-cli organizations get` — Get organization details
- `zoho-expense-pp-cli organizations list` — List organizations accessible to the authenticated user

**projects** — Projects expenses can be associated with

- `zoho-expense-pp-cli projects activate` — Mark a project active
- `zoho-expense-pp-cli projects create` — Create a project
- `zoho-expense-pp-cli projects deactivate` — Mark a project inactive
- `zoho-expense-pp-cli projects delete` — Delete a project
- `zoho-expense-pp-cli projects get` — Get a project
- `zoho-expense-pp-cli projects list` — List projects
- `zoho-expense-pp-cli projects update` — Update a project

**receipts** — Upload receipts for autoscan

- `zoho-expense-pp-cli receipts` — Upload a receipt file (multipart). Server queues for autoscan; poll `expenses get` for status.

**reporting_tags** — Reporting tags (custom tagging schema for expenses, e.g. cost center, billable, GST treatment)

- `zoho-expense-pp-cli reporting_tags activate` — Activate a reporting tag
- `zoho-expense-pp-cli reporting_tags create` — Create a reporting tag
- `zoho-expense-pp-cli reporting_tags deactivate` — Deactivate a reporting tag
- `zoho-expense-pp-cli reporting_tags delete` — Delete a reporting tag
- `zoho-expense-pp-cli reporting_tags get` — Get a reporting tag
- `zoho-expense-pp-cli reporting_tags list` — List reporting tags
- `zoho-expense-pp-cli reporting_tags list-options` — List all options for a reporting tag
- `zoho-expense-pp-cli reporting_tags update` — Update a reporting tag

**taxes** — Taxes (GST in India) applied to expenses

- `zoho-expense-pp-cli taxes create` — Create a tax
- `zoho-expense-pp-cli taxes delete` — Delete a tax
- `zoho-expense-pp-cli taxes get` — Get a tax
- `zoho-expense-pp-cli taxes list` — List taxes
- `zoho-expense-pp-cli taxes update` — Update a tax

**trips** — Business trips that group related expenses

- `zoho-expense-pp-cli trips approve` — Approve a trip
- `zoho-expense-pp-cli trips cancel` — Cancel a trip
- `zoho-expense-pp-cli trips close` — Close a trip (no further expenses can be added)
- `zoho-expense-pp-cli trips create` — Create a trip
- `zoho-expense-pp-cli trips delete` — Delete a trip
- `zoho-expense-pp-cli trips get` — Get a trip
- `zoho-expense-pp-cli trips list` — List trips
- `zoho-expense-pp-cli trips reject` — Reject a trip
- `zoho-expense-pp-cli trips update` — Update a trip

**users** — Manage users in the organization

- `zoho-expense-pp-cli users activate` — Mark a user as active
- `zoho-expense-pp-cli users deactivate` — Mark a user as inactive
- `zoho-expense-pp-cli users delete` — Delete a user
- `zoho-expense-pp-cli users get` — Get a user
- `zoho-expense-pp-cli users invite` — Invite a user into the org
- `zoho-expense-pp-cli users list` — List users
- `zoho-expense-pp-cli users me` — Get the authenticated user's profile
- `zoho-expense-pp-cli users update` — Update a user


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `ZOHO_EXPENSE_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `zoho-expense-pp-cli currencies`
- `zoho-expense-pp-cli currencies create`
- `zoho-expense-pp-cli currencies delete`
- `zoho-expense-pp-cli currencies get`
- `zoho-expense-pp-cli currencies list`
- `zoho-expense-pp-cli currencies update`
- `zoho-expense-pp-cli customers`
- `zoho-expense-pp-cli customers create`
- `zoho-expense-pp-cli customers delete`
- `zoho-expense-pp-cli customers get`
- `zoho-expense-pp-cli customers list`
- `zoho-expense-pp-cli customers update`
- `zoho-expense-pp-cli expense_categories`
- `zoho-expense-pp-cli expense_categories create`
- `zoho-expense-pp-cli expense_categories delete`
- `zoho-expense-pp-cli expense_categories disable`
- `zoho-expense-pp-cli expense_categories enable`
- `zoho-expense-pp-cli expense_categories get`
- `zoho-expense-pp-cli expense_categories list`
- `zoho-expense-pp-cli expense_categories update`
- `zoho-expense-pp-cli expense_reports`
- `zoho-expense-pp-cli expense_reports approval_history`
- `zoho-expense-pp-cli expense_reports approve`
- `zoho-expense-pp-cli expense_reports create`
- `zoho-expense-pp-cli expense_reports get`
- `zoho-expense-pp-cli expense_reports list`
- `zoho-expense-pp-cli expense_reports reimburse`
- `zoho-expense-pp-cli expense_reports reject`
- `zoho-expense-pp-cli expense_reports update`
- `zoho-expense-pp-cli expenses`
- `zoho-expense-pp-cli expenses create`
- `zoho-expense-pp-cli expenses get`
- `zoho-expense-pp-cli expenses list`
- `zoho-expense-pp-cli expenses merge`
- `zoho-expense-pp-cli expenses update`
- `zoho-expense-pp-cli organizations`
- `zoho-expense-pp-cli organizations get`
- `zoho-expense-pp-cli organizations list`
- `zoho-expense-pp-cli projects`
- `zoho-expense-pp-cli projects activate`
- `zoho-expense-pp-cli projects create`
- `zoho-expense-pp-cli projects deactivate`
- `zoho-expense-pp-cli projects delete`
- `zoho-expense-pp-cli projects get`
- `zoho-expense-pp-cli projects list`
- `zoho-expense-pp-cli projects update`
- `zoho-expense-pp-cli reporting_tags`
- `zoho-expense-pp-cli reporting_tags activate`
- `zoho-expense-pp-cli reporting_tags create`
- `zoho-expense-pp-cli reporting_tags deactivate`
- `zoho-expense-pp-cli reporting_tags delete`
- `zoho-expense-pp-cli reporting_tags get`
- `zoho-expense-pp-cli reporting_tags list`
- `zoho-expense-pp-cli reporting_tags list_options`
- `zoho-expense-pp-cli reporting_tags update`
- `zoho-expense-pp-cli taxes`
- `zoho-expense-pp-cli taxes create`
- `zoho-expense-pp-cli taxes delete`
- `zoho-expense-pp-cli taxes get`
- `zoho-expense-pp-cli taxes list`
- `zoho-expense-pp-cli taxes update`
- `zoho-expense-pp-cli trips`
- `zoho-expense-pp-cli trips approve`
- `zoho-expense-pp-cli trips cancel`
- `zoho-expense-pp-cli trips close`
- `zoho-expense-pp-cli trips create`
- `zoho-expense-pp-cli trips delete`
- `zoho-expense-pp-cli trips get`
- `zoho-expense-pp-cli trips list`
- `zoho-expense-pp-cli trips reject`
- `zoho-expense-pp-cli trips update`
- `zoho-expense-pp-cli users`
- `zoho-expense-pp-cli users activate`
- `zoho-expense-pp-cli users deactivate`
- `zoho-expense-pp-cli users delete`
- `zoho-expense-pp-cli users get`
- `zoho-expense-pp-cli users invite`
- `zoho-expense-pp-cli users list`
- `zoho-expense-pp-cli users me`
- `zoho-expense-pp-cli users update`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zoho-expense-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `zoho-expense-pp-cli auth login` — Exchange a 10-min self-client authorization code for a refresh token (India region)
- `zoho-expense-pp-cli auth refresh` — Force-refresh the access token using the stored refresh token
- `zoho-expense-pp-cli auth status` — Show token validity, organization, and connected account
- `zoho-expense-pp-cli org use <organization_id>` — Set the active organization for subsequent commands
- `zoho-expense-pp-cli receipt upload <file>` — Upload a receipt file, poll until autoscan completes, return the populated expense (the Hermes hot path)
- `zoho-expense-pp-cli invoice ingest <dir-or-file>` — Batch upload invoices from a directory with hash dedup, parallel autoscan polling
- `zoho-expense-pp-cli expense tag <expense_id>` — Tag an expense with category, project, customer, billable flag, reporting tags via flags (--category, --project
- `zoho-expense-pp-cli expense-untagged` — List expenses missing a category (typically scanned receipts awaiting tagging)
- `zoho-expense-pp-cli close` — Close out a month: bundle unreported expenses for the month, alert on still-processing autoscans and untagged items
- `zoho-expense-pp-cli merchant list` — List merchants synthesized from local expense history with learned category mapping
- `zoho-expense-pp-cli merchant map <merchant>` — Train the local merchant→category/project mapping for future auto-tag
- `zoho-expense-pp-cli gst-split <expense_id>` — India-specific: parse tax_id and line items, compute CGST/SGST/IGST split, emit the breakdown
- `zoho-expense-pp-cli sync` — Sync expenses, categories, reporting tags, projects, customers into the local SQLite store
- `zoho-expense-pp-cli search <query>` — Full-text search across synced expenses (merchant, description, line items, reference numbers)
- `zoho-expense-pp-cli stale` — Show what local data is older than configured staleness thresholds and would benefit from a sync
- `zoho-expense-pp-cli doctor` — Check auth, region, organization, and API reachability

## Recipes


### Monthly Hermes flow

```bash
zoho-expense-pp-cli invoice ingest ~/Inbox/expenses --auto-tag --agent | tee /tmp/this-month.json && zoho-expense-pp-cli close --month $(date +%Y-%m) --auto-submit
```

Ingest every saved invoice with hash dedup + auto-tagging, then bundle the month and submit. The pattern Hermes runs end-of-month.

### Audit untagged expenses

```bash
zoho-expense-pp-cli expense-untagged --agent --select expense_id,merchant_name,amount,expense_date | jq '.[] | select(.amount > 1000)'
```

Find untagged expenses over ₹1000 — the ones most worth manually tagging when auto-fix can't help.

### Train the merchant memory

```bash
zoho-expense-pp-cli merchant list --agent --select merchant_name,category_name,seen_count | jq '.[] | select(.category_name == null)' | head -5
```

Find the top 5 merchants the local memory hasn't classified yet — feed them into `merchant map` to bootstrap the auto-tag map.

### Search synced expenses

```bash
zoho-expense-pp-cli search 'AWS' --agent --select expense_date,amount,merchant_name,description
```

Full-text search across synced expenses — instant local query, no API call.

### GST report for the CA

```bash
for id in $(zoho-expense-pp-cli expense list --month=2026-10 --json | jq -r '.[].expense_id'); do zoho-expense-pp-cli gst-split $id --emit-csv; done > october-gst.csv
```

Generate a CGST/SGST/IGST breakdown CSV for the month's expenses, ready to hand to a chartered accountant.

## Auth Setup

Zoho Expense uses OAuth 2.0 with a self-client authorization-code flow. Create a self-client at https://api-console.zoho.in/, generate a 10-minute authorization code, then run `zoho-expense-pp-cli auth login --client-id <id> --client-secret <secret>`. The CLI exchanges the code for a long-lived refresh token (stored at `~/.config/zoho-expense-pp-cli/config.toml`) and refreshes access tokens transparently. Tokens use the literal `Zoho-oauthtoken` Authorization prefix (not `Bearer`).

Run `zoho-expense-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zoho-expense-pp-cli currencies list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
zoho-expense-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zoho-expense-pp-cli feedback --stdin < notes.txt
zoho-expense-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.zoho-expense-pp-cli/feedback.jsonl`. They are never POSTed unless `ZOHO_EXPENSE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZOHO_EXPENSE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zoho-expense-pp-cli profile save briefing --json
zoho-expense-pp-cli --profile briefing currencies list
zoho-expense-pp-cli profile list --json
zoho-expense-pp-cli profile show briefing
zoho-expense-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `zoho-expense-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/cmd/zoho-expense-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add zoho-expense-pp-mcp -- zoho-expense-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zoho-expense-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zoho-expense-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zoho-expense-pp-cli <command> --help`.
