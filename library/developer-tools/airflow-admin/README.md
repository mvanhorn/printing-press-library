# Airflow Admin CLI

Apache Airflow is an open-source workflow orchestrator used by data teams to schedule and monitor data pipelines. A pipeline is usually modeled as a DAG: a set of tasks with dependencies, retries, schedules, and run history. Data engineers use Airflow to keep warehouse loads, API extracts, dbt jobs, reports, and machine-learning workflows running on time.

This CLI gives engineers and agents a read-first way to inspect an Airflow environment from the terminal. It can list DAGs, inspect recent DAG runs, check task instance failures, review pools, verify Airflow health, and sync useful metadata into a local SQLite store for search and analysis. It is not a replacement for the Airflow UI; it is a compact operations surface for answering questions such as "what failed overnight?", "which tasks are still retrying?", and "is the scheduler healthy?" without opening the browser.

Printed by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

## Install

The recommended path installs both the `airflow-admin-pp-cli` binary and the `pp-airflow-admin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install airflow-admin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install airflow-admin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install airflow-admin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install airflow-admin --agent claude-code
npx -y @mvanhorn/printing-press-library install airflow-admin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/airflow-admin/cmd/airflow-admin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airflow-admin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airflow-admin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airflow-admin --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-airflow-admin skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-airflow-admin. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airflow-admin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AIRFLOW_ADMIN_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/airflow-admin/cmd/airflow-admin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "airflow-admin": {
      "command": "airflow-admin-pp-mcp",
      "env": {
        "AIRFLOW_ADMIN_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Point the CLI at Airflow

The default base URL is `http://localhost:8080`, which matches a common local Airflow setup. For another environment, set `AIRFLOW_ADMIN_BASE_URL`:

```bash
export AIRFLOW_ADMIN_BASE_URL="https://airflow.example.com"
```

### 3. Get and store an Airflow token

For a local Airflow instance using the default demo credentials, request a JWT token from Airflow:

```bash
airflow-admin-pp-cli apache-airflow-admin-auth \
  --username airflow \
  --password airflow \
  --json
```

Copy the `access_token` value from the response and store it:

```bash
airflow-admin-pp-cli auth set-token <access_token>
```

You can also provide the token only for the current shell:

```bash
export AIRFLOW_ADMIN_BEARER_AUTH="<access_token>"
```

Do not commit tokens or Airflow passwords. Keep them in your shell environment, local config file, or your MCP client's secret prompt.

### 4. Verify setup

```bash
airflow-admin-pp-cli doctor
```

This checks your configuration and credentials.

### 5. Try the common operations commands

```bash
airflow-admin-pp-cli apache-airflow-admin-version
airflow-admin-pp-cli monitor
airflow-admin-pp-cli dags list
```

## Usage

Run `airflow-admin-pp-cli --help` for the full command reference and flag list.

## Commands

### apache-airflow-admin-auth

Manage apache airflow admin auth

- **`airflow-admin-pp-cli apache-airflow-admin-auth`** - Authenticate with an Airflow username and password and return a JWT access token.

### apache-airflow-admin-version

Manage apache airflow admin version

- **`airflow-admin-pp-cli apache-airflow-admin-version`** - Get Airflow version

### connections

Airflow connection metadata.

- **`airflow-admin-pp-cli connections`** - List connections

### dags

DAG inventory and metadata.

- **`airflow-admin-pp-cli dags get`** - Get DAG
- **`airflow-admin-pp-cli dags list`** - List DAGs
- **`airflow-admin-pp-cli dags dag-runs list`** - List DAG runs for a DAG
- **`airflow-admin-pp-cli dags dag-runs get`** - Get a single DAG run
- **`airflow-admin-pp-cli dags dag-runs list-task-instances`** - List task instances for a DAG run
- **`airflow-admin-pp-cli dags dag-runs get-task-instance`** - Get one task instance
- **`airflow-admin-pp-cli dags tasks list`** - List tasks in a DAG
- **`airflow-admin-pp-cli dags details`** - Get detailed DAG metadata

### monitor

Manage monitor

- **`airflow-admin-pp-cli monitor`** - Get health

### pools

Airflow pool capacity and occupancy.

- **`airflow-admin-pp-cli pools get`** - Get pool
- **`airflow-admin-pp-cli pools list`** - List pools

### variables

Airflow variable metadata.

- **`airflow-admin-pp-cli variables`** - List variables

## Data Engineering Workflows

Check whether Airflow itself is healthy:

```bash
airflow-admin-pp-cli monitor --json
airflow-admin-pp-cli apache-airflow-admin-version --json
```

List active DAGs and keep the output small for a script or agent:

```bash
airflow-admin-pp-cli dags list --json --select dag_id,is_active,is_paused,last_parsed_time
```

Inspect failed DAG runs for a pipeline:

```bash
airflow-admin-pp-cli dags dag-runs list daily_warehouse_refresh \
  --state failed \
  --start-date-gte 2026-06-01T00:00:00Z \
  --json \
  --select dag_id,dag_run_id,state,start_date,end_date
```

Inspect failed task instances inside a DAG run:

```bash
airflow-admin-pp-cli dags dag-runs list-task-instances \
  daily_warehouse_refresh \
  scheduled__2026-06-14T00:00:00+00:00 \
  --state failed \
  --json \
  --select task_id,state,try_number,start_date,end_date
```

Sync metadata locally, then search it without repeatedly paging the API:

```bash
airflow-admin-pp-cli sync --resources dags,pools --json
airflow-admin-pp-cli search "failed" --data-source local --json --limit 20
airflow-admin-pp-cli analytics count --type dag-runs
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
airflow-admin-pp-cli apache-airflow-admin-version

# JSON for scripting and agents
airflow-admin-pp-cli apache-airflow-admin-version --json

# Filter to specific fields
airflow-admin-pp-cli apache-airflow-admin-version --json --select id,name,status

# Dry run — show the request without sending
airflow-admin-pp-cli apache-airflow-admin-version --dry-run

# Agent mode — JSON + compact + no prompts in one flag
airflow-admin-pp-cli apache-airflow-admin-version --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
airflow-admin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/airflow-admin-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AIRFLOW_ADMIN_BASE_URL` | connection | No | Airflow webserver/API base URL. Defaults to `http://localhost:8080`. |
| `AIRFLOW_ADMIN_BEARER_AUTH` | credential | Yes | Airflow JWT access token, without the `Bearer` prefix. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `airflow-admin-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `airflow-admin-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AIRFLOW_ADMIN_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
