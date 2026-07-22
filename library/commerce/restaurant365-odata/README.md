# Restaurant365 OData CLI

Read-only Restaurant365 OData CLI for data engineers building reporting, warehouse refreshes, and reconciliation jobs.

Restaurant365 is a restaurant back-office platform for accounting, operations, payroll, inventory, sales, and restaurant-level financial reporting. Its OData connector exposes reporting views that teams commonly load into warehouses and BI tools.

This CLI focuses on the engineering work around that feed: discovering available views, checking schemas, taking redacted samples, planning daily refreshes, building bounded backfills, tracking rowVersion-based incremental loads, and checking deleted-record tombstones.

Created by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

## Install

The recommended path installs both the `restaurant365-odata-pp-cli` binary and the `pp-restaurant365-odata` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install restaurant365-odata
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install restaurant365-odata --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install restaurant365-odata --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install restaurant365-odata --agent claude-code
npx -y @mvanhorn/printing-press-library install restaurant365-odata --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/restaurant365-odata/cmd/restaurant365-odata-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/restaurant365-odata-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install restaurant365-odata --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-restaurant365-odata --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-restaurant365-odata --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install restaurant365-odata --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/restaurant365-odata-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RESTAURANT365_ODATA_USERNAME` and `RESTAURANT365_ODATA_PASSWORD` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/restaurant365-odata/cmd/restaurant365-odata-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "restaurant365-odata": {
      "command": "restaurant365-odata-pp-mcp",
      "env": {
        "RESTAURANT365_ODATA_USERNAME": "<domain\\username>",
        "RESTAURANT365_ODATA_PASSWORD": "<password>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Restaurant365 OData uses HTTP Basic authentication. Use the OData username format provided for your tenant, commonly `domain\username`, plus the matching password.

```bash
export RESTAURANT365_ODATA_USERNAME="domain\\username"
export RESTAURANT365_ODATA_PASSWORD="<password>"
export RESTAURANT365_ODATA_BASE_URL="https://odata.restaurant365.net/api/v2/views"
```

Short aliases are also supported:

```bash
export R365_ODATA_USERNAME="domain\\username"
export R365_ODATA_PASSWORD="<password>"
export R365_ODATA_BASE_URL="https://odata.restaurant365.net/api/v2/views"
```

You can also persist credentials in your config file at `~/.config/restaurant365-odata-pp-cli/config.toml`.

### 3. Verify Setup

```bash
restaurant365-odata-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
restaurant365-odata-pp-cli list-views --agent
```

## Usage

Run `restaurant365-odata-pp-cli --help` for the full command reference and flag list.

## Data Engineering Workflow

```bash
# Confirm auth and metadata access
restaurant365-odata-pp-cli doctor

# Discover views and schema without fetching row values
restaurant365-odata-pp-cli list-views --agent
restaurant365-odata-pp-cli describe-view Location --agent

# Validate a private tenant without printing row values
restaurant365-odata-pp-cli sample --view Location --limit 5 --agent

# Plan a daily or monthly sales refresh
restaurant365-odata-pp-cli backfill-plan --view SalesDetail --from 2026-05-01 --to 2026-06-15 --agent

# Plan an incremental rowVersion pull
restaurant365-odata-pp-cli backfill-plan --view Transaction --watermark 0 --agent

# Check deletion tombstones by entity without printing IDs
restaurant365-odata-pp-cli deleted-records --entity TransactionDetail --since-row-version 0 --limit 5 --agent

# Export one bounded slice
restaurant365-odata-pp-cli export --view SalesDetail --from 2026-05-01 --to 2026-05-01 --format jsonl --output sales-detail.jsonl
```

`sample` and `deleted-records` redact row values by default. Use `--include-values` only when you intentionally need row-level output in a trusted environment.

For date-backed views, the CLI builds Restaurant365-compatible DateTimeOffset filters such as `date ge 2026-05-01T00:00:00Z and date le 2026-05-31T23:59:59Z`.

## Refresh Patterns

| Pattern | Views | Use |
| --- | --- | --- |
| rowVersion incremental | `Transaction`, `TransactionDetail`, `EntityDeleted` | Continue from the last stored rowVersion watermark. |
| date-window refresh | `SalesDetail`, `SalesEmployee`, `SalesPayment`, `LaborDetail`, `PayrollSummary` | Refresh bounded daily, weekly, or monthly windows. |
| small dimension full refresh | `Company`, `Employee`, `GLAccount`, `Item`, `JobTitle`, `Location`, `POSEmployee` | Reload the whole view when dimensions are small enough. |

## Commands

### list-views

List documented Restaurant365 OData views, detected field counts from `$metadata`, and the suggested refresh pattern.

- **`restaurant365-odata-pp-cli list-views --agent`** - Returns view names, resource names, field counts, date fields, rowVersion support, and sync pattern.

### describe-view

Describe one Restaurant365 OData view using `$metadata`.

- **`restaurant365-odata-pp-cli describe-view Location --agent`** - Returns field names and OData types without fetching row values.

### sample

Fetch a small sample from one view while redacting values by default.

- **`restaurant365-odata-pp-cli sample --view Location --limit 5 --agent`** - Returns row count and column names only.
- **`restaurant365-odata-pp-cli sample --view Location --limit 5 --include-values --agent`** - Includes row values when explicitly requested.

### backfill-plan

Plan safe Restaurant365 OData requests without fetching rows.

- **`restaurant365-odata-pp-cli backfill-plan --view SalesDetail --from 2026-05-01 --to 2026-06-15 --agent`** - Splits date windows into bounded chunks.
- **`restaurant365-odata-pp-cli backfill-plan --view Transaction --watermark 0 --agent`** - Builds a rowVersion incremental filter.

### deleted-records

Inspect `EntityDeleted` tombstones for delete handling.

- **`restaurant365-odata-pp-cli deleted-records --entity TransactionDetail --since-row-version 0 --limit 5 --agent`** - Returns counts by entity and redacts values by default.

### export

Export a bounded view slice to JSONL or CSV.

- **`restaurant365-odata-pp-cli export --view SalesDetail --from 2026-05-01 --to 2026-05-01 --format jsonl --output sales-detail.jsonl`** - Writes one date-window slice.

### company

Manage company

- **`restaurant365-odata-pp-cli company`** - Returns Company reporting rows from Restaurant365 OData.

### employee

Manage employee

- **`restaurant365-odata-pp-cli employee`** - Returns Employee reporting rows from Restaurant365 OData.

### entity-deleted

Manage entity deleted

- **`restaurant365-odata-pp-cli entity-deleted`** - Returns deletion tombstone rows from Restaurant365 OData.

### gl-account

Manage gl account

- **`restaurant365-odata-pp-cli gl-account`** - Returns GlAccount reporting rows from Restaurant365 OData.

### glaccount

Manage glaccount

- **`restaurant365-odata-pp-cli glaccount`** - Returns GLAccount reporting rows from Restaurant365 OData.

### item

Manage item

- **`restaurant365-odata-pp-cli item`** - Returns Item reporting rows from Restaurant365 OData.

### job-title

Manage job title

- **`restaurant365-odata-pp-cli job-title`** - Returns JobTitle reporting rows from Restaurant365 OData.

### labor-detail

Manage labor detail

- **`restaurant365-odata-pp-cli labor-detail`** - Returns LaborDetail reporting rows from Restaurant365 OData.

### location

Manage location

- **`restaurant365-odata-pp-cli location`** - Returns Location reporting rows from Restaurant365 OData.

### metadata

Manage metadata

- **`restaurant365-odata-pp-cli metadata`** - Returns the service metadata document describing available Restaurant365 OData views and fields.

### payroll-summary

Manage payroll summary

- **`restaurant365-odata-pp-cli payroll-summary`** - Returns PayrollSummary reporting rows from Restaurant365 OData.

### posemployee

Manage posemployee

- **`restaurant365-odata-pp-cli posemployee`** - Returns POSEmployee reporting rows from Restaurant365 OData.

### sales-detail

Manage sales detail

- **`restaurant365-odata-pp-cli sales-detail`** - Returns SalesDetail reporting rows from Restaurant365 OData.

### sales-employee

Manage sales employee

- **`restaurant365-odata-pp-cli sales-employee`** - Returns SalesEmployee reporting rows from Restaurant365 OData.

### sales-payment

Manage sales payment

- **`restaurant365-odata-pp-cli sales-payment`** - Returns SalesPayment reporting rows from Restaurant365 OData.

### transaction

Manage transaction

- **`restaurant365-odata-pp-cli transaction`** - Returns Transaction reporting rows from Restaurant365 OData.

### transaction-detail

Manage transaction detail

- **`restaurant365-odata-pp-cli transaction-detail`** - Returns TransactionDetail reporting rows from Restaurant365 OData.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
restaurant365-odata-pp-cli company

# JSON for scripting and agents
restaurant365-odata-pp-cli company --json

# Filter to specific fields
restaurant365-odata-pp-cli company --json --select id,name,status

# Dry run — show the request without sending
restaurant365-odata-pp-cli company --dry-run

# Agent mode — JSON + compact + no prompts in one flag
restaurant365-odata-pp-cli company --agent
```

## Automation Usage

This CLI is designed for scripted and non-interactive use:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Automation-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
restaurant365-odata-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/restaurant365-odata-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RESTAURANT365_ODATA_USERNAME` | per_call | Yes | Restaurant365 OData username, commonly `domain\username`. |
| `RESTAURANT365_ODATA_PASSWORD` | per_call | Yes | Restaurant365 OData password. |
| `RESTAURANT365_ODATA_BASE_URL` | per_call | No | Defaults to `https://odata.restaurant365.net/api/v2/views`. |
| `R365_ODATA_USERNAME` | per_call | No | Short alias for `RESTAURANT365_ODATA_USERNAME`. |
| `R365_ODATA_PASSWORD` | per_call | No | Short alias for `RESTAURANT365_ODATA_PASSWORD`. |
| `R365_ODATA_BASE_URL` | per_call | No | Short alias for `RESTAURANT365_ODATA_BASE_URL`. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `restaurant365-odata-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `restaurant365-odata-pp-cli doctor` to check credentials
- Verify both credential variables are set without printing values: `test -n "$RESTAURANT365_ODATA_USERNAME" && test -n "$RESTAURANT365_ODATA_PASSWORD"`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `restaurant365-odata-pp-cli list-views --agent` to see supported views

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
