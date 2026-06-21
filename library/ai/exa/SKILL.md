---
name: pp-exa
description: "Printing Press CLI for Exa."
author: "Anchal Sharma"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - exa-pp-cli
---

# Exa — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `exa-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install exa --cli-only
   ```
2. Verify: `exa-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.



## Command Reference

**agent** — Manage agent

- `exa-pp-cli agent cancel-run` — Cancel a queued or running Agent run.
- `exa-pp-cli agent create-run` — Create an asynchronous Agent run. By default, the API returns the run object immediately.
- `exa-pp-cli agent delete-run` — Delete a stored Agent run.
- `exa-pp-cli agent get-run` — Retrieve a single Agent run by ID.
- `exa-pp-cli agent list-run-events` — List stored events for an Agent run. Set `Accept: text/event-stream` to replay stored events as server-sent events.
- `exa-pp-cli agent list-runs` — List Agent runs for your team, ordered from newest to oldest.

**answer** — Manage answer

- `exa-pp-cli answer` — Performs a search based on the query and generates either a direct answer or a detailed summary with citations

**contents** — Manage contents

- `exa-pp-cli contents` — Contents

**events** — Manage events

- `exa-pp-cli events get` — Get a single Event by id. You can subscribe to Events by creating a Webhook.
- `exa-pp-cli events list` — List all events that have occurred in the system. You can paginate through the results using the `cursor` parameter.

**imports** — Manage imports

- `exa-pp-cli imports create` — Creates a new import to upload your data into Websets.
- `exa-pp-cli imports delete` — Deletes a import.
- `exa-pp-cli imports get` — Gets a specific import.
- `exa-pp-cli imports list` — Lists all imports for the Webset.
- `exa-pp-cli imports update` — Updates a import configuration.

**monitors** — Manage monitors

- `exa-pp-cli monitors batch` — Perform a batch action on monitors matching the provided filters.
- `exa-pp-cli monitors create` — Creates a new Monitor to run recurring Exa searches on a schedule.
- `exa-pp-cli monitors create-endpoint` — Creates a new `Monitor` to continuously keep your Websets updated with fresh data.
- `exa-pp-cli monitors delete` — Deletes a monitor. This cannot be undone.
- `exa-pp-cli monitors delete-id` — Deletes a monitor.
- `exa-pp-cli monitors get` — Retrieves a single monitor by its ID.
- `exa-pp-cli monitors get-id` — Gets a specific monitor.
- `exa-pp-cli monitors list` — Lists all monitors for the authenticated team. Supports filtering by status and cursor-based pagination.
- `exa-pp-cli monitors list-endpoint` — Lists all monitors for the Webset.
- `exa-pp-cli monitors update` — Updates an existing monitor. All fields are optional.
- `exa-pp-cli monitors update-id` — Updates a monitor configuration.

**neural_search** — Manage neural search

- `exa-pp-cli neural-search` — Perform a search with an Exa prompt-engineered query and retrieve a list of relevant results. Optionally get contents.

**research** — Manage research

- `exa-pp-cli research create` — Create a new research request
- `exa-pp-cli research get` — Retrieve research by ID. Add ?stream=true for real-time SSE updates.
- `exa-pp-cli research list` — Get a paginated list of research requests

**teams** — Manage teams

- `exa-pp-cli teams` — Returns information about the authenticated team, including current concurrency usage and limits.

**webhooks** — Manage webhooks

- `exa-pp-cli webhooks create` — Create a Webhook
- `exa-pp-cli webhooks delete` — Delete a Webhook
- `exa-pp-cli webhooks get` — Get a Webhook
- `exa-pp-cli webhooks list` — List webhooks
- `exa-pp-cli webhooks update` — Update a Webhook

**websets** — Manage websets

- `exa-pp-cli websets create` — Creates a new Webset with optional search, import, and enrichment configurations.
- `exa-pp-cli websets delete` — Deletes a Webset. Once deleted, the Webset and all its Items will no longer be available.
- `exa-pp-cli websets get` — Get a Webset
- `exa-pp-cli websets list` — Returns a list of Websets. You can paginate through the results using the `cursor` parameter.
- `exa-pp-cli websets preview` — Preview how a search query will be decomposed before creating a webset.
- `exa-pp-cli websets update` — Update a Webset


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
exa-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `exa-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
exa-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `EXA_TOKEN` as an environment variable.

Run `exa-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  exa-pp-cli contents --agent --select id,name,status
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
exa-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
exa-pp-cli feedback --stdin < notes.txt
exa-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/exa-pp-cli/feedback.jsonl`. They are never POSTed unless `EXA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EXA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
exa-pp-cli profile save briefing --json
exa-pp-cli --profile briefing contents
exa-pp-cli profile list --json
exa-pp-cli profile show briefing
exa-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `exa-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add exa-pp-mcp -- exa-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which exa-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   exa-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `exa-pp-cli <command> --help`.
