---
name: pp-productive
description: "Full Productive.io v2 API client — list/get across ~48 resource types (projects, tasks, deals, people, bookings, time entries, invoices, and more), generic create/update/delete, 11 report types, and a local sync/search/export/tail data layer, plus first-class revenue-recognition commands (recognized-revenue, invoiced, reconcile, aging). Trigger phrases: `list Productive tasks`, `Productive projects/deals/people/bookings`, `query the Productive API`, `recognized revenue by budget`, `reconcile recognized vs invoiced`, `pull Productive invoices`, `export line items to csv`, `productive report`, `use productive-pp-cli`, `run productive-pp-cli`."
author: "Derick Ng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - productive-pp-cli
    install:
      - kind: go
        bins: [productive-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/productive/cmd/productive-pp-cli
---

# Productive — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `productive-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install productive --cli-only
   ```
2. Verify: `productive-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/productive/cmd/productive-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A CLI over the entire Productive.io v2 JSON:API — projects, tasks, deals/budgets, people, bookings, time entries, invoices, line items, expenses, and every other resource, each with `list`/`get` plus a generic `create`/`update`/`delete` write surface and 11 report types. It removes the MCP token ceiling that forced line-item pulls in 15–20-invoice batches, paginates large result sets to exhaustion, and ships a local SQLite mirror for offline SQL/full-text search. On top of that full-API coverage it adds hand-built revenue-recognition commands (recognized-revenue, invoiced, reconcile, aging).

## When to Use This CLI

Use this CLI to extract Productive.io financial data for a revenue-recognition reconciliation pipeline: recognized revenue by budget and month, invoiced amounts via invoice attributions, unpaid receivables, and bulk raw pulls of invoices, line items, and attributions in bulk. It is the right tool when you need paginated, scriptable, offline-queryable financial data rather than interactive chat answers.

## Anti-triggers

Do not use this CLI for:
- Do not use it as a substitute for the Productive web UI for complex multi-step editing — it offers generic create/update/delete, not guided forms.
- Do not use it for interactive one-off questions where the official MCP connector is more convenient.
- Do not assume server-side aggregation beyond what the report endpoints provide; heavy cross-resource analytics belong in the local SQLite store via sync + sql.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Revenue recognition
- **`recognized-revenue`** — Pull recognized (earned) revenue grouped by budget and month across a date range in one call.

  _Reach for this when you need earned revenue per budget per month for revenue recognition — it replaces a chain of manual report calls._

  ```bash
  productive-pp-cli recognized-revenue --from 2026-01-01 --to 2026-06-30 --json
  ```
- **`invoiced`** — Net finalized invoiced amount attributed per budget per month, drafts reported separately.

  _Use for the billed side of the ledger when comparing against recognized revenue._

  ```bash
  productive-pp-cli invoiced --from 2026-01-01 --to 2026-06-30 --json
  ```
- **`reconcile`** — Join recognized revenue and invoiced amounts per budget×month and flag the deltas.

  _This is the reconciliation itself — pick it to surface budgets where earned and billed revenue diverge._

  ```bash
  productive-pp-cli reconcile --from 2026-01-01 --to 2026-06-30 --threshold 100 --json
  ```

### Bulk extraction
- **`export`** — Stream full paginated pulls of any resource to JSONL or JSON (auto-paginated, no memory pressure).

  _Use to refresh bulk raw pulls for a downstream pipeline; pair with list --csv when you need CSV._

  ```bash
  productive-pp-cli export line_items --format jsonl --output line_items.jsonl
  ```

### Local state that compounds
- **`sync`** — Mirror any resource into local SQLite, then query it with SQL and full-text search offline.

  _Sync once, then run sql/search repeatedly with zero API cost and no rate-limit risk._

  ```bash
  productive-pp-cli sync --resources invoices,line_items,invoice_attributions,deals
  ```
- **`aging`** — Bucket unpaid invoice amounts by age past their due date.

  _Use for a quick receivables snapshot — which invoices are overdue and by how long._

  ```bash
  productive-pp-cli aging --as-of 2026-07-04 --json
  ```

## Command Reference

**activities** — Activity-feed events

- `productive-pp-cli activities get` — Get one activities by id
- `productive-pp-cli activities list` — List activities

**attachments** — Uploaded files

- `productive-pp-cli attachments get` — Get one attachments by id
- `productive-pp-cli attachments list` — List attachments

**bookings** — Planned work / absence scheduling

- `productive-pp-cli bookings get` — Get one bookings by id
- `productive-pp-cli bookings list` — List bookings

**comments** — Comments on tasks/deals/etc.

- `productive-pp-cli comments get` — Get one comments by id
- `productive-pp-cli comments list` — List comments

**companies** — Client organizations

- `productive-pp-cli companies get` — Get one companies by id
- `productive-pp-cli companies list` — List companies

**contact_entries** — Contact data (email/phone/address)

- `productive-pp-cli contact-entries get` — Get one contact entries by id
- `productive-pp-cli contact-entries list` — List contact entries

**custom_fields** — User-defined fields

- `productive-pp-cli custom-fields get` — Get one custom fields by id
- `productive-pp-cli custom-fields list` — List custom fields

**deal_statuses** — Pipeline stages

- `productive-pp-cli deal-statuses get` — Get one deal statuses by id
- `productive-pp-cli deal-statuses list` — List deal statuses

**deals** — Sales deals and production budgets (filter --stage-type budget for budgets)

- `productive-pp-cli deals get` — Get one deals by id
- `productive-pp-cli deals list` — List deals

**events** — Absence categories

- `productive-pp-cli events get` — Get one events by id
- `productive-pp-cli events list` — List events

**expenses** — Non-labor costs on a budget service

- `productive-pp-cli expenses get` — Get one expenses by id
- `productive-pp-cli expenses list` — List expenses

**filters** — Saved views/reports/widgets

- `productive-pp-cli filters get` — Get one filters by id
- `productive-pp-cli filters list` — List filters

**folders** — Collections of task lists (boards) within a project

- `productive-pp-cli folders get` — Get one folders by id
- `productive-pp-cli folders list` — List folders

**holiday_calendars** — Holiday calendars per country/region

- `productive-pp-cli holiday-calendars get` — Get one holiday calendars by id
- `productive-pp-cli holiday-calendars list` — List holiday calendars

**holidays** — Non-working days within a holiday calendar

- `productive-pp-cli holidays get` — Get one holidays by id
- `productive-pp-cli holidays list` — List holidays

**invoice_attributions** — Invoice<->budget attribution amounts

- `productive-pp-cli invoice-attributions get` — Get one invoice attributions by id
- `productive-pp-cli invoice-attributions list` — List invoice attributions

**invoices** — Invoices and credit notes

- `productive-pp-cli invoices get` — Get one invoices by id
- `productive-pp-cli invoices list` — List invoices

**line_items** — Invoice line items (paginate by --invoice)

- `productive-pp-cli line-items get` — Get one line items by id
- `productive-pp-cli line-items list` — List line items

**memberships** — Access-sharing entries

- `productive-pp-cli memberships get` — Get one memberships by id
- `productive-pp-cli memberships list` — List memberships

**pages** — Rich-text docs pages

- `productive-pp-cli pages get` — Get one pages by id
- `productive-pp-cli pages list` — List pages

**payments** — Payments and write-offs against invoices

- `productive-pp-cli payments get` — Get one payments by id
- `productive-pp-cli payments list` — List payments

**people** — Employees, contractors, contacts, placeholders

- `productive-pp-cli people get` — Get one people by id
- `productive-pp-cli people list` — List people

**pipelines** — Sales/production pipelines

- `productive-pp-cli pipelines get` — Get one pipelines by id
- `productive-pp-cli pipelines list` — List pipelines

**projects** — Project workspaces

- `productive-pp-cli projects get` — Get one projects by id
- `productive-pp-cli projects list` — List projects

**proposals** — Sales quotes for deals

- `productive-pp-cli proposals get` — Get one proposals by id
- `productive-pp-cli proposals list` — List proposals

**reports** — Aggregated report endpoints

- `productive-pp-cli reports booking` — Scheduling/resourcing report
- `productive-pp-cli reports budget` — Budget-level tracking report
- `productive-pp-cli reports deal` — Deal/sales pipeline report
- `productive-pp-cli reports expense` — Expense report
- `productive-pp-cli reports financial-item` — Financial line-item report (revenue/cost/profit across sources)
- `productive-pp-cli reports invoice` — Invoice report (amounts, tax, payments, aging)
- `productive-pp-cli reports line-item` — Invoice line-item report
- `productive-pp-cli reports payment` — Invoice payments report
- `productive-pp-cli reports project` — Project portfolio report
- `productive-pp-cli reports task` — Task report (worked/estimated/remaining)
- `productive-pp-cli reports time-entry` — Time tracking report (hours, billable, cost)

**roles** — Permission sets

- `productive-pp-cli roles get` — Get one roles by id
- `productive-pp-cli roles list` — List roles

**service_types** — Work categories the org delivers

- `productive-pp-cli service-types get` — Get one service types by id
- `productive-pp-cli service-types list` — List service types

**services** — Line items on a deal/budget

- `productive-pp-cli services get` — Get one services by id
- `productive-pp-cli services list` — List services

**subsidiaries** — Legal entities within the organization

- `productive-pp-cli subsidiaries get` — Get one subsidiaries by id
- `productive-pp-cli subsidiaries list` — List subsidiaries

**task_lists** — Task groupings (milestones/sprints) within a project

- `productive-pp-cli task-lists get` — Get one task lists by id
- `productive-pp-cli task-lists list` — List task lists

**tasks** — Work items in projects

- `productive-pp-cli tasks get` — Get one tasks by id
- `productive-pp-cli tasks list` — List tasks

**tax_rates** — Tax/VAT percentages per subsidiary

- `productive-pp-cli tax-rates get` — Get one tax rates by id
- `productive-pp-cli tax-rates list` — List tax rates

**teams** — Groups of people

- `productive-pp-cli teams get` — Get one teams by id
- `productive-pp-cli teams list` — List teams

**time_entries** — Logged work time entries

- `productive-pp-cli time-entries get` — Get one time entries by id
- `productive-pp-cli time-entries list` — List time entries

**todos** — Checklist items on tasks/deals

- `productive-pp-cli todos get` — Get one todos by id
- `productive-pp-cli todos list` — List todos

**workflow_statuses** — Stages in a task workflow

- `productive-pp-cli workflow-statuses get` — Get one workflow statuses by id
- `productive-pp-cli workflow-statuses list` — List workflow statuses

**workflows** — Task status groups

- `productive-pp-cli workflows get` — Get one workflows by id
- `productive-pp-cli workflows list` — List workflows


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
productive-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Recognized revenue for H1, by budget and month

```bash
productive-pp-cli recognized-revenue --from 2026-01-01 --to 2026-06-30 --json
```

The primary input for revenue reconciliation: earned revenue grouped by budget × month.

### Reconcile earned vs billed and flag gaps

```bash
productive-pp-cli reconcile --from 2026-01-01 --to 2026-06-30 --threshold 100 --json
```

Shows budgets where recognized and invoiced revenue diverge beyond the threshold.

### Bulk-export line items to JSONL

```bash
productive-pp-cli export line_items --format jsonl --output line_items.jsonl
```

Auto-paginated streaming pull of every line item; use line-items list --csv for CSV instead.

### Narrow a deep report response for an agent

```bash
productive-pp-cli recognized-revenue --from 2026-01-01 --to 2026-06-30 --agent --select rows.budget_name,rows.date,rows.total_recognized_revenue
```

Uses --select dotted paths to keep only the fields an agent needs from a large report payload.

### Offline: full-text search over synced data

```bash
productive-pp-cli search "Acme" --json
```

After 'sync' mirrors resources to local SQLite, search runs instant offline FTS (use 'sql' for arbitrary SELECTs).

## Auth Setup

Productive uses two headers: X-Auth-Token (secret API token) and X-Organization-Id (your org id, not secret). Set PRODUCTIVE_API_TOKEN and PRODUCTIVE_ORGANIZATION_ID. Get a token from Productive → Settings → API access. Run `productive-pp-cli doctor` to confirm both are set and the API is reachable.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  productive-pp-cli activities list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Writes are explicit** — read/list/report commands never mutate; `create`, `update`, and `delete` do (JSON:API envelope, `application/vnd.api+json`). Preview any write with `--dry-run`

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

- Use `--home <dir>` for one invocation, or set `PRODUCTIVE_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PRODUCTIVE_CONFIG_DIR`, `PRODUCTIVE_DATA_DIR`, `PRODUCTIVE_STATE_DIR`, `PRODUCTIVE_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PRODUCTIVE_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `productive-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "productive": {
        "command": "productive-pp-mcp",
        "env": {
          "PRODUCTIVE_HOME": "/srv/productive"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PRODUCTIVE_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PRODUCTIVE_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
productive-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
productive-pp-cli feedback --stdin < notes.txt
productive-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PRODUCTIVE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PRODUCTIVE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
productive-pp-cli profile save briefing --json
productive-pp-cli --profile briefing activities list
productive-pp-cli profile list --json
productive-pp-cli profile show briefing
productive-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `productive-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/productive/cmd/productive-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add productive-pp-mcp -- productive-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which productive-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   productive-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `productive-pp-cli <command> --help`.
