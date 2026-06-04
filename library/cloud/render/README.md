# Render CLI

**Every Render endpoint, plus diff, drift, cost, audit, and orphan analytics no other Render tool ships.**

render-pp-cli is the only Render CLI that lets you reason about your full Render footprint offline. It absorbs every command from the official CLI, MCP server, and Terraform provider, then adds the analytical primitives the dashboard refuses to: env-var diff/promote across environments, blueprint drift vs live state, cost rollups grouped by project, preview cleanup by age, and incident timelines merging deploys + events + audit logs.

Learn more at [Render](https://community.render.com).

Created by [@giacaglia](https://github.com/giacaglia) (Giuliano Giacaglia).

## Install

The recommended path installs both the `render-pp-cli` binary and the `pp-render` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install render
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install render --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install render --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install render --agent claude-code
npx -y @mvanhorn/printing-press-library install render --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/render/cmd/render-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/render-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-render --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-render --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install render --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/render-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RENDER_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "render": {
      "command": "render-pp-mcp",
      "env": {
        "RENDER_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Render uses bearer-token auth. Set RENDER_API_KEY (generate one at https://dashboard.render.com/u/settings#api-keys), or run `render-pp-cli auth set-token <RENDER_API_KEY>` to paste it interactively. Every command sends `Authorization: Bearer <key>`; doctor verifies the key is live.

## Quick Start

```bash
# Paste your RENDER_API_KEY (or set the env var).
render-pp-cli auth set-token <RENDER_API_KEY>

# Confirm auth + reachability before doing anything else.
render-pp-cli doctor

# Populate the local SQLite store so cross-resource commands work offline.
render-pp-cli sync

# Smoke-check that data made it into the cache.
render-pp-cli services list --json

# First analytical command — flags any dashboard-edited resources.
render-pp-cli drift --blueprint render.yaml

# Friday cost rollup.
render-pp-cli cost --group-by project --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`env diff`** — Compare env vars between any two services, env-groups, or a service vs an env-group, three-way diff style.

  _Reach for this when promoting config across environments — it's the cheapest way to see what's actually different without click-through._

  ```bash
  render-pp-cli env diff srv-staging-api srv-prod-api --json
  ```
- **`env promote`** — Copy env vars from a source service or env-group to a target with --only/--exclude/--dry-run.

  _Use when shipping a known-good staging config to prod without dragging dev-only keys along._

  ```bash
  render-pp-cli env promote --from srv-staging-api --to srv-prod-api --exclude DEBUG_TOKEN --dry-run
  ```
- **`drift`** — Compare a checked-in render.yaml against live workspace state and report added/removed/modified entities by name.

  _Run this in CI to fail builds when someone edits prod through the dashboard instead of the blueprint._

  ```bash
  render-pp-cli drift --blueprint render.yaml --json
  ```
- **`cost`** — Sum monthly cost across services, postgres, key-value, redis, and disks; group by project, environment, or owner.

  _Reach for this on the Friday cost-review or when finance asks 'what would happen if we downsized these three services?'_

  ```bash
  render-pp-cli cost --group-by project --json
  ```
- **`rightsize`** — For each service, fetch last --since 7d of CPU and memory; compute p95 utilization; flag services breaching --high or --low thresholds against plan capacity.

  _Run this before the cost rollup to find quick downsizing wins or services about to throttle._

  ```bash
  render-pp-cli rightsize --since 7d --high 80 --low 20 --json
  ```

### Lifecycle hygiene
- **`preview-cleanup`** — List preview services older than --stale-days N; delete in bulk on --confirm.

  _Use when the monthly bill spikes and you suspect orphaned preview envs._

  ```bash
  render-pp-cli preview-cleanup --stale-days 14 --confirm
  ```
- **`orphans`** — Find unattached disks, empty env-groups, custom-domains pointing at deleted services, registry credentials referenced by no service, and disk snapshots beyond retention.

  _Audit cleanup, billing review, and SOC 2 evidence-gathering all start here._

  ```bash
  render-pp-cli orphans --json
  ```

### Agent-native plumbing
- **`env where`** — For a given env-var key, list every service and env-group that defines it with hash, length, and last-modified timestamp (never raw value).

  _First call during a credential rotation or a 'who uses this secret' audit._

  ```bash
  render-pp-cli env where STRIPE_KEY --json
  ```
- **`incident-timeline`** — Merge deploys, events, and audit-logs for one service into one chronological table for a given window.

  _First command on-call should run during a 2am page after a deploy._

  ```bash
  render-pp-cli incident-timeline srv-checkout-api --since 2h --json
  ```
- **`audit search`** — FTS5 search across cached audit logs by actor, target, action, and time window.

  _Use during SOC 2 evidence-gathering or to answer 'who rotated this key last quarter'._

  ```bash
  render-pp-cli audit search --actor riley@example.com --target STRIPE_KEY --since 30d --json
  ```
- **`deploys diff`** — Show commit range, env-var changes, image-tag changes, and plan/region/scale changes between two deploys of one service.

  _First command in a post-deploy regression triage — answers 'what actually changed'._

  ```bash
  render-pp-cli deploys diff srv-checkout-api dep-aaa dep-bbb --json
  ```

## Usage

Run `render-pp-cli --help` for the full command reference and flag list.

## Commands

### blueprints

[Blueprints](https://render.com/docs/infrastructure-as-code) allow you to define your resources in a `render.yaml` file and automatically sync changes to your Render services.
The API gives control over how your Blueprints are used to create and manage resources.

- **`render-pp-cli blueprints disconnect`** - Disconnect the Blueprint with the provided ID.

Disconnecting a Blueprint stops automatic resource syncing via the associated `render.yaml` file. It does not _delete_ any services or other resources that were managed by the blueprint.
- **`render-pp-cli blueprints list`** - List Blueprints for the specified workspaces. If no workspaces are provided, returns all Blueprints the API key has access to.
- **`render-pp-cli blueprints retrieve`** - Retrieve the Blueprint with the provided ID.
- **`render-pp-cli blueprints update`** - Update the Blueprint with the provided ID.
- **`render-pp-cli blueprints validate`** - Validate a `render.yaml` Blueprint file without creating or modifying any resources. This endpoint checks the syntax and structure of the Blueprint, validates that all required fields are present, and returns a plan indicating the resources that would be created.

Requests to this endpoint use `Content-Type: multipart/form-data`. The provided Blueprint file cannot exceed 10MB in size.

### cron-jobs

Manage cron jobs

### disks

[Disks](https://render.com/docs/disks) allow you to attach persistent storage to your services.

- **`render-pp-cli disks add`** - Attach a persistent disk to a web service, private service, or background worker.

The service must be redeployed for the disk to be attached.
- **`render-pp-cli disks delete`** - Delete a persistent disk attached to a service.

**All data on the disk will be lost.** The disk's associated service will immediately lose access to it.
- **`render-pp-cli disks list`** - List persistent disks matching the provided filters. If no filters are provided, returns all disks you have permissions to view.
- **`render-pp-cli disks retrieve`** - Retrieve the persistent disk with the provided ID.
- **`render-pp-cli disks update`** - Update the persistent disk with the provided ID.

The disk's associated service must be deployed and active for updates to take effect.

When resizing a disk, the new size must be greater than the current size.

### env-groups

Manage env groups

- **`render-pp-cli env-groups create`** - Create a new environment group.
- **`render-pp-cli env-groups delete`** - Delete the environment group with the provided ID, including all environment variables and secret files it contains.
- **`render-pp-cli env-groups list`** - List environment groups matching the provided filters. If no filters are provided, all environment groups are returned.
- **`render-pp-cli env-groups retrieve`** - Retrieve an environment group by ID.
- **`render-pp-cli env-groups update`** - Update the attributes of an environment group.

### environments

Manage environments

- **`render-pp-cli environments create`** - Create a new environment belonging to the project with the provided ID.
- **`render-pp-cli environments delete`** - Delete the environment with the provided ID.

Requires the environment to be empty (i.e., it must contain no services or other resources). Otherwise, deletion fails with a `409` response.

To delete a non-empty environment, do one of the following:
- First move or delete all contained services and other resources.
- Delete the environment in the [Render Dashboard](https://dashboard.render.com).
- **`render-pp-cli environments list`** - List a particular project's environments matching the provided filters. If no filters are provided, all environments are returned.
- **`render-pp-cli environments retrieve`** - Retrieve the environment with the provided ID.
- **`render-pp-cli environments update`** - Update the details of the environment with the provided ID.

### events

View events for a service, postgres or key value

- **`render-pp-cli events retrieve`** - Retrieve the details of a particular event

### key-value

[Key Value](https://render.com/docs/key-value) allows you to interact with your Render Key Value instances.

- **`render-pp-cli key-value create`** - Create a new Key Value instance.
- **`render-pp-cli key-value delete`** - Delete a Key Value instance by ID.
- **`render-pp-cli key-value list`** - List Key Value instances matching the provided filters. If no filters are provided, all Key Value instances are returned.
- **`render-pp-cli key-value retrieve`** - Retrieve a Key Value instance by ID.
- **`render-pp-cli key-value update`** - Update a Key Value instance by ID.

### logs

[Logs](https://render.com/docs/logging) allow you to retrieve logs for your services, Postgres databases, and redis instances.
You can query for logs or subscribe to logs in real-time via a websocket.

- **`render-pp-cli logs delete-owner-stream`** - Removes the log stream for the specified workspace.
- **`render-pp-cli logs delete-resource-stream`** - Removes the log stream override for the specified resource. After deletion, the resource will use the workspace's default log stream setting.
- **`render-pp-cli logs get-owner-stream`** - Returns log stream information for the specified workspace.
- **`render-pp-cli logs get-resource-stream`** - Returns log stream override information for the specified resource. A log stream override takes precedence over a workspace's default log stream.
- **`render-pp-cli logs list`** - List logs matching the provided filters. Logs are paginated by start and end timestamps.
There are more logs to fetch if `hasMore` is true in the response. Provide the `nextStartTime`
and `nextEndTime` timestamps as the `startTime` and `endTime` query parameters to fetch the next page of logs.

You can query for logs across multiple resources, but all resources must be in the same region and belong to the same owner.
- **`render-pp-cli logs list-resource-streams`** - Lists log stream overrides for the provided workspace that match the provided filters. These overrides take precedence over the workspace's default log stream.
- **`render-pp-cli logs list-values`** - List all values for a given log label in the logs matching the provided filters.
- **`render-pp-cli logs subscribe`** - Open a websocket connection to subscribe to logs matching the provided filters. Logs are streamed in real-time as they are generated.

You can query for logs across multiple resources, but all resources must be in the same region and belong to the same owner.
- **`render-pp-cli logs update-owner-stream`** - Updates log stream information for the specified workspace. All logs for resources owned by this workspace will be sent to this log stream unless overridden by individual resources.
- **`render-pp-cli logs update-resource-stream`** - Updates log stream override information for the specified resource. A log stream override takes precedence over a workspace's default log stream.

### maintenance

The `Maintenance` endpoints allow you to retrieve the latest maintenance runs for your Render services. You can also reschedule maintenance or trigger it to start immediately.

- **`render-pp-cli maintenance list`** - List scheduled and/or recent maintenance runs for specified resources.
- **`render-pp-cli maintenance retrieve`** - Retrieve the maintenance run with the provided ID.
- **`render-pp-cli maintenance update`** - Update the maintenance run with the provided ID.

Updates from this endpoint are asynchronous. To check your update's status, use the [Retrieve maintenance run](https://api-docs.render.com/reference/retrieve-maintenance) endpoint.

### metrics

The `Metrics` endpoints allow you to retrieve metrics for your services, Postgres databases, and redis instances.

- **`render-pp-cli metrics get-active-connections`** - Get the number of active connections for one or more Postgres databases or Redis instances.
- **`render-pp-cli metrics get-bandwidth`** - Get bandwidth usage for one or more resources.
- **`render-pp-cli metrics get-bandwidth-sources`** - Get bandwidth usage for one or more resources broken down by traffic source (HTTP, WebSocket, NAT, PrivateLink).

Returns hourly data points with traffic source breakdown. Traffic source data is available from March 9, 2025 onwards.
Queries for earlier dates will return a 400 Bad Request error.
- **`render-pp-cli metrics get-cpu`** - Get CPU usage for one or more resources.
- **`render-pp-cli metrics get-cpu-limit`** - Get the CPU limit for one or more resources.
- **`render-pp-cli metrics get-cpu-target`** - Get CPU target for one or more resources.
- **`render-pp-cli metrics get-disk-capacity`** - Get persistent disk capacity for one or more resources.
- **`render-pp-cli metrics get-disk-usage`** - Get persistent disk usage for one or more resources.
- **`render-pp-cli metrics get-http-latency`** - Get HTTP latency metrics for one or more resources.
- **`render-pp-cli metrics get-http-requests`** - Get the HTTP request count for one or more resources.
- **`render-pp-cli metrics get-instance-count`** - Get the instance count for one or more resources.
- **`render-pp-cli metrics get-memory`** - Get memory usage for one or more resources.
- **`render-pp-cli metrics get-memory-limit`** - Get the memory limit for one or more resources.
- **`render-pp-cli metrics get-memory-target`** - Get memory target for one or more resources.
- **`render-pp-cli metrics get-replication-lag`** - Get seconds of replica lag of a Postgres replica.
- **`render-pp-cli metrics get-task-runs-completed`** - Get the total number of task runs completed for one or more tasks. Optionally filter by state (succeeded/failed) or aggregate by state.
- **`render-pp-cli metrics get-task-runs-queued`** - Get the total number of task runs queued for one or more tasks.
- **`render-pp-cli metrics list-application-filter-values`** - List instance values to filter by for one or more resources.
- **`render-pp-cli metrics list-http-filter-values`** - List status codes and host values to filter by for one or more resources.
- **`render-pp-cli metrics list-path-filter-values`** - The path suggestions are based on the most recent 5000 log lines as filtered by the provided filters

### metrics-stream

Manage metrics stream

- **`render-pp-cli metrics-stream delete-owner`** - Deletes the metrics stream for the specified workspace.
- **`render-pp-cli metrics-stream get-owner`** - Returns metrics stream information for the specified workspace.
- **`render-pp-cli metrics-stream upsert-owner`** - Creates or updates the metrics stream for the specified workspace.

### notification-settings

[Notification Settings](https://render.com/docs/notifications) allow you to configure which notifications you want to recieve, and
where you will receive them.

- **`render-pp-cli notification-settings list-notification-overrides`** - List notification overrides matching the provided filters. If no filters are provided, returns all notification overrides for all workspaces the user belongs to.
- **`render-pp-cli notification-settings patch-owner`** - Update notification settings for the owner with the provided ID.
- **`render-pp-cli notification-settings patch-service-notification-overrides`** - Update the notification override for the service with the provided ID.
- **`render-pp-cli notification-settings retrieve-owner`** - Retrieve notification settings for the owner with the provided ID.

Note that you provide an owner ID to this endpoint, not the ID for a particular resource.
- **`render-pp-cli notification-settings retrieve-service-notification-overrides`** - Retrieve the notification override for the service with the provided ID.

Note that you provide a service ID to this endpoint, not the ID for a particular override.

### organizations

Manage organizations

### owners

Manage owners

- **`render-pp-cli owners list`** - List the workspaces that your API key has access to, optionally filtered by name or owner email address.
- **`render-pp-cli owners retrieve`** - Retrieve the workspace with the provided ID.

Workspace IDs start with `tea-`. If you provide a user ID (starts with `own-`), this endpoint returns the user's default workspace.

### postgres

[Postgres](https://render.com/docs/postgresql) endpoints enable you to interact with your Render Postgres databases.
You can manage databases, exports, recoveries, and failovers.

- **`render-pp-cli postgres create`** - Create a new Postgres instance.
- **`render-pp-cli postgres delete`** - Delete a Postgres instance by ID. This operation is irreversible, and
all data will be lost.
- **`render-pp-cli postgres list`** - List Postgres instances matching the provided filters. If no filters are provided, all Postgres instances are returned.
- **`render-pp-cli postgres retrieve`** - Retrieve a Postgres instance by ID.
- **`render-pp-cli postgres update`** - Update a Postgres instance by ID.

### projects

Manage projects

- **`render-pp-cli projects create`** - Create a new project.
- **`render-pp-cli projects delete`** - Delete the project with the provided ID.

Requires _all_ of the project's environments to be empty (i.e., they must contain no services or other resources). Otherwise, deletion fails with a `409` response.

To delete a non-empty project, do one of the following:
- First move or delete all contained services and other resources.
- Delete the project in the [Render Dashboard](https://dashboard.render.com).
- **`render-pp-cli projects list`** - List projects matching the provided filters. If no filters are provided, all projects are returned.
- **`render-pp-cli projects retrieve`** - Retrieve the project with the provided ID.
- **`render-pp-cli projects update`** - Update the details of a project.

To update the details of a particular _environment_ in the project, instead use the [Update environment](https://api-docs.render.com/reference/update-environment) endpoint.

### redis

Manage redis

- **`render-pp-cli redis create`** - Create a new Redis instance. This API is deprecated in favor of the Key Value API.
- **`render-pp-cli redis delete`** - Delete a Redis instance by ID. This API is deprecated in favor of the Key Value API.
- **`render-pp-cli redis list`** - List Redis instances matching the provided filters. If no filters are provided, all Redis instances are returned.
This API is deprecated in favor of the Key Value API.
- **`render-pp-cli redis retrieve`** - Retrieve a Redis instance by ID. This API is deprecated in favor of the Key Value API.
- **`render-pp-cli redis update`** - Update a Redis instance by ID. This API is deprecated in favor of the Key Value API.

### registrycredentials

Manage registrycredentials

- **`render-pp-cli registrycredentials create-registry-credential`** - Create a new registry credential.
- **`render-pp-cli registrycredentials delete-registry-credential`** - Delete the registry credential with the provided ID.
- **`render-pp-cli registrycredentials list-registry-credentials`** - List registry credentials matching the provided filters. If no filters are provided, returns all registry credentials you have permissions to view.
- **`render-pp-cli registrycredentials retrieve-registry-credential`** - Retrieve the registry credential with the provided ID.
- **`render-pp-cli registrycredentials update-registry-credential`** - Update the registry credential with the provided ID. Services that use this credential must be redeployed to use updated values.

### services

[Services](https://render.com/docs/service-types) allow you to manage your web services, private services, background workers, cron jobs, and static sites.

- **`render-pp-cli services create`** - Creates a new Render service in the specified workspace with the specified configuration.
- **`render-pp-cli services delete`** - Delete the service with the provided ID.
- **`render-pp-cli services list`** - List services matching the provided filters. If no filters are provided, returns all services you have permissions to view.
- **`render-pp-cli services retrieve`** - Retrieve the service with the provided ID.
- **`render-pp-cli services update`** - Update the service with the provided ID.

### task-runs

Manage task runs

- **`render-pp-cli task-runs cancel`** - Cancel a running task run with the provided ID.
- **`render-pp-cli task-runs create-task`** - Kicks off a run of the workflow task with the provided ID, passing the provided input data.
- **`render-pp-cli task-runs get`** - Retrieve the workflow task run with the provided ID.
- **`render-pp-cli task-runs list`** - List task runs that match the provided filters. If no filters are provided, all task runs accessible by the authenticated user are returned.
- **`render-pp-cli task-runs stream-events`** - Establishes a unidirectional event stream. The server sends events as lines
formatted per the SSE spec. Clients SHOULD set `Accept: text/event-stream`
and keep the connection open.

### tasks

Manage tasks

- **`render-pp-cli tasks get`** - Retrieve the workflow task with the provided ID.
- **`render-pp-cli tasks list`** - List workflow tasks that match the provided filters. If no filters are provided, all task definitions accessible by the authenticated user are returned.

### users

The `User` endpoints allow you to retrieve information about the authenticated user

- **`render-pp-cli users get`** - Retrieve the user associated with the provided API key.

### webhooks

[Webhooks](https://render.com/docs/webhooks) allows you to manage your Render webhook configuration.

- **`render-pp-cli webhooks create`** - Create a new webhook.
- **`render-pp-cli webhooks delete`** - Delete the webhook with the provided ID.
- **`render-pp-cli webhooks list`** - List webhooks
- **`render-pp-cli webhooks retrieve`** - Retrieve the webhook with the provided ID
- **`render-pp-cli webhooks update`** - Update the webhook with the provided ID.

### workflows

Manage workflows

- **`render-pp-cli workflows create`** - Create a new workflow service with the specified configuration.
- **`render-pp-cli workflows delete`** - Delete the workflow service with the provided ID.
- **`render-pp-cli workflows get`** - Retrieve the workflow service with the provided ID.
- **`render-pp-cli workflows list`** - List workflows that match the provided filters. If no filters are provided, all workflows accessible by the authenticated user are returned.
- **`render-pp-cli workflows update`** - Update the workflow service with the provided ID.

### workflowversions

Manage workflowversions

- **`render-pp-cli workflowversions create-workflow-version`** - Creates and deploys a new version of a workflow.
- **`render-pp-cli workflowversions get-workflow-version`** - Retrieve the specific workflow service version with the provided ID.
- **`render-pp-cli workflowversions list-workflow-versions`** - List known versions of the workflow service with the provided ID.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
render-pp-cli blueprints list

# JSON for scripting and agents
render-pp-cli blueprints list --json

# Filter to specific fields
render-pp-cli blueprints list --json --select id,name,status

# Dry run — show the request without sending
render-pp-cli blueprints list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
render-pp-cli blueprints list --agent
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
render-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/render-public-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RENDER_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `render-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RENDER_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every command** — Run `render-pp-cli doctor`. If it reports `auth: failed`, regenerate the API key at dashboard.render.com/u/settings#api-keys and re-export RENDER_API_KEY.
- **drift / cost / orphans return empty results** — Run `render-pp-cli sync` to populate the local cache; then re-run the command.
- **429 Too Many Requests during sync** — Pass `--rate-limit 10` to slow the cursor walker, or wait 60s. Render rate-limits are not documented but the CLI surfaces the `render-request-id` header in the error for support.
- **logs tail disconnects after a few minutes** — The /v1/logs/subscribe SSE stream times out periodically; re-run the command.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**render-oss/render-mcp-server**](https://github.com/render-oss/render-mcp-server) — Go (129 stars)
- [**render-oss/cli**](https://github.com/render-oss/cli) — Go (93 stars)
- [**render-oss/terraform-provider-render**](https://github.com/render-oss/terraform-provider-render) — Go (51 stars)
- [**kurtbuilds/render**](https://github.com/kurtbuilds/render) — Rust (20 stars)
- [**niyogi/render-mcp**](https://github.com/niyogi/render-mcp) — TypeScript (15 stars)
- [**render-oss/skills**](https://github.com/render-oss/skills) — Markdown

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
