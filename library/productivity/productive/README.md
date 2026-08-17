# Productive CLI

**Full Productive.io v2 API client — list/get across ~48 resource types, generic create/update/delete, 11 report types, and a local sync/search/export/tail data layer, plus first-class revenue-recognition commands (recognized-revenue, invoiced, reconcile, aging).**

A CLI over the entire Productive.io v2 JSON:API — projects, tasks, deals/budgets, people, bookings, time entries, invoices, line items, expenses, and every other resource, each with `list`/`get` plus a generic `create`/`update`/`delete` write surface and 11 report types. It removes the MCP token ceiling that forced line-item pulls in 15–20-invoice batches, paginates large result sets to exhaustion, and ships a local SQLite mirror for offline SQL/full-text search. On top of that full-API coverage it adds hand-built revenue-recognition commands (recognized-revenue, invoiced, reconcile, aging).

## Install

The recommended path installs both the `productive-pp-cli` binary and the `pp-productive` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install productive
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install productive --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install productive --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install productive --agent claude-code
npx -y @mvanhorn/printing-press-library install productive --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/productive/cmd/productive-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/productive-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install productive --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-productive --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-productive --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install productive --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/productive-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PRODUCTIVE_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/productive/cmd/productive-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "productive": {
      "command": "productive-pp-mcp",
      "env": {
        "PRODUCTIVE_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Productive uses two headers: X-Auth-Token (secret API token) and X-Organization-Id (your org id, not secret). Set PRODUCTIVE_API_TOKEN and PRODUCTIVE_ORGANIZATION_ID. Get a token from Productive → Settings → API access. Run `productive-pp-cli doctor` to confirm both are set and the API is reachable.

## Quick Start

```bash
# confirm the binary and flags before wiring credentials
productive-pp-cli doctor --dry-run

# list production budgets (the units revenue is grouped by)
productive-pp-cli deals list --stage-type budget --page-size 200 --json

# the headline report: recognized revenue by budget × month
productive-pp-cli recognized-revenue --from 2026-01-01 --to 2026-06-30 --json

# recognized vs invoiced per budget × month with deltas
productive-pp-cli reconcile --from 2026-01-01 --to 2026-06-30 --json

```

## Unique Features

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

## Usage

Run `productive-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PRODUCTIVE_CONFIG_DIR`, `PRODUCTIVE_DATA_DIR`, `PRODUCTIVE_STATE_DIR`, or `PRODUCTIVE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PRODUCTIVE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PRODUCTIVE_HOME=/srv/productive
productive-pp-cli doctor
```

Under `PRODUCTIVE_HOME=/srv/productive`, the four dirs resolve to `/srv/productive/config`, `/srv/productive/data`, `/srv/productive/state`, and `/srv/productive/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `PRODUCTIVE_DATA_DIR` overrides an explicit `--home` for that kind. Use `PRODUCTIVE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PRODUCTIVE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `productive-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### activities

Activity-feed events

- **`productive-pp-cli activities get`** - Get one activities by id
- **`productive-pp-cli activities list`** - List activities

### attachments

Uploaded files

- **`productive-pp-cli attachments get`** - Get one attachments by id
- **`productive-pp-cli attachments list`** - List attachments

### bookings

Planned work / absence scheduling

- **`productive-pp-cli bookings get`** - Get one bookings by id
- **`productive-pp-cli bookings list`** - List bookings

### comments

Comments on tasks/deals/etc.

- **`productive-pp-cli comments get`** - Get one comments by id
- **`productive-pp-cli comments list`** - List comments

### companies

Client organizations

- **`productive-pp-cli companies get`** - Get one companies by id
- **`productive-pp-cli companies list`** - List companies

### contact_entries

Contact data (email/phone/address)

- **`productive-pp-cli contact-entries get`** - Get one contact entries by id
- **`productive-pp-cli contact-entries list`** - List contact entries

### custom_fields

User-defined fields

- **`productive-pp-cli custom-fields get`** - Get one custom fields by id
- **`productive-pp-cli custom-fields list`** - List custom fields

### deal_statuses

Pipeline stages

- **`productive-pp-cli deal-statuses get`** - Get one deal statuses by id
- **`productive-pp-cli deal-statuses list`** - List deal statuses

### deals

Sales deals and production budgets (filter --stage-type budget for budgets)

- **`productive-pp-cli deals get`** - Get one deals by id
- **`productive-pp-cli deals list`** - List deals

### events

Absence categories

- **`productive-pp-cli events get`** - Get one events by id
- **`productive-pp-cli events list`** - List events

### expenses

Non-labor costs on a budget service

- **`productive-pp-cli expenses get`** - Get one expenses by id
- **`productive-pp-cli expenses list`** - List expenses

### filters

Saved views/reports/widgets

- **`productive-pp-cli filters get`** - Get one filters by id
- **`productive-pp-cli filters list`** - List filters

### folders

Collections of task lists (boards) within a project

- **`productive-pp-cli folders get`** - Get one folders by id
- **`productive-pp-cli folders list`** - List folders

### holiday_calendars

Holiday calendars per country/region

- **`productive-pp-cli holiday-calendars get`** - Get one holiday calendars by id
- **`productive-pp-cli holiday-calendars list`** - List holiday calendars

### holidays

Non-working days within a holiday calendar

- **`productive-pp-cli holidays get`** - Get one holidays by id
- **`productive-pp-cli holidays list`** - List holidays

### invoice_attributions

Invoice<->budget attribution amounts

- **`productive-pp-cli invoice-attributions get`** - Get one invoice attributions by id
- **`productive-pp-cli invoice-attributions list`** - List invoice attributions

### invoices

Invoices and credit notes

- **`productive-pp-cli invoices get`** - Get one invoices by id
- **`productive-pp-cli invoices list`** - List invoices

### line_items

Invoice line items (paginate by --invoice)

- **`productive-pp-cli line-items get`** - Get one line items by id
- **`productive-pp-cli line-items list`** - List line items

### memberships

Access-sharing entries

- **`productive-pp-cli memberships get`** - Get one memberships by id
- **`productive-pp-cli memberships list`** - List memberships

### pages

Rich-text docs pages

- **`productive-pp-cli pages get`** - Get one pages by id
- **`productive-pp-cli pages list`** - List pages

### payments

Payments and write-offs against invoices

- **`productive-pp-cli payments get`** - Get one payments by id
- **`productive-pp-cli payments list`** - List payments

### people

Employees, contractors, contacts, placeholders

- **`productive-pp-cli people get`** - Get one people by id
- **`productive-pp-cli people list`** - List people

### pipelines

Sales/production pipelines

- **`productive-pp-cli pipelines get`** - Get one pipelines by id
- **`productive-pp-cli pipelines list`** - List pipelines

### projects

Project workspaces

- **`productive-pp-cli projects get`** - Get one projects by id
- **`productive-pp-cli projects list`** - List projects

### proposals

Sales quotes for deals

- **`productive-pp-cli proposals get`** - Get one proposals by id
- **`productive-pp-cli proposals list`** - List proposals

### reports

Aggregated report endpoints

- **`productive-pp-cli reports booking`** - Scheduling/resourcing report
- **`productive-pp-cli reports budget`** - Budget-level tracking report
- **`productive-pp-cli reports deal`** - Deal/sales pipeline report
- **`productive-pp-cli reports expense`** - Expense report
- **`productive-pp-cli reports financial-item`** - Financial line-item report (revenue/cost/profit across sources)
- **`productive-pp-cli reports invoice`** - Invoice report (amounts, tax, payments, aging)
- **`productive-pp-cli reports line-item`** - Invoice line-item report
- **`productive-pp-cli reports payment`** - Invoice payments report
- **`productive-pp-cli reports project`** - Project portfolio report
- **`productive-pp-cli reports task`** - Task report (worked/estimated/remaining)
- **`productive-pp-cli reports time-entry`** - Time tracking report (hours, billable, cost)

### roles

Permission sets

- **`productive-pp-cli roles get`** - Get one roles by id
- **`productive-pp-cli roles list`** - List roles

### service_types

Work categories the org delivers

- **`productive-pp-cli service-types get`** - Get one service types by id
- **`productive-pp-cli service-types list`** - List service types

### services

Line items on a deal/budget

- **`productive-pp-cli services get`** - Get one services by id
- **`productive-pp-cli services list`** - List services

### subsidiaries

Legal entities within the organization

- **`productive-pp-cli subsidiaries get`** - Get one subsidiaries by id
- **`productive-pp-cli subsidiaries list`** - List subsidiaries

### task_lists

Task groupings (milestones/sprints) within a project

- **`productive-pp-cli task-lists get`** - Get one task lists by id
- **`productive-pp-cli task-lists list`** - List task lists

### tasks

Work items in projects

- **`productive-pp-cli tasks get`** - Get one tasks by id
- **`productive-pp-cli tasks list`** - List tasks

### tax_rates

Tax/VAT percentages per subsidiary

- **`productive-pp-cli tax-rates get`** - Get one tax rates by id
- **`productive-pp-cli tax-rates list`** - List tax rates

### teams

Groups of people

- **`productive-pp-cli teams get`** - Get one teams by id
- **`productive-pp-cli teams list`** - List teams

### time_entries

Logged work time entries

- **`productive-pp-cli time-entries get`** - Get one time entries by id
- **`productive-pp-cli time-entries list`** - List time entries

### todos

Checklist items on tasks/deals

- **`productive-pp-cli todos get`** - Get one todos by id
- **`productive-pp-cli todos list`** - List todos

### workflow_statuses

Stages in a task workflow

- **`productive-pp-cli workflow-statuses get`** - Get one workflow statuses by id
- **`productive-pp-cli workflow-statuses list`** - List workflow statuses

### workflows

Task status groups

- **`productive-pp-cli workflows get`** - Get one workflows by id
- **`productive-pp-cli workflows list`** - List workflows


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
productive-pp-cli activities list

# JSON for scripting and agents
productive-pp-cli activities list --json

# Filter to specific fields
productive-pp-cli activities list --json --select id,name,status

# Dry run — show the request without sending
productive-pp-cli activities list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
productive-pp-cli activities list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-first, write when you ask** - list/get/report commands are read-only; explicit `create`/`update`/`delete` commands mutate via the JSON:API envelope (preview any of them with `--dry-run`)
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
productive-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `productive-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/productive-pp-cli/config.toml`; `--home`, `PRODUCTIVE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PRODUCTIVE_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `productive-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `productive-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PRODUCTIVE_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Set PRODUCTIVE_API_TOKEN and PRODUCTIVE_ORGANIZATION_ID; verify with `productive-pp-cli doctor`.
- **429 / rate limited on report commands** — Report endpoints allow only 10 requests / 30s; sync once with `sync` and query the local store, or narrow the date range.
- **Money values look 100x too large** — Raw --json is in smallest currency unit (cents); human and --csv output divide by the currency subunit (100 for SGD/most).
- **Line-item pull seems truncated** — Increase --page-size (max 200); the CLI paginates to exhaustion by default — check total_count in meta.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**berwickgeek/productive-mcp**](https://github.com/berwickgeek/productive-mcp) — TypeScript
- [**druellan/Productive-Simple-MCP**](https://github.com/druellan/Productive-Simple-MCP) — TypeScript
- [**BenEdgeContra/productive-client**](https://github.com/BenEdgeContra/productive-client) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
