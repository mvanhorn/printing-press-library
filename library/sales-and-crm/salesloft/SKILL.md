---
name: pp-salesloft
description: "Printing Press CLI for Salesloft. Read-first Salesloft API surface for agent workflows, based on the public Salesloft v2 developer documentation."
author: "Deb Mukherjee"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - salesloft-pp-cli
    install:
      - kind: go
        bins: [salesloft-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesloft/cmd/salesloft-pp-cli
---

# Salesloft — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `salesloft-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install salesloft --cli-only
   ```
2. Verify: `salesloft-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesloft/cmd/salesloft-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-first Salesloft API surface for agent workflows, based on the public Salesloft v2 developer documentation.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**accounts** — Manage accounts

- `salesloft-pp-cli accounts get` — Get a Salesloft account by ID.
- `salesloft-pp-cli accounts list` — List Salesloft accounts.

**activities** — Manage activities

- `salesloft-pp-cli activities` — List recent Salesloft activities.

**cadences** — Manage cadences

- `salesloft-pp-cli cadences get` — Get a Salesloft cadence by ID.
- `salesloft-pp-cli cadences list` — List Salesloft cadences.

**calls** — Manage calls

- `salesloft-pp-cli calls get` — Get a Salesloft call by ID.
- `salesloft-pp-cli calls list` — List Salesloft calls.

**me** — Manage me

- `salesloft-pp-cli me` — Get the authenticated Salesloft user.

**meetings** — Manage meetings

- `salesloft-pp-cli meetings` — List Salesloft meetings.

**notes** — Manage notes

- `salesloft-pp-cli notes get` — Get a Salesloft note by ID.
- `salesloft-pp-cli notes list` — List Salesloft notes.

**opportunities** — Manage opportunities

- `salesloft-pp-cli opportunities get-opportunity` — Get a Salesloft opportunity by ID.
- `salesloft-pp-cli opportunities list` — List Salesloft opportunities.

**people** — Manage people

- `salesloft-pp-cli people get-person` — Get a Salesloft person by ID.
- `salesloft-pp-cli people list` — List Salesloft people.

**tasks** — Manage tasks

- `salesloft-pp-cli tasks get` — Get a Salesloft task by ID.
- `salesloft-pp-cli tasks list` — List Salesloft tasks.

**team** — Manage team

- `salesloft-pp-cli team` — Get the authenticated user's Salesloft team.

**users** — Manage users

- `salesloft-pp-cli users get` — Get a Salesloft user by ID.
- `salesloft-pp-cli users list` — List Salesloft users.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
salesloft-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `salesloft-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
salesloft-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `SALESLOFT_BEARER_AUTH` as an environment variable.

Run `salesloft-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  salesloft-pp-cli accounts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
salesloft-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
salesloft-pp-cli feedback --stdin < notes.txt
salesloft-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesloft/feedback.jsonl`. They are never POSTed unless `SALESLOFT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SALESLOFT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
salesloft-pp-cli profile save briefing --json
salesloft-pp-cli --profile briefing accounts list
salesloft-pp-cli profile list --json
salesloft-pp-cli profile show briefing
salesloft-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `salesloft-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesloft/cmd/salesloft-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add salesloft-pp-mcp -- salesloft-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which salesloft-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   salesloft-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `salesloft-pp-cli <command> --help`.
