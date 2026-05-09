---
name: pp-zylos
description: "Every Zylos conversation, from the terminal — with offline search, analytics, and streaming no other tool offers. Trigger phrases: `send message to zylos`, `check zylos status`, `zylos conversation history`, `search zylos messages`, `use zylos`, `run zylos`."
author: "summingyu"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zylos-pp-cli
---

# Zylos Console — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zylos-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install zylos --cli-only
   ```
2. Verify: `zylos-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Zylos Console CLI connects to your local Zylos instance to send messages, monitor status, and sync conversations to a local SQLite database. Search past messages offline, analyze response patterns, and stream new messages in real-time.

## When to Use This CLI

Use the Zylos CLI when you need to interact with a local Zylos Console instance from scripts, automation pipelines, or a terminal. Ideal for monitoring AI agent status, archiving conversations, or piping messages between tools.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local analytics
- **`stats`** — See message counts, response patterns, and activity trends across all your Zylos conversations.

  _Use this to understand your interaction patterns and Zylos usage trends without manual counting._

  ```bash
  zylos-pp-cli stats --days 7 --json
  ```
- **`timeline`** — View your conversation history as a chronological timeline with gap detection.

  _See the full arc of a conversation day at a glance, including gaps where the AI was idle._

  ```bash
  zylos-pp-cli timeline --today --json
  ```
- **`search`** — Search conversation history with surrounding messages for context.

  _Find what was discussed around a keyword, not just the matching message alone._

  ```bash
  zylos-pp-cli search "deployment" --context 3 --json
  ```
- **`latency`** — Analyze AI response times across conversations.

  _Track whether the AI agent is getting slower or faster over time._

  ```bash
  zylos-pp-cli latency --last 10 --json
  ```

### Data portability
- **`export`** — Export conversations to JSON or Markdown for archival and sharing.

  _Archive important conversations before clearing history or migrating setups._

  ```bash
  zylos-pp-cli export --format markdown --output ./conversations/
  ```

### Monitoring
- **`status watch`** — Monitor AI agent status with state-change detection and auto-exit.

  _Wait for the AI to become available before sending a message, scriptable in CI._

  ```bash
  zylos-pp-cli status watch --watch --until idle
  ```
- **`conversations follow`** — Stream new messages in real-time, pipeable to other tools.

  _Tail the AI conversation in real-time from scripts or other CLIs._

  ```bash
  zylos-pp-cli conversations follow --follow --json | jq '.content'
  ```

## Command Reference

**conversations** — Conversation history and messaging

- `zylos-pp-cli conversations poll` — Poll for new messages since a given message ID
- `zylos-pp-cli conversations recent` — Get recent conversation messages
- `zylos-pp-cli conversations send` — Send a message to the AI

**session** — Authentication and session management

- `zylos-pp-cli session check` — Check if authentication is required and if the current session is authenticated
- `zylos-pp-cli session login` — Authenticate with a password to establish a session
- `zylos-pp-cli session logout` — End the current authenticated session

**status** — AI agent status monitoring

- `zylos-pp-cli status` — Get the current status of the AI agent


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zylos-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Wait for AI to be ready, then send

```bash
zylos-pp-cli status watch --watch --until idle
```

Blocks until the AI agent is idle — useful in automation scripts.

### Search conversations and extract content

```bash
zylos-pp-cli search "error" --context 2 --json --select messages.content
```

Search for messages mentioning 'error' with surrounding context, extract just the content field.

### Stream new messages to a file

```bash
zylos-pp-cli conversations follow --follow --json >> conversation-log.jsonl
```

Append new messages as JSON lines to a log file for persistent monitoring.

### Export today's conversations as Markdown

```bash
zylos-pp-cli export --today --format markdown --output ./exports/
```

Create a Markdown file with today's full conversation for documentation.

## Auth Setup

This CLI uses a browser session. Log in to  in Chrome, then:

```bash
zylos-pp-cli auth login --chrome
```

Requires a cookie extraction tool (`pycookiecheat` via pip, or `cookies` via Homebrew).

Run `zylos-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  zylos-pp-cli status --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
zylos-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zylos-pp-cli feedback --stdin < notes.txt
zylos-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.zylos-pp-cli/feedback.jsonl`. They are never POSTed unless `ZYLOS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZYLOS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zylos-pp-cli profile save briefing --json
zylos-pp-cli --profile briefing status
zylos-pp-cli profile list --json
zylos-pp-cli profile show briefing
zylos-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `zylos-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add zylos-pp-mcp -- zylos-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zylos-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zylos-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zylos-pp-cli <command> --help`.
