---
name: pp-techmeme
description: "Every Techmeme headline, searchable and cached locally — plus topic tracking, trending analysis, and catch-up workflows no other tool has. Trigger phrases: `what's happening in tech`, `tech news today`, `check techmeme`, `what did I miss in tech`, `trending tech stories`, `use techmeme`, `run techmeme`."
author: "Dave Morin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - techmeme-pp-cli
    install:
      - kind: go
        bins: [techmeme-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/techmeme/cmd/techmeme-pp-cli
---

# Techmeme — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `techmeme-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install techmeme --cli-only
   ```
2. Verify: `techmeme-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.25+):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/techmeme/cmd/techmeme-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Techmeme CLI puts the tech industry's most trusted news curation into your terminal. Sync headlines to a local SQLite store, then search, filter by time, track topics, and analyze which stories and sources are dominating. The 'since' command answers the question every tech professional asks: 'what did I miss?'

## When to Use This CLI

Use the Techmeme CLI when you need curated, authoritative tech news without opening a browser. Ideal for morning catch-up ('since 8h'), topic monitoring ('track add OpenAI'), source analysis ('sources'), and AI agents that need to answer 'what's happening in tech right now' with structured data.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Time intelligence
- **`since`** — See every tech headline from the last N hours — the perfect catch-up when you've been away

  _When an agent needs to brief a user on what happened in tech while they were in meetings, this is the single command that answers it_

  ```bash
  techmeme-pp-cli since 4h --agent
  ```
- **`digest`** — Get a day's tech news grouped by topic — the briefing you'd write if you had time

  _When an agent needs to produce a tech news briefing for a specific date, this structures raw headlines into a readable summary_

  ```bash
  techmeme-pp-cli digest --date 2026-05-08 --agent
  ```

### Persistent monitoring
- **`track`** — Save topics and get alerts when they hit Techmeme — persistent monitoring without browser tabs

  _Agents monitoring specific companies or technologies can subscribe to exactly what matters without polling the full feed_

  ```bash
  techmeme-pp-cli track add 'OpenAI' && techmeme-pp-cli track check --agent
  ```

### News intelligence
- **`sources`** — See which publications dominate Techmeme and track source frequency over time

  _When analyzing media landscape or choosing which publications to prioritize, this gives hard data on source influence_

  ```bash
  techmeme-pp-cli sources --top 20 --agent
  ```
- **`trending`** — Extract the hottest topics from recent headlines using frequency analysis on cached data

  _When an agent needs to answer 'what's hot in tech right now' with data instead of vibes_

  ```bash
  techmeme-pp-cli trending --hours 24 --agent
  ```
- **`velocity`** — Find stories that are blowing up — multiple sources covering the same topic in a short window

  _When an agent needs to identify breaking news vs steady coverage, velocity shows what's accelerating now_

  ```bash
  techmeme-pp-cli velocity --agent
  ```
- **`author`** — Find all Techmeme headlines by a specific journalist across the cached archive

  _When tracking a specific journalist's coverage or building a media contact list, this surfaces their Techmeme footprint_

  ```bash
  techmeme-pp-cli author 'Kara Swisher' --agent
  ```

## Command Reference

**feed-xml** — Manage feed xml

- `techmeme-pp-cli feed-xml` — Top 15 headlines currently on Techmeme. RSS 2.0 format. Each item has title, link (to Techmeme permalink),...

**lb-opml** — Manage lb opml

- `techmeme-pp-cli lb-opml` — OPML file listing Techmeme's top 51 sources with source name, website URL, and RSS feed URL. Updated regularly based...

**river** — Manage river

- `techmeme-pp-cli river` — 5-day rolling archive of all Techmeme headlines in reverse chronological order. 150+ headlines with timestamp,...

**techmeme-search** — Manage techmeme search

- `techmeme-pp-cli techmeme-search headlines` — Search Techmeme headlines. Supports quoted phrases, wildcards, +/-, AND/OR/NOT, parentheses. Can filter by url,...
- `techmeme-pp-cli techmeme-search rss` — RSS feed of search results. Same query syntax as /search/query. Subscribe in any RSS reader for alerts on specific...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
techmeme-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning briefing

```bash
techmeme-pp-cli since 12h --agent --select title,source,time
```

Catch up on overnight tech news with just the essentials — title, source, and timestamp

### Track a company

```bash
techmeme-pp-cli search 'Apple' --agent --select title,source,link
```

Find all recent Techmeme headlines about Apple

### Media landscape

```bash
techmeme-pp-cli sources --top 20 --agent
```

See which 20 publications dominate Techmeme's curated feed

### Breaking news detection

```bash
techmeme-pp-cli velocity --agent
```

Find stories gaining momentum — multiple sources in a short window

### Daily summary

```bash
techmeme-pp-cli digest --agent
```

Today's tech news grouped by topic for a quick read

## Auth Setup

No authentication required.

Run `techmeme-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  techmeme-pp-cli river --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
techmeme-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
techmeme-pp-cli feedback --stdin < notes.txt
techmeme-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.techmeme-pp-cli/feedback.jsonl`. They are never POSTed unless `TECHMEME_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TECHMEME_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
techmeme-pp-cli profile save briefing --json
techmeme-pp-cli --profile briefing river
techmeme-pp-cli profile list --json
techmeme-pp-cli profile show briefing
techmeme-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `techmeme-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/techmeme/cmd/techmeme-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add techmeme-pp-mcp -- techmeme-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which techmeme-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   techmeme-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `techmeme-pp-cli <command> --help`.
