# Trigger.dev CLI

**Every Trigger.dev management endpoint, plus offline FTS over runs, span-cost rollups, and zombie-schedule detection no other tool gives you.**

trigger-dev-pp-cli wraps the full v3 management API (47 endpoints across runs, schedules, deployments, batches, queues, waitpoints, env vars, and TRQL queries) and adds the cross-run aggregations the dashboard hides one click deep — LLM span cost rollups, recurring-failure patterns, real-time failure watch with desktop notifications, env-var diffs across environments, and substring grep over cached run errors.

Learn more at [Trigger.dev](https://trigger.dev).

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trigger-dev-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-cli@latest
```

## Authentication

Authentication uses a Trigger.dev secret key in the `Authorization: Bearer` header. Set `TRIGGER_SECRET_KEY` to your environment's key — `tr_dev_…` for development, `tr_prod_…` for production, `tr_pat_…` for personal access tokens. Each key is environment-scoped: a dev key cannot manage prod runs.

## Quick Start

```bash
# Validate TRIGGER_SECRET_KEY and confirm reachability.
trigger-dev-pp-cli doctor


# Cache last 7 days of runs locally so search/aggregations work offline.
trigger-dev-pp-cli sync --resources runs --since 7d


# See the recurring failure signatures across all tasks in one shell call.
trigger-dev-pp-cli failures top --since 7d --top 20 --agent


# Rank LLM cost by model+task — the report finance asks for.
trigger-dev-pp-cli runs span-cost --since 7d --by model,task --top 20


# Find zombie crons that stopped firing or whose runs all fail.
trigger-dev-pp-cli schedules stale --days 14 --agent


# Stay in your terminal — get pinged the second a failure lands.
trigger-dev-pp-cli runs watch --task my-task --notify

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Real-time terminal alerting
- **`runs watch`** — Watches runs for new failures and interrupts your terminal the moment one happens — desktop notification, sound, and a non-zero exit so it composes with shell loops.

  _Reach for this when an agent or oncall engineer needs to react to failures the second they appear, instead of polling the dashboard or waiting for an email._

  ```bash
  trigger-dev-pp-cli runs watch --task daily-digest --notify --sound
  ```

### Local state that compounds
- **`schedules stale`** — Lists schedules that stopped firing or whose recent runs have a low success rate — the zombie cron audit no other tool gives you.

  _Reach for this after an env var rotation, a Postgres URL change, or any silent-config event that might have killed crons without anyone noticing._

  ```bash
  trigger-dev-pp-cli schedules stale --days 7 --min-success-rate 0.5 --agent
  ```
- **`runs span-cost`** — Walks recent runs, surfaces the most expensive LLM spans grouped by model and task, with token totals and dollar cost — the report your finance lead actually wants.

  _Reach for this when an AI-product engineer or agent needs to pinpoint which model + task pair is eating the LLM budget, before the next billing cycle closes._

  ```bash
  trigger-dev-pp-cli runs span-cost --since 7d --by model,task --top 20 --agent --select spans.model,spans.task_identifier,spans.total_cost_cents
  ```
- **`failures top`** — Top recurring (task, error-signature) pairs over a time window — mechanical group-by, no LLM, no NLP.

  _Reach for this in an incident loop when an agent needs to find the dominant failure signature instead of reading 200 stack traces by hand._

  ```bash
  trigger-dev-pp-cli failures top --since 7d --top 20 --agent --select patterns.task_identifier,patterns.error_signature,patterns.count,patterns.last_occurred_at
  ```
- **`runs find`** — FTS5 grep over the synced runs table — error messages, tags, metadata, task identifiers — ranked by recency.

  _Reach for this when an agent has a fragment of an error message and needs the matching runs without typing TRQL._

  ```bash
  trigger-dev-pp-cli runs find "payload too large" --status FAILED --since 30d --agent
  ```

### Operator hygiene
- **`envvars diff`** — Side-by-side diff of environment variables between two environments — keys missing, keys extra, values that differ (masked).

  _Reach for this when an agent is debugging "works in staging, fails in prod" — the answer is almost always an env var, and diff makes the answer one command away._

  ```bash
  trigger-dev-pp-cli envvars diff --from prod --to staging --project proj_abc --agent
  ```

### Agent ergonomics
- **`runs get`** — runs get returns typed exit codes: 0=COMPLETED, 20=FAILED, 21=CRASHED, 22=SYSTEM_FAILURE, 23=CANCELED, 3=not-found, 4=auth-error. Cobra annotation pp:typed-exit-codes makes verify and agents read the contract directly.

  _Reach for this when an agent is writing a shell loop over many runs and wants to branch on success/failure without parsing JSON._

  ```bash
  trigger-dev-pp-cli runs get run_abc123 --json && echo COMPLETED || echo $?
  ```

## Usage

Run `trigger-dev-pp-cli --help` for the full command reference and flag list.

## Commands

### batches

Manage batches

- **`trigger-dev-pp-cli batches retrieve-batch-v1`** - Retrieve a batch by its ID, including its status and the IDs of all runs in the batch.

### deployments

Manage deployments

- **`trigger-dev-pp-cli deployments get-latest-v1`** - Retrieve information about the latest unmanaged deployment for the authenticated project.
- **`trigger-dev-pp-cli deployments get-v1`** - Retrieve information about a specific deployment by its ID.
- **`trigger-dev-pp-cli deployments list-v1`** - List deployments for the authenticated environment, ordered by most recent first.

### projects

Manage projects


### query

Manage query

- **`trigger-dev-pp-cli query execute-v1`** - Execute a TRQL (Trigger.dev Query Language) query against your run data. TRQL is a SQL-style query language that allows you to analyze runs, calculate metrics, and export data.
- **`trigger-dev-pp-cli query get-schema-v1`** - Get the schema for TRQL queries, including all available tables, their columns, data types, descriptions, and allowed values.
- **`trigger-dev-pp-cli query list-dashboards-v1`** - List available built-in dashboards with their widgets. Each dashboard contains pre-built TRQL queries for common metrics like run success rates, costs, and LLM usage.

### queues

Manage queues

- **`trigger-dev-pp-cli queues list-v1`** - List all queues in your environment with pagination support.
- **`trigger-dev-pp-cli queues retrieve-v1`** - Get a queue by its ID, or by type and name.

### runs

Manage runs

- **`trigger-dev-pp-cli runs list-v1`** - List runs in a specific environment. You can filter the runs by status, created at, task identifier, version, and more.
- **`trigger-dev-pp-cli runs retrieve-v1`** - Retrieve information about a run, including its status, payload, output, and attempts. If you authenticate with a Public API key, we will omit the payload and output fields for security reasons.

### schedules

Manage schedules

- **`trigger-dev-pp-cli schedules create-v1`** - Create a new `IMPERATIVE` schedule based on the specified options.
- **`trigger-dev-pp-cli schedules delete-v1`** - Delete a schedule by its ID. This will only work on `IMPERATIVE` schedules that were created in the dashboard or using the imperative SDK functions like `schedules.create()`.
- **`trigger-dev-pp-cli schedules get-v1`** - Get a schedule by its ID.
- **`trigger-dev-pp-cli schedules list-v1`** - List all schedules. You can also paginate the results.
- **`trigger-dev-pp-cli schedules update-v1`** - Update a schedule by its ID. This will only work on `IMPERATIVE` schedules that were created in the dashboard or using the imperative SDK functions like `schedules.create()`.

### tasks

Manage tasks

- **`trigger-dev-pp-cli tasks batch-trigger-v1`** - Batch trigger tasks with up to 1,000 payloads with SDK 4.3.1+ (500 in prior versions).

### timezones

Manage timezones

- **`trigger-dev-pp-cli timezones get-v1`** - Get all supported timezones that schedule tasks support.

### waitpoints

Manage waitpoints

- **`trigger-dev-pp-cli waitpoints complete-token-callback-v1`** - Completes a waitpoint token using the pre-signed callback URL returned in the `url` field when the token was created. No API key is required — the `callbackHash` in the URL acts as the authentication token.

This is designed to be given directly to external services (e.g. as a webhook URL) so they can unblock a waiting run without needing access to your API key. The entire request body is passed as the output data to the waiting run.

If the token is already completed, this is a no-op and returns `success: true`.
- **`trigger-dev-pp-cli waitpoints complete-token-v1`** - Completes a waitpoint token, unblocking any run that is waiting for it via `wait.forToken()`. An optional `data` payload can be passed and will be returned to the waiting run. If the token is already completed, this is a no-op and returns `success: true`.

This endpoint accepts both secret API keys and short-lived JWTs (public access tokens), making it safe to call from frontend clients.
- **`trigger-dev-pp-cli waitpoints create-token-v1`** - Creates a new waitpoint token that can be used to pause a run until an external event completes it. The token includes a `url` which can be called via HTTP POST to complete the waitpoint. Use the token ID with `wait.forToken()` inside a task to pause execution until the token is completed.
- **`trigger-dev-pp-cli waitpoints list-tokens-v1`** - Returns a paginated list of waitpoint tokens for the current environment. Results are ordered by creation date, newest first. Use cursor-based pagination with `page[after]` and `page[before]` to navigate pages.
- **`trigger-dev-pp-cli waitpoints retrieve-token-v1`** - Retrieves a waitpoint token by its ID, including its current status and output if it has been completed.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
trigger-dev-pp-cli batches mock-value

# JSON for scripting and agents
trigger-dev-pp-cli batches mock-value --json

# Filter to specific fields
trigger-dev-pp-cli batches mock-value --json --select id,name,status

# Dry run — show the request without sending
trigger-dev-pp-cli batches mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
trigger-dev-pp-cli batches mock-value --agent
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

## Cookbook

### Triage today's failures

```bash
# Top recurring failure signatures across all tasks in the last 24 hours
trigger-dev-pp-cli failures top --since 1d --top 20 --agent
```

### Find runs by error fragment (offline)

```bash
# Sync first, then grep — works without an internet connection
trigger-dev-pp-cli sync --resources runs --since 30d
trigger-dev-pp-cli runs find "payload too large" --status FAILED --since 30d --json
```

### Rank LLM cost by model and task

```bash
# Drill into the most expensive model+task pairs over the last week
trigger-dev-pp-cli runs span-cost --since 7d --by model,task --top 20 --agent
```

### Watch for new failures in your terminal

```bash
# Default behavior is plain stdout; --notify and --sound opt in to OS alerts
trigger-dev-pp-cli runs watch --task daily-digest --interval 30s --notify --sound
```

### Diff env vars across environments

```bash
# Mask values, machine-readable output for diff in CI
trigger-dev-pp-cli envvars diff --from prod --to staging --project proj_abc --json
```

### Find zombie schedules

```bash
# Schedules that haven't fired in 14 days OR whose recent runs all fail
trigger-dev-pp-cli schedules stale --days 14 --min-success-rate 0.5 --agent
```

### Run a TRQL query

```bash
# Inspect the schema first, then execute against your run data
trigger-dev-pp-cli query get-schema-v1 --json
trigger-dev-pp-cli query execute-v1 --query "SELECT task_identifier, COUNT(*) FROM runs GROUP BY task_identifier" --json
```

### Get a run with typed exit codes for shell branching

```bash
# Exit code is 0=COMPLETED, 20=FAILED, 21=CRASHED, 22=SYSTEM_FAILURE, 23=CANCELED, 3=not-found, 4=auth-error
trigger-dev-pp-cli runs get run_abc123 --json && echo "ok" || echo "exit=$?"
```

### Trigger a task

```bash
# Mutation example — pass payload via --stdin (JSON request body) or --payload
echo '{"payload": {"name": "alex"}}' | trigger-dev-pp-cli tasks trigger task-v1 my-task-id --stdin --agent
```

### Stream live changes

```bash
# Polls the API and emits NDJSON to stdout for piping into downstream tools
trigger-dev-pp-cli tail --resource runs --interval 5s --json
```

### Use the local store for offline analysis

```bash
# Sync first, then run analytics queries against the synced data offline
trigger-dev-pp-cli sync --resources runs,schedules --since 30d
trigger-dev-pp-cli analytics --json
```

### Audit unowned items

```bash
# Items missing assignee/project/priority/labels — surfaces unowned work
trigger-dev-pp-cli orphans --json
```

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-trigger-dev -g
```

Then invoke `/pp-trigger-dev <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-mcp@latest
```

Then register it:

```bash
claude mcp add trigger-dev trigger-dev-pp-mcp -e TRIGGER_SECRET_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trigger-dev-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TRIGGER_SECRET_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-trigger-dev --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-trigger-dev --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-trigger-dev skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-trigger-dev. The skill defines how its required CLI can be installed.
```

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "trigger-dev": {
      "command": "trigger-dev-pp-mcp",
      "env": {
        "TRIGGER_SECRET_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
trigger-dev-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/trigger-dev-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TRIGGER_SECRET_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `trigger-dev-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TRIGGER_SECRET_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — Set TRIGGER_SECRET_KEY to a `tr_dev_…`, `tr_prod_…`, or `tr_pat_…` key and run `trigger-dev-pp-cli doctor`. The key prefix encodes which environment you can manage.
- **403 environment mismatch when calling /api/v1/runs** — Your key is scoped to a different environment than the resource. A `tr_dev_…` key cannot list prod runs. Switch keys.
- **429 rate-limited** — Trigger.dev returns standard `Retry-After` in seconds. The CLI's adaptive limiter honors it automatically; if you're seeing this in scripts, lower concurrency or add `--limit`.
- **`runs find` returns nothing** — Run `trigger-dev-pp-cli sync --resources runs --since 30d` first — substring grep is over the local store, not the live API.
- **`schedules stale` says all schedules are stale on a fresh project** — The store is empty until sync. Run `trigger-dev-pp-cli sync --resources schedules` and `--resources runs` so the join has data.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**trigger.dev (official CLI)**](https://github.com/triggerdotdev/trigger.dev) — TypeScript (14803 stars)
- [**@trigger.dev/sdk**](https://github.com/triggerdotdev/trigger.dev/tree/main/packages/trigger-sdk) — TypeScript (14803 stars)
- [**trigger.dev MCP server**](https://github.com/triggerdotdev/trigger.dev/tree/main/packages/cli-v3/src/mcp) — TypeScript (14803 stars)
- [**Inngest CLI**](https://github.com/inngest/inngest) — Go (5093 stars)
- [**Temporal CLI**](https://github.com/temporalio/cli) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
