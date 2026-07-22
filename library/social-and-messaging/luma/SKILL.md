---
name: pp-luma
description: "Browse, search, and export public Luma events from one Go binary — no account, no API key Trigger phrases: `events in sf`, `AI events this week`, `what's happening in nyc on luma`, `luma events near me`, `use luma`, `run luma`."
author: "richardadonnell"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - luma-pp-cli
    install:
      - kind: go
        bins: [luma-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/cmd/luma-pp-cli
---

# Luma — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `luma-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install luma --cli-only
   ```
2. Verify: `luma-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/cmd/luma-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

luma reads Luma's public discovery surface (api.luma.com) to list events by city and category, fetch full event and calendar details, and mirror it all into a local SQLite store. On top of the raw API it adds what Luma never exposed publicly: full-text search, geo-radius lookup, cross-city aggregation, change tracking, and ICS export — all agent-native with --json and --select.

## When to Use This CLI

Use luma when an agent needs to discover or report on public Luma events — what's happening in a city, events for a topic, full details for an event or community, or a calendar export. It is ideal for read-only event discovery, aggregation across cities, and tracking how events fill up over time.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to create, edit, or cancel events — that needs the paid Luma Plus API.
- Do not use it to manage guest lists, check-ins, RSVPs, or ticketing.
- Do not use it to read private or subscriber-only calendars; it only sees public discovery data.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Query the API can't
- **`agenda`** — One flat, date-sorted list of upcoming events across multiple cities and categories at once.

  _Reach for this when the user asks 'what AI events are on this week' across several cities — no single Luma call can answer it._

  ```bash
  luma-pp-cli agenda --city sf --city nyc --category cat-ai --window 7d --agent
  ```
- **`near`** — Find events within N km of a lat/lng, ranked by distance.

  _Use when the user wants events near a point or venue, which no Luma API call supports._

  ```bash
  luma-pp-cli near --lat 37.77 --lng -122.42 --radius-km 5 --window 14d --agent
  ```

### Take it with you
- **`ics`** — Export a synced/filtered set of events to a .ics calendar file your calendar app can import.

  _Use when the user wants events on their own calendar instead of re-reading JSON._

  ```bash
  luma-pp-cli ics --city sf --category cat-ai --window 30d --out ai.ics
  ```

### Track over time
- **`watch`** — Show what changed on synced events since the previous sync — new, removed, filling up, sold out, rescheduled.

  _Reach for this to monitor events over time; a single API call can never show change._

  ```bash
  luma-pp-cli watch --city sf --category cat-ai --agent
  ```
- **`calendars compare`** — Side-by-side upcoming-event counts and total guest counts across several calendars (communities).

  _Use to benchmark which communities are most active or drawing the biggest crowds._

  ```bash
  luma-pp-cli calendars compare cal-AAA cal-BBB cal-CCC --window 14d --agent
  ```

## Command Reference

**calendars** — Fetch Luma calendar (community) details

- `luma-pp-cli calendars` — Get a calendar (community) by its api_id (cal-...)

**discover** — Discover featured events, places, and categories on Luma

- `luma-pp-cli discover categories` — List event categories (AI, Crypto, Tech, Arts, ...) with upcoming-event counts
- `luma-pp-cli discover home` — Featured place, popular places, categories, and calendars on the Luma discover home

**events** — Browse and fetch public Luma events

- `luma-pp-cli events get` — Get full details for an event by its api_id (evt-...)
- `luma-pp-cli events list` — List upcoming events filtered by city (--city/--place-id) or category (--category)

**places** — Browse cities/places that host Luma events

- `luma-pp-cli places calendars` — List calendars (communities) active in a place
- `luma-pp-cli places get` — Get a place (city) by slug (sf, nyc, miami, ...) or place api_id
- `luma-pp-cli places map` — Geographic points for events in a place, for mapping


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
luma-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Event detail, narrowed for an agent

```bash
luma-pp-cli events get --event-id evt-XXXX --agent --select api_id,event.name,event.start_at,event.timezone,guest_count,ticket_count
```

Event detail is large and deeply nested; --select pulls only the fields you need so an agent doesn't burn context.

### AI events across three cities this week

```bash
luma-pp-cli agenda --city sf --city nyc --category cat-ai --window 7d --agent
```

Unions several cities filtered to a category into one flat, date-sorted list the API can't return.

### Events near a point

```bash
luma-pp-cli near --lat 37.77 --lng -122.42 --radius-km 5 --window 14d --agent
```

Haversine radius search over synced event coordinates — no Luma endpoint offers this.

### Export a city to your calendar

```bash
luma-pp-cli ics --city sf --out sf.ics
```

Writes an ICS file you can import into any calendar app.

## Auth Setup

No account or API key required. luma reads Luma's public, read-only discovery API the website itself uses. It cannot create events, manage guests, or read private/subscribed calendars — that is the paid Luma Plus API's job.

Run `luma-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  luma-pp-cli calendars --calendar-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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
luma-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
luma-pp-cli feedback --stdin < notes.txt
luma-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/luma-pp-cli/feedback.jsonl`. They are never POSTed unless `LUMA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LUMA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
luma-pp-cli profile save briefing --json
luma-pp-cli --profile briefing calendars --calendar-id 550e8400-e29b-41d4-a716-446655440000
luma-pp-cli profile list --json
luma-pp-cli profile show briefing
luma-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `luma-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/cmd/luma-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add luma-pp-mcp -- luma-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which luma-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   luma-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `luma-pp-cli <command> --help`.
