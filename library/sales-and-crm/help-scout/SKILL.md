---
name: pp-help-scout
description: "Printing Press CLI for Help Scout. Read-first Help Scout Inbox API surface for support operations and agent workflows."
author: "Deb Mukherjee"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - help-scout-pp-cli
    install:
      - kind: go
        bins: [help-scout-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/sales-and-crm/help-scout/cmd/help-scout-pp-cli
---

# Help Scout — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `help-scout-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install help-scout --cli-only
   ```
2. Verify: `help-scout-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/help-scout/cmd/help-scout-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-first Help Scout Inbox API surface for support operations and agent workflows.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**conversations** — Manage conversations

- `help-scout-pp-cli conversations get` — Get a conversation by ID.
- `help-scout-pp-cli conversations get-v3` — Get a conversation by ID using the v3 conversation representation.
- `help-scout-pp-cli conversations list` — List conversations.

**customers** — Manage customers

- `help-scout-pp-cli customers get` — Get a customer by ID.
- `help-scout-pp-cli customers list` — List customers.

**mailboxes** — Manage mailboxes

- `help-scout-pp-cli mailboxes get-mailbox` — Get a Help Scout mailbox by ID.
- `help-scout-pp-cli mailboxes list` — List Help Scout mailboxes.

**reports** — Manage reports

- `help-scout-pp-cli reports get-conversation` — Get conversation reporting metrics.
- `help-scout-pp-cli reports get-happiness` — Get customer happiness reporting metrics.
- `help-scout-pp-cli reports get-productivity` — Get productivity reporting metrics.

**tags** — Manage tags

- `help-scout-pp-cli tags` — List Help Scout tags.

**teams** — Manage teams

- `help-scout-pp-cli teams` — List Help Scout teams.

**users** — Manage users

- `help-scout-pp-cli users get` — Get a Help Scout user by ID.
- `help-scout-pp-cli users get-current` — Get the authenticated Help Scout user.
- `help-scout-pp-cli users list` — List Help Scout users.

**webhooks** — Manage webhooks

- `help-scout-pp-cli webhooks` — List webhook subscriptions.

**workflows** — Manage workflows

- `help-scout-pp-cli workflows` — List Help Scout workflows.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
help-scout-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `help-scout-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
help-scout-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `HELP_SCOUT_BEARER_AUTH` as an environment variable.

Run `help-scout-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  help-scout-pp-cli conversations list --agent --select id,name,status
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
help-scout-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
help-scout-pp-cli feedback --stdin < notes.txt
help-scout-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/github.com/mvanhorn/printing-press-library/library/sales-and-crm/help-scout/feedback.jsonl`. They are never POSTed unless `HELP_SCOUT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HELP_SCOUT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
help-scout-pp-cli profile save briefing --json
help-scout-pp-cli --profile briefing conversations list
help-scout-pp-cli profile list --json
help-scout-pp-cli profile show briefing
help-scout-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `help-scout-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/help-scout/cmd/help-scout-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add help-scout-pp-mcp -- help-scout-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which help-scout-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   help-scout-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `help-scout-pp-cli <command> --help`.
