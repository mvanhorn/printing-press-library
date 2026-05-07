---
name: pp-cloud-run-admin
description: "A focused Cloud Run Admin API CLI with agent-native output, local inventory, and command discovery. Trigger phrases: `inspect Cloud Run services`, `list Cloud Run jobs`, `find Cloud Run revisions`, `check Cloud Run executions`, `search Cloud Run inventory`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cloud-run-admin-pp-cli
    install:
      - kind: go
        bins: [cloud-run-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/cloud-run-admin/cmd/cloud-run-admin-pp-cli
---

# Google Cloud Run — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cloud-run-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cloud-run-admin --cli-only
   ```
2. Verify: `cloud-run-admin-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.25+):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/cloud-run-admin/cmd/cloud-run-admin-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI wraps the Cloud Run Admin API directly for services, jobs, executions, tasks, operations, revisions, and IAM helpers. It complements gcloud by giving agents a smaller API-shaped surface with JSON-first output, field selection, local sync, search, analytics, and workflow commands.

## When to Use This CLI

Use this CLI when an agent or script needs focused Cloud Run Admin API access without navigating the full gcloud surface. It is strongest for read-heavy inspection, local inventory sync, search, field-selected JSON, and workflows that correlate services, jobs, executions, revisions, tasks, and operations.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native Cloud Run operations
- **`services list`** — Lists Cloud Run services through the Admin API with agent-friendly JSON, compact output, and field selection.

  _Use this when an agent needs a compact inventory of Cloud Run services in a project and region._

  ```bash
  cloud-run-admin-pp-cli services list projects/PROJECT_ID/locations/REGION --agent --select services.name,services.uri,nextPageToken
  ```

### Local state and analysis
- **`sync`** — Syncs Cloud Run resources into a local SQLite store so services, jobs, revisions, executions, tasks, and operations can be searched without repeating every API call.

  _Use this before offline search, SQL inspection, or repeated multi-resource analysis._

  ```bash
  cloud-run-admin-pp-cli sync --json
  ```
- **`search`** — Searches synced Cloud Run inventory locally or falls back to live API calls depending on the data-source mode.

  _Use this when you need to locate Cloud Run resources by remembered fragments._

  ```bash
  cloud-run-admin-pp-cli search "api" --type services --json --select resource_type,title
  ```
- **`analytics`** — Runs local analysis against synced Cloud Run data for inventory and status-oriented questions.

  _Use this after sync when the question is about trends or inventory health rather than a single resource._

  ```bash
  cloud-run-admin-pp-cli analytics --json
  ```

### Workflow status checks
- **`workflow`** — Provides compound workflows that combine multiple Cloud Run Admin API operations into one operator-facing flow.

  _Use this when the task spans more than one Cloud Run resource type._

  ```bash
  cloud-run-admin-pp-cli workflow --help
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**cloud-run-admin-jobs** — Manage cloud run admin jobs

- `cloud-run-admin-pp-cli cloud-run-admin-jobs create` — Creates a Job.
- `cloud-run-admin-pp-cli cloud-run-admin-jobs list` — Lists Jobs.
- `cloud-run-admin-pp-cli cloud-run-admin-jobs run` — Triggers creation of a new Execution of this Job.

**executions** — Manage executions


**operations** — Manage operations

- `cloud-run-admin-pp-cli operations list` — Lists operations that match the specified filter in the request. If the server doesn't support this method, it...
- `cloud-run-admin-pp-cli operations wait` — Waits until the specified long-running operation is done or reaches at most a specified timeout, returning the...

**services** — Manage services

- `cloud-run-admin-pp-cli services create` — Creates a new Service in a given project and location.
- `cloud-run-admin-pp-cli services get-iam-policy` — Gets the IAM Access Control policy currently in effect for the given Cloud Run Service. This result does not include...
- `cloud-run-admin-pp-cli services list` — Lists Services.
- `cloud-run-admin-pp-cli services patch` — Updates a Service.
- `cloud-run-admin-pp-cli services set-iam-policy` — Sets the IAM Access control policy for the specified Service. Overwrites any existing policy.
- `cloud-run-admin-pp-cli services test-iam-permissions` — Returns permissions that a caller has on the specified Project. There are no permissions required for making this...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cloud-run-admin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find the right Cloud Run command

```bash
cloud-run-admin-pp-cli which "show service revisions" --json
```

Use intent search before calling a specific Cloud Run Admin command.

### List services with compact agent output

```bash
cloud-run-admin-pp-cli services list projects/PROJECT_ID/locations/REGION --agent --select services.name,services.uri,nextPageToken
```

Combines agent defaults with field selection so agents avoid parsing large service payloads.

### Search synced inventory

```bash
cloud-run-admin-pp-cli search "api" --type services --json --select resource_type,title
```

Search local Cloud Run inventory after a sync.

## Auth Setup

Cloud Run Admin uses Google OAuth bearer tokens with the cloud-platform scope. For local use, run `gcloud auth print-access-token` and export the value as `CLOUD_RUN_ADMIN_OAUTH2C`; do not commit the token or paste it into docs.

Run `cloud-run-admin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cloud-run-admin-pp-cli cloud-run-admin-jobs list mock-value --agent --select id,name,status
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
cloud-run-admin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cloud-run-admin-pp-cli feedback --stdin < notes.txt
cloud-run-admin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cloud-run-admin-pp-cli/feedback.jsonl`. They are never POSTed unless `CLOUD_RUN_ADMIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLOUD_RUN_ADMIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
cloud-run-admin-pp-cli profile save briefing --json
cloud-run-admin-pp-cli --profile briefing cloud-run-admin-jobs list mock-value
cloud-run-admin-pp-cli profile list --json
cloud-run-admin-pp-cli profile show briefing
cloud-run-admin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `cloud-run-admin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/cloud-run-admin/cmd/cloud-run-admin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add cloud-run-admin-pp-mcp -- cloud-run-admin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cloud-run-admin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cloud-run-admin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cloud-run-admin-pp-cli <command> --help`.
