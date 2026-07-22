---
name: pp-wahoo
description: "Every Wahoo Cloud API capability, plus the local training database and offline analysis Wahoo's app never gives you. Trigger phrases: `back up my wahoo rides`, `check my training load`, `what's my form on wahoo`, `my FTP progression`, `find a saved route around 100k`, `this week's riding totals`, `use wahoo`, `run wahoo`."
author: "Greg Stellato"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - wahoo-pp-cli
    install:
      - kind: go
        bins: [wahoo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/wahoo/cmd/wahoo-pp-cli
---

# Wahoo Cloud API — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `wahoo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install wahoo --cli-only
   ```
2. Verify: `wahoo-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/wahoo/cmd/wahoo-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync your Wahoo rides, routes, plans, and power zones into a local SQLite mirror, then do what the app can't: back up every ride and FIT file, compute Fitness/Fatigue/Form and FTP progression, find a saved route by distance and elevation, and roll up any window of training. Agent-native output (--json/--agent/--select) makes it scriptable end to end.

## When to Use This CLI

Reach for this CLI when an agent needs a user's Wahoo ride history, FIT files, routes, plans, or power zones, or wants training analytics — load/form, FTP progression, personal bests, recent totals — computed locally. It keeps a local SQLite mirror, so repeated queries are fast and offline. It is the right tool for backing up rides, scripting training dashboards, and pushing routes or plans to an ELEMNT.

## Anti-triggers

Do not use this CLI for:
- Do not use for live Bluetooth control of the ELEMNT device — this is the Cloud API and only sees data that has synced to Wahoo's cloud.
- Do not use for per-second power/HR stream analysis or mean-maximal power curves — the API exposes only per-workout summary averages, not streams.
- Do not use to push activities to Strava or TrainingPeaks — it talks only to Wahoo's cloud.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Own your data
- **`backup`** — Download every workout record plus its raw FIT file into a resumable local directory tree so you own a permanent backup.

  _Reach for this when the user wants their rides backed up or exported off Wahoo's cloud — the app has no bulk export._

  ```bash
  wahoo backup --out ./wahoo-archive --full
  ```

### Training analytics the app won't compute
- **`load`** — Compute CTL (Fitness), ATL (Fatigue), and TSB (Form) time series from your synced rides — the analysis TrainingPeaks charges for.

  _Pick this when the user asks how fit, fatigued, or fresh they are, or whether to push or rest this week._

  ```bash
  wahoo load --days 90 --agent
  ```
- **`digest`** — One-shot rollup of the last N days: ride count, distance, time, ascent, work, and load delta — pipeable to any tool.

  _Use for 'how much did I ride this week/month/year' (--days 365 covers year-in-review)._

  ```bash
  wahoo digest --days 7 --agent
  ```
- **`bests`** — All-time and per-period records for distance, ascent, average power, duration, and work from your ride summaries.

  _Pick this for PRs and bragging-rights records; note it uses ride summaries, not per-second streams._

  ```bash
  wahoo bests --metric power --agent
  ```
- **`ftp-history`** — Dated timeline of your FTP and critical power from power-zone records and sync snapshots, with watts/kg when your weight is known.

  _Use when the user wants to see whether their FTP is trending up over a training block._

  ```bash
  wahoo ftp-history --agent
  ```

### Offline queries over your library
- **`routes find`** — Filter your saved routes by distance band, climbing, and proximity to a point — query a route library the app only scrolls.

  _Pick this when the user wants 'a ~100km route under 1000m of climbing near here' from their own saved routes._

  ```bash
  wahoo routes find --distance 80-120 --max-ascent 1000 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**permissions** — Revoke OAuth app access.

- `wahoo-pp-cli permissions` — Revoke App Access

**plans** — Structured workout plans.

- `wahoo-pp-cli plans create` — Create Plan
- `wahoo-pp-cli plans delete` — Delete Plan
- `wahoo-pp-cli plans get` — Get Plan
- `wahoo-pp-cli plans list` — List Plans
- `wahoo-pp-cli plans update` — Update Plan

**power-zones** — Cycling power training zones.

- `wahoo-pp-cli power-zones create` — Create Power Zones
- `wahoo-pp-cli power-zones delete` — Delete Power Zones
- `wahoo-pp-cli power-zones get` — Get Power Zones
- `wahoo-pp-cli power-zones list` — List Power Zones
- `wahoo-pp-cli power-zones update` — Update Power Zones

**routes** — Navigation / course data backed by FIT files.

- `wahoo-pp-cli routes create` — Create Route
- `wahoo-pp-cli routes delete` — Delete Route
- `wahoo-pp-cli routes get` — Get Route
- `wahoo-pp-cli routes list` — List Routes
- `wahoo-pp-cli routes update` — Update Route

**user** — Authenticated user profile.

- `wahoo-pp-cli user get` — Get Authenticated User
- `wahoo-pp-cli user update` — Update Authenticated User

**workout-file-uploads** — Asynchronous FIT-file ingestion.

- `wahoo-pp-cli workout-file-uploads create` — Upload Workout FIT File
- `wahoo-pp-cli workout-file-uploads get` — Get Workout File Upload Status

**workouts** — Workout records (CRUD + listing).

- `wahoo-pp-cli workouts create` — Create Workout
- `wahoo-pp-cli workouts delete` — Delete Workout
- `wahoo-pp-cli workouts get` — Get Workout
- `wahoo-pp-cli workouts list` — List Workouts
- `wahoo-pp-cli workouts update` — Update Workout


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
wahoo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Inspect a ride's key metrics for an agent

```bash
wahoo workouts get 12345 --agent --select id,name,workout_summary.distance_accum,workout_summary.power_bike_avg,workout_summary.work_accum
```

Narrows a deeply nested workout (with embedded summary) to just the fields an agent needs, avoiding tens of KB of payload.

### Back up an entire ride history

```bash
wahoo sync --resources workouts && wahoo backup --out ./wahoo-archive --full
```

Mirrors all workouts, then downloads each one's FIT file into a resumable local archive.

### Check form before a hard week

```bash
wahoo load --days 90 --agent
```

Returns the Fitness/Fatigue/Form series so an agent can advise whether to add intensity or rest.

### Pick a Saturday route

```bash
wahoo routes find --distance 90-110 --near 40.7,-74.0 --radius 25 --agent
```

Filters saved routes to ~90-110km within 25km of a point using stored geo and elevation fields.

## Auth Setup

Auth is OAuth 2.0 Authorization Code with PKCE. A Sandbox app works immediately against your own account with NO Wahoo approval — approval is only needed for higher rate limits or accessing other people's data. Register an app at https://developers.wahooligan.com/applications/new with the redirect URI http://localhost:8085/callback, pick your scopes, then set WAHOO_CLIENT_ID and WAHOO_CLIENT_SECRET (or pass --client-id/--client-secret) and run 'wahoo auth login' to authorize in your browser (use --port to change the callback port; sign in to Wahoo however you normally do, including Sign in with Google). Access tokens last 2 hours and refresh automatically (refresh tokens are single-use); you can also paste a pre-obtained token via WAHOO_CLOUD_OAUTH2. Revoke this app's access any time with 'wahoo permissions'. Run 'wahoo-pp-cli auth setup' for the full step-by-step.

Run `wahoo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  wahoo-pp-cli plans list --agent --select id,name,status
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
wahoo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
wahoo-pp-cli feedback --stdin < notes.txt
wahoo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/wahoo-pp-cli/feedback.jsonl`. They are never POSTed unless `WAHOO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WAHOO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
wahoo-pp-cli profile save briefing --json
wahoo-pp-cli --profile briefing plans list
wahoo-pp-cli profile list --json
wahoo-pp-cli profile show briefing
wahoo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `wahoo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/wahoo/cmd/wahoo-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add wahoo-pp-mcp -- wahoo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which wahoo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   wahoo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `wahoo-pp-cli <command> --help`.
