---
name: pp-infotbm
description: "Every Bordeaux tram, bus, and ferry schedule offline in SQLite, plus real-time arrivals, ghost service detection Trigger phrases: `next tram Bordeaux`, `TBM arrivals`, `Bordeaux bus schedule`, `check disruptions TBM`, `plan transit Bordeaux`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - infotbm-pp-cli
    install:
      - kind: go
        bins: [infotbm-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/infotbm/cmd/infotbm-pp-cli
---

# TBM Bordeaux — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `infotbm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install infotbm --cli-only
   ```
2. Verify: `infotbm-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/infotbm/cmd/infotbm-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use when an agent needs Bordeaux transit information — schedules, real-time arrivals, route planning, or disruption monitoring. Ideal for commute automation, travel planning, and transit-aware applications in the Bordeaux Métropole area.

## Anti-triggers

Do not use this CLI for:
- Journey planning outside Bordeaux Métropole
- Ticket purchase or fare payment
- Real-time traffic or driving directions
- Historical ridership or usage statistics

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Schedule intelligence
- **`schedule diff`** — Surface trips that are scheduled in GTFS but absent from live real-time data — the buses that exist on paper but never showed up

  _When an agent needs to know whether a scheduled service is actually running today, this is the only command that answers definitively_

  ```bash
  infotbm-pp-cli schedule diff --stop BEQUI --line A --json
  ```
- **`lines stops`** — Print the ordered stop list for a line and direction, optionally with scheduled departure times

  _When an agent needs the exact stop order for a tram line to plan boarding points or give directions, this is the canonical answer_

  ```bash
  infotbm-pp-cli lines stops --line A --direction 0 --json
  ```
- **`schedule changes`** — Compare GTFS sync snapshots to detect added, removed, or modified routes on a line

  _When an agent manages a commuter's routine, this proactively detects schedule changes that would break their timing_

  ```bash
  infotbm-pp-cli schedule changes --line C --since 7d --json
  ```
- **`lines frequency`** — Compute average headways per hour from the SIRI estimated timetable

  _When an agent evaluates transit reliability for scheduling, this shows the real-world gaps between departures by time of day_

  ```bash
  infotbm-pp-cli lines frequency --line B --json
  ```

### Journey intelligence
- **`trips last-departure`** — Find the latest departure from a stop that still reaches your destination before a cutoff time

  _When an agent plans evening activities, this answers 'what is the absolute latest I can leave and still get home by transit'_

  ```bash
  infotbm-pp-cli trips last-departure --from Victoire --to Pessac --before 23:30 --json
  ```
- **`trips reroute`** — When your current connection is delayed, compute the best alternate onward path using live vehicle data

  _When an agent detects a delay notification, this computes the optimal recovery without re-planning from scratch_

  ```bash
  infotbm-pp-cli trips reroute --at "Gare Saint-Jean" --to Meriadeck --delay 8 --json
  ```
- **`trips plan`** — Plan multi-modal journeys across tram, bus, and ferry using local GTFS data with live disruption awareness

  _When an agent needs transit directions in Bordeaux without relying on third-party map APIs, this plans the route locally_

  ```bash
  infotbm-pp-cli trips plan --from Victoire --to "Gare Saint-Jean" --depart 08:15 --json
  ```

### Commute intelligence
- **`alerts impact`** — Filter all active disruptions to only those affecting your specific lines or stops

  _When an agent monitors a commuter's daily lines, this filters noise to only relevant disruptions_

  ```bash
  infotbm-pp-cli alerts impact --lines A,C,15 --json
  ```

## Command Reference

**agencies** — 

- `infotbm-pp-cli agencies` — Transit agencies in the Bordeaux network

**alerts** — 

- `infotbm-pp-cli alerts` — Active service disruption alerts

**fares** — 

- `infotbm-pp-cli fares` — Fare structure including pricing and transfer rules

**feed-info** — 

- `infotbm-pp-cli feed-info` — GTFS feed metadata including validity dates and timestamps

**kml** — 

- `infotbm-pp-cli kml` — KML geographic data export with route geometry and stop locations

**realtime** — 

- `infotbm-pp-cli realtime stop` — Real-time departure information at a specific stop
- `infotbm-pp-cli realtime vehicles` — Real-time vehicle positions across the network

**routes** — 

- `infotbm-pp-cli routes` — All transit routes/lines in the TBM network

**server-info** — 

- `infotbm-pp-cli server-info` — API version and build information

**siri** — 

- `infotbm-pp-cli siri check-status` — SIRI service health check
- `infotbm-pp-cli siri estimated-timetable` — Estimated real-time timetable for a line
- `infotbm-pp-cli siri general-message` — General service messages and disruption information
- `infotbm-pp-cli siri lines-discovery` — Discover all lines with destinations and transport modes
- `infotbm-pp-cli siri stop-monitoring` — Real-time arrival/departure monitoring at a stop
- `infotbm-pp-cli siri stoppoints-discovery` — Discover all stop points with coordinates and serving lines

**stops** — 

- `infotbm-pp-cli stops` — All transit stops in the TBM network


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
infotbm-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These command groups are hand-wired outside the generated endpoint surface. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `infotbm-pp-cli schedule` — Offline schedule lookups from local GTFS data (subcommands: `diff`, `changes`)
- `infotbm-pp-cli trips` — Journey planning commands (subcommands: `plan`, `last-departure`, `reroute`)
- `infotbm-pp-cli lines` — Line analysis commands (subcommands: `stops`, `frequency`)

## Recipes

### Morning commute check

```bash
infotbm-pp-cli realtime stop --stop-id bordeaux:StopPoint:BP:3648:LOC --json --select items.lineName,items.destinationName,items.expectedDepartureTime
```

Check next departures at your stop with only the fields that matter

### Weekend disruption audit

```bash
infotbm-pp-cli alerts impact --lines A,B,C --json
```

Filter alerts to only tram lines before a weekend outing

### Late night last train

```bash
infotbm-pp-cli trips last-departure --from Victoire --to Pessac --before 23:30 --json
```

Find the latest departure that still gets you home

### Detect ghost services

```bash
infotbm-pp-cli schedule diff --stop BEQUI --line A --agent
```

Surface scheduled trams that are not showing up in real-time data

### Weekly schedule change alert

```bash
infotbm-pp-cli schedule changes --line C --since 7d --json
```

Detect any trips added, removed, or shifted since last week's sync

## Auth Setup
Run `infotbm-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export INFOTBM_API_KEY="<your-key>"
```

Or persist it in `~/.config/infotbm-pp-cli/config.json`.

Run `infotbm-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  infotbm-pp-cli agencies --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only by default** — most commands only read data; `import` and `sync` write to the local store but do not mutate remote resources

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
infotbm-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
infotbm-pp-cli feedback --stdin < notes.txt
infotbm-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/infotbm-pp-cli/feedback.jsonl`. They are never POSTed unless `INFOTBM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `INFOTBM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
infotbm-pp-cli profile save briefing --json
infotbm-pp-cli --profile briefing agencies
infotbm-pp-cli profile list --json
infotbm-pp-cli profile show briefing
infotbm-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `infotbm-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/infotbm/cmd/infotbm-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add infotbm-pp-mcp -- infotbm-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which infotbm-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   infotbm-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `infotbm-pp-cli <command> --help`.
