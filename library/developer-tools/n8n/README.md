# n8n CLI

Every n8n feature from the terminal — workflow management, execution triage, GitOps sync, multi-instance health, and analytics no other tool ships.

Learn more at [n8n](https://docs.n8n.io/api/).

## Install

The recommended path installs both the `n8n-pp-cli` binary and the `pp-n8n` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install n8n
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install n8n --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/n8n-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-n8n --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-n8n --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-n8n skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-n8n. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
n8n-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export N8N_API_KEY="your-token-here"
```

### 3. Verify Setup

```bash
n8n-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
n8n-pp-cli community-packages list
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Multi-instance operations
- **`diff`** — Diff workflow inventories between two n8n instances — missing, active mismatches, name changes.

  _Use before GitOps promotions to verify staging matches production without manual inspection._

  ```bash
  n8n-pp-cli diff --target-url https://n8n-prod.example.com --target-key $PROD_KEY --json --agent
  ```
- **`health compare`** — Side-by-side workflow counts and reachability for two n8n instances.

  _Use when verifying failover readiness or multi-tenant deployment parity._

  ```bash
  n8n-pp-cli health compare --target-url https://n8n-prod.example.com --target-key $PROD_KEY --json --agent
  ```
- **`variables diff`** — Compare environment variables between two n8n instances — find missing keys or value mismatches before deployment.

  _Use as a pre-deployment gate to ensure all required variables are set on the target instance._

  ```bash
  n8n-pp-cli variables diff --target-url https://n8n-prod.example.com --target-key $PROD_KEY --hide-values --json --agent
  ```

### Local state that compounds
- **`workflows stale`** — List workflows not updated within N days — find dormant automations before they become liabilities. Requires a local sync first: `n8n-pp-cli sync`.

  _Use when auditing automation health or preparing for n8n upgrades._

  ```bash
  n8n-pp-cli workflows stale --days 90 --active --json --agent
  ```
- **`credentials audit`** — Cross-reference every credential with the workflows using it — find unused and high-impact credentials. Requires a local sync first: `n8n-pp-cli sync`.

  _Use before rotating credentials or deleting unused ones to avoid breaking active workflows._

  ```bash
  n8n-pp-cli credentials audit --show-workflows --json --agent
  ```
- **`workflows node-inventory`** — Count every n8n node type across all workflows — map your automation footprint and audit community node dependencies. Requires a local sync first: `n8n-pp-cli sync`.

  _Use before upgrading community nodes to find every workflow that would be affected._

  ```bash
  n8n-pp-cli workflows node-inventory --show-workflows --json --agent
  ```
- **`workflows deps`** — Map execution chains via executeWorkflow nodes — see what calls what and who would break if a workflow is deleted. Requires a local sync first: `n8n-pp-cli sync`.

  _Use before deleting or refactoring a workflow to find all callers in the dependency chain._

  ```bash
  n8n-pp-cli workflows deps --json --agent
  ```
- **`executions rate-check`** — Flag workflows executing above a per-minute threshold — catch infinite loops and webhook storms before they degrade instance health. Requires a local sync first: `n8n-pp-cli sync`.

  _Use when n8n instance is slow or execution queue is growing to quickly identify the culprit workflow._

  ```bash
  n8n-pp-cli executions rate-check --runaway --threshold 5 --json --agent
  ```

### Agent-native plumbing
- **`executions wait`** — Poll an execution until it reaches a terminal state — blocks CI/CD pipelines on real n8n workflow results.

  _Use in CI scripts to block a deployment until an n8n data migration or validation workflow completes. Typed exit codes: 0 = success, 1 = error/canceled, 2 = timeout (distinct from global code 2 = usage error)._

  ```bash
  n8n-pp-cli executions wait $EXEC_ID --timeout 300 --json --agent
  ```
- **`workflows bulk`** — Activate, deactivate, or archive multiple workflows at once filtered by tag or name — hours of clicking replaced by one command.

  _Use before maintenance windows or environment teardowns to disable entire workflow sets safely._

  ```bash
  n8n-pp-cli workflows bulk --action deactivate --tag staging --dry-run
  ```
- **`executions export`** — Export structured execution history filtered by workflow, status, and time window for audit logs and analytics pipelines. Requires a local sync first: `n8n-pp-cli sync`.

  _Use to feed execution audit trails into SIEM, data warehouses, or compliance reporting pipelines._

  ```bash
  n8n-pp-cli executions export --workflow abc123 --status error --since 7d --json --agent
  ```

## Usage

Run `n8n-pp-cli --help` for the full command reference and flag list.

## Commands

### audit

Operations about security audit

- **`n8n-pp-cli audit create`** - Generate a security audit for your n8n instance.

### community-packages

Operations about community packages

- **`n8n-pp-cli community-packages create`** - Install a community package by npm name and optional version.
- **`n8n-pp-cli community-packages delete`** - Uninstall a community package by name.
- **`n8n-pp-cli community-packages list`** - Retrieve all installed community packages with pending update info.
- **`n8n-pp-cli community-packages update`** - Update an installed community package to a new version.

### credentials

Operations about credentials

- **`n8n-pp-cli credentials create`** - Creates a credential that can be used by nodes of the specified type.
- **`n8n-pp-cli credentials delete`** - Deletes a credential from your instance. You must be the owner of the credentials
- **`n8n-pp-cli credentials get`** - Retrieve all credentials from your instance. Only available for the instance owner and admin. Credential data (secrets) is not included.
- **`n8n-pp-cli credentials get-id`** - Retrieves a credential by ID. Credential data (secrets) is not included.
- **`n8n-pp-cli credentials get-schema`** - Show credential data schema
- **`n8n-pp-cli credentials update`** - Updates an existing credential. You must be the owner of the credential.

### data-tables

Operations about data tables and their rows

- **`n8n-pp-cli data-tables create`** - Create a new data table in your personal project or a team project you have access to.
- **`n8n-pp-cli data-tables delete`** - Delete a data table. This will also delete all rows in the table.
- **`n8n-pp-cli data-tables get`** - Retrieve a specific data table by ID.
- **`n8n-pp-cli data-tables list`** - Retrieve a list of all data tables with optional filtering, sorting, and pagination.
- **`n8n-pp-cli data-tables update`** - Update a data table's name.

### discover

API capability discovery

- **`n8n-pp-cli discover list`** - Returns a filtered capability map based on the caller's API key scopes. Each resource includes the operations and endpoints accessible to the authenticated API key. Use query parameters to narrow the response.

### executions

Operations about executions

- **`n8n-pp-cli executions create`** - Stop multiple executions from your instance based on filter criteria.
- **`n8n-pp-cli executions delete`** - Deletes an execution from your instance.
- **`n8n-pp-cli executions get`** - Retrieve an execution from your instance.
- **`n8n-pp-cli executions list`** - Retrieve all executions from your instance.

### insights

Operations about insights

- **`n8n-pp-cli insights list`** - Retrieve the insights summary for the selected date range.

### projects

Operations about projects

- **`n8n-pp-cli projects create`** - Create a project on your instance.
- **`n8n-pp-cli projects delete`** - Delete a project from your instance.
- **`n8n-pp-cli projects list`** - Retrieve projects from your instance.
- **`n8n-pp-cli projects update`** - Update a project on your instance.

### source-control

Operations about source control

- **`n8n-pp-cli source-control create`** - Requires the Source Control feature to be licensed and connected to a repository.

### tags

Operations about tags

- **`n8n-pp-cli tags create`** - Create a tag in your instance.
- **`n8n-pp-cli tags delete`** - Deletes a tag.
- **`n8n-pp-cli tags get`** - Retrieves a tag.
- **`n8n-pp-cli tags list`** - Retrieve all tags from your instance.
- **`n8n-pp-cli tags update`** - Update a tag.

### users

Operations about users

- **`n8n-pp-cli users create`** - Create one or more users.
- **`n8n-pp-cli users delete`** - Delete a user from your instance.
- **`n8n-pp-cli users get`** - Retrieve a user from your instance. Only available for the instance owner.
- **`n8n-pp-cli users list`** - Retrieve all users from your instance. Only available for the instance owner.

### variables

Operations about variables

- **`n8n-pp-cli variables create`** - Create a variable in your instance.
- **`n8n-pp-cli variables delete`** - Delete a variable from your instance.
- **`n8n-pp-cli variables list`** - Retrieve variables from your instance.
- **`n8n-pp-cli variables update`** - Update a variable from your instance.

### workflows

Operations about workflows

- **`n8n-pp-cli workflows create`** - Create a workflow in your instance.
- **`n8n-pp-cli workflows delete`** - Delete a workflow.
- **`n8n-pp-cli workflows get`** - Retrieve a workflow.
- **`n8n-pp-cli workflows get-id`** - Retrieves a specific version of a workflow from workflow history.
- **`n8n-pp-cli workflows list`** - Retrieve all workflows from your instance.
- **`n8n-pp-cli workflows update`** - Update a workflow. If the workflow is published, the updated version will be automatically re-published.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
n8n-pp-cli community-packages list

# JSON for scripting and agents
n8n-pp-cli community-packages list --json

# Filter to specific fields
n8n-pp-cli community-packages list --json --select id,name,status

# Dry run — show the request without sending
n8n-pp-cli community-packages list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
n8n-pp-cli community-packages list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-n8n -g
```

Then invoke `/pp-n8n <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add n8n n8n-pp-mcp -e N8N_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/n8n-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `N8N_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "n8n": {
      "command": "n8n-pp-mcp",
      "env": {
        "N8N_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Cookbook

Common recipes combining multiple commands and flags.

### Pre-deployment safety check

Before promoting to production, diff the workflow inventory and compare env variables:

```bash
n8n-pp-cli diff --target-url $PROD_URL --target-key $PROD_KEY --json --agent
n8n-pp-cli variables diff --target-url $PROD_URL --target-key $PROD_KEY --hide-values --json --agent
```

### Audit dormant automations

Find workflows that haven't been touched in 90 days and cross-check their credentials:

```bash
n8n-pp-cli workflows stale --days 90 --json --agent
n8n-pp-cli credentials audit --unused --json --agent
```

### Catch runaway workflows before they degrade n8n

```bash
# Sync execution history, then find workflows firing > 10x/min in the last 30 min
n8n-pp-cli sync
n8n-pp-cli executions rate-check --window 30 --threshold 10 --runaway --json --agent
```

### Block CI until a workflow finishes

```bash
EXEC_ID=$(n8n-pp-cli executions create --workflow-id $WF_ID --json | jq -r '.id')
n8n-pp-cli executions wait $EXEC_ID --timeout 300 --json --agent
echo "workflow exit: $?"
```

### Bulk deactivate staging workflows before teardown

```bash
# Dry run first — verify the filter matches the right set
n8n-pp-cli workflows bulk --action deactivate --tag staging --dry-run
# Then apply
n8n-pp-cli workflows bulk --action deactivate --tag staging --yes
```

### Export execution failures for compliance

```bash
n8n-pp-cli executions export --status error --since 30d --json --agent \
  | jq '.[] | {id, workflow_id, started_at, duration_sec}'
```

### Map community node dependencies before an upgrade

```bash
n8n-pp-cli workflows node-inventory --node n8n-nodes-base --show-workflows --json --agent
```

### Find workflows that call each other (dependency graph)

```bash
n8n-pp-cli workflows deps --json --agent | jq '.[] | select(.calls_ids | length > 0)'
```

## Health Check

```bash
n8n-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/n8n-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `N8N_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `n8n-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $N8N_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
