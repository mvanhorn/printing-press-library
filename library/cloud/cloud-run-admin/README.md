# Google Cloud Run CLI

**A focused Cloud Run Admin API CLI with agent-native output, local inventory, and command discovery.**

This CLI wraps the Cloud Run Admin API directly for services, jobs, executions, tasks, operations, revisions, and IAM helpers. It complements gcloud by giving agents a smaller API-shaped surface with JSON-first output, field selection, local sync, search, analytics, and workflow commands.

Learn more at [Google Cloud Run](https://google.com).

## Install

The recommended path installs both the `cloud-run-admin-pp-cli` binary and the `pp-cloud-run-admin` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cloud-run-admin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cloud-run-admin --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.25+):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/cloud-run-admin/cmd/cloud-run-admin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cloud-run-admin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cloud-run-admin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cloud-run-admin --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cloud-run-admin skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cloud-run-admin. The skill defines how its required CLI can be installed.
```

## Authentication

Cloud Run Admin uses Google OAuth bearer tokens with the cloud-platform scope. For local use, run `gcloud auth print-access-token` and export the value as `CLOUD_RUN_ADMIN_OAUTH2C`; do not commit the token or paste it into docs.

## Quick Start

```bash
# Use an existing gcloud login as the bearer token source.
export CLOUD_RUN_ADMIN_OAUTH2C="$(gcloud auth print-access-token)"


# Find the exact command path for a Cloud Run task.
cloud-run-admin-pp-cli which "list services" --json


# List services with selected fields for agent-friendly output.
cloud-run-admin-pp-cli services list projects/PROJECT_ID/locations/REGION --json --select services.name,services.uri

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native Cloud Run operations
- **`services list`** — Lists Cloud Run services through the Admin API with agent-friendly JSON, compact output, and field selection.

  _Use this when an agent needs a compact inventory of Cloud Run services in a project and region._

  ```bash
  cloud-run-admin-pp-cli services list projects/PROJECT_ID/locations/REGION --agent --select services.name,services.uri,nextPageToken
  ```

### Local state and analysis
- **`sync`** — Syncs Cloud Run resources into a local SQLite store so services, jobs, revisions, executions, tasks, and operations can be searched without repeating every API call.

  _Use this before offline search, SQL inspection, or repeated multi-resource analysis._

  ```bash
  cloud-run-admin-pp-cli sync --json
  ```
- **`search`** — Searches synced Cloud Run inventory locally or falls back to live API calls depending on the data-source mode.

  _Use this when you need to locate Cloud Run resources by remembered fragments._

  ```bash
  cloud-run-admin-pp-cli search "api" --type services --json --select resource_type,title
  ```
- **`analytics`** — Runs local analysis against synced Cloud Run data for inventory and status-oriented questions.

  _Use this after sync when the question is about trends or inventory health rather than a single resource._

  ```bash
  cloud-run-admin-pp-cli analytics --json
  ```

### Workflow status checks
- **`workflow`** — Provides compound workflows that combine multiple Cloud Run Admin API operations into one operator-facing flow.

  _Use this when the task spans more than one Cloud Run resource type._

  ```bash
  cloud-run-admin-pp-cli workflow --help
  ```

## Usage

Run `cloud-run-admin-pp-cli --help` for the full command reference and flag list.

## Commands

### cloud-run-admin-jobs

Manage cloud run admin jobs

- **`cloud-run-admin-pp-cli cloud-run-admin-jobs create`** - Creates a Job.
- **`cloud-run-admin-pp-cli cloud-run-admin-jobs list`** - Lists Jobs.
- **`cloud-run-admin-pp-cli cloud-run-admin-jobs run`** - Triggers creation of a new Execution of this Job.

### executions

Manage executions


### operations

Manage operations

- **`cloud-run-admin-pp-cli operations list`** - Lists operations that match the specified filter in the request. If the server doesn't support this method, it returns `UNIMPLEMENTED`.
- **`cloud-run-admin-pp-cli operations wait`** - Waits until the specified long-running operation is done or reaches at most a specified timeout, returning the latest state. If the operation is already done, the latest state is immediately returned. If the timeout specified is greater than the default HTTP/RPC timeout, the HTTP/RPC timeout is used. If the server does not support this method, it returns `google.rpc.Code.UNIMPLEMENTED`. Note that this method is on a best-effort basis. It may return the latest state before the specified timeout (including immediately), meaning even an immediate response is no guarantee that the operation is done.

### services

Manage services

- **`cloud-run-admin-pp-cli services create`** - Creates a new Service in a given project and location.
- **`cloud-run-admin-pp-cli services get-iam-policy`** - Gets the IAM Access Control policy currently in effect for the given Cloud Run Service. This result does not include any inherited policies.
- **`cloud-run-admin-pp-cli services list`** - Lists Services.
- **`cloud-run-admin-pp-cli services patch`** - Updates a Service.
- **`cloud-run-admin-pp-cli services set-iam-policy`** - Sets the IAM Access control policy for the specified Service. Overwrites any existing policy.
- **`cloud-run-admin-pp-cli services test-iam-permissions`** - Returns permissions that a caller has on the specified Project. There are no permissions required for making this API call.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cloud-run-admin-pp-cli cloud-run-admin-jobs list mock-value

# JSON for scripting and agents
cloud-run-admin-pp-cli cloud-run-admin-jobs list mock-value --json

# Filter to specific fields
cloud-run-admin-pp-cli cloud-run-admin-jobs list mock-value --json --select id,name,status

# Dry run — show the request without sending
cloud-run-admin-pp-cli cloud-run-admin-jobs list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cloud-run-admin-pp-cli cloud-run-admin-jobs list mock-value --agent
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
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cloud-run-admin -g
```

Then invoke `/pp-cloud-run-admin <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/cloud-run-admin/cmd/cloud-run-admin-pp-mcp@latest
```

Then register it:

```bash
claude mcp add cloud-run-admin cloud-run-admin-pp-mcp -e CLOUD_RUN_ADMIN_OAUTH2C=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cloud-run-admin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CLOUD_RUN_ADMIN_OAUTH2C` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/cloud-run-admin/cmd/cloud-run-admin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cloud-run-admin": {
      "command": "cloud-run-admin-pp-mcp",
      "env": {
        "CLOUD_RUN_ADMIN_OAUTH2C": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
cloud-run-admin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cloud-run-admin-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLOUD_RUN_ADMIN_OAUTH2C` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cloud-run-admin-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLOUD_RUN_ADMIN_OAUTH2C`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Cloud Run returns authentication or permission errors** — Refresh the token with `gcloud auth print-access-token`, export `CLOUD_RUN_ADMIN_OAUTH2C`, and confirm the account has Cloud Run Viewer or a role with the required Admin API permission.
- **A command asks for `parent` or `name`** — Use Google resource names such as `projects/PROJECT_ID/locations/REGION` for parent list calls and full resource names for get/update/delete calls.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**kameshsampath/drone-gcloud-run**](https://github.com/kameshsampath/drone-gcloud-run) — Go (2 stars)
- [**gcloud run**](https://cloud.google.com/sdk/gcloud/reference/run/) — Python
- [**Google Cloud Run Admin API REST reference**](https://docs.cloud.google.com/run/docs/reference/rest) — REST
- [**cloud.google.com/go/run/apiv2**](https://docs.cloud.google.com/go/docs/reference/cloud.google.com/go/run/latest/apiv2) — Go
- [**lpmourato/c9s**](https://github.com/lpmourato/c9s) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
