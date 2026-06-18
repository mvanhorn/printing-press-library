---
name: pp-perplexity
description: "Browse and export Perplexity research traces from the terminal."
author: "Erik Rogne"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - perplexity-pp-cli
    install:
      - kind: go
        bins: [perplexity-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/perplexity/cmd/perplexity-pp-cli
---

# Perplexity — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `perplexity-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install perplexity --cli-only
   ```
2. Verify: `perplexity-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/perplexity/cmd/perplexity-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Perplexity already holds the user's live research trail. This CLI turns that trail into a terminal-first workflow: browse recent threads, inspect full transcripts, and export individual conversations as Markdown, PDF, or DOCX for durable storage in the monorepo.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`history export`** — Export a Perplexity thread as Markdown, PDF, or DOCX from the logged-in browser session.

  ```bash
  perplexity-pp-cli history export --thread-uuid <uuid> --format md
  ```
- **`history read`** — Fetch a full Perplexity thread transcript by UUID or slug.

  ```bash
  perplexity-pp-cli history read <entry_uuid_or_slug> --agent
  ```
- **`history recent`** — List recent Perplexity threads from the signed-in account.

  ```bash
  perplexity-pp-cli history recent --agent
  ```
- **`auth login --chrome`** — Capture the browser session from Chrome instead of asking for a paid API key.

  ```bash
  perplexity-pp-cli auth login --chrome
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Recipes

### Export a thread

```bash
perplexity-pp-cli history export --thread-uuid <uuid> --format md
```

Exports the thread to Markdown so the transcript can be stored in raw knowledge or attached to a note.

### Inspect recent research

```bash
perplexity-pp-cli history recent --agent
```

Lists the newest Perplexity threads in a machine-readable form for quick triage.

### Read a transcript

```bash
perplexity-pp-cli history read <entry_uuid_or_slug> --agent
```

Fetches the entire conversation and its source metadata for review or archival.

## Command Reference

**history** — Perplexity thread history, transcripts, and export helpers.

- `perplexity-pp-cli history export` — Export a Perplexity thread as Markdown, PDF, or DOCX.
- `perplexity-pp-cli history read` — Fetch a full Perplexity thread transcript by UUID or slug.
- `perplexity-pp-cli history recent` — List recent Perplexity threads from the signed-in account.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
perplexity-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Perplexity uses a logged-in browser session for history and export flows. The CLI is designed to work from an authenticated Chrome session so agents can reuse the user's account without relying on the paid API.

Run `perplexity-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  perplexity-pp-cli history export --thread-uuid 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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
perplexity-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
perplexity-pp-cli feedback --stdin < notes.txt
perplexity-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/perplexity-pp-cli/feedback.jsonl`. They are never POSTed unless `PERPLEXITY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PERPLEXITY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
perplexity-pp-cli profile save briefing --json
perplexity-pp-cli --profile briefing history export --thread-uuid 550e8400-e29b-41d4-a716-446655440000
perplexity-pp-cli profile list --json
perplexity-pp-cli profile show briefing
perplexity-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `perplexity-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/perplexity/cmd/perplexity-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add perplexity-pp-mcp -- perplexity-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which perplexity-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   perplexity-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `perplexity-pp-cli <command> --help`.
