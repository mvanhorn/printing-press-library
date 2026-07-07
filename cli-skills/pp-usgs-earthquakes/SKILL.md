---
name: pp-usgs-earthquakes
description: "Every USGS earthquake feed and event query in one terminal, with offline SQLite cache, agent-native output, and a... Trigger phrases: `any recent earthquakes`, `earthquakes near`, `aftershocks of`, `USGS earthquake feed`, `M5 earthquake`, `tsunami earthquake`, `swarm detection`, `use usgs-earthquakes`, `run usgs-earthquakes`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - usgs-earthquakes-pp-cli
    install:
      - kind: go
        bins: [usgs-earthquakes-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/usgs-earthquakes/cmd/usgs-earthquakes-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/usgs-earthquakes/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# USGS Earthquakes — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `usgs-earthquakes-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install usgs-earthquakes --cli-only
   ```
2. Verify: `usgs-earthquakes-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/usgs-earthquakes/cmd/usgs-earthquakes-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Reach for this CLI any time an agent needs to answer questions about USGS earthquakes — recent activity, historical search, single-event detail, aftershock sequences, regional comparisons, or live monitoring. It is the right choice when speed matters (offline SQLite cache), when output must be agent-friendly (--json, --select, --compact, --csv on every command), or when the workflow spans multiple events that the raw FDSN API would require stitching together. Not the right choice for non-USGS seismic networks (IRIS event service, EMSC) or for waveform/station data — those need an FDSN station-service client like obspy.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Real-time monitoring
- **`watch`** — Long-running poll of a USGS summary feed with deduplication against the local store. Invokes an optional shell hook per new event for custom alerting.

  _Reach for this when you want a long-running monitor over USGS earthquakes without writing a polling loop. Pluggable shell hook means it composes with notify-send, Slack curl, paging systems, anything._

  ```bash
  usgs-earthquakes-pp-cli watch --min-magnitude 5 --notify "echo {id}: {place}" --interval 60s
  ```

### Cross-event analysis
- **`aftershocks`** — Show events within R km and T days after a mainshock, ordered by time. Local SQLite haversine query with FDSN fallback when uncached.

  _Reach for this any time you have a mainshock event ID and need to see what came after. The single-call alternative is dozens of /query invocations stitched in jq._

  ```bash
  usgs-earthquakes-pp-cli aftershocks us7000abcd --radius-km 100 --days 30 --min-mag 3.0 --json
  ```
- **`swarm-detect`** — Detect time-space clusters in the local earthquake store. Grid-bucket clustering finds foreshock/aftershock sequences, volcanic swarms, and induced seismicity hotspots.

  _Reach for this when monitoring a volcano, fault zone, or fracking region for unusual clustering. The output names each cluster with center, count, peak magnitude, and time range._

  ```bash
  usgs-earthquakes-pp-cli swarm-detect --bbox -122.5,46.5,-121.5,47.5 --window 7d --min-events 10 --cluster-radius-km 20 --json
  ```
- **`compare`** — Side-by-side comparison of two regions or two time periods. Returns parallel columns with counts, max magnitude, and total seismic energy.

  _Reach for this when answering 'is region A more active than region B?' or 'is this region quieter than it used to be?'. Outputs delta percentages and energy ratios._

  ```bash
  usgs-earthquakes-pp-cli compare --region-a -122.5,37.5,-122.0,37.9 --region-b -118.5,33.8,-118.0,34.2 --window 30d --json
  ```

### Agent-native output
- **`brief`** — Agent-ready briefing for a single earthquake: magnitude, place, PAGER alert, DYFI felt count, ShakeMap MMI, tsunami status, and product inventory.

  _Reach for this in newsroom or Slack contexts where you need a one-event summary an editor or downstream agent can drop straight into copy. Includes the USGS event-page URL._

  ```bash
  usgs-earthquakes-pp-cli brief us7000abcd --format markdown
  ```
- **`top`** — Rank recent events by composite editorial score: significance × alert weight × felt count × tsunami flag. Default window 24h, limit 10.

  _Reach for this when you need 'the events that matter right now' rather than 'all events by magnitude'. The composite score promotes felt + PAGER + tsunami over raw magnitude._

  ```bash
  usgs-earthquakes-pp-cli top --window 24h --limit 10 --score composite --json
  ```

### Local state that compounds
- **`changes`** — Diff since the last sync: what events appeared, what events had magnitudes/depths/alerts revised, what events were retracted. Tracks USGS solution revisions over time. Returns 'no revisions recorded yet' until at least two sync runs have completed.

  _Reach for this when a previously-automatic event might have just been reviewed by an analyst (revising magnitude or alert), or to answer 'what's new since I last looked?'_

  ```bash
  usgs-earthquakes-pp-cli changes --since 24h --type revised --min-mag-delta 0.3 --json
  ```
- **`decode-id`** — Parse a USGS event ID into its source network code, sequence, and operator name. Joins the cached contributors dictionary for the network display name.

  _Reach for this when you see an opaque USGS event ID (e.g. nc73947885, ak0202xyz) and need to know which network reported it before reading the data._

  ```bash
  usgs-earthquakes-pp-cli decode-id us7000abcd
  ```

## Command Reference

**catalogs** — USGS earthquake source catalogs (ANSS contributors and processing centers)

- `usgs-earthquakes-pp-cli catalogs` — List all source catalogs known to the FDSN Event service (XML response)

**contributors** — USGS earthquake data contributors

- `usgs-earthquakes-pp-cli contributors` — List all data contributors known to the FDSN Event service (XML response)

**events** — Search and retrieve earthquakes from the USGS FDSN Event service

- `usgs-earthquakes-pp-cli events count` — Count events matching the given filter, without returning event data (fast precheck before a full search)
- `usgs-earthquakes-pp-cli events get` — Fetch a single earthquake event by USGS event ID (e.g. us7000abcd)
- `usgs-earthquakes-pp-cli events search` — Search the USGS earthquake catalog with full FDSN parameter coverage

**feeds** — Pre-built GeoJSON earthquake summary feeds (updated every minute by USGS)

- `usgs-earthquakes-pp-cli feeds <feed>` — Fetch a named GeoJSON summary feed. Use one of: significant_{hour|day|week|month}, 4.5_{hour|day|week|month},...

**metadata** — FDSN Event service metadata: enumerated values for every parameter

- `usgs-earthquakes-pp-cli metadata` — Show enum dictionaries for catalogs, contributors, event types, product types, and magnitude types


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
usgs-earthquakes-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `usgs-earthquakes-pp-cli recent` — List recent earthquakes (default: last 24h, M2.5+). Friendly wrapper over `events search` with --since/--near flags.
- `usgs-earthquakes-pp-cli near <latitude> <longitude>` — Show recent earthquakes near a coordinate, ranked by distance + significance.
- `usgs-earthquakes-pp-cli sync` — Sync the rolling earthquake catalog (default: last 30 days, M2.5+) and the catalogs/contributors/metadata...
- `usgs-earthquakes-pp-cli search [query]` — Full-text search over locally synced earthquakes (FTS5 on place and event title).
- `usgs-earthquakes-pp-cli sql <query>` — Run a SELECT query against the local earthquake store.
- `usgs-earthquakes-pp-cli feed-list` — List all 20 USGS summary feed names with magnitude level, period, and update cadence.
- `usgs-earthquakes-pp-cli watch` — Long-running poll of a summary feed with deduplication against the local store; invokes an optional --notify hook...
- `usgs-earthquakes-pp-cli aftershocks <event-id>` — Show events within R km and T days after a mainshock, ordered by time. Local SQLite query with FDSN fallback when...
- `usgs-earthquakes-pp-cli swarm-detect` — Detect spatial-temporal clusters in the local store (foreshock/aftershock sequences, volcanic swarms, induced...
- `usgs-earthquakes-pp-cli compare` — Side-by-side comparison of two regions OR two time periods. --region-a/--region-b OR --period-a/--period-b.
- `usgs-earthquakes-pp-cli brief <event-id>` — Agent-ready newsroom briefing for an event: magnitude, place, PAGER alert, DYFI, tsunami status, ShakeMap MMI,...
- `usgs-earthquakes-pp-cli top` — Rank recent events by composite editorial score (significance × alert × felt × tsunami). Default: --window 24h...
- `usgs-earthquakes-pp-cli changes` — Stateful diff since the last sync: new events, revised magnitudes/depths/alerts, deleted events.
- `usgs-earthquakes-pp-cli decode-id <event-id>` — Parse a USGS event ID into network code, sequence, and operator (from the cached contributors dictionary).

## Recipes


### Did anything newsworthy happen in the last hour?

```bash
usgs-earthquakes-pp-cli top --window 1h --limit 10 --json --select events.id,events.mag,events.place,events.alert,events.tsunami,events.score
```

Ranks the past hour's events by composite editorial score (sig × alert × felt × tsunami) and narrows the agent context to just the editorially-relevant fields via --select.

### Track a swarm at Mount Hood for the past 7 days

```bash
usgs-earthquakes-pp-cli swarm-detect --near 45.3736,-121.6960 --radius-km 25 --window 7d --min-events 5 --json
```

Spatial-temporal clustering around the Mount Hood coordinate; returns named clusters with center, count, peak magnitude, and time range.

### Aftershock report for a recent mainshock

```bash
usgs-earthquakes-pp-cli aftershocks us7000abcd --radius-km 100 --days 14 --min-mag 3.0 --json
```

Local SQLite haversine query bounded by the mainshock's time and location; falls back to FDSN /query when the event isn't cached.

### Daily newsroom briefing for the top event

```bash
usgs-earthquakes-pp-cli top --window 24h --limit 1 --json --select events.id
```

Returns just the top-ranked event's ID; pipe to `xargs -I {} usgs-earthquakes-pp-cli brief {} --format markdown` to get a newsroom-ready briefing.

### Find events that have a ShakeMap product near San Francisco

```bash
usgs-earthquakes-pp-cli events search --latitude 37.7749 --longitude -122.4194 --max-radius-km 200 --product-type shakemap --start 2025-01-01 --json
```

Uses the FDSN --product-type filter (not exposed by the competing MCP) plus the --latitude/--longitude circle filter to find events that have a ShakeMap polygon since the start of the year.

## Auth Setup

No authentication required.

Run `usgs-earthquakes-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  usgs-earthquakes-pp-cli catalogs --agent --select id,name,status
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
usgs-earthquakes-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
usgs-earthquakes-pp-cli feedback --stdin < notes.txt
usgs-earthquakes-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.usgs-earthquakes-pp-cli/feedback.jsonl`. They are never POSTed unless `USGS_EARTHQUAKES_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `USGS_EARTHQUAKES_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
usgs-earthquakes-pp-cli profile save briefing --json
usgs-earthquakes-pp-cli --profile briefing catalogs
usgs-earthquakes-pp-cli profile list --json
usgs-earthquakes-pp-cli profile show briefing
usgs-earthquakes-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `usgs-earthquakes-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/usgs-earthquakes/cmd/usgs-earthquakes-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add usgs-earthquakes-pp-mcp -- usgs-earthquakes-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which usgs-earthquakes-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   usgs-earthquakes-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `usgs-earthquakes-pp-cli <command> --help`.
