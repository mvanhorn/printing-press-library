---
name: pp-kakao-transparency
description: "Kakao's 2012-present government data-request statistics as one queryable archive — series, latest-report resolution, and workbook mirroring no other tool offers. Trigger phrases: `kakao transparency report`, `korean government data requests kakao`, `kakao warrant statistics`, `daum government requests`, `use kakao-transparency`, `run kakao-transparency`."
author: "Kieran Maynard"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
regions: ["KR"]
api_language: "ko"
metadata:
  openclaw:
    requires:
      bins:
        - kakao-transparency-pp-cli
    install:
      - kind: go
        bins: [kakao-transparency-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/kakao-transparency/cmd/kakao-transparency-pp-cli
---

# Kakao Transparency — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `kakao-transparency-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install kakao-transparency --cli-only
   ```
2. Verify: `kakao-transparency-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/kakao-transparency/cmd/kakao-transparency-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Kakao publishes semiannual transparency reports (warrants, communication data, restriction measures for Kakao and Daum services) behind a one-period-at-a-time web page. This CLI turns that into an agent-friendly archive: `series` builds the full longitudinal table in one call, `latest` resolves the newest period without guessing, and `workbooks` indexes the official XLSX editions.

## When to Use This CLI

Use this CLI for questions about South Korean government requests for Kakao/Daum user data: warrant volumes, compliance trends, communication-data policy history. It is the fastest path to the full 2012-present series and to the official XLSX workbooks.

## Anti-triggers

Do not use this CLI for:
- Do not use it for Kakao's developer/business APIs (KakaoTalk messaging, Kakao Login, ad reports) — this wraps only the privacy transparency statistics
- Do not use it for content-moderation or DSA-style statistics; Kakao's transparency report covers government data requests only
- Do not sum numberOfAccounts across categories — categories overlap in what they count and -1 means N/A, not zero

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Whole-archive analytics
- **`series`** — One tidy table of every published half-year since 2012 — requests, processed, and affected accounts per service corporation and request category — instead of 28 separate report lookups.

  _Reach for this when a task needs trends over time (e.g. 'how did warrant volumes change since 2015') rather than a single period's numbers._

  ```bash
  kakao-transparency-pp-cli series --category warrant --service kakao --csv
  ```
- **`workbooks`** — List the official XLSX workbook download URLs (Korean and English editions) for every published half-year.

  _Use this when the deliverable needs the official source files (archival mirroring, citation) rather than the parsed numbers._

  ```bash
  kakao-transparency-pp-cli workbooks --since 2020 --agent
  ```

### Agent-native plumbing
- **`latest`** — Fetch the newest published half-year without knowing which period it is.

  _Use this as the first call whenever the task says 'current' or 'most recent' — it removes the guess-the-period failure mode._

  ```bash
  kakao-transparency-pp-cli latest --agent
  ```

## Command Reference

**transparency** — Semiannual government data-request statistics (2012–present) for Kakao and Daum services.

- `kakao-transparency-pp-cli transparency` — Returns the transparency-report statistics for one half-year: eight statistics rows (Kakao and Daum


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
kakao-transparency-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Warrant compliance trend for Kakao services

```bash
kakao-transparency-pp-cli series --service kakao --category warrant --agent --select year,halfYear,numberOfRequests,numberOfProcesses
```

One tidy JSON series of search-and-seizure warrant volumes and processing since 2012, narrowed to the four fields a chart needs.

### Newest report, narrative included

```bash
kakao-transparency-pp-cli latest --agent
```

Resolves the most recent half-year and returns its full payload including Kakao's own trend commentary.

### Mirror the official workbooks since 2020

```bash
kakao-transparency-pp-cli workbooks --since 2020 --agent
```

Lists the Korean and English XLSX download URLs per half-year for archival mirroring.

### One period as CSV

```bash
kakao-transparency-pp-cli transparency --year 2024 --half-year-id 1 --csv
```

The eight statistics rows (2 services x 4 categories) for 1H 2024 in spreadsheet-ready form.

## Auth Setup

No authentication required.

Run `kakao-transparency-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  kakao-transparency-pp-cli transparency --year 2025 --half-year-id 1 --agent --select id,name,status
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
kakao-transparency-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
kakao-transparency-pp-cli feedback --stdin < notes.txt
kakao-transparency-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/kakao-transparency-pp-cli/feedback.jsonl`. They are never POSTed unless `KAKAO_TRANSPARENCY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `KAKAO_TRANSPARENCY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
kakao-transparency-pp-cli profile save briefing --json
kakao-transparency-pp-cli --profile briefing transparency --year 2025 --half-year-id 1
kakao-transparency-pp-cli profile list --json
kakao-transparency-pp-cli profile show briefing
kakao-transparency-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `kakao-transparency-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/kakao-transparency/cmd/kakao-transparency-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add kakao-transparency-pp-mcp -- kakao-transparency-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which kakao-transparency-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   kakao-transparency-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `kakao-transparency-pp-cli <command> --help`.
