---
name: pp-tenderned
description: "Dutch public-tender CLI with offline search, document corpus, and the sub-threshold long tail TED never sees."
author: "markvandeven"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
regions: ["NL"]
api_language: "nl"
metadata:
  openclaw:
    requires:
      bins:
        - tenderned-pp-cli
---

# Tenderned — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tenderned-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tenderned --cli-only
   ```
2. Verify: `tenderned-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/tenderned/cmd/tenderned-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**buyers** — Browse contracting authorities (aanbestedende diensten) — Dutch public buyers

- `tenderned-pp-cli buyers get` — Fetch one contracting authority by ID
- `tenderned-pp-cli buyers list` — List Dutch contracting authorities (paginated)

**docs** — List and download tender documents (bestek, PvE, evaluation criteria, Q&A)

- `tenderned-pp-cli docs download` — Download all documents for one publication as a zip archive
- `tenderned-pp-cli docs get` — Download a single document's binary content (PDF/Word/etc.)
- `tenderned-pp-cli docs list` — List attached documents for one publication

**notices** — Search, list and fetch tender notices (aankondigingen) from TenderNed — mirrors 'eu-tenders notices' for the Dutch market

- `tenderned-pp-cli notices get` — Fetch full structured metadata for one publication
- `tenderned-pp-cli notices list` — Search and list tender publications with rich filters (CPV, dates, buyer, procedure, scope)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tenderned-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `tenderned-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tenderned-pp-cli buyers list --agent --select id,name,status
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
tenderned-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tenderned-pp-cli feedback --stdin < notes.txt
tenderned-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/tenderned-pp-cli/feedback.jsonl`. They are never POSTed unless `TENDERNED_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TENDERNED_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tenderned-pp-cli profile save briefing --json
tenderned-pp-cli --profile briefing buyers list
tenderned-pp-cli profile list --json
tenderned-pp-cli profile show briefing
tenderned-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `tenderned-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add tenderned-pp-mcp -- tenderned-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tenderned-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tenderned-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tenderned-pp-cli <command> --help`.
