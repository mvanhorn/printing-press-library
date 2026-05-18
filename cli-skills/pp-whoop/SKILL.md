---
name: pp-whoop
description: "Printing Press CLI for Whoop."
author: "Greg Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - whoop-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/whoop/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Whoop — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `whoop-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install whoop --cli-only
   ```
2. Verify: `whoop-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.



## Command Reference

**activity** — Manage activity

- `whoop-pp-cli activity get-sleep-by-id` — Get the sleep for the specified ID
- `whoop-pp-cli activity get-sleep-collection` — Get all sleeps for a user, paginated. Results are sorted by start time in descending order.
- `whoop-pp-cli activity get-workout-by-id` — Get the workout for the specified ID
- `whoop-pp-cli activity get-workout-collection` — Get all workouts for a user, paginated. Results are sorted by start time in descending order.

**activity-mapping** — Manage activity mapping

- `whoop-pp-cli activity-mapping <activityV1Id>` — Lookup the V2 UUID for a given V1 activity ID

**cycle** — Manage cycle

- `whoop-pp-cli cycle get-by-id` — Get the cycle for the specified ID
- `whoop-pp-cli cycle get-collection` — Get all physiological cycles for a user, paginated. Results are sorted by start time in descending order.

**partner** — Endpoints for trusted WHOOP partner operations

- `whoop-pp-cli partner add-test-data` — Generates test user and lab requisition data for partner integration testing. This endpoint is only available in...
- `whoop-pp-cli partner get-lab-requisition-by-id` — Retrieves a lab requisition with its associated service requests by its unique identifier. The requesting partner...
- `whoop-pp-cli partner get-service-request-by-id` — Retrieves a service request by its unique identifier. The requesting partner must be an owner of the service request.
- `whoop-pp-cli partner request-token` — Exchanges partner client credentials for an access token.
- `whoop-pp-cli partner update-service-request-status` — Updates the business status of a service request task. The requesting partner must be an owner of the service request.
- `whoop-pp-cli partner upload-diagnostic-report-results` — Creates a diagnostic report with results for a service request. The requesting partner must be an owner of the...

**recovery** — Manage recovery

- `whoop-pp-cli recovery` — Get all recoveries for a user, paginated. Results are sorted by start time of the related sleep in descending order.

**user** — Endpoints for retrieving user profile and measurement data.

- `whoop-pp-cli user get-body-measurement` — Retrieves the body measurements (height, weight, max heart rate) for the authenticated user.
- `whoop-pp-cli user get-profile-basic` — Retrieves the basic profile information (name, email) for the authenticated user.
- `whoop-pp-cli user revoke-oauth-access` — Revoke the access token granted by the user. If the associated OAuth client is configured to receive webhooks, it...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
whoop-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
whoop-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `WHOOP_OAUTH` as an environment variable.

Run `whoop-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  whoop-pp-cli activity-mapping mock-value --agent --select id,name,status
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
whoop-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
whoop-pp-cli feedback --stdin < notes.txt
whoop-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.whoop-pp-cli/feedback.jsonl`. They are never POSTed unless `WHOOP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WHOOP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
whoop-pp-cli profile save briefing --json
whoop-pp-cli --profile briefing activity-mapping mock-value
whoop-pp-cli profile list --json
whoop-pp-cli profile show briefing
whoop-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `whoop-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add whoop-pp-mcp -- whoop-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which whoop-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   whoop-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `whoop-pp-cli <command> --help`.
