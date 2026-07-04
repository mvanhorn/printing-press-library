---
name: pp-airflow-admin
description: "Printing Press CLI for Airflow Admin. Focused read-first API surface for inspecting Apache Airflow DAGs, DAG runs, task instances, pools, variables"
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - airflow-admin-pp-cli
    install:
      - kind: go
        bins: [airflow-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/airflow-admin/cmd/airflow-admin-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/airflow-admin/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Airflow Admin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `airflow-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install airflow-admin --cli-only
   ```
2. Verify: `airflow-admin-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/airflow-admin/cmd/airflow-admin-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Apache Airflow is a workflow orchestrator used by data teams to schedule and monitor data pipelines. Data engineers usually model each pipeline as a DAG, then watch DAG runs, task instances, retries, pools, and scheduler health to understand whether daily loads, API extracts, dbt jobs, reports, and other data workflows are running correctly.

Use this skill when a user needs an operational readout from Airflow: list DAGs, inspect failed DAG runs, check task instance status, review pools, verify API health, or sync/search Airflow metadata locally. The CLI is read-first and intended for investigation, triage, and reporting. Avoid implying that it deploys DAGs, edits schedules, clears tasks, or mutates Airflow state.

## Common Airflow Workflows

### Local Airflow auth

For a local Airflow webserver at `http://localhost:8080`, get a token and store it:

```bash
airflow-admin-pp-cli apache-airflow-admin-auth --username airflow --password airflow --json
airflow-admin-pp-cli auth set-token <access_token>
airflow-admin-pp-cli doctor --json
```

For a remote Airflow environment, set the base URL first:

```bash
export AIRFLOW_ADMIN_BASE_URL="https://airflow.example.com"
export AIRFLOW_ADMIN_BEARER_AUTH="<access_token>"
```

Never ask the user to paste Airflow credentials into a repository, issue, PR, or shared log.

### Pipeline health triage

Start broad, then drill down:

```bash
airflow-admin-pp-cli monitor --agent
airflow-admin-pp-cli dags list --agent --select dag_id,is_active,is_paused,last_parsed_time
airflow-admin-pp-cli dags dag-runs list <dag_id> --state failed --agent --select dag_id,dag_run_id,state,start_date,end_date
airflow-admin-pp-cli dags dag-runs list-task-instances <dag_id> <dag_run_id> --state failed --agent --select task_id,state,try_number,start_date,end_date
```

### Local search and analysis

When a user wants repeated investigation without paging the Airflow API each time:

```bash
airflow-admin-pp-cli sync --resources dags,pools --agent
airflow-admin-pp-cli search "failed" --data-source local --agent --limit 20
airflow-admin-pp-cli analytics count --type dag-runs --agent
```

## Command Reference

**apache-airflow-admin-auth** — Manage apache airflow admin auth

- `airflow-admin-pp-cli apache-airflow-admin-auth` — Authenticate with an Airflow username and password and return a JWT access token.

**apache-airflow-admin-version** — Manage apache airflow admin version

- `airflow-admin-pp-cli apache-airflow-admin-version` — Get Airflow version

**connections** — Airflow connection metadata.

- `airflow-admin-pp-cli connections` — List connections

**dags** — DAG inventory and metadata.

- `airflow-admin-pp-cli dags get` — Get DAG
- `airflow-admin-pp-cli dags list` — List DAGs
- `airflow-admin-pp-cli dags dag-runs list` — List DAG runs
- `airflow-admin-pp-cli dags dag-runs get` — Get DAG run
- `airflow-admin-pp-cli dags dag-runs list-task-instances` — List task instances
- `airflow-admin-pp-cli dags dag-runs get-task-instance` — Get task instance
- `airflow-admin-pp-cli dags tasks list` — List DAG tasks
- `airflow-admin-pp-cli dags details` — Get detailed DAG metadata

**monitor** — Manage monitor

- `airflow-admin-pp-cli monitor` — Get health

**pools** — Airflow pool capacity and occupancy.

- `airflow-admin-pp-cli pools get` — Get pool
- `airflow-admin-pp-cli pools list` — List pools

**variables** — Airflow variable metadata.

- `airflow-admin-pp-cli variables` — List variables


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
airflow-admin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Airflow JWT tokens can be created through the API token endpoint when the Airflow environment allows username/password auth:

```bash
airflow-admin-pp-cli apache-airflow-admin-auth --username <username> --password <password> --json
```

Store the returned `access_token`:

```bash
airflow-admin-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `AIRFLOW_ADMIN_BEARER_AUTH` as an environment variable. Set `AIRFLOW_ADMIN_BASE_URL` when the Airflow webserver is not `http://localhost:8080`.

Run `airflow-admin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  airflow-admin-pp-cli apache-airflow-admin-version --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
airflow-admin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
airflow-admin-pp-cli feedback --stdin < notes.txt
airflow-admin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/airflow-admin-pp-cli/feedback.jsonl`. They are never POSTed unless `AIRFLOW_ADMIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AIRFLOW_ADMIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
airflow-admin-pp-cli profile save briefing --json
airflow-admin-pp-cli --profile briefing apache-airflow-admin-version
airflow-admin-pp-cli profile list --json
airflow-admin-pp-cli profile show briefing
airflow-admin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `airflow-admin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/airflow-admin/cmd/airflow-admin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add airflow-admin-pp-mcp -- airflow-admin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which airflow-admin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   airflow-admin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `airflow-admin-pp-cli <command> --help`.
