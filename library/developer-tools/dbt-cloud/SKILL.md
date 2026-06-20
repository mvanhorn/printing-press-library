---
name: pp-dbt-cloud
description: "Operate dbt Cloud jobs and runs from one static Go binary — trigger, watch to completion with honest exit codes Trigger phrases: `trigger a dbt job`, `watch a dbt run`, `did my dbt run pass`, `list dbt cloud runs`, `use dbt-cloud`, `run dbt-cloud`."
author: "Nimrod Astarhan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - dbt-cloud-pp-cli
    install:
      - kind: go
        bins: [dbt-cloud-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/cmd/dbt-cloud-pp-cli
---

# dbt Cloud — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `dbt-cloud-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install dbt-cloud --cli-only
   ```
2. Verify: `dbt-cloud-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/cmd/dbt-cloud-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Covers the dbt Cloud Administrative API v2 jobs, runs, and artifacts surface, then adds the operate-and-watch loop (monitor, trigger --wait) and local run-history analytics (runs stats, failures, artifacts diff) that thin API wrappers don't. Agent-native --json output throughout.

## When to Use This CLI

Use when operating dbt Cloud jobs from a terminal, CI, or an agent: triggering jobs and blocking on the result, inspecting run history, or fetching artifacts. Strongest where you need a real exit code on run success/failure.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to run dbt models locally (that is the open-source dbt CLI).
- Do not use it for dbt Cloud account/user/project administration (out of scope; that is API v3).

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Operate and watch
- **`monitor`** — Watch a dbt Cloud run until it finishes, with live status and an exit code that reflects success or failure.

  _Reach for this to block a script or CI step on a dbt run and get a real exit code plus failure context._

  ```bash
  dbt-cloud-pp-cli monitor 460826060 --interval 15s --json
  ```
- **`trigger`** — Trigger a job and watch the resulting run to completion in one command, returning the run's exit status.

  _Reach for this as the CI-grade run-a-job-and-fail-on-error loop._

  ```bash
  dbt-cloud-pp-cli trigger 12345 --cause 'ci deploy' --wait --json
  ```

### Local run analytics
- **`runs stats`** — Success rate, average and p95 duration, and failure counts per job over a time window from the local run mirror.

  _Reach for this for reliability and velocity review across many runs, not a single run._

  ```bash
  dbt-cloud-pp-cli runs stats --days 30 --json
  ```
- **`failures`** — Recently failed runs in a time window, grouped by job, with each run's failed-step names.

  _Reach for this to triage what broke recently across all jobs._

  ```bash
  dbt-cloud-pp-cli failures --days 1 --json
  ```
- **`artifacts diff`** — Diff run_results.json between two runs to show which models newly failed or changed timing.

  _Reach for this to hunt regressions between two runs._

  ```bash
  dbt-cloud-pp-cli artifacts diff 460826060 460900111 --json
  ```

## Command Reference

**cloud-jobs** — Manage dbt Cloud jobs

- `dbt-cloud-pp-cli cloud-jobs create` — Create a new job.
- `dbt-cloud-pp-cli cloud-jobs destroy` — Delete a job
- `dbt-cloud-pp-cli cloud-jobs list` — List jobs for the given account
- `dbt-cloud-pp-cli cloud-jobs retrieve` — Retrieve the details of a job.
- `dbt-cloud-pp-cli cloud-jobs update` — Update a job.

**runs** — Manage runs

- `dbt-cloud-pp-cli runs list` — List runs for an account.
- `dbt-cloud-pp-cli runs retrieve` — Retrieve details of a run.

**steps** — Manage steps

- `dbt-cloud-pp-cli steps <account_id> <id>` — Retrieve the details of a specific step of a run.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
dbt-cloud-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Fail CI on a bad run

```bash
dbt-cloud-pp-cli trigger 12345 --cause 'ci' --wait
```

Triggers the job and exits non-zero if the run fails.

### Narrow run output for an agent

```bash
dbt-cloud-pp-cli runs list 123456789 --limit 20 --json --select data.id,data.status_humanized,data.job_definition_id,data.duration
```

Returns only the high-gravity fields from a verbose run payload.

### Triage today's failures

```bash
dbt-cloud-pp-cli failures --days 1 --json
```

Lists failed runs in the last day with their failed-step names.

## Auth Setup

Set DBT_CLOUD_TOKEN (a Personal Access Token or service token) and DBT_CLOUD_ACCOUNT_ID. For regional dbt Cloud, set DBT_CLOUD_HOST (default https://cloud.getdbt.com). Tokens are sent as Authorization: Bearer.

Run `dbt-cloud-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  dbt-cloud-pp-cli cloud-jobs list mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
dbt-cloud-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
dbt-cloud-pp-cli feedback --stdin < notes.txt
dbt-cloud-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/dbt-cloud-pp-cli/feedback.jsonl`. They are never POSTed unless `DBT_CLOUD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DBT_CLOUD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
dbt-cloud-pp-cli profile save briefing --json
dbt-cloud-pp-cli --profile briefing cloud-jobs list mock-value
dbt-cloud-pp-cli profile list --json
dbt-cloud-pp-cli profile show briefing
dbt-cloud-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `dbt-cloud-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/cmd/dbt-cloud-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add dbt-cloud-pp-mcp -- dbt-cloud-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which dbt-cloud-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   dbt-cloud-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `dbt-cloud-pp-cli <command> --help`.
