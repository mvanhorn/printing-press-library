# dbt Cloud CLI

**Operate dbt Cloud jobs and runs from one static Go binary — trigger, watch to completion with honest exit codes, and query run history offline.**

Covers the dbt Cloud Administrative API v2 jobs, runs, and artifacts surface, then adds the operate-and-watch loop (monitor, trigger --wait) and local run-history analytics (runs stats, failures, artifacts diff) that thin API wrappers don't. Agent-native --json output throughout.

## Install

The recommended path installs both the `dbt-cloud-pp-cli` binary and the `pp-dbt-cloud` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install dbt-cloud
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install dbt-cloud --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install dbt-cloud --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install dbt-cloud --agent claude-code
npx -y @mvanhorn/printing-press-library install dbt-cloud --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/cmd/dbt-cloud-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dbt-cloud-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install dbt-cloud --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-dbt-cloud --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-dbt-cloud --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install dbt-cloud --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/dbt-cloud-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `DBT_CLOUD_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/cmd/dbt-cloud-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "dbt-cloud": {
      "command": "dbt-cloud-pp-mcp",
      "env": {
        "DBT_CLOUD_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set DBT_CLOUD_TOKEN (a Personal Access Token or service token) and DBT_CLOUD_ACCOUNT_ID. For regional dbt Cloud, set DBT_CLOUD_HOST (default https://cloud.getdbt.com). Tokens are sent as Authorization: Bearer.

## Quick Start

```bash
# verify config and reachability before anything else
dbt-cloud-pp-cli doctor --dry-run

# see recent runs (replace 123456789 with your account_id or set DBT_CLOUD_ACCOUNT_ID)
dbt-cloud-pp-cli runs list 123456789 --limit 10 --json

# watch a run to completion
dbt-cloud-pp-cli monitor 460826060 --interval 15s

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Operate and watch
- **`monitor`** — Watch a dbt Cloud run until it finishes, with live status and an exit code that reflects success or failure.

  _Reach for this to block a script or CI step on a dbt run and get a real exit code plus failure context._

  ```bash
  dbt-cloud-pp-cli monitor 460826060 --interval 15s --json
  ```
- **`trigger`** — Trigger a job and watch the resulting run to completion in one command, returning the run's exit status.

  _Reach for this as the CI-grade run-a-job-and-fail-on-error loop._

  ```bash
  dbt-cloud-pp-cli trigger 12345 --cause 'ci deploy' --wait --json
  ```

### Local run analytics
- **`runs stats`** — Success rate, average and p95 duration, and failure counts per job over a time window from the local run mirror.

  _Reach for this for reliability and velocity review across many runs, not a single run._

  ```bash
  dbt-cloud-pp-cli runs stats --days 30 --json
  ```
- **`failures`** — Recently failed runs in a time window, grouped by job, with each run's failed-step names.

  _Reach for this to triage what broke recently across all jobs._

  ```bash
  dbt-cloud-pp-cli failures --days 1 --json
  ```
- **`artifacts diff`** — Diff run_results.json between two runs to show which models newly failed or changed timing.

  _Reach for this to hunt regressions between two runs._

  ```bash
  dbt-cloud-pp-cli artifacts diff 460826060 460900111 --json
  ```

## Recipes


### Fail CI on a bad run

```bash
dbt-cloud-pp-cli trigger 12345 --cause 'ci' --wait
```

Triggers the job and exits non-zero if the run fails.

### Narrow run output for an agent

```bash
dbt-cloud-pp-cli runs list 123456789 --limit 20 --json --select data.id,data.status_humanized,data.job_definition_id,data.duration
```

Returns only the high-gravity fields from a verbose run payload.

### Triage today's failures

```bash
dbt-cloud-pp-cli failures --days 1 --json
```

Lists failed runs in the last day with their failed-step names.

## Usage

Run `dbt-cloud-pp-cli --help` for the full command reference and flag list.

## Commands

### cloud-jobs

Manage dbt Cloud jobs

- **`dbt-cloud-pp-cli cloud-jobs create`** - Create a new job.
- **`dbt-cloud-pp-cli cloud-jobs destroy`** - Delete a job
- **`dbt-cloud-pp-cli cloud-jobs list`** - List jobs for the given account
- **`dbt-cloud-pp-cli cloud-jobs retrieve`** - Retrieve the details of a job.
- **`dbt-cloud-pp-cli cloud-jobs update`** - Update a job.

### runs

Manage runs

- **`dbt-cloud-pp-cli runs list`** - List runs for an account.
- **`dbt-cloud-pp-cli runs retrieve`** - Retrieve details of a run.

### steps

Manage steps

- **`dbt-cloud-pp-cli steps <account_id> <id>`** - Retrieve the details of a specific step of a run.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
dbt-cloud-pp-cli cloud-jobs list mock-value

# JSON for scripting and agents
dbt-cloud-pp-cli cloud-jobs list mock-value --json

# Filter to specific fields
dbt-cloud-pp-cli cloud-jobs list mock-value --json --select id,name,status

# Dry run — show the request without sending
dbt-cloud-pp-cli cloud-jobs list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
dbt-cloud-pp-cli cloud-jobs list mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
dbt-cloud-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/dbt-cloud-admin-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `DBT_CLOUD_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `dbt-cloud-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `dbt-cloud-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $DBT_CLOUD_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized** — Check DBT_CLOUD_TOKEN is a valid PAT or service token with API access.
- **404 on every call** — Set DBT_CLOUD_ACCOUNT_ID; confirm DBT_CLOUD_HOST matches your dbt Cloud region.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**dbtc**](https://github.com/dbt-labs/dbtc) — Python
- [**dbt-cloud-cli**](https://github.com/data-mie/dbt-cloud-cli) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
