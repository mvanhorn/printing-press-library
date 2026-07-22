# Expensify CLI

**File expenses and submit reports to Expensify in one line. Every command an agent should need, with a local cache so searches stay offline.**

expensify-pp-cli turns the Expensify web app into a terminal. Log in once, and every filing/reviewing/submitting task that used to require clicking through forms becomes a single command. A local SQLite store gives you offline search, rollups, dupe detection, and missing-receipt alerts that no other Expensify tool has.

Learn more at [Expensify](https://www.expensify.com/).

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).

## Install

The recommended path installs both the `expensify-pp-cli` binary and the `pp-expensify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install expensify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install expensify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install expensify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install expensify --agent claude-code
npx -y @mvanhorn/printing-press-library install expensify --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/expensify/cmd/expensify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/expensify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install expensify --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-expensify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-expensify --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install expensify --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/expensify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EXPENSIFY_AUTH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/expensify/cmd/expensify-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "expensify": {
      "command": "expensify-pp-mcp",
      "env": {
        "EXPENSIFY_AUTH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Two ways to authenticate: (1) `expensify auth login` opens a browser, you log in, the CLI captures your session token — works immediately for all filing/submitting commands; (2) `expensify auth set-keys` stores your Integration Server partner credentials (get them at https://www.expensify.com/tools/integrations/) — required only for export/admin commands. Most users only need option 1.

## Quick Start

```bash
# Check auth + connectivity before anything else
expensify-pp-cli doctor

# Capture your Expensify session straight from Chrome — no token paste
expensify-pp-cli auth login --from-chrome

# Pull workspaces, expenses, and reports into the local cache
expensify-pp-cli sync

# File your first expense in one line
expensify-pp-cli expense quick "Dinner at Maya $42.50"

# See the month's total expensed/pending/approved at a glance
expensify-pp-cli damage

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Filing at thought speed
- **`expense quick`** — File an expense with one line: amount, merchant, and category parsed from a short prompt — no forms, no web UI.

  _When a user tells an agent 'I just expensed dinner at Maya for $42.50,' the agent should file it in one call — not walk through six fields._

  ```bash
  expensify-pp-cli expense quick "Dinner at Maya $42.50" --agent
  ```
- **`report draft`** — Create a report and auto-attach every un-reported expense from a date range in a single command.

  _End-of-month submission turns from 45 clicks into one command._

  ```bash
  expensify-pp-cli report draft --since 2026-04-01 --title "April expenses" --policy YOUR_POLICY_ID
  ```
- **`expense from-line`** — Paste a raw bank or card CSV row and the CLI extracts date/merchant/amount/currency and files the expense.

  _Reconciling AmEx? Paste a row, file an expense. No copy-paste-copy-paste._

  ```bash
  expensify-pp-cli expense from-line "2026-04-18 DOORDASH*JOES $14.25" --category Meals
  ```

### Local state that compounds
- **`damage`** — Single-glance summary: total expensed, pending, approved, paid for the current month (or a custom range).

  _Agents asked 'how much did I expense this month' get one answer in one call._

  ```bash
  expensify-pp-cli damage --month current --json
  ```
- **`expense search`** — FTS5 search over all your expenses by merchant, comment, category, or tag. Regex-friendly.

  _Agents asked 'did I expense that Starbucks last month' get an answer in one local query._

  ```bash
  expensify-pp-cli expense search "coffee" --since 2026-01-01 --json
  ```
- **`expense missing-receipts`** — Lists expenses without attached receipts so you can catch them before submitting a report.

  _Submit-report-and-get-bounced feels bad; surface missing receipts upfront._

  ```bash
  expensify-pp-cli expense missing-receipts --json
  ```
- **`expense rollup`** — Pivot-table expenses by category, tag, or merchant for any time range.

  _Build your own spending dashboard without burning API budget._

  ```bash
  expensify-pp-cli expense rollup --month 2026-04 --by category
  ```
- **`expense dupes`** — Finds expenses that look like duplicates by (merchant, amount, date±window).

  _Accidental double-file is a top AP pain point; surface it before submission._

  ```bash
  expensify-pp-cli expense dupes --window 3d --json
  ```

### Agent-native plumbing
- **`expense bulk`** — File a whole list of expenses in a single Expense_Create request.

  _Reach for this when filing many expenses at once instead of looping create — it is one atomic request._

  ```bash
  expensify-pp-cli expense bulk --input rows.jsonl --dry-run
  ```
- **`report submit`** — Submit a report and optionally poll until it leaves SUBMITTED.

  _Use --wait when a downstream step depends on the report actually being approved/rejected, not just submitted._

  ```bash
  expensify-pp-cli report submit --report-id 1587860702457827 --wait --timeout 1h
  ```

## Recipes


### File a quick expense

```bash
expensify-pp-cli expense quick "Lunch at Joe's $18.50"
```

One line, one call, one expense. Category auto-suggested from your history.

### Draft this month's report

```bash
expensify-pp-cli report draft --since 2026-04-01 --until 2026-04-30 --title "April" --policy 1234567
```

Creates a report and attaches every un-reported April expense in a single command.

### Submit for approval and wait

```bash
expensify-pp-cli report submit --report-id 1587860702457827 --wait --timeout 1h
```

Blocks until approval arrives — drop this in CI after a closing script.

### Find expenses with no receipt

```bash
expensify-pp-cli expense missing-receipts --json
```

Catches receipt gaps before they bounce your report.

### Search your offline expense cache

```bash
expensify-pp-cli expense search "coffee" --json
```

FTS5 search over every synced expense — merchant, comment, category, tag — with no network call.

## Usage

Run `expensify-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `EXPENSIFY_CONFIG_DIR`, `EXPENSIFY_DATA_DIR`, `EXPENSIFY_STATE_DIR`, or `EXPENSIFY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `EXPENSIFY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export EXPENSIFY_HOME=/srv/expensify
expensify-pp-cli doctor
```

Under `EXPENSIFY_HOME=/srv/expensify`, the four dirs resolve to `/srv/expensify/config`, `/srv/expensify/data`, `/srv/expensify/state`, and `/srv/expensify/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "expensify": {
      "command": "expensify-pp-mcp",
      "env": {
        "EXPENSIFY_HOME": "/srv/expensify"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `EXPENSIFY_DATA_DIR` overrides an explicit `--home` for that kind. Use `EXPENSIFY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `EXPENSIFY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `expensify-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### admin

Integration Server: policy, employee, and rules admin

- **`expensify-pp-cli admin cards-list`** - List domain cards (Domain Cards Getter)
- **`expensify-pp-cli admin cards-owners`** - List card owners (Card Owner Data)
- **`expensify-pp-cli admin employee-add`** - Add an employee to a policy (Advanced Employee Updater)
- **`expensify-pp-cli admin employee-remove`** - Remove an employee from a policy
- **`expensify-pp-cli admin employee-update`** - Update an employee (Advanced Employee Updater)
- **`expensify-pp-cli admin policy-get`** - Get a policy's full config (Policy Getter)
- **`expensify-pp-cli admin policy-list`** - List all policies you admin (Policy List Getter)
- **`expensify-pp-cli admin policy-new`** - Create a new policy (Policy Creator)
- **`expensify-pp-cli admin policy-set-categories`** - Update categories for a policy from YAML
- **`expensify-pp-cli admin policy-set-fields`** - Update report fields for a policy
- **`expensify-pp-cli admin policy-set-tags`** - Update tags for a policy from YAML
- **`expensify-pp-cli admin report-set-status`** - Force a report status transition (Report Status Updater)
- **`expensify-pp-cli admin rules-new`** - Create an expense rule (Expense Rules Creator)
- **`expensify-pp-cli admin rules-update`** - Update an expense rule
- **`expensify-pp-cli admin tag-approvers-set`** - Set tag approvers (Tag Approvers Updater)

### category

Workspace categories (for expense classification)

- **`expensify-pp-cli category`** - List categories for a workspace

### expense

Create, list, and manage personal expenses

- **`expensify-pp-cli expense attach`** - Attach or replace a receipt on an expense
- **`expensify-pp-cli expense create`** - Create a new expense
- **`expensify-pp-cli expense delete`** - Delete an expense
- **`expensify-pp-cli expense edit`** - Edit an existing expense
- **`expensify-pp-cli expense get`** - Get expense detail by transaction ID
- **`expensify-pp-cli expense list`** - List your expenses with filters

### export_resource

Integration Server: export reports to accounting systems (admin)

- **`expensify-pp-cli export-resource download`** - Download a previously generated export file
- **`expensify-pp-cli export-resource run`** - Export reports via Report Exporter (Integration Server)

### me

Current user profile

- **`expensify-pp-cli me`** - Get current user profile

### recon

Integration Server: corporate card reconciliation (admin)

- **`expensify-pp-cli recon`** - Export reconciliation data for a domain

### report

Create, manage, and submit expense reports

- **`expensify-pp-cli report add`** - Add expenses to a report
- **`expensify-pp-cli report approve`** - Approve a report (manager action)
- **`expensify-pp-cli report comment`** - Add a comment to a report thread
- **`expensify-pp-cli report create`** - Create a new report
- **`expensify-pp-cli report delete`** - Delete a draft report
- **`expensify-pp-cli report get`** - Get report detail
- **`expensify-pp-cli report list`** - List your reports
- **`expensify-pp-cli report pay`** - Mark a report as reimbursed
- **`expensify-pp-cli report reopen`** - Reopen a submitted report back to draft
- **`expensify-pp-cli report submit`** - Submit a report for approval

### tag

Workspace tags (multi-level, for expense classification)

- **`expensify-pp-cli tag`** - List tags for a workspace

### workspace

View workspaces (policies) you have access to

- **`expensify-pp-cli workspace get`** - Get workspace detail
- **`expensify-pp-cli workspace list`** - List workspaces accessible to your account


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
expensify-pp-cli category --policy-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
expensify-pp-cli category --policy-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
expensify-pp-cli category --policy-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
expensify-pp-cli category --policy-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
expensify-pp-cli category --policy-id 550e8400-e29b-41d4-a716-446655440000 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
expensify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `expensify-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/expensify-pp-cli/config.toml`; `--home`, `EXPENSIFY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EXPENSIFY_AUTH_TOKEN` | per_call | Yes | Set to your API credential. |
| `EXPENSIFY_PARTNER_USER_ID` | per_call | Yes | Set to your API credential. |
| `EXPENSIFY_PARTNER_USER_SECRET` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `expensify-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `expensify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EXPENSIFY_AUTH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **`403 Forbidden` on every command** — Your session expired. Run `expensify-pp-cli auth login` again.
- **`429 Too Many Requests`** — Integration Server enforces 5 req / 10s and 20 req / 60s. The CLI backs off automatically; if you see this, wait 60 seconds before retrying.
- **`export run` asks for partner credentials** — Export commands use the Integration Server, not your session. Run `expensify-pp-cli auth set-keys` with credentials from https://www.expensify.com/tools/integrations/.
- **`expense quick` can't parse my input** — Format: `<description> <merchant> $<amount>`. Example: `"Dinner at Maya $42.50"`. Use `--amount/--merchant/--category` flags for explicit control.
- **Policy/category autocomplete is empty** — Run `expensify-pp-cli sync` to refresh the local cache.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**justexpenseit**](https://github.com/meyskens/justexpenseit) — Go (1 stars)
- [**primrose-mcp-expensify**](https://github.com/primrose-mcp/primrose-mcp-expensify) — TypeScript
- [**expensify-mcp-http**](https://github.com/agenticledger/expensify-mcp-http) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
