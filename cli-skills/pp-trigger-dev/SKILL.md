---
name: pp-trigger-dev
description: "Trigger.dev background-job monitoring and observability from the terminal. List runs, analyze failures, watch live for failures, inspect queues, schedules, deployments, batches, cost by task, and task health. Use when the user wants to check Trigger.dev status, find failing tasks, watch runs live, audit schedules, look up a specific run, compare cost across tasks or machine types, or debug a stuck batch. Offline search via local SQLite sync."
version: "1.2.1"
author: "Matt Van Horn"
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

# Trigger.dev - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `trigger-dev-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install trigger-dev --cli-only
   ```
2. Verify: `trigger-dev-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this when the user wants:

- list recent runs filtered by status, task, tags, or date range (`runs`)
- find failing tasks or patterns in failures (`failures`, `stale`)
- watch runs live and alert on failures (`watch`)
- dashboard of task health: success rate, duration, cost trends (`health`)
- break down run cost by task, period, or machine type (`costs`)
- inspect queue depth and backlog (`queues`, `batches`)
- audit schedules and deployments (`schedules`, `deployments`)
- list environment variables per project environment (`envvars`)
- search across runs with text + filter (`search`)

Skip it when the user wants to write or deploy Trigger.dev task code; this CLI is read-only for monitoring. Use the official `trigger.dev` CLI (npm) to author tasks.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** -> show `trigger-dev-pp-cli --help`
2. **Starts with `install`** -> ends with `mcp` -> MCP installation; otherwise -> CLI installation
3. **Anything else** -> Direct Use (map to the best command and run it)
## MCP Server Installation

The CLI ships an MCP server at `trigger-dev-pp-mcp`:

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/cmd/trigger-dev-pp-mcp@latest
claude mcp add -e TRIGGER_SECRET_KEY=tr_... trigger-dev-pp-mcp -- trigger-dev-pp-mcp
```

## Direct Use

1. Check installed: `which trigger-dev-pp-cli`. If missing, offer CLI installation.
2. Run `trigger-dev-pp-cli sync` to populate the local store if you'll be running repeated analytics.
3. Discover commands: `trigger-dev-pp-cli --help`; drill into `trigger-dev-pp-cli <cmd> --help`.
4. Execute with `--agent` for structured output:
   ```bash
   trigger-dev-pp-cli <command> [args] --agent
   ```

## Notable Commands

| Command | What it does |
|---------|--------------|
| `runs` | List runs with status / task / tag / date filters |
| `failures` | Failure patterns across tasks and time periods |
| `watch` | Real-time run monitoring with failure alerting |
| `health` | Task health: success rate, duration, cost trends |
| `costs` | Cost breakdown by task, period, machine type |
| `queues` | Queues with running + queued counts |
| `batches` | Batch status and item counts |
| `schedules` | All schedules and their next-run times |
| `deployments` | Most recent deployment info |
| `envvars` | Environment variables per project env |
| `waitpoints` | List waitpoint tokens |
| `search` | Text + filter search across synced data |
| `sync` | Populate local SQLite for offline queries |

Run any command with `--help` for full flag documentation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, with dotted-path support (see below)
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Non-interactive** — never prompts, every input is a flag


### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
trigger-dev-pp-cli <command> --agent --select id,name
trigger-dev-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.


## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (run, task, schedule) |
| 4 | Authentication required (TRIGGER_SECRET_KEY missing or invalid) |
| 5 | API error (Trigger.dev upstream) |
| 7 | Rate limited |
