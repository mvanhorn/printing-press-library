---
name: pp-render
description: "Every Render endpoint, plus diff, drift, cost, audit, and orphan analytics no other Render tool ships. Trigger phrases: `diff render env vars`, `promote env vars between render services`, `check render blueprint drift`, `render monthly cost`, `clean up stale render preview environments`, `where is this render env var used`, `render incident timeline`, `render audit log search`, `use render`, `run render-pp-cli`."
author: "Giuliano Giacaglia"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - render-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/cloud/render/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Render — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `render-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install render --cli-only
   ```
2. Verify: `render-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/render/cmd/render-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.


## When to Use This CLI

Use render-pp-cli when you need to reason about your Render footprint as a whole, not one resource at a time. It shines for env-var promotion across environments, blueprint drift checks in CI, monthly cost rollups, preview cleanup, and audit-log forensics. For one-off interactive workflows like `render psql` or `render ssh`, the official CLI is fine — render-pp-cli covers those too but its real value is the analytical commands the dashboard hides.

## Unique Capabilities

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

## Command Reference

**blueprints** — [Blueprints](https://render.com/docs/infrastructure-as-code) allow you to define your resources in a `render.yaml` file and automatically sync changes to your Render services.
The API gives control over how your Blueprints are used to create and manage resources.

- `render-pp-cli blueprints disconnect` — Disconnect the Blueprint with the provided ID. Disconnecting a Blueprint stops automatic resource syncing via the...
- `render-pp-cli blueprints list` — List Blueprints for the specified workspaces. If no workspaces are provided, returns all Blueprints the API key has...
- `render-pp-cli blueprints retrieve` — Retrieve the Blueprint with the provided ID.
- `render-pp-cli blueprints update` — Update the Blueprint with the provided ID.
- `render-pp-cli blueprints validate` — Validate a `render.yaml` Blueprint file without creating or modifying any resources. This endpoint checks the syntax...

**cron-jobs** — Manage cron jobs


**disks** — [Disks](https://render.com/docs/disks) allow you to attach persistent storage to your services.

- `render-pp-cli disks add` — Attach a persistent disk to a web service, private service, or background worker. The service must be redeployed for...
- `render-pp-cli disks delete` — Delete a persistent disk attached to a service. **All data on the disk will be lost.** The disk's associated service...
- `render-pp-cli disks list` — List persistent disks matching the provided filters. If no filters are provided, returns all disks you have...
- `render-pp-cli disks retrieve` — Retrieve the persistent disk with the provided ID.
- `render-pp-cli disks update` — Update the persistent disk with the provided ID. The disk's associated service must be deployed and active for...

**env-groups** — Manage env groups

- `render-pp-cli env-groups create` — Create a new environment group.
- `render-pp-cli env-groups delete` — Delete the environment group with the provided ID, including all environment variables and secret files it contains.
- `render-pp-cli env-groups list` — List environment groups matching the provided filters. If no filters are provided, all environment groups are returned.
- `render-pp-cli env-groups retrieve` — Retrieve an environment group by ID.
- `render-pp-cli env-groups update` — Update the attributes of an environment group.

**environments** — Manage environments

- `render-pp-cli environments create` — Create a new environment belonging to the project with the provided ID.
- `render-pp-cli environments delete` — Delete the environment with the provided ID. Requires the environment to be empty (i.e., it must contain no services...
- `render-pp-cli environments list` — List a particular project's environments matching the provided filters. If no filters are provided, all environments...
- `render-pp-cli environments retrieve` — Retrieve the environment with the provided ID.
- `render-pp-cli environments update` — Update the details of the environment with the provided ID.

**events** — View events for a service, postgres or key value

- `render-pp-cli events <eventId>` — Retrieve the details of a particular event

**key-value** — [Key Value](https://render.com/docs/key-value) allows you to interact with your Render Key Value instances.

- `render-pp-cli key-value create` — Create a new Key Value instance.
- `render-pp-cli key-value delete` — Delete a Key Value instance by ID.
- `render-pp-cli key-value list` — List Key Value instances matching the provided filters. If no filters are provided, all Key Value instances are...
- `render-pp-cli key-value retrieve` — Retrieve a Key Value instance by ID.
- `render-pp-cli key-value update` — Update a Key Value instance by ID.

**logs** — [Logs](https://render.com/docs/logging) allow you to retrieve logs for your services, Postgres databases, and redis instances.
You can query for logs or subscribe to logs in real-time via a websocket.

- `render-pp-cli logs delete-owner-stream` — Removes the log stream for the specified workspace.
- `render-pp-cli logs delete-resource-stream` — Removes the log stream override for the specified resource. After deletion, the resource will use the workspace's...
- `render-pp-cli logs get-owner-stream` — Returns log stream information for the specified workspace.
- `render-pp-cli logs get-resource-stream` — Returns log stream override information for the specified resource. A log stream override takes precedence over a...
- `render-pp-cli logs list` — List logs matching the provided filters. Logs are paginated by start and end timestamps. There are more logs to...
- `render-pp-cli logs list-resource-streams` — Lists log stream overrides for the provided workspace that match the provided filters. These overrides take...
- `render-pp-cli logs list-values` — List all values for a given log label in the logs matching the provided filters.
- `render-pp-cli logs subscribe` — Open a websocket connection to subscribe to logs matching the provided filters. Logs are streamed in real-time as...
- `render-pp-cli logs update-owner-stream` — Updates log stream information for the specified workspace. All logs for resources owned by this workspace will be...
- `render-pp-cli logs update-resource-stream` — Updates log stream override information for the specified resource. A log stream override takes precedence over a...

**maintenance** — The `Maintenance` endpoints allow you to retrieve the latest maintenance runs for your Render services. You can also reschedule maintenance or trigger it to start immediately.

- `render-pp-cli maintenance list` — List scheduled and/or recent maintenance runs for specified resources.
- `render-pp-cli maintenance retrieve` — Retrieve the maintenance run with the provided ID.
- `render-pp-cli maintenance update` — Update the maintenance run with the provided ID. Updates from this endpoint are asynchronous. To check your update's...

**metrics** — The `Metrics` endpoints allow you to retrieve metrics for your services, Postgres databases, and redis instances.

- `render-pp-cli metrics get-active-connections` — Get the number of active connections for one or more Postgres databases or Redis instances.
- `render-pp-cli metrics get-bandwidth` — Get bandwidth usage for one or more resources.
- `render-pp-cli metrics get-bandwidth-sources` — Get bandwidth usage for one or more resources broken down by traffic source (HTTP, WebSocket, NAT, PrivateLink)....
- `render-pp-cli metrics get-cpu` — Get CPU usage for one or more resources.
- `render-pp-cli metrics get-cpu-limit` — Get the CPU limit for one or more resources.
- `render-pp-cli metrics get-cpu-target` — Get CPU target for one or more resources.
- `render-pp-cli metrics get-disk-capacity` — Get persistent disk capacity for one or more resources.
- `render-pp-cli metrics get-disk-usage` — Get persistent disk usage for one or more resources.
- `render-pp-cli metrics get-http-latency` — Get HTTP latency metrics for one or more resources.
- `render-pp-cli metrics get-http-requests` — Get the HTTP request count for one or more resources.
- `render-pp-cli metrics get-instance-count` — Get the instance count for one or more resources.
- `render-pp-cli metrics get-memory` — Get memory usage for one or more resources.
- `render-pp-cli metrics get-memory-limit` — Get the memory limit for one or more resources.
- `render-pp-cli metrics get-memory-target` — Get memory target for one or more resources.
- `render-pp-cli metrics get-replication-lag` — Get seconds of replica lag of a Postgres replica.
- `render-pp-cli metrics get-task-runs-completed` — Get the total number of task runs completed for one or more tasks. Optionally filter by state (succeeded/failed) or...
- `render-pp-cli metrics get-task-runs-queued` — Get the total number of task runs queued for one or more tasks.
- `render-pp-cli metrics list-application-filter-values` — List instance values to filter by for one or more resources.
- `render-pp-cli metrics list-http-filter-values` — List status codes and host values to filter by for one or more resources.
- `render-pp-cli metrics list-path-filter-values` — The path suggestions are based on the most recent 5000 log lines as filtered by the provided filters

**metrics-stream** — Manage metrics stream

- `render-pp-cli metrics-stream delete-owner` — Deletes the metrics stream for the specified workspace.
- `render-pp-cli metrics-stream get-owner` — Returns metrics stream information for the specified workspace.
- `render-pp-cli metrics-stream upsert-owner` — Creates or updates the metrics stream for the specified workspace.

**notification-settings** — [Notification Settings](https://render.com/docs/notifications) allow you to configure which notifications you want to recieve, and
where you will receive them.

- `render-pp-cli notification-settings list-notification-overrides` — List notification overrides matching the provided filters. If no filters are provided, returns all notification...
- `render-pp-cli notification-settings patch-owner` — Update notification settings for the owner with the provided ID.
- `render-pp-cli notification-settings patch-service-notification-overrides` — Update the notification override for the service with the provided ID.
- `render-pp-cli notification-settings retrieve-owner` — Retrieve notification settings for the owner with the provided ID. Note that you provide an owner ID to this...
- `render-pp-cli notification-settings retrieve-service-notification-overrides` — Retrieve the notification override for the service with the provided ID. Note that you provide a service ID to this...

**organizations** — Manage organizations


**owners** — Manage owners

- `render-pp-cli owners list` — List the workspaces that your API key has access to, optionally filtered by name or owner email address.
- `render-pp-cli owners retrieve` — Retrieve the workspace with the provided ID. Workspace IDs start with `tea-`. If you provide a user ID (starts with...

**postgres** — [Postgres](https://render.com/docs/postgresql) endpoints enable you to interact with your Render Postgres databases.
You can manage databases, exports, recoveries, and failovers.

- `render-pp-cli postgres create` — Create a new Postgres instance.
- `render-pp-cli postgres delete` — Delete a Postgres instance by ID. This operation is irreversible, and all data will be lost.
- `render-pp-cli postgres list` — List Postgres instances matching the provided filters. If no filters are provided, all Postgres instances are returned.
- `render-pp-cli postgres retrieve` — Retrieve a Postgres instance by ID.
- `render-pp-cli postgres update` — Update a Postgres instance by ID.

**projects** — Manage projects

- `render-pp-cli projects create` — Create a new project.
- `render-pp-cli projects delete` — Delete the project with the provided ID. Requires _all_ of the project's environments to be empty (i.e., they must...
- `render-pp-cli projects list` — List projects matching the provided filters. If no filters are provided, all projects are returned.
- `render-pp-cli projects retrieve` — Retrieve the project with the provided ID.
- `render-pp-cli projects update` — Update the details of a project. To update the details of a particular _environment_ in the project, instead use the...

**redis** — Manage redis

- `render-pp-cli redis create` — Create a new Redis instance. This API is deprecated in favor of the Key Value API.
- `render-pp-cli redis delete` — Delete a Redis instance by ID. This API is deprecated in favor of the Key Value API.
- `render-pp-cli redis list` — List Redis instances matching the provided filters. If no filters are provided, all Redis instances are returned....
- `render-pp-cli redis retrieve` — Retrieve a Redis instance by ID. This API is deprecated in favor of the Key Value API.
- `render-pp-cli redis update` — Update a Redis instance by ID. This API is deprecated in favor of the Key Value API.

**registrycredentials** — Manage registrycredentials

- `render-pp-cli registrycredentials create-registry-credential` — Create a new registry credential.
- `render-pp-cli registrycredentials delete-registry-credential` — Delete the registry credential with the provided ID.
- `render-pp-cli registrycredentials list-registry-credentials` — List registry credentials matching the provided filters. If no filters are provided, returns all registry...
- `render-pp-cli registrycredentials retrieve-registry-credential` — Retrieve the registry credential with the provided ID.
- `render-pp-cli registrycredentials update-registry-credential` — Update the registry credential with the provided ID. Services that use this credential must be redeployed to use...

**services** — [Services](https://render.com/docs/service-types) allow you to manage your web services, private services, background workers, cron jobs, and static sites.

- `render-pp-cli services create` — Creates a new Render service in the specified workspace with the specified configuration.
- `render-pp-cli services delete` — Delete the service with the provided ID.
- `render-pp-cli services list` — List services matching the provided filters. If no filters are provided, returns all services you have permissions...
- `render-pp-cli services retrieve` — Retrieve the service with the provided ID.
- `render-pp-cli services update` — Update the service with the provided ID.

**task-runs** — Manage task runs

- `render-pp-cli task-runs cancel` — Cancel a running task run with the provided ID.
- `render-pp-cli task-runs create-task` — Kicks off a run of the workflow task with the provided ID, passing the provided input data.
- `render-pp-cli task-runs get` — Retrieve the workflow task run with the provided ID.
- `render-pp-cli task-runs list` — List task runs that match the provided filters. If no filters are provided, all task runs accessible by the...
- `render-pp-cli task-runs stream-events` — Establishes a unidirectional event stream. The server sends events as lines formatted per the SSE spec. Clients...

**tasks** — Manage tasks

- `render-pp-cli tasks get` — Retrieve the workflow task with the provided ID.
- `render-pp-cli tasks list` — List workflow tasks that match the provided filters. If no filters are provided, all task definitions accessible by...

**users** — The `User` endpoints allow you to retrieve information about the authenticated user

- `render-pp-cli users` — Retrieve the user associated with the provided API key.

**webhooks** — [Webhooks](https://render.com/docs/webhooks) allows you to manage your Render webhook configuration.

- `render-pp-cli webhooks create` — Create a new webhook.
- `render-pp-cli webhooks delete` — Delete the webhook with the provided ID.
- `render-pp-cli webhooks list` — List webhooks
- `render-pp-cli webhooks retrieve` — Retrieve the webhook with the provided ID
- `render-pp-cli webhooks update` — Update the webhook with the provided ID.

**workflows** — Manage workflows

- `render-pp-cli workflows create` — Create a new workflow service with the specified configuration.
- `render-pp-cli workflows delete` — Delete the workflow service with the provided ID.
- `render-pp-cli workflows get` — Retrieve the workflow service with the provided ID.
- `render-pp-cli workflows list` — List workflows that match the provided filters. If no filters are provided, all workflows accessible by the...
- `render-pp-cli workflows update` — Update the workflow service with the provided ID.

**workflowversions** — Manage workflowversions

- `render-pp-cli workflowversions create-workflow-version` — Creates and deploys a new version of a workflow.
- `render-pp-cli workflowversions get-workflow-version` — Retrieve the specific workflow service version with the provided ID.
- `render-pp-cli workflowversions list-workflow-versions` — List known versions of the workflow service with the provided ID.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
render-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Promote staging env to prod, exclude debug keys

```bash
render-pp-cli env promote --from srv-staging --to srv-prod --exclude DEBUG_TOKEN,SENTRY_DEV_DSN
```

First diff to see what would change, then promote with the dev-only keys filtered out. Add --apply after review to actually issue the writes.

### Find every service that uses a secret before rotating it

```bash
render-pp-cli env where STRIPE_KEY --json --select 'service.name,service.id,updatedAt'
```

Returns service names, IDs, and timestamps for every place STRIPE_KEY is referenced — your rotation checklist.

### Friday cost rollup grouped by project

```bash
render-pp-cli cost --group-by project --json --select 'project,monthly_usd,resource_count'
```

Joins cached service/postgres/kv/disk plans against a curated price table; export to CSV for finance.

### Detect blueprint drift in CI

```bash
render-pp-cli drift --blueprint render.yaml --json --quiet || exit 1
```

Exits non-zero when the live workspace diverges from the checked-in blueprint; wire it into your CI pipeline.

### Incident timeline for a service over the last 2 hours

```bash
render-pp-cli incident-timeline srv-checkout-api --since 2h --json --select 'timestamp,kind,actor,summary'
```

Merges deploys + events + audit-logs into one chronological table — answers 'what changed' during an on-call page.

## Auth Setup

Render uses bearer-token auth. Set RENDER_API_KEY (generate one at https://dashboard.render.com/u/settings#api-keys), or run `render-pp-cli auth set-token <RENDER_API_KEY>` to paste it interactively. Every command sends `Authorization: Bearer <key>`; doctor verifies the key is live.

Run `render-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  render-pp-cli blueprints list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
render-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
render-pp-cli feedback --stdin < notes.txt
render-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.render-pp-cli/feedback.jsonl`. They are never POSTed unless `RENDER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RENDER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
render-pp-cli profile save briefing --json
render-pp-cli --profile briefing blueprints list
render-pp-cli profile list --json
render-pp-cli profile show briefing
render-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `render-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add render-pp-mcp -- render-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which render-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   render-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `render-pp-cli <command> --help`.
