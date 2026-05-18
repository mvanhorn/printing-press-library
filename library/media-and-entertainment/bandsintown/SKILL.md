---
name: pp-bandsintown
description: "Turn Bandsintown's two read-only endpoints into a tour-routing brain: local cache, calendar-gap detection, lineup... Trigger phrases: `is artist X touring near jakarta`, `find routing candidates for a jakarta show`, `who's playing southeast asia in august`, `co-bills for phoenix`, `rising tracker count artists`, `use bandsintown`, `run bandsintown`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bandsintown-pp-cli
---

# Bandsintown — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bandsintown-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install bandsintown --cli-only
   ```
2. Verify: `bandsintown-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/bandsintown/cmd/bandsintown-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every existing Bandsintown wrapper is a 10-year-old language binding for the same two API endpoints. This CLI absorbs all of them and adds the queries promoters and tour routers actually need: feasibility-ranked routing candidates for a target city and date, empty windows in an artist's calendar, co-bill patterns across past festivals, and tracker_count trends over time. Local SQLite store, FTS search, agent-native --json output, and MCP exposure are standard.

## When to Use This CLI

Reach for this CLI when a booking question depends on aggregating Bandsintown data across artists or over time — tour-routing feasibility, calendar-gap detection, co-bill patterns, tracker_count deltas. For one-off 'what shows does Phoenix have' queries, the bandsintown.com web UI is fine. For repeatable, agent-driven, multi-artist intelligence, this is the right tool.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Routing intelligence
- **`route`** — Find which tracked artists already have shows near a target city within a date window — feasibility-ranked routing candidates for booking.

  _When booking an event in a specific city, the cheapest path is finding an artist already routing through the region. Reach for this before any cold outreach._

  ```bash
  bandsintown-pp-cli route --to "Jakarta,ID" --on 2026-08-15 --window 7d --tracked --json
  ```
- **`gaps`** — Find empty windows in an artist's tour calendar that match an event slot length, optionally constrained to a region.

  _Use this to identify artists with a touring gap that aligns with your event date — the most actionable booking lead._

  ```bash
  bandsintown-pp-cli gaps "Beach House" --min 5d --max 21d --in SEA --json
  ```
- **`sea-radar`** — One-shot Southeast Asia briefing: all upcoming shows in a date range across tracked artists, grouped by city, tagged with tracker tier.

  _Monday-morning briefing in one invocation: who is touring SEA in your event window, ranked by demand._

  ```bash
  bandsintown-pp-cli sea-radar --date 2026-08-01,2026-08-31 --tier mid --json
  ```

### Lineup intelligence
- **`lineup co-bill`** — Surface which artists frequently co-bill with a given artist by aggregating lineup arrays across many events.

  _When building festival lineups, find natural co-bill pairings backed by real shared-stage history._

  ```bash
  bandsintown-pp-cli lineup co-bill "Phoenix" --since 2024-01-01 --min-shared 2 --json
  ```

### Demand intelligence
- **`trend`** — Track tracker_count and upcoming_event_count over time per artist; surface rising and falling demand signals.

  _Rising tracker_count is a leading indicator of audience demand; use to decide who to book before peers notice._

  ```bash
  bandsintown-pp-cli trend --top 20 --period 30d --json
  ```

### Agent-native plumbing
- **`pull`** — Re-fetch every tracked artist's events with a staleness window; emit a structured diff (added / removed / changed events) for downstream agents.

  _Run this in a daily cron; pipe the diff into Slack or a project tracker to alert when tracked artists add new shows._

  ```bash
  bandsintown-pp-cli pull --tracked --since-stale 12 --json
  ```

### Local state that compounds
- **`track`** — Local watchlist of artists you care about. Drives sync, snapshot, route, and sea-radar.

  _Build a curated watchlist once; every other intelligence command reads from it._

  ```bash
  bandsintown-pp-cli track add "Phoenix" "Tame Impala" "Beach House"
  ```

## Command Reference

**artists** — Manage artists

- `bandsintown-pp-cli artists <artistname>` — Get artist information

**events** — Manage events

- `bandsintown-pp-cli events <artistname>` — Get upcoming, past, or all artist events, or events within a date range


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bandsintown-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday-morning SEA routing briefing

```bash
bandsintown-pp-cli sea-radar --date 2026-08-01,2026-08-31 --tier mid --json --select 'shows.city,shows.artist,shows.datetime,shows.tracker_count'
```

Print a SEA-only briefing for the August event window with only the fields you need. Run `bandsintown-pp-cli pull --tracked` first to hydrate the local store.

### Rising-demand artists this month

```bash
bandsintown-pp-cli trend --top 20 --period 30d --json
```

Compare today's tracker_count snapshot against snapshots from the last 30 days. Output the top 20 rising artists. Run `bandsintown-pp-cli snapshot` more than once over time to grow the time series.

### Festival co-bill for a candidate headliner

```bash
bandsintown-pp-cli lineup co-bill "Phoenix" --since 2024-01-01 --min-shared 2 --json
```

Surface every artist who shared a stage with Phoenix on 2+ events since 2024. Pipes cleanly into your lineup planner.

### Find a gap in an artist's calendar that fits your event

```bash
bandsintown-pp-cli gaps "Beach House" --min 5d --max 21d --in SEA --json
```

Show empty windows of 5–21 days in Beach House's SEA touring calendar — the actionable booking leads.

## Auth Setup

No authentication required.

Run `bandsintown-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bandsintown-pp-cli artists mock-value --app-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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
bandsintown-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bandsintown-pp-cli feedback --stdin < notes.txt
bandsintown-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.bandsintown-pp-cli/feedback.jsonl`. They are never POSTed unless `BANDSINTOWN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BANDSINTOWN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
bandsintown-pp-cli profile save briefing --json
bandsintown-pp-cli --profile briefing artists mock-value --app-id 550e8400-e29b-41d4-a716-446655440000
bandsintown-pp-cli profile list --json
bandsintown-pp-cli profile show briefing
bandsintown-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `bandsintown-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add bandsintown-pp-mcp -- bandsintown-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bandsintown-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bandsintown-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bandsintown-pp-cli <command> --help`.
