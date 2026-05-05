# Trigger.dev CLI

Monitor runs, trigger tasks, manage schedules, and detect failures via the Trigger.dev API

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/trigger-dev-current).

## Quick Start

### 1. Set Up Credentials

Get your secret key from [Trigger.dev Settings](https://cloud.trigger.dev) and store it:

```bash
trigger-dev-pp-cli auth set-token YOUR_SECRET_KEY
```

Or set it via environment variable:

```bash
export TRIGGER_SECRET_KEY="tr_dev_..."
```

### 2. Verify Setup

```bash
trigger-dev-pp-cli doctor
```

### 3. Sync and Explore

```bash
# Sync runs and schedules to local SQLite for fast search
trigger-dev-pp-cli sync

# See health metrics for every task
trigger-dev-pp-cli health

# Watch for failures in real time
trigger-dev-pp-cli watch --notify
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`watch`** - Monitor runs in real time and get instant desktop notifications when tasks fail
- **`failures`** - Automatically identify recurring failure patterns across tasks, error types, and time periods
- **`health`** - See success rates, durations, costs, and failure trends for every task in one view
- **`costs`** - Track spending by task, time period, and machine type to find cost spikes before they hit your bill
- **`schedules stale`** - Find schedules that stopped producing runs or have high failure rates
- **`queues bottleneck`** - Identify queues with growing backlogs and concurrency limits causing delays
- **`envvars diff`** - Compare environment variables across dev, staging, and prod to catch drift

## Usage

```
trigger-dev-pp-cli [command]

Available Commands:
  api         Browse all API endpoints by interface name
  auth        Manage authentication tokens
  batches     Retrieve batch status and item counts
  costs       Analyze run costs by task, time period, and machine type
  deployments Get the most recent deployment
  doctor      Check CLI health
  envvars     List environment variables for a project environment
  export      Export data to JSONL or JSON for backup, migration, or analysis
  failures    Analyze failure patterns across tasks and time periods
  health      Task health dashboard with success rates, durations, and cost trends
  import      Import data from JSONL file via API create/upsert calls
  load        Show workload distribution per assignee
  orphans     Find items missing key fields like assignee or project
  queues      List all queues with running and queued counts
  runs        List runs with filtering by status, task, tags, date range
  schedules   List all schedules
  search      Full-text search across synced data or live API
  stale       Find items with no updates in N days
  sync        Sync API data to local SQLite for offline search and analysis
  watch       Monitor runs in real time and alert on failures
  workflow    Compound workflows that combine multiple API operations
```

## Commands

### Runs and Tasks

Task execution and triggering

- **`trigger-dev-pp-cli runs`** - List runs with filtering by status, task, tags, date range
- **`trigger-dev-pp-cli runs get <runId>`** - Retrieve detailed run with payload, output, attempts, related runs
- **`trigger-dev-pp-cli runs cancel <runId>`** - Cancel a running or queued run
- **`trigger-dev-pp-cli runs replay <runId>`** - Replay a completed or failed run
- **`trigger-dev-pp-cli runs reschedule <runId>`** - Reschedule a delayed run with a new delay
- **`trigger-dev-pp-cli runs add-tags <runId>`** - Add tags to a run (max 10 tags, 128 chars each)
- **`trigger-dev-pp-cli runs update-metadata <runId>`** - Update run metadata JSON
- **`trigger-dev-pp-cli runs timeline`** - Visual ASCII timeline of run starts, completions, and failures
- **`trigger-dev-pp-cli tasks trigger <taskIdentifier>`** - Trigger a task with a JSON payload
- **`trigger-dev-pp-cli tasks batch-trigger <taskIdentifier>`** - Batch trigger multiple runs of a task
- **`trigger-dev-pp-cli batches get <batchId>`** - Retrieve batch status and item counts

### Scheduling

Cron-based scheduled task execution

- **`trigger-dev-pp-cli schedules`** - List all schedules
- **`trigger-dev-pp-cli schedules create`** - Create a new cron schedule for a task
- **`trigger-dev-pp-cli schedules get <scheduleId>`** - Retrieve a specific schedule
- **`trigger-dev-pp-cli schedules update <scheduleId>`** - Update a schedule's cron, timezone, or external ID
- **`trigger-dev-pp-cli schedules activate <scheduleId>`** - Activate a paused schedule
- **`trigger-dev-pp-cli schedules deactivate <scheduleId>`** - Pause a schedule
- **`trigger-dev-pp-cli schedules delete <scheduleId>`** - Delete a schedule permanently
- **`trigger-dev-pp-cli schedules timezones`** - List supported IANA timezones
- **`trigger-dev-pp-cli schedules stale`** - Find schedules that stopped producing runs

### Queues and Infrastructure

Queue management and concurrency control

- **`trigger-dev-pp-cli queues`** - List all queues with running and queued counts
- **`trigger-dev-pp-cli queues get <queueName>`** - Retrieve a specific queue's details
- **`trigger-dev-pp-cli queues pause <queueName>`** - Pause a queue (stops processing new runs)
- **`trigger-dev-pp-cli queues resume <queueName>`** - Resume a paused queue
- **`trigger-dev-pp-cli queues override-concurrency <queueName>`** - Override the concurrency limit
- **`trigger-dev-pp-cli queues reset-concurrency <queueName>`** - Reset concurrency to default
- **`trigger-dev-pp-cli queues bottleneck`** - Identify queues with growing backlogs
- **`trigger-dev-pp-cli deployments`** - Get the most recent deployment

### Environment Variables

Environment variable management across dev, staging, and prod

- **`trigger-dev-pp-cli envvars <env>`** - List environment variables for a project environment
- **`trigger-dev-pp-cli envvars create`** - Create an environment variable
- **`trigger-dev-pp-cli envvars delete`** - Delete an environment variable
- **`trigger-dev-pp-cli envvars import`** - Import environment variables in bulk
- **`trigger-dev-pp-cli envvars diff <env1> <env2>`** - Compare environment variables between two environments

### Waitpoints

Waitpoint tokens for human-in-the-loop approval workflows

- **`trigger-dev-pp-cli waitpoints`** - List waitpoint tokens
- **`trigger-dev-pp-cli waitpoints create`** - Create a waitpoint token for human approval
- **`trigger-dev-pp-cli waitpoints get <tokenId>`** - Retrieve a waitpoint token
- **`trigger-dev-pp-cli waitpoints complete <tokenId>`** - Complete a waitpoint token with approval output

### Analytics and Observability

- **`trigger-dev-pp-cli health`** - Task health dashboard with success rates, durations, and cost trends
- **`trigger-dev-pp-cli failures`** - Analyze failure patterns across tasks and time periods
- **`trigger-dev-pp-cli costs`** - Track spending by task, time period, and machine type
- **`trigger-dev-pp-cli watch`** - Monitor runs in real time and alert on failures
- **`trigger-dev-pp-cli query execute`** - Execute a TRQL query (SQL-like syntax against runs and metrics)

### Utilities

- **`trigger-dev-pp-cli doctor`** - Check CLI health and credentials
- **`trigger-dev-pp-cli sync`** - Sync API data to local SQLite for offline search and analysis
- **`trigger-dev-pp-cli search <query>`** - Full-text search across synced data or live API
- **`trigger-dev-pp-cli export`** - Export data to JSONL or JSON for backup
- **`trigger-dev-pp-cli import`** - Import data from JSONL file
- **`trigger-dev-pp-cli stale`** - Find items with no updates in N days
- **`trigger-dev-pp-cli workflow archive`** - Sync all resources to local store for offline access

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
trigger-dev-pp-cli runs --status FAILED

# JSON for scripting and agents
trigger-dev-pp-cli runs --json

# Filter to specific fields
trigger-dev-pp-cli runs --json --select id,taskIdentifier,status

# CSV for spreadsheets
trigger-dev-pp-cli runs --csv

# Compact mode for minimal token usage
trigger-dev-pp-cli health --compact

# Dry run - show the request without sending
trigger-dev-pp-cli runs --dry-run

# Agent mode - JSON + compact + no prompts in one flag
trigger-dev-pp-cli runs --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | trigger-dev-pp-cli tasks trigger my-task --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add trigger-dev trigger-dev-pp-mcp -e TRIGGER_SECRET_KEY=<your-token>
```

### Claude Desktop

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

## Cookbook

```bash
# Trigger a task with a JSON payload
trigger-dev-pp-cli tasks trigger send-welcome-email \
  --payload '{"userId": "user_abc123", "template": "onboarding"}'

# Trigger with delay and concurrency key
trigger-dev-pp-cli tasks trigger process-invoice \
  --payload '{"invoiceId": "inv_456"}' \
  --options-delay 30m \
  --options-concurrency-key "invoice-processing"

# List failed runs from the last 24 hours
trigger-dev-pp-cli runs --status FAILED --filter-created-at-period 1d

# Get full details on a specific run
trigger-dev-pp-cli runs get run_1234abc --json

# Cancel a stuck run
trigger-dev-pp-cli runs cancel run_1234abc

# Replay a failed run
trigger-dev-pp-cli runs replay run_1234abc

# See failure patterns over the last week
trigger-dev-pp-cli failures --period 7d --group-by task

# Health dashboard for a specific task
trigger-dev-pp-cli health --task send-welcome-email --period 30d

# Cost breakdown - find your most expensive tasks
trigger-dev-pp-cli costs --period 30d --top 5

# Compare env vars between staging and production
trigger-dev-pp-cli envvars diff staging prod --project proj_abc123

# Watch for failures with desktop notifications
trigger-dev-pp-cli watch --task send-welcome-email --notify --interval 5s

# Sync and search offline
trigger-dev-pp-cli sync --full
trigger-dev-pp-cli search "payment failed"

# Visual timeline of runs
trigger-dev-pp-cli runs timeline --task process-invoice --period 24h

# Pause a queue during maintenance
trigger-dev-pp-cli queues pause my-queue

# Export runs for backup
trigger-dev-pp-cli export runs --format jsonl > trigger-dev-runs-backup.jsonl
```

## Health Check

```bash
trigger-dev-pp-cli doctor
```

```
  OK Config: ok
  OK Auth: configured
  OK API: reachable
  config_path: ~/.config/trigger-dev-pp-cli/config.json
  base_url: https://api.trigger.dev
  version: 4.4
```

## Configuration

Config file: `~/.config/trigger-dev-pp-cli/config.json`

Environment variables:
- `TRIGGER_SECRET_KEY` - Primary authentication key (secret key from Trigger.dev dashboard)
- `TRIGGER_DEV_API_KEY` - Alternative API key for authentication
- `TRIGGER_DEV_BASE_URL` - Override the API base URL (default: `https://api.trigger.dev`). Useful for self-hosted Trigger.dev instances.
- `TRIGGER_DEV_CONFIG` - Override the config file path

## Self-Hosting

If you run a self-hosted Trigger.dev instance, point the CLI at your server:

```bash
export TRIGGER_DEV_BASE_URL="https://trigger.internal.company.com"
trigger-dev-pp-cli doctor
```

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `trigger-dev-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TRIGGER_SECRET_KEY`
- Get your key from [Trigger.dev Settings](https://cloud.trigger.dev)

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the list command to see available items (e.g., `trigger-dev-pp-cli runs`)

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- Use `--rate-limit 2` to cap requests per second
- If persistent, wait a few minutes and try again

**Stale local data**
- Run `trigger-dev-pp-cli sync --full` to resync everything
- Use `--data-source live` to bypass local cache entirely

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**trigger.dev CLI**](https://github.com/triggerdotdev/trigger.dev) - TypeScript (10000 stars)
- [**@trigger.dev/sdk**](https://github.com/triggerdotdev/trigger.dev) - TypeScript (10000 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `trigger-dev-pp-cli profile save <name> --<flag> <value>`, then `trigger-dev-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.trigger-dev-pp-cli/feedback.jsonl`. When `TRIGGER_DEV_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `TRIGGER_DEV_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
