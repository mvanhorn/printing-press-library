---
name: pp-fireflies
description: "Every Fireflies meeting feature, plus offline search, cross-meeting intelligence, and a local database no other tool... Trigger phrases: `find stale action items from meetings`, `search my meeting transcripts for`, `who talked most in recent meetings`, `sync fireflies meetings`, `use fireflies-pp-cli`, `run fireflies`, `what did we discuss with`."
author: "Nikica Jokic"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - fireflies-pp-cli
    install:
      - kind: go
        bins: [fireflies-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/fireflies/cmd/fireflies-pp-cli
---

# Fireflies.ai — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `fireflies-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install fireflies --cli-only
   ```
2. Verify: `fireflies-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/fireflies/cmd/fireflies-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync your entire meeting history once, then search, analyze, and correlate across every conversation without touching the API. Find stale action items, track topic escalation over weeks, reconstruct the full history with any person or account — all offline, all composable with jq and SQL.

## When to Use This CLI

Use fireflies-pp-cli when you need to mine your meeting history at scale: finding dropped action items, preparing for calls with full relationship context, tracking sentiment or topic trends over weeks, or searching across thousands of meeting sentences offline. Not a substitute for the Fireflies dashboard for day-to-day recording configuration or notetaking.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`transcripts search`** — Full-text search across all synced meeting transcripts without consuming any API quota.

  _Use to find every time a specific topic was mentioned across all meetings without burning rate-limited API calls._

  ```bash
  fireflies-pp-cli transcripts search "pricing objection" --agent --select id,title,dateString
  ```
- **`action-items list`** — Aggregate action items from all meetings in a date range — weekly commitment audit in one command.

  _Use at the end of the week to harvest all commitments made in meetings and push to a TODO file._

  ```bash
  fireflies-pp-cli action-items list --from 7 --agent
  ```
- **`transcripts find`** — Find meetings by participant email, channel name, keyword, or date range — all client-side, no broken API date filters.

  _Use when you need meetings with a specific person or channel — the API's title-based search fails when meeting names don't contain participant names._

  ```bash
  fireflies-pp-cli transcripts find --participant danijel.latin@verybigthings.com --from 30 --processed-only --agent
  ```
- **`transcripts status`** — Show PROCESSED / PROCESSING / FAILED status for recent meetings upfront — know before you fetch.

  _Use when fetching same-day or next-morning meetings — avoids the loop of 'meeting not ready yet' failures._

  ```bash
  fireflies-pp-cli transcripts status --since 48h --agent
  ```
- **`topics list`** — Most frequent topics across all meetings in a date range — what is actually consuming meeting time.

  _Use during quarterly planning to identify recurring themes before deciding where to invest time._

  ```bash
  fireflies-pp-cli topics list --from 30 --top 15 --agent
  ```
- **`digest`** — Aggregate view of all recent meetings: titles, gists, topics, and action items in one structured output.

  _Use at session start or in a morning cron to orient on what happened yesterday before taking action._

  ```bash
  fireflies-pp-cli digest --since 24h --agent
  ```
- **`transcripts export`** — Export a transcript as markdown to a vault directory with auto-generated YYYY-MM-DD_title.md filename.

  _Use after a client meeting to save the formatted transcript directly to the right project folder._

  ```bash
  fireflies-pp-cli transcripts export abc123 --vault ~/vaults/VBT/Projects/1_Active/Ryder/transcripts/ --agent
  ```

### Person-centric intelligence
- **`person timeline`** — Chronological meeting history with a specific person — topics, action items, and talk ratio per meeting.

  _Use before a QBR or renewal call to reconstruct the full relationship history without reading every transcript._

  ```bash
  fireflies-pp-cli person timeline danijel.latin@verybigthings.com --from 90 --agent
  ```

## Command Reference

**sync** — Sync API data to local SQLite

- `fireflies-pp-cli sync` — Sync transcripts, channels, and users to local SQLite
- `fireflies-pp-cli sync --full` — Full resync from scratch
- `fireflies-pp-cli sync --resources transcripts,channels` — Sync specific resources

**transcripts** — List, search, and manage meeting transcripts

- `fireflies-pp-cli transcripts list` — List transcripts from local store
- `fireflies-pp-cli transcripts get <id>` — Get a transcript from local store (or API with --live)
- `fireflies-pp-cli transcripts find` — Find by participant, channel, keyword, or date
- `fireflies-pp-cli transcripts search <query>` — Offline full-text search
- `fireflies-pp-cli transcripts recent` — Show recently processed transcripts
- `fireflies-pp-cli transcripts status` — Show PROCESSED / PROCESSING / FAILED status
- `fireflies-pp-cli transcripts pull <id>` — Fetch full transcript from API and store locally
- `fireflies-pp-cli transcripts export <id>` — Export as markdown to file or vault path
- `fireflies-pp-cli transcripts update <id>` — Update title or privacy
- `fireflies-pp-cli transcripts share <id>` — Share with external email addresses
- `fireflies-pp-cli transcripts delete <id>` — Delete a transcript

**summary** — AI-generated meeting summary

- `fireflies-pp-cli summary <id>` — Get summary (formats: overview, bullets, gist, topics, keywords, actions)

**action-items** — Extract action items

- `fireflies-pp-cli action-items get <id>` — Action items from a specific transcript
- `fireflies-pp-cli action-items list` — Aggregate action items across a date range

**topics** — Extract and analyze topics

- `fireflies-pp-cli topics get <id>` — Topics from a specific transcript
- `fireflies-pp-cli topics list` — Most frequent topics across all transcripts

**keywords** — AI-extracted keywords

- `fireflies-pp-cli keywords <id>` — Keywords for a transcript

**speakers** — Per-speaker analytics

- `fireflies-pp-cli speakers <id>` — Talk time, word count, filler words, questions per speaker

**analytics** — Meeting analytics

- `fireflies-pp-cli analytics team` — Team-wide analytics (requires Business+ plan)
- `fireflies-pp-cli analytics meeting <id>` — Per-meeting speaker analytics

**person** — Person-centric views

- `fireflies-pp-cli person timeline <email-or-name>` — Chronological meeting history with a person
- `fireflies-pp-cli person complaints <email-or-name>` — Negative-sentiment mentions across meetings

**digest** — Aggregate meeting briefing

- `fireflies-pp-cli digest` — Summary of all recent meetings in one view

**channels** — Channel management

- `fireflies-pp-cli channels <id>` — Get a single channel

**users** — User management

- `fireflies-pp-cli users` — Get user information


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
fireflies-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning triage: what happened in meetings yesterday

```bash
fireflies-pp-cli transcripts list --from 24h --agent --select id,title,dateString,duration,summary.gist
```

List yesterday's meetings with one-line AI summaries — no API calls if already synced.

### Find every time a client mentioned pricing

```bash
fireflies-pp-cli search "pricing" --from 90d --agent --select id,title,speaker_name,text
```

Full-text search across synced sentences. The --select dotted path drills into nested fields.

### Prepare for a QBR with account history

```bash
fireflies-pp-cli person timeline "Acme" --from 180 --agent
```

Chronological view of every meeting where 'Acme' appeared — topics, action items, talk ratios.

### Harvest this week's action items

```bash
fireflies-pp-cli action-items list --from 7 --append ~/vaults/VBT/TODO.md
```

Aggregate action items from all meetings in the last 7 days and append to a markdown TODO file.

### Find what topics came up most this month

```bash
fireflies-pp-cli topics list --from 30 --top 15 --agent
```

Most frequent topics across all meetings in the last 30 days — what is actually consuming meeting time.

## Auth Setup

Requires a Fireflies API key set as FIREFLIES_API_KEY. API access requires a Business plan or higher. Get your key at app.fireflies.ai → Settings → Developer.

Run `fireflies-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  fireflies-pp-cli transcripts list --agent --select id,title,dateString,summary.gist
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

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
fireflies-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
fireflies-pp-cli feedback --stdin < notes.txt
fireflies-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.fireflies-pp-cli/feedback.jsonl`. They are never POSTed unless `FIREFLIES_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FIREFLIES_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
fireflies-pp-cli profile save briefing --json
fireflies-pp-cli --profile briefing transcripts list
fireflies-pp-cli profile list --json
fireflies-pp-cli profile show briefing
fireflies-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `fireflies-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/fireflies/cmd/fireflies-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add fireflies-pp-mcp -- fireflies-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which fireflies-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   fireflies-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `fireflies-pp-cli <command> --help`.
