---
name: pp-trigger-dev
description: "Every Trigger.dev management endpoint, plus offline FTS over runs, span-cost rollups, and zombie-schedule detection no other tool gives you. Trigger phrases: `trigger.dev failed runs`, `trigger.dev cost rollup`, `trigger.dev schedule health`, `audit trigger.dev env vars`, `watch trigger.dev failures`, `use trigger-dev`, `run trigger-dev`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - trigger-dev-pp-cli
    install:
      - kind: go
        bins: [trigger-dev-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-cli
---

# Trigger.dev — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `trigger-dev-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install trigger-dev
   ```
2. Verify: `trigger-dev-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

trigger-dev-pp-cli wraps the full v3 management API (47 endpoints across runs, schedules, deployments, batches, queues, waitpoints, env vars, and TRQL queries) and adds the cross-run aggregations the dashboard hides one click deep — LLM span cost rollups, recurring-failure patterns, real-time failure watch with desktop notifications, env-var diffs across environments, and substring grep over cached run errors.

## When to Use This CLI

Reach for this CLI when an agent task needs to operate Trigger.dev programmatically: triaging failed runs, auditing schedule health, diffing env vars across environments, computing LLM costs across runs, or watching for new failures. It complements the official trigger.dev CLI (which focuses on dev-time deploy/init/dev) by exposing the management API surface as scriptable, agent-native commands with offline FTS and typed exit codes.

## Unique Capabilities

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

## Command Reference

**batches** — Manage batches

- `trigger-dev-pp-cli batches <batchId>` — Retrieve a batch by its ID, including its status and the IDs of all runs in the batch.

**deployments** — Manage deployments

- `trigger-dev-pp-cli deployments get-latest-v1` — Retrieve information about the latest unmanaged deployment for the authenticated project.
- `trigger-dev-pp-cli deployments get-v1` — Retrieve information about a specific deployment by its ID.
- `trigger-dev-pp-cli deployments list-v1` — List deployments for the authenticated environment, ordered by most recent first.

**projects** — Manage projects


**query** — Manage query

- `trigger-dev-pp-cli query execute-v1` — Execute a TRQL (Trigger.dev Query Language) query against your run data. TRQL is a SQL-style query language that...
- `trigger-dev-pp-cli query get-schema-v1` — Get the schema for TRQL queries, including all available tables, their columns, data types, descriptions, and...
- `trigger-dev-pp-cli query list-dashboards-v1` — List available built-in dashboards with their widgets. Each dashboard contains pre-built TRQL queries for common...

**queues** — Manage queues

- `trigger-dev-pp-cli queues list-v1` — List all queues in your environment with pagination support.
- `trigger-dev-pp-cli queues retrieve-v1` — Get a queue by its ID, or by type and name.

**runs** — Manage runs

- `trigger-dev-pp-cli runs list-v1` — List runs in a specific environment. You can filter the runs by status, created at, task identifier, version, and more.
- `trigger-dev-pp-cli runs retrieve-v1` — Retrieve information about a run, including its status, payload, output, and attempts. If you authenticate with a...

**schedules** — Manage schedules

- `trigger-dev-pp-cli schedules create-v1` — Create a new `IMPERATIVE` schedule based on the specified options.
- `trigger-dev-pp-cli schedules delete-v1` — Delete a schedule by its ID. This will only work on `IMPERATIVE` schedules that were created in the dashboard or...
- `trigger-dev-pp-cli schedules get-v1` — Get a schedule by its ID.
- `trigger-dev-pp-cli schedules list-v1` — List all schedules. You can also paginate the results.
- `trigger-dev-pp-cli schedules update-v1` — Update a schedule by its ID. This will only work on `IMPERATIVE` schedules that were created in the dashboard or...

**tasks** — Manage tasks

- `trigger-dev-pp-cli tasks` — Batch trigger tasks with up to 1,000 payloads with SDK 4.3.1+ (500 in prior versions).

**timezones** — Manage timezones

- `trigger-dev-pp-cli timezones` — Get all supported timezones that schedule tasks support.

**waitpoints** — Manage waitpoints

- `trigger-dev-pp-cli waitpoints complete-token-callback-v1` — Completes a waitpoint token using the pre-signed callback URL returned in the `url` field when the token was...
- `trigger-dev-pp-cli waitpoints complete-token-v1` — Completes a waitpoint token, unblocking any run that is waiting for it via `wait.forToken()`. An optional `data`...
- `trigger-dev-pp-cli waitpoints create-token-v1` — Creates a new waitpoint token that can be used to pause a run until an external event completes it. The token...
- `trigger-dev-pp-cli waitpoints list-tokens-v1` — Returns a paginated list of waitpoint tokens for the current environment. Results are ordered by creation date,...
- `trigger-dev-pp-cli waitpoints retrieve-token-v1` — Retrieves a waitpoint token by its ID, including its current status and output if it has been completed.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
trigger-dev-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Triage today's failures

```bash
trigger-dev-pp-cli sync --resources runs --since 1d && trigger-dev-pp-cli failures top --since 1d --top 10 --agent --select 'task,error_signature,count,last_seen'
```

Sync today's runs, then group by task and normalized error signature to surface the dominant failure patterns.

### Watch a deploy window

```bash
trigger-dev-pp-cli runs watch --task my-task --notify --sound
```

Stay in the terminal during a deploy — the command exits non-zero and beeps on the first new FAILED run.

### Audit env-var drift across environments

```bash
trigger-dev-pp-cli envvars diff --project proj_abc --from prod --to staging --agent
```

Compare two environments' variables; values are masked, drift flagged.

### Find runs that hit a specific error

```bash
trigger-dev-pp-cli runs find "connection timeout" --since 7d --status FAILED --agent --select 'id,taskIdentifier,error.message,createdAt'
```

substring grep over the local store with --select narrowing the agent context to the fields that matter.

### Run a curated TRQL recipe

```bash
trigger-dev-pp-cli query run cost-by-model-7d --param env=prod --agent
```

Execute a curated TRQL query without authoring SQL. Recipes cover failure-rate, cost, and latency by common dimensions.

## Auth Setup

Authentication uses a Trigger.dev secret key in the `Authorization: Bearer` header. Set `TRIGGER_SECRET_KEY` to your environment's key — `tr_dev_…` for development, `tr_prod_…` for production, `tr_pat_…` for personal access tokens. Each key is environment-scoped: a dev key cannot manage prod runs.

Run `trigger-dev-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  trigger-dev-pp-cli batches mock-value --agent --select id,name,status
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
trigger-dev-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
trigger-dev-pp-cli feedback --stdin < notes.txt
trigger-dev-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.trigger-dev-pp-cli/feedback.jsonl`. They are never POSTed unless `TRIGGER_DEV_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRIGGER_DEV_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
trigger-dev-pp-cli profile save briefing --json
trigger-dev-pp-cli --profile briefing batches mock-value
trigger-dev-pp-cli profile list --json
trigger-dev-pp-cli profile show briefing
trigger-dev-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

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

1. **Empty, `help`, or `--help`** → show `trigger-dev-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add trigger-dev-pp-mcp -- trigger-dev-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which trigger-dev-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   trigger-dev-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `trigger-dev-pp-cli <command> --help`.
