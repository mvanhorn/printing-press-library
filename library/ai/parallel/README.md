# Parallel CLI

**Agent-native Parallel web research with a local SQLite memory no other Parallel CLI keeps.**

Search, extract, deep research, FindAll, monitors, and Account balance/apps/keys in one Go binary. Session stitch, research recall, and monitor digests compound every run into offline memory. Product API key auth stays separate from Account OAuth so headline search never requires a dashboard login.

## Install

The recommended path installs both the `parallel-pp-cli` binary and the `pp-parallel` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install parallel
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install parallel --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install parallel --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install parallel --agent claude-code
npx -y @mvanhorn/printing-press-library install parallel --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/parallel/cmd/parallel-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/parallel-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install parallel --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-parallel --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-parallel --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install parallel --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/parallel-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PARALLEL_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/parallel/cmd/parallel-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "parallel": {
      "command": "parallel-pp-mcp",
      "env": {
        "PARALLEL_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Product commands use PARALLEL_API_KEY via the x-api-key header. Account commands (balance, apps, keys) need an OAuth device-flow Bearer JWT (see docs.parallel.ai/integrations/account-api); a Product API key alone cannot call Account endpoints. Never commit API key values.

## Quick Start

```bash
# Verify binary health without spending credits
parallel-pp-cli doctor --dry-run

# Recall prior local research before another paid call
parallel-pp-cli research recall "Parallel Web Systems" --json --agent --select hits.source,hits.id,hits.title

# Learn how to bind search/extract/task IDs into one session
parallel-pp-cli session stitch --help

# Weekly mechanical monitor triage
parallel-pp-cli monitors digest --since 7d --json --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`session stitch`** — Bind search, extract, and task/findall runs into one local session chain for agent resume.

  _Use when an agent needs to resume a multi-step Parallel research loop without re-fetching._

  ```bash
  parallel-pp-cli session stitch --search-id search_demo --json --agent
  ```
- **`monitors digest`** — Mechanical per-monitor event counts and top titles since a duration window.

  _Use for Monday triage of which monitors fired or went quiet._

  ```bash
  parallel-pp-cli monitors digest --since 7d --json --agent
  ```
- **`research recall`** — FTS across local searches, extracts, and task summaries with typed hit IDs.

  _Use before paying for a live search when prior local research may already answer._

  ```bash
  parallel-pp-cli research recall --query "Anthropic funding" --json --agent --select hits.source,hits.id,hits.title
  ```

### Spend control
- **`tasks guard`** — Refuse Task creates when prepaid balance is below a threshold.

  _Use before expensive Task Groups when you must avoid surprise credit burn._

  ```bash
  parallel-pp-cli tasks guard --min-balance 500 --dry-run --json --agent
  ```
- **`balance burn`** — Diff local balance snapshots against local run volume over a window.

  _Use when explaining weekly credit burn without opening the dashboard._

  ```bash
  parallel-pp-cli balance burn --since 7d --json --agent
  ```

### Research pipelines
- **`findall promote`** — Turn FindAll candidates into a Task Group enrichment job.

  _Use when entity discovery should immediately become batch enrichment._

  ```bash
  parallel-pp-cli findall promote --findall-id findall_demo --limit 10 --json --agent
  ```
- **`tasks lineage`** — Print the offline previous_interaction_id follow-up chain for a run.

  _Use when debugging multi-turn deep research context chains offline._

  ```bash
  parallel-pp-cli tasks lineage trun_demo --json --agent
  ```

## Recipes

### Doctor before spend

```bash
parallel-pp-cli doctor --dry-run
```

Confirm auth wiring without calling paid endpoints

### Recall then decide

```bash
parallel-pp-cli research recall "Anthropic" --json --agent --select hits.source,hits.id,hits.title
```

Check local memory before paying for live Search

### Monitor weekly digest

```bash
parallel-pp-cli monitors digest --since 7d --json --agent
```

Mechanical triage of monitor events

### Promote FindAll to enrichment

```bash
parallel-pp-cli findall promote --findall-id findall_demo --limit 5 --json --agent
```

Entity discovery into Task Group

### Balance burn check

```bash
parallel-pp-cli balance burn --since 7d --json --agent
```

Explain weekly credit burn from local snapshots

## Usage

Run `parallel-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PARALLEL_CONFIG_DIR`, `PARALLEL_DATA_DIR`, `PARALLEL_STATE_DIR`, or `PARALLEL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PARALLEL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PARALLEL_HOME=/srv/parallel
parallel-pp-cli doctor
```

Under `PARALLEL_HOME=/srv/parallel`, the four dirs resolve to `/srv/parallel/config`, `/srv/parallel/data`, `/srv/parallel/state`, and `/srv/parallel/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "parallel": {
      "command": "parallel-pp-mcp",
      "env": {
        "PARALLEL_HOME": "/srv/parallel"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PARALLEL_DATA_DIR` overrides an explicit `--home` for that kind. Use `PARALLEL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PARALLEL_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `parallel-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### chat

Manage chat

- **`parallel-pp-cli chat`** - Chat completions.

This endpoint can be used to get realtime chat completions. It can also be used
with the Task API processors to get structured, research outputs via a chat
interface.

### extract

Extract returns excerpts or full content from one or more URLs. Inputs are a list of URLs and an optional search objective and keyword queries. The returned excerpts or full content is formatted as markdown and suitable for LLM consumption.
- Result: excerpts or full content from the URL formatted as markdown

- **`parallel-pp-cli extract`** - Extracts relevant content from specific web URLs.

The legacy Extract API reference (`/v1beta/extract` endpoint) is available
[here](https://docs.parallel.ai/api-reference/legacy/extract-beta/extract), and
migration guide is [here](https://docs.parallel.ai/extract/extract-migration-guide).

### findall

The FindAll API discovers and evaluates entities that match complex criteria from natural language objectives. Submit a high-level goal and the service automatically generates structured match conditions, discovers relevant candidates, and evaluates each against the criteria. Returns comprehensive results with detailed reasoning, citations, and confidence scores for each match decision. Streaming events and webhooks are supported.

- **`parallel-pp-cli findall cancel-run`** - Cancel a FindAll run.
- **`parallel-pp-cli findall enrich-run`** - Add an enrichment to a FindAll run.
- **`parallel-pp-cli findall entity-search`** - Return ranked entities matching a natural language objective.

This endpoint performs a best-effort search optimized for low latency. To keep
responses fast, it returns a fixed set of attributes and supports queries of
limited complexity.

For comprehensive match evaluation and enrichment, use the
[FindAll API](https://docs.parallel.ai/findall-api/findall-quickstart).
- **`parallel-pp-cli findall extend-run`** - Extend a FindAll run by adding additional matches to the current match limit.
- **`parallel-pp-cli findall get-events`** - Stream events from a FindAll run.

Args:
    request: The Shapi request
    findall_id: The FindAll run ID
    last_event_id: Optional event ID to resume from.
    timeout: Optional timeout in seconds. If None, keep connection alive
    as long as the run is going. If set, stop after specified duration.
- **`parallel-pp-cli findall get-result`** - Retrieve the FindAll run result at the time of the request.
- **`parallel-pp-cli findall get-schema`** - Get FindAll Run Schema
- **`parallel-pp-cli findall ingest-run`** - Transforms a natural language search objective into a structured FindAll spec.

The generated specification serves as a suggested starting point and can be further
customized by the user.
- **`parallel-pp-cli findall runs-v1`** - Starts a FindAll run.

This endpoint immediately returns a FindAll run object with status set to 'queued'.
You can get the run result snapshot using the GET /v1beta/findall/runs/{findall_id}/result endpoint.
You can track the progress of the run by:
- Polling the status using the GET /v1beta/findall/runs/{findall_id} endpoint,
- Subscribing to real-time updates via the /v1beta/findall/runs/{findall_id}/events
endpoint,
- Or specifying a webhook with relevant event types during run creation to receive
notifications.
- **`parallel-pp-cli findall runs-v1-get`** - Retrieve FindAll Run Status

### monitors

The Monitor API watches the web for material changes on a fixed frequency. Each monitor runs once on creation and then on its configured schedule, emitting events when meaningful changes are detected.
- `event_stream` monitors track a search query and emit an event for each new material change.
- `snapshot` monitors track a specific task run's output and emit an event when the output changes.

Results can be polled via the events endpoint or delivered via webhooks.

- **`parallel-pp-cli monitors create`** - Create a monitor.

Monitors run on a fixed frequency to detect material changes in web content.
Set `type=event_stream` to monitor a search query, or `type=snapshot` to
monitor a specific task run's output. The monitor runs once immediately at
creation, then continues on the configured schedule.
- **`parallel-pp-cli monitors list`** - List monitors ordered by creation time, newest first.

Monitors are sorted by `created_at` descending. `limit` defaults to 100.
Use `next_cursor` from the response and pass it as `cursor` to fetch the
next page. Pagination ends when `next_cursor` is absent.

By default only `active` monitors are returned. Pass `status=cancelled`
or both values to include cancelled monitors.

The legacy Monitor API (`/v1alpha/monitors` endpoints) is documented under
the `Monitor (Alpha)` tag.
- **`parallel-pp-cli monitors retrieve`** - Retrieve a monitor.

Retrieves a specific monitor by `monitor_id`. Returns the monitor
configuration including status, frequency, query, and webhook settings.

### service

Service utility endpoints

- **`parallel-pp-cli service account-add-balance`** - Charge the organization's default payment method and add the amount to the prepaid credit balance. The default payment method configured on the org's Stripe customer is always used; no payment method id is accepted from the client.
- **`parallel-pp-cli service account-create-app`** - Create a new app for the authenticated organization
- **`parallel-pp-cli service account-create-key`** - Create a new API key for an app
- **`parallel-pp-cli service account-delete-app`** - Delete an app from the authenticated organization
- **`parallel-pp-cli service account-delete-key`** - Delete an API key from an app
- **`parallel-pp-cli service account-get-balance`** - Get the authenticated organization's prepaid credit balance
- **`parallel-pp-cli service account-list-apps`** - List all apps for the authenticated organization

### tasks

The Task API executes web research and extraction tasks. Clients submit a natural-language objective with an optional input schema; the service plans retrieval, fetches relevant URLs, and returns outputs that conform to a provided or inferred JSON schema. Supports deep research style queries and can return rich structured JSON outputs. Processors trade-off between cost, latency, and quality. Each processor supports calibrated confidences.
- Output metadata: citations, excerpts, reasoning, and confidence per field

Task Groups enable batch execution of many independent Task runs with group-level monitoring and failure handling.
- Submit hundreds or thousands of Tasks as a single group
- Observe group progress and receive results as they complete
- Real-time updates via Server-Sent Events (SSE)
- Add tasks to an existing group while it is running
- Group-level retry and error aggregation

- **`parallel-pp-cli tasks runs-events-get`** - Streams events for a task run.

Returns a stream of events showing progress updates and state changes for the task
run.

For task runs that did not have enable_events set to true during creation, the
frequency of events will be reduced.
- **`parallel-pp-cli tasks runs-events-get-runs`** - Streams events for a task run.

Returns a stream of events showing progress updates and state changes for the task
run.

For task runs that did not have enable_events set to true during creation, the
frequency of events will be reduced.
- **`parallel-pp-cli tasks runs-get`** - Retrieves run status by run_id.

The run result is available from the `/result` endpoint.
- **`parallel-pp-cli tasks runs-input-get`** - Retrieves the input of a run by run_id.
- **`parallel-pp-cli tasks runs-post`** - Initiates a task run.

Returns immediately with a run object in status 'queued'.

Beta features can be enabled by setting the 'parallel-beta' header.
- **`parallel-pp-cli tasks runs-result-get`** - Retrieves a run result by run_id, blocking until the run is completed.
- **`parallel-pp-cli tasks sessions-events-get`** - Streams events from a TaskGroup: status updates and run completions.

The connection will remain open for up to an hour as long as at least one run in the
group is still active.
- **`parallel-pp-cli tasks taskgroups-get`** - Retrieves aggregated status across runs in a TaskGroup.
- **`parallel-pp-cli tasks taskgroups-post`** - Initiates a TaskGroup to group and track multiple runs.
- **`parallel-pp-cli tasks taskgroups-runs-get`** - Retrieves task runs in a TaskGroup and optionally their inputs and outputs.

All runs within a TaskGroup are returned as a stream. To get the inputs and/or
outputs back in the stream, set the corresponding `include_input` and
`include_output` parameters to `true`.

The stream is resumable using the `event_id` as the cursor. To resume a stream,
specify the `last_event_id` parameter with the `event_id` of the last event in the
stream. The stream will resume from the next event after the `last_event_id`.
- **`parallel-pp-cli tasks taskgroups-runs-id-get`** - Retrieves run status by run_id.

This endpoint is equivalent to fetching run status directly using the
`retrieve()` method or the `tasks/runs` GET endpoint.

The run result is available from the `/result` endpoint.
- **`parallel-pp-cli tasks taskgroups-runs-post`** - Initiates multiple task runs within a TaskGroup.

### websearch

Manage websearch

- **`parallel-pp-cli websearch`** - Searches the web.

The legacy Search API reference (`/v1beta/search` endpoint) is available
[here](https://docs.parallel.ai/api-reference/legacy/search-beta/search), and
migration guide is [here](https://docs.parallel.ai/search/search-migration-guide).


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`parallel-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`parallel-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`parallel-pp-cli learnings list`** - Inspect taught rows
- **`parallel-pp-cli learnings forget <query>`** - Undo a teach
- **`parallel-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`parallel-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`parallel-pp-cli teach-pattern`** - Install a query/resource template up front
- **`parallel-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PARALLEL_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `parallel-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
parallel-pp-cli monitors list

# JSON for scripting and agents
parallel-pp-cli monitors list --json

# Filter to specific fields
parallel-pp-cli monitors list --json --select id,name,status

# Dry run — show the request without sending
parallel-pp-cli monitors list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
parallel-pp-cli monitors list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
parallel-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `parallel-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/parallel-pp-cli/config.toml`; `--home`, `PARALLEL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PARALLEL_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `parallel-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `parallel-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PARALLEL_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Account balance returns 401 with API key** — Obtain an OAuth device-flow Bearer JWT for Account API; Product PARALLEL_API_KEY cannot call balance
- **Search 401/403** — Obtain an OAuth device-flow Bearer JWT for Account API; Product PARALLEL_API_KEY cannot call balance
- **Rate limited on Search** — Back off; Search defaults to 600 POST/min — prefer turbo mode and local research recall

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**parallel-agent-skills**](https://github.com/parallel-web/parallel-agent-skills) — TypeScript (59 stars)
- [**parallel-cli**](https://github.com/parallel-web/parallel-web-tools) — Python (42 stars)
- [**parallel-sdk-python**](https://github.com/parallel-web/parallel-sdk-python) — Python (26 stars)
- [**@rikalabs/parallel**](https://github.com/Rika-Labs/parallel) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
