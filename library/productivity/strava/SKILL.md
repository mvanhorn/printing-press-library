---
name: pp-strava
description: "Every Strava feature, plus offline analytics — training load, power curves, zone time Trigger phrases: `strava training load`, `check my CTL ATL`, `power curve from Strava`, `sync my Strava activities`, `segment progression strava`, `use strava-pp-cli`, `run strava cli`."
author: "azaaron"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - strava-pp-cli
---

# Strava — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `strava-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install strava --cli-only
   ```
2. Verify: `strava-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/strava/cmd/strava-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use strava-pp-cli when an agent needs to query an athlete's historical training data, compute fitness metrics unavailable from the Strava web interface, manage activities in bulk, or build a training analysis pipeline. It is the right choice when the question involves aggregating across more than 5 activities, involves stream-level data (heart rate, power, GPS), or requires offline access to Strava data.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`segments progress`** — See your entire effort history on a segment — date, time, avg power, avg HR, delta from PR — so you can track if you're actually improving.

  _Use this when an agent needs to assess whether an athlete is progressing on a target training segment over a season._

  ```bash
  strava-pp-cli segments progress 229781 --json --select start_date,elapsed,avg_watts,delta_pr_seconds
  ```
- **`training load`** — See your Chronic Training Load, Acute Training Load, and Training Stress Balance as sparklines so you can spot overtraining or undertaper before a race.

  _Use when an agent needs to assess an athlete's fitness/fatigue state and readiness for an upcoming event._

  ```bash
  strava-pp-cli training load --weeks 12 --ftp 285 --agent
  ```
- **`training zones`** — See how many minutes per week you actually spent in each heart rate or power zone, so you can tell if your training distribution matches your plan.

  _Use when evaluating whether a training block was executed as prescribed (polarized, sweet-spot, base)._

  ```bash
  strava-pp-cli training zones --weeks 8 --type Run --zone-type heartrate --agent
  ```
- **`athlete power-curve`** — See your best mean power for every standard duration (1s to 60min) so you can identify strengths, weaknesses, and fitness changes across seasons.

  _Use when an agent needs to characterize a cyclist's physiological profile or compare peak power across training blocks._

  ```bash
  strava-pp-cli athlete power-curve --since 2025-01-01 --weight 75 --agent
  ```
- **`activities drift`** — Measure aerobic decoupling in an activity — the ratio of HR rise to pace drop in the second half — to assess aerobic fitness without lab testing.

  _Use when an agent needs to identify which long runs or rides showed aerobic decoupling, indicating the athlete was above their aerobic threshold._

  ```bash
  strava-pp-cli activities drift --min-duration 45m --since 2025-01-01 --threshold 5 --agent
  ```
- **`segments kom-gap`** — See exactly how far you are from the KOM on each starred segment, ranked by the gap you're most likely to close.

  _Use when an agent needs to surface the most achievable KOM targets for a training plan or pre-ride goal setting._

  ```bash
  strava-pp-cli segments kom-gap --top 10 --agent
  ```

### Agent-native plumbing
- **`activities bulk-update`** — Update gear, name template, or description across hundreds of activities at once with a preview-before-commit safety net.

  _Use when an agent needs to mass-migrate a gear assignment after equipment replacement or retroactively organize a training block's activities._

  ```bash
  strava-pp-cli activities bulk-update --type Ride --after 2024-01-01 --set-gear b12345678 --dry-run
  ```
- **`gear status`** — See total mileage on each shoe and bike, your configured replacement threshold, and an estimated retirement date based on your recent usage rate.

  _Use when an agent needs to check whether any gear is approaching retirement before an important race._

  ```bash
  strava-pp-cli gear status --threshold shoes=500mi --agent
  ```

## Command Reference

**activities** — Manage activities

- `strava-pp-cli activities create-activity` — Creates a manual activity for an athlete, requires activity:write scope.
- `strava-pp-cli activities get-activity-by-id` — Returns the given activity that is owned by the authenticated athlete.
- `strava-pp-cli activities update-activity-by-id` — Updates the given activity that is owned by the authenticated athlete. Requires activity:write.

**athlete** — Manage athlete

- `strava-pp-cli athlete get-logged-in` — Returns the currently authenticated athlete.
- `strava-pp-cli athlete get-logged-in-activities` — Returns the activities of an athlete for a specific identifier. Requires activity:read.
- `strava-pp-cli athlete get-logged-in-clubs` — Returns a list of the clubs whose membership includes the authenticated athlete.
- `strava-pp-cli athlete get-logged-in-zones` — Returns the the authenticated athlete's heart rate and power zones. Requires profile:read_all.
- `strava-pp-cli athlete update-logged-in` — Update the currently authenticated athlete. Requires profile:write scope.

**athletes** — Manage athletes


**clubs** — Manage clubs

- `strava-pp-cli clubs <id>` — Returns a given a club using its identifier.

**gear** — Manage gear

- `strava-pp-cli gear <id>` — Returns an equipment using its identifier.

**routes** — Manage routes

- `strava-pp-cli routes <id>` — Returns a route using its identifier. Requires read_all scope for private routes.

**segment-efforts** — Manage segment efforts

- `strava-pp-cli segment-efforts get-by-id` — Returns a segment effort from an activity that is owned by the authenticated athlete. Requires subscription.
- `strava-pp-cli segment-efforts get-efforts-by-segment-id` — Returns a set of the authenticated athlete's segment efforts for a given segment. Requires subscription.

**segments** — Manage segments

- `strava-pp-cli segments explore` — Returns the top 10 segments matching a specified query.
- `strava-pp-cli segments get-by-id` — Returns the specified segment.
- `strava-pp-cli segments get-logged-in-athlete-starred` — List of the authenticated athlete's starred segments.

**uploads** — Manage uploads

- `strava-pp-cli uploads create` — Uploads a new data file to create an activity from. Requires activity:write scope.
- `strava-pp-cli uploads get-by-id` — Returns an upload for a given identifier. Requires activity:write scope.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
strava-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Check training readiness before a race

```bash
strava-pp-cli training load --weeks 8 --ftp 285 --agent --select ctl,atl,tsb,label
```

Shows TSB (freshness) for each of the past 8 weeks; negative TSB = fatigue, positive = fresh. Ideal for race week targeting TSB > 10.

### Find your best power efforts this year

```bash
strava-pp-cli athlete power-curve --since 2025-01-01 --weight 72 --agent --select duration,watts,wkg
```

Returns best mean power per standard window normalized to W/kg — share with a coach for physiological profiling.

### Identify aerobically decoupled long runs

```bash
strava-pp-cli activities drift --type Run --min-duration 60m --threshold 8 --agent --select id,name,start_date,drift_pct
```

Lists all runs where HR drifted more than 8% relative to pace in the second half — these are training days where you went too hard.

### See your segment progression this season

```bash
strava-pp-cli segments progress 229781 --since 2025-01-01 --agent --select start_date,elapsed,avg_watts,delta_pr_seconds
```

Shows every effort on segment 229781, ordered by date, with delta from your PR so you can see the trend clearly.

### Check gear retirement status before a marathon

```bash
strava-pp-cli gear status --threshold run_shoes=400 --agent --select gear_name,total_distance_mi,pct_threshold,est_retirement_date
```

Shows all running gear with mileage and estimated retirement date; use --select to narrow to the fields a race-day checklist needs.

## Auth Setup

Strava uses OAuth2. Run `strava-pp-cli auth login` to open the browser authorization page — the CLI handles the local callback, token exchange, and storage automatically. Set STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET before running. To re-authenticate, run `auth login` again.

Run `strava-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  strava-pp-cli activities create-activity --name example-resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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
strava-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
strava-pp-cli feedback --stdin < notes.txt
strava-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.strava-pp-cli/feedback.jsonl`. They are never POSTed unless `STRAVA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `STRAVA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
strava-pp-cli profile save briefing --json
strava-pp-cli --profile briefing activities create-activity --name example-resource
strava-pp-cli profile list --json
strava-pp-cli profile show briefing
strava-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `strava-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add strava-pp-mcp -- strava-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which strava-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   strava-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `strava-pp-cli <command> --help`.
