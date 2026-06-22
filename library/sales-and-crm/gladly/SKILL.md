---
name: pp-gladly
description: "Printing Press CLI for Gladly. Read-first Gladly API surface for customer timelines, conversations, agents, topics, answers, and support operations."
author: "Deb Mukherjee"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gladly-pp-cli
    install:
      - kind: go
        bins: [gladly-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/gladly/cmd/gladly-pp-cli
---

# Gladly — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gladly-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install gladly --cli-only
   ```
2. Verify: `gladly-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gladly/cmd/gladly-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-first Gladly API surface for customer timelines, conversations, agents, topics, answers, and support operations.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**agents** — Manage agents

- `gladly-pp-cli agents` — List agents.

**answers** — Manage answers

- `gladly-pp-cli answers` — List knowledge answers.

**conversation-items** — Manage conversation items

- `gladly-pp-cli conversation-items` — List conversation timeline items.

**conversations** — Manage conversations

- `gladly-pp-cli conversations get` — Get a conversation.
- `gladly-pp-cli conversations list` — List customer conversations.

**customers** — Manage customers

- `gladly-pp-cli customers get` — Get a Gladly customer profile.
- `gladly-pp-cli customers list` — List Gladly customers.

**inboxes** — Manage inboxes

- `gladly-pp-cli inboxes` — List inboxes.

**reports** — Manage reports

- `gladly-pp-cli reports` — List available reports.

**tasks** — Manage tasks

- `gladly-pp-cli tasks` — List Gladly tasks.

**topics** — Manage topics

- `gladly-pp-cli topics` — List conversation topics.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gladly-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `gladly-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export GLADLY_USERNAME="<your-username>"
export GLADLY_PASSWORD="<your-password>"
```

Or persist them in `~/.config/gladly-pp-cli/config.toml`:

```bash
gladly-pp-cli auth set-token "<username>" "<password>"
```

Run `gladly-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gladly-pp-cli agents --agent --select id,name,status
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
gladly-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gladly-pp-cli feedback --stdin < notes.txt
gladly-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/gladly-pp-cli/feedback.jsonl`. They are never POSTed unless `GLADLY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GLADLY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gladly-pp-cli profile save briefing --json
gladly-pp-cli --profile briefing agents
gladly-pp-cli profile list --json
gladly-pp-cli profile show briefing
gladly-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gladly-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/gladly/cmd/gladly-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add gladly-pp-mcp -- gladly-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gladly-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gladly-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gladly-pp-cli <command> --help`.
