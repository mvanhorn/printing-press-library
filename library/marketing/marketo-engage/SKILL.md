---
name: pp-marketo-engage
description: "Printing Press CLI for Marketo Engage. Read-first Adobe Marketo Engage REST API surface for leads, activities, campaigns, and marketing assets."
author: "Deb Mukherjee"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - marketo-engage-pp-cli
    install:
      - kind: go
        bins: [marketo-engage-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/marketo-engage/cmd/marketo-engage-pp-cli
---

# Marketo Engage — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `marketo-engage-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install marketo-engage --cli-only
   ```
2. Verify: `marketo-engage-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/marketo-engage/cmd/marketo-engage-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-first Adobe Marketo Engage REST API surface for leads, activities, campaigns, and marketing assets.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**activities** — Manage activities

- `marketo-engage-pp-cli activities` — List activity types.

**activities-json** — Manage activities json

- `marketo-engage-pp-cli activities-json` — List lead activities.

**asset** — Manage asset

- `marketo-engage-pp-cli asset get-program` — Get a program by ID.
- `marketo-engage-pp-cli asset list-emails` — List email assets.
- `marketo-engage-pp-cli asset list-folders` — List folders.
- `marketo-engage-pp-cli asset list-forms` — List form assets.
- `marketo-engage-pp-cli asset list-landing-pages` — List landing page assets.
- `marketo-engage-pp-cli asset list-programs` — List programs.
- `marketo-engage-pp-cli asset list-smart-lists` — List smart lists.

**campaign** — Manage campaign

- `marketo-engage-pp-cli campaign <id>` — Get a smart campaign by ID.

**campaigns-json** — Manage campaigns json

- `marketo-engage-pp-cli campaigns-json` — List smart campaigns.

**lead** — Manage lead

- `marketo-engage-pp-cli lead <id>` — Get a lead by ID.

**leads-json** — Manage leads json

- `marketo-engage-pp-cli leads-json` — List or filter leads.

**list** — Manage list


**lists-json** — Manage lists json

- `marketo-engage-pp-cli lists-json` — List static lists.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
marketo-engage-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `marketo-engage-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
marketo-engage-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `MARKETO_ENGAGE_BEARER_AUTH` as an environment variable.

Run `marketo-engage-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  marketo-engage-pp-cli campaign mock-value --agent --select id,name,status
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
marketo-engage-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
marketo-engage-pp-cli feedback --stdin < notes.txt
marketo-engage-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/marketo-engage-pp-cli/feedback.jsonl`. They are never POSTed unless `MARKETO_ENGAGE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MARKETO_ENGAGE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
marketo-engage-pp-cli profile save briefing --json
marketo-engage-pp-cli --profile briefing campaign mock-value
marketo-engage-pp-cli profile list --json
marketo-engage-pp-cli profile show briefing
marketo-engage-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `marketo-engage-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/marketo-engage/cmd/marketo-engage-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add marketo-engage-pp-mcp -- marketo-engage-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which marketo-engage-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   marketo-engage-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `marketo-engage-pp-cli <command> --help`.
