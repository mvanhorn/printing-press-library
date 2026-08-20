---
name: pp-ridewithgps
description: "Your whole Ride with GPS library, offline — bulk export, gear mileage, and ride analytics no other tool ships. Trigger phrases: `export my ridewithgps routes to my garmin`, `get this event's routes on my wahoo`, `bulk export gpx from ride with gps`, `how many miles on my bike`, `my biggest climbing month`, `find duplicate routes`, `use ridewithgps`, `run ridewithgps`."
author: "Greg Stellato"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ridewithgps-pp-cli
    install:
      - kind: go
        bins: [ridewithgps-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/cmd/ridewithgps-pp-cli
---

# Ride with GPS — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ridewithgps-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ridewithgps --cli-only
   ```
2. Verify: `ridewithgps-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/cmd/ridewithgps-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A fast, scriptable, agent-native CLI for Ride with GPS with a local SQLite mirror of your routes and trips. Bulk-export everything (with cue sheets) to GPX/TCX/CSV for your bike computer with 'export', pull an organized ride's routes with 'event-routes', track per-bike mileage with 'gear', see this month's climbing with 'stats', surface all-time bests with 'records', and clean a junk-drawer library with 'dedup' and 'audit' — none of which the web app or any existing tool does.

## When to Use This CLI

Use this CLI to bulk-export Ride with GPS routes and trips to files, mirror a library offline and query it without the API, roll up training volume and personal records from logged rides, track per-bike gear mileage and maintenance, and script route/event/POI operations. It is the right tool when a task is about your existing library and ride history rather than building or navigating a new route.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for live turn-by-turn navigation — use the Ride with GPS mobile app.
- Do not use this CLI to draw or build a new route on a map — use the web route planner.
- Do not use this CLI to record a ride from GPS in real time — use the mobile app.
- Do not use this CLI for the social activity feed or following other riders.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Routes onto your bike computer
- **`export`** — Export many routes or trips to GPX, TCX, CSV, or KML files in one command — with cue sheets included, ready to side-load onto a Garmin, Wahoo, or Hammerhead. The bulk export Ride with GPS itself won't do.

  _Reach for this when a user wants routes onto a bike computer or rides as files on disk — one command instead of clicking export per route in the web app._

  ```bash
  ridewithgps-pp-cli export --type routes --format gpx --out ./gpx
  ```
- **`event-routes`** — List the routes attached to an event (group ride, gran fondo, brevet) and optionally export them straight to files for your bike computer.

  _Reach for this when a user has signed up for an organized ride and wants that event's official routes on their head unit._

  ```bash
  ridewithgps-pp-cli event-routes 12345678 --export gpx --out ./fondo
  ```

### Ride analytics that compound
- **`gear`** — Per-bike accumulated mileage from your logged rides, plus maintenance-due flags against wear thresholds.

  _Reach for this to answer 'how many miles since my last chain swap' or 'which bike needs service' without a spreadsheet._

  ```bash
  ridewithgps-pp-cli gear --due-km 4000 --agent
  ```
- **`stats`** — Time-windowed distance, elevation, and moving-time totals with activity-type breakdowns over your logged rides.

  _Reach for this for 'how far did I ride this month' or 'how much have I climbed this year' — period totals, not single efforts._

  ```bash
  ridewithgps-pp-cli stats --period month --agent
  ```
- **`records`** — All-time best efforts — longest ride, most climbing, fastest average speed, biggest power — from your trip metrics.

  _Reach for this for 'what's my longest ride' or 'my biggest climbing day' — top-N extremes, not period sums._

  ```bash
  ridewithgps-pp-cli records --metric elevation --top 10 --agent
  ```

### Library hygiene
- **`dedup`** — Find near-duplicate routes by distance and start/end geometry, and optionally delete the extras.

  _Reach for this to clean a route library full of near-identical versions of the same loop._

  ```bash
  ridewithgps-pp-cli dedup --threshold 100 --agent
  ```
- **`audit`** — Flag routes that are stale, private, or incomplete (missing name, description, or distance) — catalog hygiene across your library.

  _Reach for this before an event season to find routes that are stale, private when they should be shared, or missing details._

  ```bash
  ridewithgps-pp-cli audit --checks stale,private --agent
  ```

## Command Reference

**auth_tokens** — Manage auth tokens

- `ridewithgps-pp-cli auth-tokens` — Creates a new authentication token for API access using user credentials.

**collections** — Manage collections

- `ridewithgps-pp-cli collections get` — Returns detailed information about a specific collection
- `ridewithgps-pp-cli collections get-pinned` — Returns the authenticated user's pinned collection
- `ridewithgps-pp-cli collections list` — Returns a paginated list of collections for the authenticated user. Supports filtering by `name` and `visibility`.

**events** — Manage events

- `ridewithgps-pp-cli events create` — Creates a new event for the authenticated user.
- `ridewithgps-pp-cli events delete` — Deletes an event owned by the authenticated user
- `ridewithgps-pp-cli events get` — Returns detailed information about a specific event
- `ridewithgps-pp-cli events list` — Returns a paginated list of events accessible to the authenticated user. Supports filtering by `name` and `visibility`.
- `ridewithgps-pp-cli events update` — Updates an existing event owned by the authenticated user.

**members** — Manage members

- `ridewithgps-pp-cli members get-club` — Returns detailed information about a specific club member
- `ridewithgps-pp-cli members list-club` — Returns a paginated list of members for the authenticated organization account.
- `ridewithgps-pp-cli members update-club` — Updates a club member's permissions and status.

**points_of_interest** — Manage points of interest

- `ridewithgps-pp-cli points-of-interest associate-point-of-interest-with-route` — Associates a point of interest with a specific route **Note**: This endpoint is available for organizations only.
- `ridewithgps-pp-cli points-of-interest create-point-of-interest` — Create a new point of interest in the authenticated organization's library.
- `ridewithgps-pp-cli points-of-interest delete-point-of-interest` — Deletes a point of interest owned by the authenticated organization.
- `ridewithgps-pp-cli points-of-interest disassociate-point-of-interest-from-route` — Removes the association between a point of interest and a specific route **Note**
- `ridewithgps-pp-cli points-of-interest get-point-of-interest` — Returns detailed information about a specific point of interest.
- `ridewithgps-pp-cli points-of-interest list` — Returns a paginated list of points of interest for the authenticated organization.
- `ridewithgps-pp-cli points-of-interest update-point-of-interest` — Updates an existing point of interest in the authenticated organization's library.

**routes** — Manage routes

- `ridewithgps-pp-cli routes delete` — Deletes a route owned by the authenticated user
- `ridewithgps-pp-cli routes get` — Returns detailed information about a specific route
- `ridewithgps-pp-cli routes get-polyline` — Returns an encoded polyline string for the specified route.
- `ridewithgps-pp-cli routes list` — Returns a paginated list of routes for the authenticated user.

**sync-json** — Manage sync json

- `ridewithgps-pp-cli sync-json` — Returns a list of items (routes and/or trips) that the user has interacted with since the given datetime.

**trips** — Manage trips

- `ridewithgps-pp-cli trips delete` — Deletes a trip owned by the authenticated user
- `ridewithgps-pp-cli trips get` — Returns detailed information about a specific trip
- `ridewithgps-pp-cli trips get-polyline` — Returns an encoded polyline string for the specified trip.
- `ridewithgps-pp-cli trips list` — Returns a paginated list of trips for the authenticated user.

**users** — Manage users

- `ridewithgps-pp-cli users` — Returns information about the currently authenticated user


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ridewithgps-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Bulk-export the whole route library to GPX for your head unit

```bash
ridewithgps-pp-cli export --type routes --format gpx --out ./gpx
```

Writes one GPX file per route (with cue sheets) from the local mirror — ready to side-load onto a Garmin or Wahoo.

### Get an event's routes onto your bike computer

```bash
ridewithgps-pp-cli event-routes 12345678 --export gpx --out ./fondo
```

Pulls the routes attached to an organized ride and writes them as files for your head unit.

### Find and remove duplicate routes

```bash
ridewithgps-pp-cli dedup --threshold 100 --apply
```

Clusters near-identical routes within 100m and deletes the extras (omit --apply to preview).

### Check which bikes are due for service

```bash
ridewithgps-pp-cli gear --due-km 4000
```

Flags bikes past 4000km of accumulated trip mileage.

### Agent: pull a route's cue sheet with narrow fields

```bash
ridewithgps-pp-cli routes get 12345678 --agent --select route.name,route.distance,route.course_points.n,route.course_points.d
```

Route detail is deeply nested and large; --select narrows to cue-sheet names and distances so an agent does not parse tens of KB of track points.

### Agent: this year's training totals

```bash
ridewithgps-pp-cli stats --period year --agent
```

Returns structured yearly distance/elevation/time totals for downstream reasoning.

## Auth Setup

Ride with GPS uses two credentials: a client API key and a per-user auth token, sent as the x-rwgps-api-key and x-rwgps-auth-token headers. Create an API client under your account's developers tab for the API key, then set RIDEWITHGPS_API_KEY and RIDEWITHGPS_AUTH_TOKEN. To get your auth token from your email and password, run 'auth-tokens --user-email <you> --user-password <pw>', which returns it. The v1 API does not accept browser cookies; run 'doctor' to verify.

Run `ridewithgps-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ridewithgps-pp-cli collections list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ridewithgps-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ridewithgps-pp-cli feedback --stdin < notes.txt
ridewithgps-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/ridewithgps-pp-cli/feedback.jsonl`. They are never POSTed unless `RIDEWITHGPS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `RIDEWITHGPS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ridewithgps-pp-cli profile save briefing --json
ridewithgps-pp-cli --profile briefing collections list
ridewithgps-pp-cli profile list --json
ridewithgps-pp-cli profile show briefing
ridewithgps-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ridewithgps-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/cmd/ridewithgps-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ridewithgps-pp-mcp -- ridewithgps-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ridewithgps-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ridewithgps-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ridewithgps-pp-cli <command> --help`.
