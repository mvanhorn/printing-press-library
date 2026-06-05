---
name: pp-powerbi
description: "Pull data from the Microsoft Power BI REST API for analysis in Claude — every read-only feature pbicli has, plus... Trigger phrases: `pull data from power bi`, `query a power bi dataset`, `run a dax query`, `export a power bi report`, `check power bi refresh status`, `list power bi workspaces`, `use powerbi-pp-cli`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - powerbi-pp-cli
---

# Power BI — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `powerbi-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install powerbi --cli-only
   ```
2. Verify: `powerbi-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/data-analytics/powerbi/cmd/powerbi-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only Power BI CLI optimized for AI agents. Execute DAX queries with --csv and --select, export reports to PDF in a single command, browse workspaces/datasets/reports offline via the local catalog, and surface refresh failures across your tenant without admin scope.

## When to Use This CLI

Use when an agent needs to pull data out of Power BI for analysis in Claude — running DAX queries against semantic models, exporting report pages as PDFs, or browsing the workspace/dataset/report hierarchy to find the right resource ID. Read-only — no model authoring, no admin operations, no embed-token issuance. Pair with a Power BI workspace where you've added either a service principal or your AAD user as a member.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Workflow shortcuts
- **`report-export`** — Export a Power BI or paginated report to PDF/PPTX/PNG/XLSX/CSV/DOCX with a single command — POST, poll, and download handled internally.

  _Use when an agent needs a Power BI report as a file (for OCR, attachment, or visual analysis). One call instead of three plus a poll loop._

  ```bash
  powerbi-pp-cli report-export 37ae3f5d-665b-4c6b-affe-37ebd176d9e5 --group 804c5edc-6653-4149-8d08-a11279824b7a --format PDF --output report.pdf --wait
  ```

### Local state that compounds
- **`dax`** — Save named DAX queries to a local catalog and re-run them by name. Parameterize with --var key=value.

  _Use when iterating on DAX with an agent — save once, refine, re-run by name instead of re-pasting the query each turn._

  ```bash
  powerbi-pp-cli dax save monthly-revenue 'EVALUATE SUMMARIZECOLUMNS(Dates[Month], "Revenue", [Total Revenue])' && powerbi-pp-cli dax run monthly-revenue --group W --dataset D --csv
  ```
- **`refreshes failures`** — Surface every dataset whose most recent refresh failed in the last N days, with error messages.

  _Use for the 'what broke since yesterday' question. Joins local catalog + live refresh history; admin scope not required._

  ```bash
  powerbi-pp-cli refreshes failures --days 7 --json
  ```

### Agent-native plumbing
- **`dax run`** — Read a DAX query from a file and emit CSV — pipe-friendly for downstream tooling.

  _Use when a DAX query is long enough that shell quoting is painful, or when downstream tooling wants tabular input._

  ```bash
  powerbi-pp-cli dax run --file query.dax --group W --dataset D --csv > out.csv
  ```
- **`datasets describe`** — Best-effort tables/columns/measures listing for a dataset. Tries INFO.TABLES() DAX (Premium), falls back to dataset metadata.

  _Use before writing a DAX query to know table and column names. Reduces 'EVALUATE [guessed table name]' trial-and-error._

  ```bash
  powerbi-pp-cli datasets describe DATASET_ID --group W
  ```

### Auth & diagnostics
- **`auth doctor`** — Decode the AAD token, show tenant/app/scopes/expiry, probe /groups, and explain which Power BI tenant setting is blocking you if the probe fails.

  _Use when 'it returned 401/403' but you don't know why. Generic doctor commands probe network; this one decodes the JWT and explains the failure class._

  ```bash
  powerbi-pp-cli auth doctor
  ```
- **`auth login`** — Interactive device-code flow by default (no Azure CLI needed); service-principal client_credentials flow when --client-secret is passed. Token cached and refreshed.

  _Use the no-arg form for personal use; pass --client-secret in CI or headless agents. Tokens cached at ~/.config/powerbi-pp-cli/token.json with refresh-on-expiry._

  ```bash
  powerbi-pp-cli auth login                    # device code, opens browser
powerbi-pp-cli auth login --tenant T --client-id C --client-secret S    # service principal
  ```

## Command Reference

**apps** — Published bundles of dashboards and reports.

- `powerbi-pp-cli apps get` — Get an app by ID.
- `powerbi-pp-cli apps list` — List the apps the user has installed.
- `powerbi-pp-cli apps list_dashboards` — List the dashboards in an app.
- `powerbi-pp-cli apps list_reports` — List the reports in an app.

**dashboards** — Pinned tile collections.

- `powerbi-pp-cli dashboards get_tiles_in_group` — List the tiles pinned to a dashboard.
- `powerbi-pp-cli dashboards list_in_group` — List dashboards in a specific workspace.
- `powerbi-pp-cli dashboards list_my` — List dashboards in My workspace.

**dataflows** — Power Query workflows that produce datasets.

- `powerbi-pp-cli dataflows get_definition_in_group` — Get the dataflow definition (model.json with entities, M queries).
- `powerbi-pp-cli dataflows list_in_group` — List dataflows in a specific workspace.
- `powerbi-pp-cli dataflows transactions_in_group` — Get the transaction (refresh) history of a dataflow.

**datasets** — Datasets (semantic models). The thing you query with DAX.

- `powerbi-pp-cli datasets get_in_group` — Get a single dataset from a specific workspace.
- `powerbi-pp-cli datasets get_my` — Get a single dataset from My workspace.
- `powerbi-pp-cli datasets list_datasources_in_group` — List the datasources backing a dataset in a specific workspace.
- `powerbi-pp-cli datasets list_datasources_my` — List the datasources backing a dataset (My workspace).
- `powerbi-pp-cli datasets list_in_group` — List datasets in a specific workspace.
- `powerbi-pp-cli datasets list_my` — List datasets in My workspace.
- `powerbi-pp-cli datasets parameters_in_group` — Get the parameters of a dataset in a specific workspace.
- `powerbi-pp-cli datasets parameters_my` — Get the parameters of a dataset (My workspace).
- `powerbi-pp-cli datasets refresh_history_in_group` — Get the recent refresh history of a dataset in a specific workspace.
- `powerbi-pp-cli datasets refresh_history_my` — Get the recent refresh history of a dataset (My workspace).

**groups** — Workspaces (called 'groups' in the REST API). Containers for datasets, reports, dashboards, and dataflows.

- `powerbi-pp-cli groups get` — Get a single workspace by ID.
- `powerbi-pp-cli groups list` — List all workspaces (groups) the authenticated identity can access.

**reports** — Power BI reports and paginated reports.

- `powerbi-pp-cli reports export_status_in_group` — Get the status of an export-to-file job.
- `powerbi-pp-cli reports get_in_group` — Get a report from a specific workspace.
- `powerbi-pp-cli reports get_my` — Get a report from My workspace.
- `powerbi-pp-cli reports get_pages_in_group` — Get the pages of a Power BI report.
- `powerbi-pp-cli reports list_in_group` — List reports in a specific workspace.
- `powerbi-pp-cli reports list_my` — List reports in My workspace.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
powerbi-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Pull a slice of report data as CSV for spreadsheet analysis

```bash
powerbi-pp-cli dax run --query 'EVALUATE TOPN(1000, FactSales)' --group W --dataset D --csv > sales.csv
```

DAX EVALUATE returns rows; --csv flattens the executeQueries JSON into columns suitable for spreadsheets or pandas.

### Find a workspace by fuzzy name across hundreds

```bash
powerbi-pp-cli sync && powerbi-pp-cli search 'finance'
```

sync caches every workspace/dataset/report into local SQLite; search runs offline FTS so finding the right ID takes one command instead of paginated list calls.

### Narrow a deeply-nested response to the fields the agent actually needs

```bash
powerbi-pp-cli reports list-in-group --group-id W --json --select value.id,value.name,value.webUrl
```

Power BI list responses return ~20 fields per row including embedUrl, createdBy, modifiedBy. --select with dotted paths drops the rest — typical 5x reduction in agent context bytes.

### Export a report as PDF in one command

```bash
powerbi-pp-cli report-export REPORT_ID --group W --format PDF --output report.pdf --wait
```

One-shot wrapper: POST the export job, poll until Succeeded, download to the output path. Pass `--wait=false` to get the export job ID and poll later via `reports export-status-in-group --exportId ID`.

### Find datasets that failed to refresh this week

```bash
powerbi-pp-cli sync && powerbi-pp-cli refreshes failures --days 7 --json
```

Iterates the local catalog after sync, pulls each dataset's refresh history, returns only datasets whose most recent run is Failed with the error message.

## Auth Setup

Power BI auth is OAuth2 against Azure AD. Two paths: (1) bring your own bearer token via the POWERBI_TOKEN env var (get one with `az account get-access-token --resource https://analysis.windows.net/powerbi/api`), or (2) `powerbi-pp-cli auth login --tenant T --client-id C --client-secret S` to exchange a service principal for a token (cached locally, refreshed when expired). The `auth doctor` command decodes your token and explains which of the five common failure modes is biting if requests are being denied.

Run `powerbi-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  powerbi-pp-cli apps list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
powerbi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
powerbi-pp-cli feedback --stdin < notes.txt
powerbi-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.powerbi-pp-cli/feedback.jsonl`. They are never POSTed unless `POWERBI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `POWERBI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
powerbi-pp-cli profile save briefing --json
powerbi-pp-cli --profile briefing apps list
powerbi-pp-cli profile list --json
powerbi-pp-cli profile show briefing
powerbi-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `powerbi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add powerbi-pp-mcp -- powerbi-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which powerbi-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   powerbi-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `powerbi-pp-cli <command> --help`.
