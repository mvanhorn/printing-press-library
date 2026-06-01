# Google Cloud Run CLI

**Every Cloud Run service, job, and revision -- plus a deployment gate, exposure audit, and config diff that gcloud never shipped.**

Replaces multi-tab Console sessions and fragile gcloud scripts with a single agent-native CLI. Syncs your entire Cloud Run fleet to a local SQLite store for offline queries, cross-project inventory, and zero-lag CI/CD primitives. Ships the wait-for-traffic gate, public exposure audit, and revision diff workflows that power users have been scripting by hand for years.

Learn more at [Google Cloud Run](https://google.com).

## Install

The recommended path installs both the `google-cloud-run-pp-cli` binary and the `pp-google-cloud-run` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install google-cloud-run
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install google-cloud-run --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install google-cloud-run --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install google-cloud-run --agent claude-code
npx -y @mvanhorn/printing-press-library install google-cloud-run --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/google-cloud-run/cmd/google-cloud-run-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-cloud-run-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-cloud-run --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-cloud-run --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-google-cloud-run skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-google-cloud-run. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-cloud-run-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLE_CLOUD_RUN_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/google-cloud-run/cmd/google-cloud-run-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-cloud-run": {
      "command": "google-cloud-run-pp-mcp",
      "env": {
        "GOOGLE_CLOUD_RUN_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Cloud Run uses OAuth2 with the cloud-platform scope. Set GOOGLE_CLOUD_RUN_TOKEN to a bearer token from gcloud auth print-access-token. Falls back to Application Default Credentials (ADC) -- run gcloud auth application-default login for automatic refresh.

## Quick Start

```bash
# verify config and auth before making API calls
google-cloud-run-pp-cli doctor --dry-run

# sync your Cloud Run fleet to local SQLite
google-cloud-run-pp-cli sync

# list services in a project and region
google-cloud-run-pp-cli services list projects/my-project/locations/us-central1 --json

# check the last 7 executions of a job
google-cloud-run-pp-cli executions summary --job nightly-etl --last 7 --agent

# find publicly accessible services
google-cloud-run-pp-cli iam audit --project my-project --show-public --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Fleet-wide visibility
- **`services list-all`** — List Cloud Run services across multiple GCP projects in a single unified table.

  _Use when an agent needs a fleet-wide view of services without multiple separate list calls per project._

  ```bash
  google-cloud-run-pp-cli services list-all --projects my-proj-1,my-proj-2 --agent
  ```

### Lifecycle management
- **`revisions prune`** — Delete old revisions that are not serving any traffic, keeping only the N most recent.

  _Use to free up Cloud Run revision quota without manually identifying and deleting inactive revisions._

  ```bash
  google-cloud-run-pp-cli revisions prune --service my-service --keep-last 3 --dry-run --agent
  ```

### Deploy visibility
- **`services traffic`** — Show the current traffic split across revisions with image tag and deploy timestamp in one table.

  _Use when an agent needs to verify which image version is serving what percentage of traffic after a canary deploy._

  ```bash
  google-cloud-run-pp-cli services traffic --service my-service --agent
  ```
- **`revisions diff`** — Show a field-by-field diff between two revisions: image, CPU/memory, scaling config, and env var key names.

  _Use during incident triage to see what configuration changed between the last good and first bad revision._

  ```bash
  google-cloud-run-pp-cli revisions diff --service my-service --from my-service-00041-abc --to my-service-00042-def --agent
  ```

### Job observability
- **`executions summary`** — Show a per-execution summary of a Cloud Run Job with succeeded/failed task counts and duration.

  _Use as the git log for Cloud Run Jobs to quickly assess whether recent job executions succeeded._

  ```bash
  google-cloud-run-pp-cli executions summary --job nightly-etl --last 7 --agent
  ```

### Security posture
- **`iam audit`** — List all services in a project that are publicly accessible via allUsers or allAuthenticatedUsers IAM bindings.

  _Use for security posture checks to surface any Cloud Run service unintentionally exposed to the public internet._

  ```bash
  google-cloud-run-pp-cli iam audit --project my-project --show-public --agent
  ```
- **`iam diff`** — Compare the current IAM policy of a service against a saved snapshot to detect added or removed bindings.

  _Use for compliance audits to confirm no IAM bindings changed since the last approved baseline._

  ```bash
  google-cloud-run-pp-cli iam diff --service my-service --snapshot ./iam-baseline.json --agent
  ```

### CI/CD primitives
- **`revisions wait-traffic`** — Block until a specific revision reaches a target traffic percentage -- a CI/CD gate primitive.

  _Use in CI/CD pipelines after deploying -- prevents running smoke tests before the revision is actually serving traffic._

  ```bash
  google-cloud-run-pp-cli revisions wait-traffic --service my-service --revision my-service-00042-abc --target-pct 100 --timeout 300s
  ```

## Recipes


### CI/CD post-deploy gate

```bash
google-cloud-run-pp-cli revisions wait-traffic --service my-service --revision my-service-00042-abc --target-pct 100 --timeout 300s
```

Block the pipeline until the new revision is fully serving traffic before running smoke tests.

### Weekly revision cleanup

```bash
google-cloud-run-pp-cli revisions prune --service my-service --keep-last 5 --dry-run
```

Preview which old revisions will be deleted, then re-run without --dry-run to free up revision quota.

### Security posture check

```bash
google-cloud-run-pp-cli iam audit --project my-project --show-public --agent --select services.name,services.uri,services.offending_binding
```

Compact agent-readable list of publicly exposed services.

### Incident diff

```bash
google-cloud-run-pp-cli revisions diff --service my-service --from my-service-00041-abc --to my-service-00042-def --agent
```

Immediately see what config changed between the last good and first bad revision.

### Multi-project fleet view

```bash
google-cloud-run-pp-cli services list-all --projects prod-us,prod-eu,staging --agent --select services.name,services.region,services.uri,services.serving_revision
```

Cross-project service inventory with fields an agent needs for deploy decisions.

## Usage

Run `google-cloud-run-pp-cli --help` for the full command reference and flag list.

## Commands

### cloud-run-admin-jobs

Manage cloud run admin jobs

- **`google-cloud-run-pp-cli cloud-run-admin-jobs create`** - Creates a Job.
- **`google-cloud-run-pp-cli cloud-run-admin-jobs list`** - Lists Jobs.
- **`google-cloud-run-pp-cli cloud-run-admin-jobs run`** - Triggers creation of a new Execution of this Job.

### executions

Manage executions


### operations

Manage operations

- **`google-cloud-run-pp-cli operations list`** - Lists operations that match the specified filter in the request. If the server doesn't support this method, it returns `UNIMPLEMENTED`.
- **`google-cloud-run-pp-cli operations wait`** - Waits until the specified long-running operation is done or reaches at most a specified timeout, returning the latest state. If the operation is already done, the latest state is immediately returned. If the timeout specified is greater than the default HTTP/RPC timeout, the HTTP/RPC timeout is used. If the server does not support this method, it returns `google.rpc.Code.UNIMPLEMENTED`. Note that this method is on a best-effort basis. It may return the latest state before the specified timeout (including immediately), meaning even an immediate response is no guarantee that the operation is done.

### services

Manage services

- **`google-cloud-run-pp-cli services create`** - Creates a new Service in a given project and location.
- **`google-cloud-run-pp-cli services get-iam-policy`** - Gets the IAM Access Control policy currently in effect for the given Cloud Run Service. This result does not include any inherited policies.
- **`google-cloud-run-pp-cli services list <parent>`** - Lists Services. Requires a positional `<parent>` argument: `projects/<PROJECT>/locations/<REGION>`.
- **`google-cloud-run-pp-cli services patch`** - Updates a Service.
- **`google-cloud-run-pp-cli services set-iam-policy`** - Sets the IAM Access control policy for the specified Service. Overwrites any existing policy.
- **`google-cloud-run-pp-cli services test-iam-permissions`** - Returns permissions that a caller has on the specified Project. There are no permissions required for making this API call.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-cloud-run-pp-cli cloud-run-admin-jobs list mock-value

# JSON for scripting and agents
google-cloud-run-pp-cli cloud-run-admin-jobs list mock-value --json

# Filter to specific fields
google-cloud-run-pp-cli cloud-run-admin-jobs list mock-value --json --select id,name,status

# Dry run — show the request without sending
google-cloud-run-pp-cli cloud-run-admin-jobs list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-cloud-run-pp-cli cloud-run-admin-jobs list mock-value --agent
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
google-cloud-run-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cloud-run-admin-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOOGLE_CLOUD_RUN_TOKEN` | per_call | No | Set to your API credential. |
| `GOOGLE_CLOUD_RUN_OAUTH2C` | per_call | No | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `google-cloud-run-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-cloud-run-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOGLE_CLOUD_RUN_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on all API calls** — Run gcloud auth print-access-token and set GOOGLE_CLOUD_RUN_TOKEN, or run gcloud auth application-default login.
- **404 Not Found on service or job operations** — Check --project and --region flags match where the resource was created.
- **revisions wait-traffic times out** — Increase --timeout; check Cloud Console for stuck traffic migration or a failing health check.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**GoogleCloudPlatform/cloud-run-mcp**](https://github.com/GoogleCloudPlatform/cloud-run-mcp) — TypeScript
- [**JulienBreux/run-cli**](https://github.com/JulienBreux/run-cli) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
