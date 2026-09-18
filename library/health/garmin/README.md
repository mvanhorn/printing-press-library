# Garmin Connect CLI

**Your whole Garmin history in a local database, not one 28-day page at a time.**

Garmin Connect caps every daily-stats request at 28 calendar days and issues no personal API token, which is why its ecosystem is a handful of Python libraries rather than a tool an agent can call. `history` walks the whole account into a local SQLite archive, oldest day first, and `insights sleep` and `insights training` answer months-long sleep, training-load and heart-rate-zone questions from that archive with JSON on stdout. `auth login` keeps each household account in its own home and refuses to store a token that belongs to somebody else.

Created by [@prashantkamani](https://github.com/prashantkamani) (Prashant Kamani).

## Install

The recommended path installs both the `garmin-pp-cli` binary and the `pp-garmin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install garmin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install garmin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install garmin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install garmin --agent claude-code
npx -y @mvanhorn/printing-press-library install garmin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/garmin/cmd/garmin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/garmin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install garmin --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-garmin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-garmin --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install garmin --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/garmin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Leave the credential prompt blank and close it: Garmin issues no personal API key, and this bundle has none to paste. Connect the account once from a terminal with `garmin-pp-cli auth login --email <your Garmin account email>`; the MCP server reads the same home the CLI writes.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/garmin/cmd/garmin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`). No credential belongs in this block: run `garmin-pp-cli auth login --email <your Garmin account email>` once, and the server picks the tokens up from the resolved home. Add `"env": {"GARMIN_HOME": "/path/to/that/home"}` only if you keep more than one Garmin account on this machine.

```json
{
  "mcpServers": {
    "garmin": {
      "command": "garmin-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Garmin issues no personal API key, so `garmin-pp-cli auth login --email you@example.com` is the way in: it signs the browser out of Garmin, opens Garmin's own sign-in page, catches the one-time ticket on a loopback port on 127.0.0.1, and then asks Garmin which account it just authenticated. If that is not the address you passed, nothing is written to disk. Your password never reaches this CLI, and the refresh token keeps the session alive afterwards without another browser visit. A token supplied in GARMIN_ACCESS_TOKEN or GARMIN_TOKEN is used as-is instead: never refreshed, and not this home's stored chain. One Garmin account per home; `auth status --verify` asks Garmin which one this home holds. `garmin-pp-cli auth logout` clears that home's stored chain.

## Quick Start

```bash
# Health check first: it names the directory every path kind resolved to and whether this home has credentials.
garmin-pp-cli doctor

# What each series already holds and what it still owes. It makes no request, so it is safe before anything is filled.
garmin-pp-cli history --status

# Do this once. It is the only command that fills the archive, and everything below reads the archive rather than the API.
garmin-pp-cli history --since all

# The first archive question: four weeks of sleep with the four weeks before them alongside.
garmin-pp-cli insights sleep --days 28 --agent

# The same window for activities: totals by type, time in heart-rate zone and weekly load.
garmin-pp-cli insights training --days 28 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Questions answered from the archive
- **`insights sleep`** — Duration, score, stage split and resting heart rate for a window of nights, with the equally long window before it beside them.

  _Pick this over per-night endpoint calls whenever the question spans more than a few nights: one local read replaces one request per night, and nights the watch did not measure are counted as missing rather than as zero._

  ```bash
  garmin-pp-cli insights sleep --days 90 --agent --select period,coverage.nights_with_stats,duration_minutes.avg,score.avg
  ```
- **`insights training`** — Archived activities grouped by type with their time and distance, time summed per heart-rate zone, weekly load buckets, and the VO2-max and readiness trends beside them.

  _Pick this over the activity feed when the question is about volume, intensity or balance rather than one workout; it reports how many activities had zones archived so a strapless ride is not read as easy training._

  ```bash
  garmin-pp-cli insights training --days 28 --agent --select totals,by_type.type,by_type.duration_minutes,hr_zones
  ```

### Local state that compounds
- **`history`** — Fills a local SQLite archive of every Garmin daily series from the oldest day forward, keeping one bookmark per series so an interrupted run resumes instead of restarting.

  _Run this before any question that reaches past the last 28 days: `--since all` or `--since YYYY-MM-DD` is the only depth control it offers, `--status` reports what each series still owes, `--strict` makes a scheduled run exit non-zero instead of warning past a series that did not answer, and every insights command reads the archive rather than the API._

  ```bash
  garmin-pp-cli history --since all
  ```
- **`sql`** — Ask the archive anything in SQL: one read-only SELECT across every series `history` has filled.

  _Reach for this when the question is neither the sleep nor the training recipe: one SELECT joins any two series on the id they share, and a single-statement gate over a mode=ro, query_only handle means no query can write, whatever it says._

  ```bash
  garmin-pp-cli sql "SELECT substr(json_extract(data,'$.startTimeLocal'),1,7) AS month, count(*) AS activities, round(sum(json_extract(data,'$.duration'))/3600.0,1) AS hours FROM resources WHERE resource_type='activities' GROUP BY month ORDER BY month DESC LIMIT 12" --agent
  ```

### Auth you can trust with a household
- **`auth login`** — Refuses to store a token whose account is not the address you named, so a shared browser cannot sign the wrong household member in.

  _Run this once per Garmin account per home before anything else; afterwards every command works unattended from the refresh token._

  ```bash
  garmin-pp-cli auth login --email you@example.com
  ```

## Recipes

### Before you run `history`

```bash
garmin-pp-cli history --since all --dry-run
```

The dry run prints the same estimate the real run prints before its expensive half, and makes no request at all, so read it before committing to the fill. What it is counting: the daily series start at 2007-01-01, the oldest day Garmin answers for whatever the account's age, and the activity feed starts at the account's first activity; one request covers a 28-day or 364-day window of a daily series, one activity for each of the three per-activity fetches, or one signal day for each of the four per-day series, and each request takes about 0.4 s. An account worn daily for years comes to several thousand requests, about an hour, and a few hundred MB on disk; a sparse account is about a thousand requests and under ten minutes. The per-activity fetches — detail, splits and heart-rate zones — cover only the activities that start on or after the day the run starts from, so a later, deeper `--since` extends them downward along with the daily series. When the estimate reads acceptable, start the fill with `garmin-pp-cli history --since all`.

### A quarter of sleep, narrowed to the numbers that answer the question

```bash
garmin-pp-cli insights sleep --days 90 --agent --select period,coverage.nights_with_stats,duration_minutes.avg,score.avg,trend_vs_prior_period.duration_minutes.delta_pct
```

Ninety nights come out of the archive in one local read; `--select` keeps five numbers instead of the full nightly payload, and `coverage.nights_with_stats` says how many nights actually fed the averages.

### Four weeks of training load and the zone split behind it

```bash
garmin-pp-cli insights training --days 28 --agent --select totals,by_type.type,by_type.duration_minutes,hr_zones
```

Totals, a per-type breakdown and the heart-rate-zone split in one read; drop the `--select` to also see `coverage.activities_with_zones`, which is how many of those activities were recorded with a strap.

### Join two series on a value read out of the JSON

```bash
garmin-pp-cli sql "WITH days AS MATERIALIZED (SELECT substr(json_extract(data,'$.startTimeLocal'),1,10) AS day, count(*) AS activities, round(sum(json_extract(data,'$.duration'))/3600.0,1) AS hours FROM resources WHERE resource_type='activities' GROUP BY 1), steps AS MATERIALIZED (SELECT id AS day, json_extract(data,'$.totalSteps') AS steps FROM resources WHERE resource_type='steps') SELECT d.day, d.activities, d.hours, s.steps FROM days d JOIN steps s ON s.day = d.day ORDER BY d.day DESC LIMIT 14" --agent
```

Every archived series lives in one table, `resources`, keyed by (resource_type, id), so joining two of them is a self-join. When the join keys on a value read out of the JSON — here the activity's own start day against the step series' date id — give each side its own `WITH ... AS MATERIALIZED (...)` CTE: without MATERIALIZED, SQLite inlines the CTE and re-parses every JSON row once per joined pair. `sql` accepts a single SELECT or WITH statement and runs it against a handle opened read-only, so no query can change the archive whatever it says. Run `garmin-pp-cli sql "SELECT resource_type, count(*) FROM resources GROUP BY 1"` first to see which series this archive holds. `analytics --type activities --group-by activityName` stays the shorthand when one top-level field of one series is all you need, and `search <text> --type activities --data-source local` is the full-text equivalent over the same rows.

### Connect a second household account

```bash
garmin-pp-cli auth login --email other@example.com --home ~/.local/share/garmin-homes/other
```

The second account gets its own home: `--home` for a single invocation, GARMIN_HOME to make it durable for a session.

## Usage

Run `garmin-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GARMIN_CONFIG_DIR`, `GARMIN_DATA_DIR`, `GARMIN_STATE_DIR`, or `GARMIN_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GARMIN_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GARMIN_HOME=/srv/garmin
garmin-pp-cli doctor
```

Under `GARMIN_HOME=/srv/garmin`, the four dirs resolve to `/srv/garmin/config`, `/srv/garmin/data`, `/srv/garmin/state`, and `/srv/garmin/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "garmin": {
      "command": "garmin-pp-mcp",
      "env": {
        "GARMIN_HOME": "/srv/garmin"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GARMIN_DATA_DIR` overrides an explicit `--home` for that kind. Use `GARMIN_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GARMIN_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `garmin-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Why `sync` is a dead-end

`garmin-pp-cli` keeps one local archive and `history` is the only command that fills it. The generated `sync` exists because CLI Printing Press emits it with every SQLite store, and for Garmin it can only do three things: page a flat list, add a `since=` filter that the list endpoint declares, and walk rows of a parent table into a child URL. Garmin's daily statistics sit behind date-range endpoints whose URL carries the range and which refuse more than 28 days per call, or behind per-day endpoints that answer for one date. Neither fits those shapes, so the calendar walk and its per-series filled-range state live in `history`. Running one account through both verbs stored the activity feed and the per-activity heart-rate zones twice under different names, and re-paged the whole activity feed on every `sync`, because Garmin's activity list declares no filter the generator recognises. `history` therefore also carries the three fetches only `sync` used to make (activity detail, splits, heart-rate-zone configuration), and `sync` prints a pointer to `history` and exits. Generated messages that say "run `garmin-pp-cli sync` first" still lead to the right place.

## Commands

### account

Bootstrap and identity: social profile, unit settings, and the account email a login is checked against.

- **`garmin-pp-cli account personal-information`** - Returns the identity block for the signed-in account, including the account email. That email is
the only field on any Garmin response that names which account a token belongs to, so `auth
login` reads it immediately after the token exchange and refuses to store a token whose account
does not match the one the operator named. Run it before trusting a home that a shared browser
session may have filled with the wrong household member.
- **`garmin-pp-cli account settings`** - Returns the account's settings. Read this once per home to learn whether the account reports
distances in metric or statute units before formatting any distance, pace or weight.
- **`garmin-pp-cli account social-profile`** - Returns the account's public profile. The `displayName` field is the account key that several
other Garmin paths interpolate, so this is the first call after a login and the cheapest way to
confirm a stored token still works: a 401 or 403 here means the token was rejected by the data
tier.

### activities

The activity feed, per-activity detail and splits, lifetime breakdowns, and original file downloads.

- **`garmin-pp-cli activities breakdown`** - Server-side aggregation of every activity on the account into totals per parent activity type,
by duration or distance. Answers 'what does this account actually do, and how much' in one
request instead of paging the whole activity list.
- **`garmin-pp-cli activities download-original`** - Fetches the original recorded file for one activity as a ZIP containing the device FIT file.
This is the sanctioned export route for raw per-second data that the JSON endpoints do not
expose. Binary response; write it to a file rather than parsing it.
- **`garmin-pp-cli activities get`** - Everything Garmin holds about one activity at summary level: type, timing, distance and elevation.
Aggregate heart rate and power come with it. Take the id from the activity list.
- **`garmin-pp-cli activities hr-time-in-zones`** - Seconds spent in each configured heart-rate zone during one activity.
That is the raw material for a training-polarisation ratio across a season. Combine with the zone
settings to label the zones.
- **`garmin-pp-cli activities list`** - The account's activity feed, one row per recorded activity, newest first. Pages with `--offset`
and `--limit`; an empty page means the end of the account's history. Optional filters narrow by
date range, activity type and sort order. Distinct local start dates over this list are how
'days active' is counted — Garmin exposes no server-side count.
- **`garmin-pp-cli activities splits`** - The lap or split breakdown of a single activity, each with its own distance, duration and
averages. Present for activities the device or the user split.

### fitness

Fitness level over time: VO2 max, max-met values, and Garmin's fitness-age estimate.

- **`garmin-pp-cli fitness age`** - Garmin's fitness-age estimate and the components it was computed from for a single date. There
is no range form, so a fitness-age trend costs one request per day and is what the local archive
exists for.
- **`garmin-pp-cli fitness max-metrics`** - Fitness-level trend: VO2 max, generic and cycling, plus Garmin's max-met value, one row per date.
Only dates with a measurement appear; days without a qualifying activity are absent rather than
zero. At most 28 calendar days per request.

### heart_rate

Configured heart-rate zones, daily heart-rate detail, and per-activity time in zone.

- **`garmin-pp-cli heart-rate daily`** - The intraday heart-rate series for a single date plus the resting, minimum and maximum values
Garmin derived from it. Sampled values only exist for dates a wearable was worn. This is the
account-scoped path variant.
- **`garmin-pp-cli heart-rate daily-alt`** - Identical payload to `daily`, on the wellness-service path that takes the date as a query parameter.
It needs no displayName, so it is kept as a fallback and for use before the bootstrap profile
call has run.
- **`garmin-pp-cli heart-rate zones`** - The zone boundaries the account has configured, per sport. These are settings, not measurements:
read them to label time-in-zone numbers, and expect them to change only when the user edits them
or Garmin auto-detects a new threshold.

### sleep

Sleep score trends, server-aggregated nightly summaries, and full per-night detail.

- **`garmin-pp-cli sleep night`** - Everything Garmin recorded for a single night: stage minutes, sleep windows, restlessness and
the sleep-score breakdown. One request per night, so a trend question should use the range
commands instead. This is the account-scoped path variant; `night-alt` is the same payload
without the displayName in the path.
- **`garmin-pp-cli sleep night-alt`** - Identical payload to `night`, on the sleep-service path that takes the date as a query parameter.
It needs no displayName, so it works before the bootstrap profile call has run, and it is the
fallback if the account-scoped path ever changes.
- **`garmin-pp-cli sleep score-stats`** - The sleep score trend: one row per calendar date with the score value and qualifier. Cheaper
than fetching per-night detail when the question is about a trend rather than one night. At most
28 calendar days per request.
- **`garmin-pp-cli sleep stats`** - One row per night between `start` and `end`, aggregated by Garmin. At most 28 calendar days per
request; `history` fills longer ranges into the local archive in 28-day windows, oldest day
first, de-duplicating on the calendar date. Rows arrive under `individualStats`.

### steps

Daily and weekly step totals against the account's goal.

- **`garmin-pp-cli steps daily`** - One row per calendar date with total steps, the step goal and the distance walked. At most 28
calendar days per request; `history` fills longer ranges into the local archive in 28-day
windows, oldest day first.
- **`garmin-pp-cli steps weekly`** - Weekly step buckets instead of daily ones, about a year of them in a single request.
The daily form would take thirteen requests for the same span. Use this for long-horizon trend
questions and fall back to the daily form when a specific date matters.

### training

Training status over a window and the daily training-readiness score.

- **`garmin-pp-cli training readiness`** - Garmin's training-readiness score for a single date, with the inputs it was built from.
Those inputs are sleep, recovery, HRV and acute load. One request per day; a readiness trend
comes from the local archive, not from this endpoint.
Requires `--date`.
- **`garmin-pp-cli training status`** - Training status with acute and chronic load, for the days ending on the given date.
The status values are productive, maintaining, unproductive, detraining and their siblings.
Returns a page of days, not a single day; treat the end date as the anchor.
Requires `--end`.

### wellness

Daily wellness roll-ups: the day summary, intensity minutes, resting heart rate, and hydration.

- **`garmin-pp-cli wellness daily-summary`** - The single-day roll-up: steps, floors, intensity minutes, calories, resting heart rate and
stress. Check `includesWellnessData` before reading any wellness field — an account with no
wearable returns the envelope with the series absent, not zeroes.
- **`garmin-pp-cli wellness hydration`** - Water intake logged for a single date against the day's goal. Only populated for accounts where
the user logs hydration by hand or from a paired bottle.
- **`garmin-pp-cli wellness hydration-alt`** - The hydration payload plus the activity sweat-loss and goal-adjustment fields.
It is a superset of everything the shorter path returns; prefer this variant unless the extra
fields are unwanted.
- **`garmin-pp-cli wellness intensity-minutes-weekly`** - Weekly buckets of moderate and vigorous intensity minutes against the account's weekly goal.
This is the series behind Garmin's 'intensity minutes' badge.
- **`garmin-pp-cli wellness metrics-daily`** - A single named metric as a daily time series over a date range, selected by the numeric metric
id. Metric 60 is resting heart rate. Longer ranges are accepted here than on the 28-day stats
endpoints, but the exact ceiling is not published.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`garmin-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`garmin-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`garmin-pp-cli learnings list`** - Inspect taught rows
- **`garmin-pp-cli learnings forget <query>`** - Undo a teach
- **`garmin-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`garmin-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`garmin-pp-cli teach-pattern`** - Install a query/resource template up front
- **`garmin-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GARMIN_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `garmin-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
garmin-pp-cli activities list

# JSON for scripting and agents
garmin-pp-cli activities list --json
# Filter to specific fields
garmin-pp-cli activities list --json --select activityId,activityName,activityType

# Dry run — show the request without sending
garmin-pp-cli activities list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
garmin-pp-cli activities list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
garmin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `garmin-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/garmin-pp-cli/config.toml`; `--home`, `GARMIN_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GARMIN_ACCESS_TOKEN` | per_call | No | An access token obtained elsewhere. Garmin issues no personal API key, so there is normally nothing to put here — use `auth login`. A token set here is used as-is, is never refreshed, and is not the account `auth status` reports for this home. |
| `GARMIN_TOKEN` | per_call | No | Same as `GARMIN_ACCESS_TOKEN`. |
| `GARMIN_HOME` | path | No | Root directory for this account's config, data, state and cache. One Garmin account per home. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `garmin-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `garmin-pp-cli doctor` to check credentials
- Run `garmin-pp-cli auth status` to see which home is resolved and which account it is bound to
- Run `garmin-pp-cli auth status --verify` to confirm the stored tokens still work; if they do not, run `garmin-pp-cli auth login --email <your Garmin account email>` again
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **`auth login` reports that Garmin authenticated an account other than the one passed to `--email`.** — A Garmin session survived the sign-out. Nothing was stored: sign out at sso.garmin.com/sso/logout in the browser, then run the login again.
- **Every insights number is null or zero and a hint on stderr names `history`.** — The archive holds no rows for that window. Run `garmin-pp-cli history --since all` once, then `garmin-pp-cli history --status` to see how far each series got.
- **A `history` run stopped partway, or you interrupted it.** — Run it again; `garmin-pp-cli history --status` shows what is still outstanding.
- **A date range longer than 28 days comes back with fewer rows than were asked for.** — Garmin caps daily-stats requests at 28 calendar days. Ask the archive instead, for example `garmin-pp-cli insights sleep --days 90`.
- **`sync` prints that it is superseded and exits 2.** — `history` is the only command that fills the archive. Run `garmin-pp-cli history` instead.
- **Commands read the wrong household member's data on a shared machine.** — Each Garmin account needs its own home. Pass `--home <dir>` or set GARMIN_HOME, then run `garmin-pp-cli doctor --json` to see which directory each path kind resolved to.
- **Sleep and heart-rate series stay empty while activities fill normally.** — The account has no wearable paired; a bike computer records activities but no wellness series. Check `includesWellnessData` on the daily summary before treating it as a bug.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tcgoetz/GarminDB**](https://github.com/tcgoetz/GarminDB) — Python (3292 stars)
- [**cyberjunky/python-garminconnect**](https://github.com/cyberjunky/python-garminconnect) — Python (2953 stars)
- [**matin/garth**](https://github.com/matin/garth) — Python (814 stars)
- [**bpauli/gccli**](https://github.com/bpauli/gccli) — Go (27 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
