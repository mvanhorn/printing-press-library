---
name: pp-morgen
description: "Printing Press CLI for Morgen. Morgen calendar & tasks API CLI — unified access to calendars, events, tasks, and tags across connected providers"
author: "Nick Scarabosio"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - morgen-pp-cli
    install:
      - kind: go
        bins: [morgen-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/morgen/cmd/morgen-pp-cli
---

# Morgen — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `morgen-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install morgen --cli-only
   ```
2. Verify: `morgen-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/morgen/cmd/morgen-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Morgen calendar & tasks API CLI — unified access to calendars, events, tasks, and tags across connected providers

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-source synthesis
- **`agenda`** — Unified chronological day view merging calendar events and due tasks across all connected accounts.

  _Reach for this to see everything on a day in one call instead of querying events per calendar and tasks separately._

  ```bash
  morgen-pp-cli agenda --date 2026-06-16
  ```

## Command Reference

**calendars** — List calendars and update Morgen-specific calendar metadata

- `morgen-pp-cli calendars list` — List calendars across all connected accounts
- `morgen-pp-cli calendars update` — Update Morgen-specific metadata for a calendar (busy, color, name overrides)

**events** — List, create, update, and delete calendar events

- `morgen-pp-cli events create` — Create an event in a calendar
- `morgen-pp-cli events delete` — Delete an event. Use --series-update-mode for recurring events.
- `morgen-pp-cli events list` — List events from one or more calendars in a time window
- `morgen-pp-cli events update` — Update an event (patch). Use --series-update-mode for recurring events.

**integrations** — View connected accounts and available providers (read-only; connect/disconnect happens in the Morgen app)

- `morgen-pp-cli integrations accounts` — List connected calendar/task accounts
- `morgen-pp-cli integrations providers` — List available integration providers (Google, Microsoft 365, iCloud, Todoist, etc.)

**tags** — Manage tags

- `morgen-pp-cli tags create` — Create a tag
- `morgen-pp-cli tags delete` — Delete a tag
- `morgen-pp-cli tags get` — Get a single tag by ID
- `morgen-pp-cli tags list` — List tags
- `morgen-pp-cli tags update` — Update a tag

**tasks** — Manage tasks across connected task providers

- `morgen-pp-cli tasks close` — Mark a task complete
- `morgen-pp-cli tasks create` — Create a task
- `morgen-pp-cli tasks delete` — Delete a task
- `morgen-pp-cli tasks get` — Get a single task by ID
- `morgen-pp-cli tasks list` — List tasks
- `morgen-pp-cli tasks move` — Reorder or re-parent a task
- `morgen-pp-cli tasks reopen` — Reopen a completed task
- `morgen-pp-cli tasks update` — Update a task (patch)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
morgen-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `morgen-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export MORGEN_API_KEY="<your-key>"
```

Or persist it in `~/.config/morgen/config.toml`.

Run `morgen-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  morgen-pp-cli calendars list --agent --select id,name,status
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
morgen-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
morgen-pp-cli feedback --stdin < notes.txt
morgen-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/morgen-pp-cli/feedback.jsonl`. They are never POSTed unless `MORGEN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MORGEN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
morgen-pp-cli profile save briefing --json
morgen-pp-cli --profile briefing calendars list
morgen-pp-cli profile list --json
morgen-pp-cli profile show briefing
morgen-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `morgen-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/morgen/cmd/morgen-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add morgen-pp-mcp -- morgen-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which morgen-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   morgen-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `morgen-pp-cli <command> --help`.
