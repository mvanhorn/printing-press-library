---
name: pp-atlas-obscura
description: "Every Atlas Obscura search, plus a local database, road-trip corridor routing Trigger phrases: `weird places near me`, `atlas obscura`, `hidden gems along my route`, `offbeat things to do in`, `unusual places to visit`, `use atlas-obscura`, `run atlas-obscura`."
author: "David Bryson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - atlas-obscura-pp-cli
    install:
      - kind: go
        bins: [atlas-obscura-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/cmd/atlas-obscura-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/atlas-obscura/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Atlas Obscura — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `atlas-obscura-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install atlas-obscura --cli-only
   ```
2. Verify: `atlas-obscura-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/cmd/atlas-obscura-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search the world's hidden wonders by keyword or coordinates, pull full place detail with Know-Before-You-Go notes, and mirror it all into a local SQLite store for offline, agent-native use. Then go further than any scraper: find wonders along a driving route, save and export trips, and track what you've visited. Community-sourced from atlasobscura.com — not an official API.

## When to Use This CLI

Use Atlas Obscura when a user wants offbeat, unusual, or hidden points of interest — for trip planning, road trips, or adding curiosity stops to an itinerary. It excels at corridor routing between two cities, accumulating saved trips, and offline re-querying of a local mirror. Reach for it when the task is discovery of weird/wonderful places rather than mainstream travel booking.

## Anti-triggers

Do not use this CLI for:
- Do not use for booking hotels, flights, or restaurants — it only surfaces places.
- Do not use for mainstream tourist attractions or reviews; it is curated oddities, not a general POI directory.
- Do not use for real-time opening hours or ticketing — Atlas Obscura has no structured hours; check the place's own site.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Trip planning
- **`route`** — Find Atlas Obscura wonders along the driving corridor between two cities, not just in one place.

  _Reach for this when a user is driving between two places and wants worthwhile stops along the way._

  ```bash
  atlas-obscura-pp-cli route "San Francisco" "Los Angeles" --min-score 6 --limit 15 --json
  ```
- **`cluster`** — Group nearby wonders into spatially tight clusters that make a walkable half-day.

  _Use to turn a pile of nearby wonders into an efficient day on foot._

  ```bash
  atlas-obscura-pp-cli cluster "Edinburgh" --radius 3 --min 3 --json
  ```
- **`export`** — Serialize a saved trip to GPX, GeoJSON, or a markdown itinerary, fully offline.

  _Use to hand a road trip to a GPS, a map tool, or a human-readable log._

  ```bash
  atlas-obscura-pp-cli export california-oddities --format md
  ```

### Local state that compounds
- **`trip`** — Accumulate places into named itineraries that persist across sessions.

  _Use to build up a trip over multiple sessions instead of re-searching each time._

  ```bash
  atlas-obscura-pp-cli trip add winchester-mystery-house --trip california-oddities
  ```
- **`visited`** — Record which wonders you've seen, with optional date and note.

  _Use to remember what you've already seen so gaps and surprise can skip them._

  ```bash
  atlas-obscura-pp-cli visited mark salvation-mountain --note "worth the desert drive"
  ```
- **`gaps`** — Show good wonders near a point that you haven't visited yet, ranked by interestingness.

  _Use to plan what's left to see near a place you're revisiting._

  ```bash
  atlas-obscura-pp-cli gaps "Portland, Oregon" --radius 40 --min-score 6 --json
  ```
- **`surprise`** — Pick one high-interest wonder you haven't visited, seeded by date so it's stable per day.

  _Use in a daily agent heartbeat to surface a fresh wonder without repeats._

  ```bash
  atlas-obscura-pp-cli surprise --near "Tokyo" --exclude-visited --json
  ```

## Command Reference

**categories** — Browse places by Atlas Obscura category

- `atlas-obscura-pp-cli categories <slug>` — List place links in a category (e.g. cemeteries, caves, ruins)

**destinations** — Browse places by destination (city/region)

- `atlas-obscura-pp-cli destinations <slug>` — List place links for a destination (e.g. paris-france, new-york)

**places** — Atlas Obscura places (wonders)

- `atlas-obscura-pp-cli places <slug>` — Fetch a place detail page by slug or numeric id


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
atlas-obscura-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Plan stops along a road trip

```bash
atlas-obscura-pp-cli route "Denver" "Moab" --min-score 6 --limit 12 --json
```

Surfaces the best-scoring wonders within the driving corridor between two cities.

### Lean nearby scan for an agent

```bash
atlas-obscura-pp-cli near "35.0116,135.7681" --radius 3 --json --select results.title,results.distance_from_query,results.url
```

Deeply nested response narrowed to just title, distance, and URL so an agent doesn't parse image and coordinate noise.

### Build and export a trip

```bash
atlas-obscura-pp-cli trip add winchester-mystery-house --trip ca && atlas-obscura-pp-cli export ca --format md
```

Accumulate places into a named trip, then render a markdown itinerary with descriptions and Know Before You Go.

### What haven't I seen near home

```bash
atlas-obscura-pp-cli gaps "Austin, Texas" --radius 30 --min-score 6 --json
```

Cross-references cached wonders against your visited log and returns only the worthwhile unseen ones.

## Auth Setup

No authentication required.

Run `atlas-obscura-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  atlas-obscura-pp-cli places mock-value --agent --select id,name,status
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
atlas-obscura-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
atlas-obscura-pp-cli feedback --stdin < notes.txt
atlas-obscura-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/atlas-obscura-pp-cli/feedback.jsonl`. They are never POSTed unless `ATLAS_OBSCURA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ATLAS_OBSCURA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
atlas-obscura-pp-cli profile save briefing --json
atlas-obscura-pp-cli --profile briefing places mock-value
atlas-obscura-pp-cli profile list --json
atlas-obscura-pp-cli profile show briefing
atlas-obscura-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `atlas-obscura-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/cmd/atlas-obscura-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add atlas-obscura-pp-mcp -- atlas-obscura-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which atlas-obscura-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   atlas-obscura-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `atlas-obscura-pp-cli <command> --help`.
