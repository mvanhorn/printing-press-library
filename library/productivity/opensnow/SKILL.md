---
name: pp-opensnow
description: "Every OpenSnow forecast, snow report, and Daily Snow post — plus powder scoring, storm tracking, and historical trends no other tool has. Trigger phrases: `check snow conditions`, `how much snow at alta`, `powder forecast this week`, `compare ski resorts`, `daily snow report`, `use opensnow`, `run opensnow`."
author: "Dave Morin"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - opensnow-pp-cli
    install:
      - kind: go
        bins: [opensnow-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/opensnow/cmd/opensnow-pp-cli
---

# OpenSnow — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `opensnow-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install opensnow --cli-only
   ```
2. Verify: `opensnow-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/opensnow/cmd/opensnow-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The OpenSnow CLI puts the same hyper-local mountain forecasts trusted by millions of skiers directly into your terminal. Sync resort data to a local SQLite store, then score powder days, rank resorts, and track storms offline. The Daily Snow digest brings expert meteorologist forecasts to your terminal without opening a browser.

## When to Use This CLI

Use the OpenSnow CLI when you need mountain weather forecasts with snow-specific detail — day/night snowfall splits, powder quality, and resort operating status. Ideal for trip planning agents that need to compare resorts, track storms, or answer 'where should I ski this weekend?' The Daily Snow digest provides expert meteorologist analysis not available from generic weather APIs.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Powder intelligence
- **`powder-score`** — Rate upcoming days 1-10 for powder quality by combining forecast snow totals, wind, temperature, and historical averages

  _When an agent needs to recommend the single best day to ski this week, this command gives a scored ranking instead of raw forecast numbers_

  ```bash
  opensnow-pp-cli powder-score alta --days 5 --agent
  ```
- **`powder-rank`** — Rank all synced resorts by powder potential, combining expected snowfall, base depth, and terrain openness

  _When an agent needs to answer 'where should I ski this weekend in Colorado', this gives a ranked list with scores_

  ```bash
  opensnow-pp-cli powder-rank --slugs alta,steamboat,telluride --agent
  ```
- **`storm-track`** — Show storm progression — when snow starts, peaks, and ends — by correlating rolling hourly forecasts over time

  _When planning travel around a storm, this shows exactly when to arrive and when the storm window closes_

  ```bash
  opensnow-pp-cli storm-track telluride --agent
  ```
- **`overnight`** — Check the semi-daily forecast for the overnight period at favorited resorts — the powder hunter's morning ritual

  _The first question every skier asks: 'How much snow fell overnight?' This answers it for all favorites in one call_

  ```bash
  opensnow-pp-cli overnight --agent
  ```

### Local state that compounds
- **`dashboard`** — One-command view of all favorited locations with current temp, 24h snow, 5-day total, and operating status

  _Replaces opening the OpenSnow app — one command shows everything an agent needs about the user's preferred mountains_

  ```bash
  opensnow-pp-cli dashboard --agent
  ```
- **`diff`** — Compare current snow report against the last-synced version to see what changed: new snow, lifts opened or closed, status changes

  _Agents monitoring resort conditions can detect meaningful changes without parsing raw reports_

  ```bash
  opensnow-pp-cli diff alta --agent
  ```
- **`history`** — Show snowfall trends, season totals vs averages, and base depth progression from cached report snapshots

  _When planning a trip weeks out, historical context shows whether the mountain is trending up or down_

  ```bash
  opensnow-pp-cli history steamboat --days 30 --agent
  ```

### Agent-native plumbing
- **`digest`** — Pull all Daily Snow posts for favorited regions, strip HTML to clean text, and show a summary digest

  _Expert forecasts from OpenSnow meteorologists, rendered for agents and terminals instead of requiring a browser_

  ```bash
  opensnow-pp-cli digest --region colorado,tahoe --agent
  ```

## Command Reference

**daily-reads** — Manage daily reads


**forecast** — Manage forecast

- `opensnow-pp-cli forecast get-by-point` — Get a forecast for any lng,lat point on earth (on land). Returns current conditions, hourly detail, day/night...
- `opensnow-pp-cli forecast get-detail` — Get a 5-day forecast for any named OpenSnow location. Returns current, hourly, and daily forecasts along with...
- `opensnow-pp-cli forecast get-snow-detail` — Get a 5-day day + night snowfall forecast for any named OpenSnow location. Periods split into daytime (6am-6pm) and...

**snow-report** — Manage snow report

- `opensnow-pp-cli snow-report <id_or_slug>` — Retrieve the most recent snow report — resort-reported snowfall, base depth, operating status, and conditions....


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
opensnow-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Best powder day this week

```bash
opensnow-pp-cli powder-rank --slugs alta,steamboat,telluride --agent --select resort,score,expected_snow,conditions
```

Ranks all synced Colorado resorts by powder potential for the next 5 days, returning only key fields for agent consumption

### Morning snow check

```bash
opensnow-pp-cli overnight --agent --select name,precip_snow,conditions_label
```

Quick check of overnight snowfall at all favorited resorts — the powder hunter's morning ritual

### Compare two resorts

```bash
opensnow-pp-cli compare alta steamboat --agent
```

Side-by-side comparison of forecasts and conditions for trip planning

### Expert forecast digest

```bash
opensnow-pp-cli digest --region colorado --agent
```

Read today's Daily Snow from OpenSnow's meteorologists in terminal-friendly format

### Storm watch

```bash
opensnow-pp-cli storm-track telluride --agent --select period,precip_snow,wind_speed
```

Track when the next storm starts, peaks, and ends at Telluride with narrowed fields for agent context

## Auth Setup

OpenSnow API access is partnership-only. Set your API key with `opensnow-pp-cli config set-token <key>`. The key is stored locally and passed as a query parameter on every request. Access level 5 covers point-based forecasts; level 10 unlocks named locations, snow reports, and Daily Snow.

Run `opensnow-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  opensnow-pp-cli snow-report mock-value --agent --select id,name,status
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
opensnow-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
opensnow-pp-cli feedback --stdin < notes.txt
opensnow-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.opensnow-pp-cli/feedback.jsonl`. They are never POSTed unless `OPENSNOW_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OPENSNOW_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
opensnow-pp-cli profile save briefing --json
opensnow-pp-cli --profile briefing snow-report mock-value
opensnow-pp-cli profile list --json
opensnow-pp-cli profile show briefing
opensnow-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `opensnow-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/opensnow/cmd/opensnow-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add opensnow-pp-mcp -- opensnow-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which opensnow-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   opensnow-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `opensnow-pp-cli <command> --help`.
