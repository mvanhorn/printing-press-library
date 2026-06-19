---
name: pp-oura
description: "The only Oura Ring CLI that remembers everything: sync all metrics to local SQLite, correlate habits with scores Trigger phrases: `how did I sleep last night`, `check my Oura readiness`, `show my HRV trend`, `does alcohol affect my sleep`, `am I recovered enough to train`, `use oura`, `run oura ring`, `correlate my sleep tags`."
author: "slinsmaier"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - oura-pp-cli
    install:
      - kind: go
        bins: [oura-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/oura/cmd/oura-pp-cli
---

# Oura Ring — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `oura-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install oura --cli-only
   ```
2. Verify: `oura-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/oura/cmd/oura-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every existing Oura tool is a thin API wrapper — it answers today's question and forgets the answer. oura-pp-cli syncs all 15+ data types to a local SQLite store, enabling offline queries, cross-metric joins, and time-series analysis that require months of history to be meaningful. The result is a CLI where agents can ask 'does alcohol actually hurt my sleep?' and get a statistically grounded answer, not a raw score.

## When to Use This CLI

Use oura-pp-cli when an agent needs to reason about health trends across multiple days or weeks rather than fetching today's snapshot. It is the right tool for personalized anomaly detection, habit-outcome correlation, training load management, recovery alerting, and any analysis that requires joining Oura data types together. It is especially useful when an agent is building a health briefing, managing a training schedule, or monitoring for recovery deficits over a multi-day window.

## Anti-triggers

Do not use this CLI for:
- Writing or editing ring firmware, settings, or hardware configuration (the Oura API is read-only except for tags and webhook subscriptions)
- Real-time heart rate monitoring (ring data syncs overnight; there is no streaming HR endpoint for live use)
- Accessing another user's health data (OAuth scopes are per-user; this CLI is single-account by design)
- Complex statistical modeling beyond trend lines and correlations (use a data science environment like pandas or R after exporting via 'oura-pp-cli query')

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`correlate`** — Find which habits actually move your scores: correlate any enhanced tag string with the next-day change in sleep, readiness, or any other metric.

  _Use this when an agent needs to surface which user-logged behaviors causally predict next-day recovery metrics, rather than just reporting today's score._

  ```bash
  oura-pp-cli correlate --tag "alcohol" --metric readiness --since 90d --agent
  ```
- **`baseline`** — See today's metric against your personal rolling mean and standard deviation bands — not Oura's population norms, your own history.

  _Use this when an agent needs to decide whether today's metric is meaningfully low or high for this specific user, not just relative to the general population._

  ```bash
  oura-pp-cli baseline --metric readiness --window 30 --agent
  ```
- **`hrv-trend`** — Track whether overnight HRV is improving or declining: 7-day and 30-day rolling means, coefficient of variation, and a plain-English training-load verdict.

  _Use this when an agent managing a training schedule needs a structured HRV signal rather than a single-day reading that is too noisy to act on._

  ```bash
  oura-pp-cli hrv-trend --agent --select mean_7day,mean_30day,coefficient_of_variation,verdict
  ```
- **`training-load`** — See your accumulated training stress (7-day rolling) alongside readiness scores with a 1-day lag to reveal the recovery debt curve before it peaks.

  _Use this when an agent needs to recommend rest vs. training based on load-readiness dynamics, not just today's readiness score alone._

  ```bash
  oura-pp-cli training-load --since 21d --agent
  ```
- **`sleep-stages`** — Compare any night's sleep stage durations against your personal 30-day averages, flagging stages that deviate significantly from your own baseline.

  _Use this when an agent needs to identify whether a specific night's sleep architecture was unusual for this user, not just whether the total score was low._

  ```bash
  oura-pp-cli sleep-stages --date 2026-06-15 --agent
  ```
- **`event`** — Show all key metrics for N days before and after a named date to measure the health impact of travel, illness, a race, or any life event.

  _Use this when an agent or user needs to objectively measure whether a specific event (flight, illness, dietary experiment) shifted health metrics._

  ```bash
  oura-pp-cli event --date 2026-06-01 --label "marathon" --window 7 --agent
  ```

### Agent-native alerting
- **`alert`** — Exit 1 when a metric has been above or below a threshold for N consecutive days — the scripting primitive that Oura has never exposed.

  _Use this in cron jobs or agent tool-use when you need a binary trigger — act now vs. not yet — based on a multi-day recovery trend._

  ```bash
  oura-pp-cli alert --metric readiness --threshold 70 --consecutive 3 --direction below
  ```
- **`anomalies`** — Flag days where any metric fell more than N standard deviations from your personal rolling mean, with z-scores and structured JSON output for agent consumption.

  _Use this in a weekly health monitoring agent to catch emerging problems automatically before the user notices them in the app._

  ```bash
  oura-pp-cli anomalies --metric readiness --since 30d --sigma 2 --agent
  ```
- **`gaps`** — List every day in a date range where expected sync data is missing from the local store — identifies ring non-wear and sync failures before they corrupt analysis.

  _Use this before running any bulk analysis to verify data completeness; exits 1 when gaps are found so agents can abort and alert rather than reporting on incomplete history._

  ```bash
  oura-pp-cli gaps --since 30d
  ```

### Structured health reporting
- **`digest`** — One structured call returns the week's metric averages, week-over-week deltas, best and worst recovery days, workout totals, and most-logged tags.

  _Use this when an agent produces a weekly health briefing for a coach, journal, or Notion log — it replaces five separate API calls with one structured payload._

  ```bash
  oura-pp-cli digest --json
  ```
- **`webhook serve`** — Start a local HTTP server that registers with Oura's webhook API and writes incoming events to the local SQLite store as they arrive.

  _Use this to enable event-driven agents that react to new sleep data as it syncs, rather than polling the API on a fixed schedule._

  ```bash
  oura-pp-cli webhook serve --port 8899
  ```

## Command Reference

**daily-activity** — Daily activity scores, steps, calories, and MET data

- `oura-pp-cli daily-activity` — List daily activity summaries

**daily-cardiovascular-age** — Estimated daily cardiovascular age

- `oura-pp-cli daily-cardiovascular-age` — List daily cardiovascular age estimates

**daily-readiness** — Daily readiness scores and contributor breakdown

- `oura-pp-cli daily-readiness` — List daily readiness scores with contributors

**daily-resilience** — Daily resilience scores

- `oura-pp-cli daily-resilience` — List daily resilience scores

**daily-sleep** — Daily sleep scores and contributor breakdown

- `oura-pp-cli daily-sleep` — List daily sleep summaries with scores and contributors

**daily-spo2** — Daily blood oxygen (SpO2) averages

- `oura-pp-cli daily-spo2` — List daily SpO2 blood oxygen averages

**daily-stress** — Daily stress levels and recovery status

- `oura-pp-cli daily-stress` — List daily stress and recovery data

**enhanced-tag** — User-created enhanced tags with timestamps and text notes

- `oura-pp-cli enhanced-tag list` — List enhanced tags (user annotations with optional text)

**heartrate** — Continuous heart rate time-series samples

- `oura-pp-cli heartrate` — List heart rate samples for a datetime range (up to 5-min resolution)

**personal-info** — Personal profile information (age, weight, height, biological sex)

- `oura-pp-cli personal-info` — Get personal profile information

**rest-mode-period** — Rest mode periods (user-activated recovery mode)

- `oura-pp-cli rest-mode-period` — List rest mode periods

**ring-configuration** — Ring hardware configuration and firmware version

- `oura-pp-cli ring-configuration` — List ring configuration records

**session** — Mindfulness and breathing sessions (meditation, napping, breathing exercises)

- `oura-pp-cli session` — List mindfulness and breathing sessions

**sleep** — Detailed sleep sessions with stage breakdown, HRV, and respiratory rate

- `oura-pp-cli sleep` — List detailed sleep sessions with stage timelines and HRV data

**sleep-time** — Optimal sleep time recommendations

- `oura-pp-cli sleep-time` — List optimal sleep time recommendations

**webhook-subscription** — Webhook subscriptions for real-time event delivery

- `oura-pp-cli webhook-subscription create` — Create a webhook subscription
- `oura-pp-cli webhook-subscription delete` — Delete a webhook subscription
- `oura-pp-cli webhook-subscription list` — List webhook subscriptions

**workout** — Workout sessions with type, duration, calories, and heart rate data

- `oura-pp-cli workout` — List workout sessions


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `OURA_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `oura-pp-cli daily-activity`
- `oura-pp-cli daily-activity get`
- `oura-pp-cli daily-activity list`
- `oura-pp-cli daily-activity search`
- `oura-pp-cli daily-cardiovascular-age`
- `oura-pp-cli daily-cardiovascular-age get`
- `oura-pp-cli daily-cardiovascular-age list`
- `oura-pp-cli daily-cardiovascular-age search`
- `oura-pp-cli daily-readiness`
- `oura-pp-cli daily-readiness get`
- `oura-pp-cli daily-readiness list`
- `oura-pp-cli daily-readiness search`
- `oura-pp-cli daily-resilience`
- `oura-pp-cli daily-resilience get`
- `oura-pp-cli daily-resilience list`
- `oura-pp-cli daily-resilience search`
- `oura-pp-cli daily-sleep`
- `oura-pp-cli daily-sleep get`
- `oura-pp-cli daily-sleep list`
- `oura-pp-cli daily-sleep search`
- `oura-pp-cli daily-spo2`
- `oura-pp-cli daily-spo2 get`
- `oura-pp-cli daily-spo2 list`
- `oura-pp-cli daily-spo2 search`
- `oura-pp-cli daily-stress`
- `oura-pp-cli daily-stress get`
- `oura-pp-cli daily-stress list`
- `oura-pp-cli daily-stress search`
- `oura-pp-cli enhanced-tag`
- `oura-pp-cli enhanced-tag get`
- `oura-pp-cli enhanced-tag list`
- `oura-pp-cli enhanced-tag search`
- `oura-pp-cli heartrate`
- `oura-pp-cli heartrate get`
- `oura-pp-cli heartrate list`
- `oura-pp-cli heartrate search`
- `oura-pp-cli rest-mode-period`
- `oura-pp-cli rest-mode-period get`
- `oura-pp-cli rest-mode-period list`
- `oura-pp-cli rest-mode-period search`
- `oura-pp-cli ring-configuration`
- `oura-pp-cli ring-configuration get`
- `oura-pp-cli ring-configuration list`
- `oura-pp-cli ring-configuration search`
- `oura-pp-cli session`
- `oura-pp-cli session get`
- `oura-pp-cli session list`
- `oura-pp-cli session search`
- `oura-pp-cli sleep`
- `oura-pp-cli sleep get`
- `oura-pp-cli sleep list`
- `oura-pp-cli sleep search`
- `oura-pp-cli sleep-time`
- `oura-pp-cli sleep-time get`
- `oura-pp-cli sleep-time list`
- `oura-pp-cli sleep-time search`
- `oura-pp-cli webhook-subscription`
- `oura-pp-cli webhook-subscription get`
- `oura-pp-cli webhook-subscription list`
- `oura-pp-cli webhook-subscription search`
- `oura-pp-cli workout`
- `oura-pp-cli workout get`
- `oura-pp-cli workout list`
- `oura-pp-cli workout search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
oura-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning agent briefing

```bash
oura-pp-cli dashboard --agent --select date,sleep_score,readiness_score,activity_score,stress_level,spo2_average
```

Structured morning snapshot for an AI agent to summarize in a daily briefing; --select keeps the payload compact.

### Weekly health digest for coach or journal

```bash
oura-pp-cli digest --json
```

One call returns the week's averages, WoW deltas, best/worst recovery days, and top tags — ready to paste into Notion or send to a health coach.

### Habit correlation analysis

```bash
oura-pp-cli correlate --tag "no alcohol" --metric sleep --since 90d --agent --select tags,metric,tagged_next_day_avg_delta,baseline_next_day_avg_delta,verdict
```

Find statistically grounded evidence for whether a tracked habit improves sleep; --select exposes only the fields an agent needs to make a recommendation.

### Recovery alerting in cron or CI

```bash
oura-pp-cli alert --metric readiness --threshold 70 --consecutive 3 --direction below
```

Exits 0 (condition not met) or 1 (readiness below 70 for 3+ consecutive days); wire into a shell script or agent tool-use to trigger a rest-day recommendation.

### Training load check before a workout

```bash
oura-pp-cli training-load --since 14d --agent --select rows.day,rows.rolling_load_7d,rows.next_day_readiness,load_readiness_correlation,verdict
```

Surface the accumulated training stress vs. recovery curve so an agent can recommend rest, reduced effort, or full training based on the ratio.

## Auth Setup

Oura deprecated Personal Access Tokens in December 2025 — OAuth2 Authorization Code is now the only supported flow, and this CLI does not yet automate that exchange. Register an OAuth application at https://cloud.ouraring.com/oauth/applications, complete the Authorization Code exchange yourself (any OAuth2 client works) to obtain an access_token, then run:

  oura-pp-cli auth set-token <token>

or export OURA_ACCESS_TOKEN=<token> before running any command. Access tokens are valid for 30 days; when one expires, repeat the OAuth2 exchange and re-run 'auth set-token' with the new value. Run 'oura-pp-cli doctor' to verify setup.

Run `oura-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  oura-pp-cli daily-activity --agent --select id,name,status
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
oura-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
oura-pp-cli feedback --stdin < notes.txt
oura-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/oura-pp-cli/feedback.jsonl`. They are never POSTed unless `OURA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OURA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
oura-pp-cli profile save briefing --json
oura-pp-cli --profile briefing daily-activity
oura-pp-cli profile list --json
oura-pp-cli profile show briefing
oura-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `oura-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/oura/cmd/oura-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add oura-pp-mcp -- oura-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which oura-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   oura-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `oura-pp-cli <command> --help`.
