---
name: pp-hotel-tonight
description: "Last-minute hotel deals with a local price-history database no HotelTonight client has — snapshot deals over time,... Trigger phrases: `find a last-minute hotel tonight`, `watch this city for hotel price drops`, `is this hotel price a good deal`, `what's the daily drop in San Francisco`, `cheapest hotel night this weekend`, `use hotel-tonight`, `run hotel-tonight`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - hotel-tonight-pp-cli
    install:
      - kind: go
        bins: [hotel-tonight-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/cmd/hotel-tonight-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/hotel-tonight/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# HotelTonight — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `hotel-tonight-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install hotel-tonight --cli-only
   ```
2. Verify: `hotel-tonight-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/cmd/hotel-tonight-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent or script needs last-minute hotel deal data from HotelTonight that persists and compounds: monitoring a city for price drops, building a price history for a hotel, judging whether a quoted rate is actually cheap, or comparing neighborhoods and dates. It is read-only and anonymous — it never books and never needs a login.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local price intelligence

- **`watch`** — Snapshot a location's deals now and get told which rooms dropped below your threshold or fell since you last looked.

  _Reach for this when an agent needs to monitor a city for last-minute hotel price drops over time instead of re-querying blind._

  ```bash
  hotel-tonight-pp-cli watch --lat 37.7749 --lng -122.4194 --when tonight --below 150 --agent
  ```
- **`history`** — Show the recorded price and % off history for one hotel from the local store.

  _Use when you need the actual price trajectory of a hotel rather than a single ephemeral quote._

  ```bash
  hotel-tonight-pp-cli history "Argonaut Hotel" --days 30 --agent
  ```
- **`verdict`** — Classify a hotel's current quoted price against its own observed low/median/high as cheap, typical, or expensive.

  _Reach for this to answer 'is this actually cheap?' with a grounded baseline instead of a guess._

  ```bash
  hotel-tonight-pp-cli verdict "Argonaut Hotel" --agent
  ```

### Live deal views

- **`compare-neighborhoods`** — Group tonight's deals in a metro by neighborhood and rank the neighborhoods by median price or best % off.

  _Use when deciding which area of a city has the best last-minute value tonight._

  ```bash
  hotel-tonight-pp-cli compare-neighborhoods --metro 1 --when tonight --agent
  ```
- **`datescan`** — Compare a location's deals across tonight, tomorrow, and the weekend in one ranked side-by-side view.

  _Reach for this to find the cheapest night to stay in an area without running the search repeatedly._

  ```bash
  hotel-tonight-pp-cli datescan --lat 30.3071 --lng -97.7354 --agent
  ```
- **`daily-drop`** — Reveal today's Daily Drop hotel and its real discounted price for a market (the app hides it behind a slide-to-unlock gate), and with --history read the recorded run of past Daily Drops.

  _Use to see and track HotelTonight's exclusive once-a-day flash deal, which the app hides until you unlock it and erases each day._

  ```bash
  hotel-tonight-pp-cli daily-drop --metro 1 --agent
  ```

## Command Reference

**inventory** — Last-minute hotel deal inventory by location

- `hotel-tonight-pp-cli inventory` — Search last-minute hotel deals near a latitude/longitude for a date range

**markets** — HotelTonight markets (cities where deals are offered)

- `hotel-tonight-pp-cli markets get` — Get a single market by its numeric id
- `hotel-tonight-pp-cli markets list` — List HotelTonight's major markets with location, slug, and category prices
- `hotel-tonight-pp-cli markets nearby` — List popular/nearby markets for a given market id


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
hotel-tonight-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Tonight's cheapest near a coordinate, trimmed for agents

```bash
hotel-tonight-pp-cli inventory --latitude 37.7749 --longitude -122.4194 --agent --select rooms.hotel.name,rooms.hotel.neighborhood,rooms.customer_price_per_night,rooms.strikethrough_price,rooms.deal_type
```

The raw inventory payload is tens of KB of nested deal data; --select with dotted paths returns just the hotel, neighborhood, price, 30-day-high, and deal type so an agent doesn't burn context.

### Watch a neighborhood for a price target

```bash
hotel-tonight-pp-cli watch --lat 37.7749 --lng -122.4194 --when tonight --below 150
```

Snapshots current deals and reports rooms now under $150 or cheaper than the last observation — run it on a cron to catch drops while you're away.

### Decide which night to stay

```bash
hotel-tonight-pp-cli datescan --lat 30.3071 --lng -97.7354
```

Lines up tonight vs tomorrow vs the weekend for the same area so you can pick the cheapest night in one view.

### Is this rate actually good?

```bash
hotel-tonight-pp-cli verdict "Argonaut Hotel"
```

Classifies the current quote against the hotel's own recorded low/median/high — requires a few prior syncs to build the baseline.

## Auth Setup

No authentication required.

Run `hotel-tonight-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  hotel-tonight-pp-cli inventory --latitude example-value --longitude example-value --agent --select id,name,status
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
hotel-tonight-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
hotel-tonight-pp-cli feedback --stdin < notes.txt
hotel-tonight-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.hotel-tonight-pp-cli/feedback.jsonl`. They are never POSTed unless `HOTEL_TONIGHT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HOTEL_TONIGHT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
hotel-tonight-pp-cli profile save briefing --json
hotel-tonight-pp-cli --profile briefing inventory --latitude example-value --longitude example-value
hotel-tonight-pp-cli profile list --json
hotel-tonight-pp-cli profile show briefing
hotel-tonight-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `hotel-tonight-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/cmd/hotel-tonight-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add hotel-tonight-pp-mcp -- hotel-tonight-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which hotel-tonight-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   hotel-tonight-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `hotel-tonight-pp-cli <command> --help`.
