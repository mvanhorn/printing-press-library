---
name: pp-surfline
description: "Every Surfline forecast, scriptable from your terminal — plus multi-spot ranking, a local forecast journal Trigger phrases: `is it worth surfing at <spot>`, `rank these surf spots`, `surf forecast for <spot>`, `check the swell and wind at <spot>`, `when is it good to surf this week`, `use surfline`, `run surfline`."
author: "Shoffner"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - surfline-pp-cli
    install:
      - kind: go
        bins: [surfline-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/surfline/cmd/surfline-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/surfline/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Surfline — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `surfline-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install surfline --cli-only
   ```
2. Verify: `surfline-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/surfline/cmd/surfline-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Pulls wave, swell, wind, tide, weather, conditions and rating straight from Surfline's API over a browser-fingerprint transport that clears Cloudflare. The differentiators live in the local SQLite store: 'rank' scans a set of spots and sorts them best-first (Surfline's own comparison is web-only), 'now' collapses one spot's next hours into a paddle/no-paddle readout, 'windows' finds the daylight blocks worth surfing, and 'alert run' evaluates your own threshold rules for cron.

## When to Use This CLI

Use this CLI when an agent or script needs Surfline forecast data as structured output: checking whether one break is worth surfing, ranking several spots to choose between them, finding the good daylight windows, or evaluating user-defined surf-condition alerts on a schedule. It is the right tool when you want raw swell/wind/tide numbers rather than a star rating, or a local history of how forecasts read over time.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to watch live cam video — it returns cam still/stream/rewind URLs, not a video stream.
- Do not use it for global non-Surfline weather or for spots Surfline doesn't cover; use a weather API instead.
- Do not use it to book sessions, post to Surfline, or manage a Surfline account — it is read-only forecast tooling.
- Do not rely on >6-day forecasts without a premium token; those horizons are gated.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Decision-shaped readouts
- **`now`** — One spot's next few hours as a paddle/no-paddle line readout: swell, wind, tide and rating joined per hour.

  _Reach for this when an agent needs a go/no-go answer for one break without parsing five separate forecast payloads._

  ```bash
  surfline-pp-cli now 5842041f4e65fad6a7708807 --agent
  ```
- **`rank`** — Score and sort several spots best-first on a transparent sum of wave, wind and swell optimalScore.

  _Reach for this to pick today's best break from a set instead of opening N forecast pages._

  ```bash
  surfline-pp-cli rank 5842041f4e65fad6a7708807 5842041f4e65fad6a7708cfd --agent
  ```
- **`windows`** — Emit only the contiguous time blocks where wave, wind and swell optimalScore are all good, daylight-only.

  _Reach for this to find the good time slots at one spot without eyeballing an hourly graph._

  ```bash
  surfline-pp-cli windows 5842041f4e65fad6a7708807 --days 3 --agent
  ```

### Raw data for scripts
- **`raw`** — Pipe-friendly table/JSON of min/max/optimalScore/humanRelation plus swell components and wind directionType/gust, no rating editorializing.

  _Reach for this when an agent needs raw numbers to feed its own scoring instead of a pre-judged rating._

  ```bash
  surfline-pp-cli raw 5842041f4e65fad6a7708807 --agent --select data.wave.surf.max,data.wave.swells.period
  ```
- **`buoy-check`** — Show nearby-buoy observed swell against the spot's wave forecast for the same window, side by side.

  _Reach for this to tell whether the forecast is tracking the actual buoy readings before trusting it._

  ```bash
  surfline-pp-cli buoy-check 5842041f4e65fad6a7708807 --agent
  ```

### Local state that compounds
- **`alert run`** — Store swell/wind/tide threshold rules locally; alert run fetches a fresh forecast, evaluates them, prints matches and sets an exit code for cron.

  _Reach for this in unattended/cron contexts to get a nonzero exit when conditions a user defined are met._

  ```bash
  surfline-pp-cli alert run --agent
  ```
- **`journal show`** — Snapshot the current forecast into local SQLite and review a spot's snapshots over time.

  _Reach for this to review how a spot's forecast has read over the past days from your own captures._

  ```bash
  surfline-pp-cli journal show 5842041f4e65fad6a7708807 --agent
  ```
- **`search`** — Resolve spot names to spotIds via FTS over the locally-synced taxonomy, with no network.

  _Reach for this to turn a spot name into an ID before calling forecast commands, even offline._

  ```bash
  surfline-pp-cli search "Ocean Beach" --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**buoys** — Nearby NDBC-style buoy observations

- `surfline-pp-cli buoys` — Buoys near a lat/lon with observed swell readings

**regions** — Region/subregion-level forecasts

- `surfline-pp-cli regions` — Subregion conditions forecast (multi-day AM/PM rating across a subregion)

**spots** — Find spots and pull per-spot forecasts and reports (wave, wind, tide, weather, conditions, rating)

- `surfline-pp-cli spots batch` — Rich per-spot info for many spots in one call (conditions, cameras, current swell/wind/tide)
- `surfline-pp-cli spots conditions` — AM/PM conditions: rating, observation, surf range, forecaster notes, humanRelation
- `surfline-pp-cli spots details` — Spot metadata: name, location, ability levels, travel info
- `surfline-pp-cli spots find` — Live spot search by name (online); returns hits with spotIds.
- `surfline-pp-cli spots forecast` — Combined forecast: forecasts + tides + sunrise/sunset in one call
- `surfline-pp-cli spots rating` — Rating forecast: VERY_POOR..EPIC key plus numeric value per time point
- `surfline-pp-cli spots report` — Spot report: forecaster narrative, current conditions, cameras
- `surfline-pp-cli spots sunlight` — Sunlight forecast: dawn, sunrise, sunset, dusk local times
- `surfline-pp-cli spots tides` — Tide forecast: HIGH/LOW/NORMAL extremes with heights and local times
- `surfline-pp-cli spots wave` — Wave and swell forecast: surf min/max, optimalScore, swell components (height/period/direction)
- `surfline-pp-cli spots weather` — Weather forecast: temperature, condition, pressure, plus sunlight times
- `surfline-pp-cli spots wind` — Wind forecast: speed, direction, directionType (Onshore/Offshore/Cross-shore), gust, optimalScore

**taxonomy** — Browse Surfline's geographic hierarchy (geoname > region > subregion > spot)

- `surfline-pp-cli taxonomy <id>` — Fetch a taxonomy node and its children (ancestors via `in`, children via `contains`)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
surfline-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Pick today's break from your favorites

```bash
surfline-pp-cli rank 5842041f4e65fad6a7708807 5842041f4e65fad6a7708cfd 5842041f4e65fad6a7708e3d --agent
```

Batch-fetches all spots and sorts them best-first on combined optimalScore.

### Narrow a deep wave payload to the fields you need

```bash
surfline-pp-cli raw 5842041f4e65fad6a7708807 --agent --select data.wave.surf.max,data.wave.swells.period,data.wave.swells.direction
```

The wave response is deeply nested; dotted --select pulls just surf height and the swell period/direction so agents don't parse tens of KB.

### Find when it's actually good this week

```bash
surfline-pp-cli windows 5842041f4e65fad6a7708807 --days 5
```

Emits only the daylight blocks where wave, wind and swell optimalScore all clear the bar.

### Cron a surf alert

```bash
surfline-pp-cli alert run --agent
```

Evaluates your stored threshold rules against a fresh forecast and exits nonzero when one matches, so cron/CI can act on it.

### Sanity-check the forecast against buoys

```bash
surfline-pp-cli buoy-check 5842041f4e65fad6a7708807 --agent
```

Puts observed nearby-buoy swell next to the spot's forecast swell for the same window.

## Auth Setup

Basic forecasts, search, taxonomy and multi-spot data need no auth at all (up to a 6-day horizon). For 7–17 day forecasts and premium cams, set a Surfline access token: run 'surfline-pp-cli auth login' with your Surfline email and password (it uses the community password-grant flow against /trusted/token), or 'surfline-pp-cli auth set-token <token>' if you already have one. The token is stored locally and sent as the accesstoken query param; SURFLINE_ACCESS_TOKEN is also honored.

Run `surfline-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  surfline-pp-cli taxonomy mock-value --agent --select id,name,status
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
surfline-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
surfline-pp-cli feedback --stdin < notes.txt
surfline-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/surfline-pp-cli/feedback.jsonl`. They are never POSTed unless `SURFLINE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SURFLINE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
surfline-pp-cli profile save briefing --json
surfline-pp-cli --profile briefing taxonomy mock-value
surfline-pp-cli profile list --json
surfline-pp-cli profile show briefing
surfline-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `surfline-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/surfline/cmd/surfline-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add surfline-pp-mcp -- surfline-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which surfline-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   surfline-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `surfline-pp-cli <command> --help`.
