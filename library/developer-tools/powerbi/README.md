# Power BI CLI

**Pull data from the Microsoft Power BI REST API for analysis in Claude — every read-only feature pbicli has, plus saved DAX queries, one-shot report export, and a local SQLite catalog no Power BI tool has.**

Read-only Power BI CLI optimized for AI agents. Execute DAX queries with --csv and --select, export reports to PDF in a single command, browse workspaces/datasets/reports offline via the local catalog, and surface refresh failures across your tenant without admin scope.

## Install

The recommended path installs both the `powerbi-pp-cli` binary and the `pp-powerbi` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install powerbi
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install powerbi --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/powerbi-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-powerbi --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-powerbi --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-powerbi skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-powerbi. The skill defines how its required CLI can be installed.
```

## Authentication

Power BI auth is OAuth2 against Azure AD. Two paths: (1) bring your own bearer token via the POWERBI_TOKEN env var (get one with `az account get-access-token --resource https://analysis.windows.net/powerbi/api`), or (2) `powerbi-pp-cli auth login --tenant T --client-id C --client-secret S` to exchange a service principal for a token (cached locally, refreshed when expired). The `auth doctor` command decodes your token and explains which of the five common failure modes is biting if requests are being denied.

## Quick Start

```bash
# Get a Power BI bearer token from your logged-in Azure CLI session
export POWERBI_TOKEN=$(az account get-access-token --resource https://analysis.windows.net/powerbi/api --query accessToken -o tsv)


# List workspaces, narrowing to id+name so the agent context stays small
powerbi-pp-cli groups list --json --select value.id,value.name


# List datasets in a specific workspace (this is the example workspace ID from your URL)
powerbi-pp-cli datasets list-in-group --group-id 804c5edc-6653-4149-8d08-a11279824b7a --json


# See the tables/columns of a dataset so you know what to query
powerbi-pp-cli datasets describe DATASET_ID --group 804c5edc-6653-4149-8d08-a11279824b7a


# Run a DAX query against the dataset and emit CSV
powerbi-pp-cli dax run --query 'EVALUATE TOPN(100, Sales)' --group 804c5edc-6653-4149-8d08-a11279824b7a --dataset DATASET_ID --csv


# Export the report from the URL you provided to PDF in one command
powerbi-pp-cli report-export 37ae3f5d-665b-4c6b-affe-37ebd176d9e5 --group 804c5edc-6653-4149-8d08-a11279824b7a --format PDF --output report.pdf --wait

```

## Unique Features

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

## Usage

Run `powerbi-pp-cli --help` for the full command reference and flag list.

## Commands

### apps

Published bundles of dashboards and reports.

- **`powerbi-pp-cli apps get`** - Get an app by ID.
- **`powerbi-pp-cli apps list`** - List the apps the user has installed.
- **`powerbi-pp-cli apps list_dashboards`** - List the dashboards in an app.
- **`powerbi-pp-cli apps list_reports`** - List the reports in an app.

### dashboards

Pinned tile collections.

- **`powerbi-pp-cli dashboards get_tiles_in_group`** - List the tiles pinned to a dashboard.
- **`powerbi-pp-cli dashboards list_in_group`** - List dashboards in a specific workspace.
- **`powerbi-pp-cli dashboards list_my`** - List dashboards in My workspace.

### dataflows

Power Query workflows that produce datasets.

- **`powerbi-pp-cli dataflows get_definition_in_group`** - Get the dataflow definition (model.json with entities, M queries).
- **`powerbi-pp-cli dataflows list_in_group`** - List dataflows in a specific workspace.
- **`powerbi-pp-cli dataflows transactions_in_group`** - Get the transaction (refresh) history of a dataflow.

### datasets

Datasets (semantic models). The thing you query with DAX.

- **`powerbi-pp-cli datasets get_in_group`** - Get a single dataset from a specific workspace.
- **`powerbi-pp-cli datasets get_my`** - Get a single dataset from My workspace.
- **`powerbi-pp-cli datasets list_datasources_in_group`** - List the datasources backing a dataset in a specific workspace.
- **`powerbi-pp-cli datasets list_datasources_my`** - List the datasources backing a dataset (My workspace).
- **`powerbi-pp-cli datasets list_in_group`** - List datasets in a specific workspace.
- **`powerbi-pp-cli datasets list_my`** - List datasets in My workspace.
- **`powerbi-pp-cli datasets parameters_in_group`** - Get the parameters of a dataset in a specific workspace.
- **`powerbi-pp-cli datasets parameters_my`** - Get the parameters of a dataset (My workspace).
- **`powerbi-pp-cli datasets refresh_history_in_group`** - Get the recent refresh history of a dataset in a specific workspace.
- **`powerbi-pp-cli datasets refresh_history_my`** - Get the recent refresh history of a dataset (My workspace).

### groups

Workspaces (called 'groups' in the REST API). Containers for datasets, reports, dashboards, and dataflows.

- **`powerbi-pp-cli groups get`** - Get a single workspace by ID.
- **`powerbi-pp-cli groups list`** - List all workspaces (groups) the authenticated identity can access.

### reports

Power BI reports and paginated reports.

- **`powerbi-pp-cli reports export_status_in_group`** - Get the status of an export-to-file job.
- **`powerbi-pp-cli reports get_in_group`** - Get a report from a specific workspace.
- **`powerbi-pp-cli reports get_my`** - Get a report from My workspace.
- **`powerbi-pp-cli reports get_pages_in_group`** - Get the pages of a Power BI report.
- **`powerbi-pp-cli reports list_in_group`** - List reports in a specific workspace.
- **`powerbi-pp-cli reports list_my`** - List reports in My workspace.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
powerbi-pp-cli apps list

# JSON for scripting and agents
powerbi-pp-cli apps list --json

# Filter to specific fields
powerbi-pp-cli apps list --json --select id,name,status

# Dry run — show the request without sending
powerbi-pp-cli apps list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
powerbi-pp-cli apps list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-powerbi -g
```

Then invoke `/pp-powerbi <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add powerbi powerbi-pp-mcp -e POWERBI_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/powerbi-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `POWERBI_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "powerbi": {
      "command": "powerbi-pp-mcp",
      "env": {
        "POWERBI_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
powerbi-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/powerbi-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `POWERBI_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `powerbi-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $POWERBI_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **All requests return 403 Forbidden** — Run `powerbi-pp-cli auth doctor`. Most often: the AAD app is not added to the workspace as a member, or the tenant setting 'Allow service principals to use Power BI APIs' is off.
- **DAX query returns 'More than 100,000 rows in a query result'** — Wrap the EVALUATE in TOPN or add a date filter. The 100K row / 1M value / 15MB hard caps are server-side per the executeQueries API contract.
- **DAX query returns 429 Too Many Requests** — 120 queries-per-minute-per-user limit applies regardless of dataset. Back off and retry after the Retry-After window.
- **Bearer token expired (401 after a working session)** — Re-run `powerbi-pp-cli auth login` if using service-principal mode, or refresh `POWERBI_TOKEN` from `az account get-access-token` (tokens last ~1 hour).
- **Service principal getting 401 on a dataset that works for users** — Service principals are not supported on datasets with RLS enabled or SSO/live-connect to on-prem Azure Analysis Services. Use a delegated-user token for those.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**powerbi-cli**](https://github.com/powerbi-cli/powerbi-cli) — JavaScript (39 stars)
- [**powerbi-modeling-mcp**](https://github.com/microsoft/powerbi-modeling-mcp) — C#
- [**powerbi-mcp**](https://github.com/sulaiman013/powerbi-mcp) — Python
- [**pbi-cli**](https://github.com/MinaSaad1/pbi-cli) — Python
- [**pbi-tools**](https://github.com/pbi-tools/pbi-tools) — C#

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
