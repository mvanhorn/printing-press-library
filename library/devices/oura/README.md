# Oura Ring CLI

**The only Oura Ring CLI that remembers everything: sync all metrics to local SQLite, correlate habits with scores, detect anomalies, and alert on multi-day recovery deficits no app can surface.**

Every existing Oura tool is a thin API wrapper — it answers today's question and forgets the answer. oura-pp-cli syncs all 15+ data types to a local SQLite store, enabling offline queries, cross-metric joins, and time-series analysis that require months of history to be meaningful. The result is a CLI where agents can ask 'does alcohol actually hurt my sleep?' and get a statistically grounded answer, not a raw score.

## Install

The recommended path installs both the `oura-pp-cli` binary and the `pp-oura` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install oura
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install oura --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install oura --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install oura --agent claude-code
npx -y @mvanhorn/printing-press-library install oura --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/oura/cmd/oura-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/oura-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install oura --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-oura --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-oura --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install oura --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/oura-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OURA_ACCESS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/oura/cmd/oura-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "oura": {
      "command": "oura-pp-mcp",
      "env": {
        "OURA_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Oura deprecated Personal Access Tokens in December 2025 — OAuth2 Authorization Code is now the only supported flow, and this CLI does not yet automate that exchange. Register an OAuth application at https://cloud.ouraring.com/oauth/applications, complete the Authorization Code exchange yourself (any OAuth2 client works) to obtain an access_token, then run:

  oura-pp-cli auth set-token <token>

or export OURA_ACCESS_TOKEN=<token> before running any command. Access tokens are valid for 30 days; when one expires, repeat the OAuth2 exchange and re-run 'auth set-token' with the new value. Run 'oura-pp-cli doctor' to verify setup.

## Quick Start

```bash
# Verify setup and connectivity before the first sync
oura-pp-cli doctor --dry-run

# Populate the local SQLite store with 30 days of history across all data types
oura-pp-cli sync --since 30d

# Morning review: sleep, readiness, activity, stress, and SpO2 in one view
oura-pp-cli dashboard

# See today's readiness annotated against your personal 30-day rolling baseline
oura-pp-cli baseline --metric readiness --window 30

# Find whether a tagged habit actually correlates with next-day readiness
oura-pp-cli correlate --tag "alcohol" --metric readiness --since 90d --agent

```

## Unique Features

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

## Usage

Run `oura-pp-cli --help` for the full command reference and flag list.

## Commands

### daily-activity

Daily activity scores, steps, calories, and MET data

- **`oura-pp-cli daily-activity`** - List daily activity summaries

### daily-cardiovascular-age

Estimated daily cardiovascular age

- **`oura-pp-cli daily-cardiovascular-age`** - List daily cardiovascular age estimates

### daily-readiness

Daily readiness scores and contributor breakdown

- **`oura-pp-cli daily-readiness`** - List daily readiness scores with contributors

### daily-resilience

Daily resilience scores

- **`oura-pp-cli daily-resilience`** - List daily resilience scores

### daily-sleep

Daily sleep scores and contributor breakdown

- **`oura-pp-cli daily-sleep`** - List daily sleep summaries with scores and contributors

### daily-spo2

Daily blood oxygen (SpO2) averages

- **`oura-pp-cli daily-spo2`** - List daily SpO2 blood oxygen averages

### daily-stress

Daily stress levels and recovery status

- **`oura-pp-cli daily-stress`** - List daily stress and recovery data

### enhanced-tag

User-created enhanced tags with timestamps and text notes

- **`oura-pp-cli enhanced-tag create`** - Create an enhanced tag
- **`oura-pp-cli enhanced-tag delete`** - Delete an enhanced tag by document ID
- **`oura-pp-cli enhanced-tag list`** - List enhanced tags (user annotations with optional text)

### heartrate

Continuous heart rate time-series samples

- **`oura-pp-cli heartrate`** - List heart rate samples for a datetime range (up to 5-min resolution)

### personal-info

Personal profile information (age, weight, height, biological sex)

- **`oura-pp-cli personal-info`** - Get personal profile information

### rest-mode-period

Rest mode periods (user-activated recovery mode)

- **`oura-pp-cli rest-mode-period`** - List rest mode periods

### ring-configuration

Ring hardware configuration and firmware version

- **`oura-pp-cli ring-configuration`** - List ring configuration records

### session

Mindfulness and breathing sessions (meditation, napping, breathing exercises)

- **`oura-pp-cli session`** - List mindfulness and breathing sessions

### sleep

Detailed sleep sessions with stage breakdown, HRV, and respiratory rate

- **`oura-pp-cli sleep`** - List detailed sleep sessions with stage timelines and HRV data

### sleep-time

Optimal sleep time recommendations

- **`oura-pp-cli sleep-time`** - List optimal sleep time recommendations

### webhook-subscription

Webhook subscriptions for real-time event delivery

- **`oura-pp-cli webhook-subscription create`** - Create a webhook subscription
- **`oura-pp-cli webhook-subscription delete`** - Delete a webhook subscription
- **`oura-pp-cli webhook-subscription list`** - List webhook subscriptions

### workout

Workout sessions with type, duration, calories, and heart rate data

- **`oura-pp-cli workout`** - List workout sessions


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
oura-pp-cli daily-activity

# JSON for scripting and agents
oura-pp-cli daily-activity --json

# Filter to specific fields
oura-pp-cli daily-activity --json --select id,name,status

# Dry run — show the request without sending
oura-pp-cli daily-activity --dry-run

# Agent mode — JSON + compact + no prompts in one flag
oura-pp-cli daily-activity --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `OURA_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
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

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
oura-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/oura-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OURA_ACCESS_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `oura-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `oura-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OURA_ACCESS_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Your access token is invalid or missing. Re-run the OAuth2 Authorization Code exchange at https://cloud.ouraring.com/oauth/applications and save the new token with 'oura-pp-cli auth set-token <token>' or 'export OURA_ACCESS_TOKEN=<token>'
- **token expired / 401 Unauthorized after working previously** — Access tokens are valid for 30 days and this CLI does not auto-refresh them; repeat the OAuth2 exchange and re-run 'oura-pp-cli auth set-token <token>' with the new value
- **baseline / correlate / anomalies returns 'not enough history'** — Run 'oura-pp-cli sync --since 90d' to populate more history; these commands need at least 14-30 days of local records
- **sync hangs on heartrate table** — Heartrate returns thousands of samples per day; reduce the window with 'oura-pp-cli sync --resources heartrate --since 7d' or skip it with '--resources daily_sleep,daily_readiness,daily_activity'

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**arzzen/oura**](https://github.com/arzzen/oura) — Bash
- [**hedgertronic/oura-ring**](https://github.com/hedgertronic/oura-ring) — Python
- [**visionik/ouracli**](https://github.com/visionik/ouracli) — Python
- [**daveremy/oura-mcp**](https://github.com/daveremy/oura-mcp) — JavaScript
- [**elizabethtrykin/oura-mcp**](https://github.com/elizabethtrykin/oura-mcp) — JavaScript
- [**FelixWag/oura-ring-mcp**](https://github.com/FelixWag/oura-ring-mcp) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
