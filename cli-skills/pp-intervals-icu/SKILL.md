---
name: pp-intervals-icu
description: "The first intervals.icu CLI with a local training database — offline search, SQL Trigger phrases: `sync my intervals.icu data`, `what's my form`, `compare my power this season`, `show my wellness trends`, `what training did I miss`, `use intervals.icu`, `run intervals.icu`."
author: "Milos Mladenovic"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - intervals-icu-pp-cli
    install:
      - kind: go
        bins: [intervals-icu-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/intervals-icu/cmd/intervals-icu-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/intervals-icu/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Intervals.icu — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `intervals-icu-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install intervals-icu --cli-only
   ```
2. Verify: `intervals-icu-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/intervals-icu/cmd/intervals-icu-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sync your activities, wellness, calendar, workouts and gear into local SQLite, then search, run SQL, and compute fitness/form trends offline. Wraps the full intervals.icu REST API for live reads and writes, and adds analytics every existing MCP server and wrapper leaves on the table.

## When to Use This CLI

Use when an agent or user needs to query intervals.icu training data programmatically: pulling activities, wellness, calendar and workouts, computing fitness/form, or comparing performance curves across seasons. Best when repeated queries make a local synced store worthwhile.

## Anti-triggers

Do not use this CLI for:
- Do not use for real-time device streaming during a ride
- Do not use to edit another athlete's data without coach permissions
- Do not use as a replacement for the web UI's interactive charts

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sync`** — Sync your full activity, wellness, event, workout and gear history into a local SQLite database for offline search and SQL.

  _Reach for this first so search/sql/form work offline without hammering the API on every query._

  ```bash
  intervals-icu-pp-cli sync --since 365d
  ```
- **`form`** — Compute CTL (fitness), ATL (fatigue) and TSB/form from your synced activity load and show the trend.

  _Use to answer 'am I fresh enough to race / overreaching?' without opening the web UI._

  ```bash
  intervals-icu-pp-cli form --days 90 --json
  ```
- **`since`** — Show planned workouts, completed activities, and what was missed within a recent time window.

  _Use for a quick 'what happened / what's coming' digest after time away._

  ```bash
  intervals-icu-pp-cli since 7d --json
  ```

### Offline analysis
- **`curve compare`** — Compare best power/pace/HR curves between two date ranges from the local store.

  _Use to quantify season-over-season fitness change for a given duration._

  ```bash
  intervals-icu-pp-cli curve compare --metric power --this 90d --vs 365d --json
  ```
- **`wellness trends`** — Correlate HRV / resting HR / sleep against training load over a window from the local store.

  _Use to spot whether HRV/resting-HR are tracking accumulated fatigue._

  ```bash
  intervals-icu-pp-cli wellness trends --days 60 --json
  ```
- **`gear status`** — Roll up distance/time per gear component against reminders to flag what needs service or replacement.

  _Use to catch chain/tyre/shoe replacement thresholds before they are overdue._

  ```bash
  intervals-icu-pp-cli gear status --json
  ```

## Command Reference

**activity** — Manage activity

- `intervals-icu-pp-cli activity delete` — Delete an activity
- `intervals-icu-pp-cli activity get` — An empty stub object is returned for Strava activities
- `intervals-icu-pp-cli activity update` — Strava activities cannot be updated

**athlete** — Manage athlete

- `intervals-icu-pp-cli athlete get` — Get the athlete with sportSettings and custom_items
- `intervals-icu-pp-cli athlete update` — Update an athlete

**athlete-plans** — Manage athlete plans

- `intervals-icu-pp-cli athlete-plans` — Change training plans for a list of athletes

**chats** — Manage chats

- `intervals-icu-pp-cli chats send-message` — Returns the new message id. If a new chat was created then it is also returned.
- `intervals-icu-pp-cli chats show` — Get a chat by id

**disconnect-app** — Manage disconnect app

- `intervals-icu-pp-cli disconnect-app` — Disconnect the athlete from the app matching the bearer token

**download-workout-ext** — Manage download workout ext

- `intervals-icu-pp-cli download-workout-ext <ext>` — The athlete to use is extracted from the bearer token and used to resolve power targets etc..

**pace-distances** — Manage pace distances

- `intervals-icu-pp-cli pace-distances` — List pace curve distances

**shared-event** — Manage shared event

- `intervals-icu-pp-cli shared-event <id>` — Get a shared event (e.g. race)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
intervals-icu-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Season-over-season power

```bash
intervals-icu-pp-cli curve compare --metric power --this 90d --vs 365d --json --select this.peaks,vs.peaks
```

Compare recent best power against a year ago, selecting just the peak arrays.

### Fresh enough to race?

```bash
intervals-icu-pp-cli form --days 42 --json
```

Show six weeks of fitness/fatigue/form to judge taper readiness.

### Catch up after a break

```bash
intervals-icu-pp-cli since 14d
```

Digest of planned, completed and missed sessions over two weeks.

## Auth Setup
Run `intervals-icu-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export INTERVALS_ICU_API_KEY="<your-key>"
```

Or persist it in `~/.config/intervals-icu-pp-cli/config.toml`.

Run `intervals-icu-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  intervals-icu-pp-cli activity get mock-value --agent --select id,name,status
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
intervals-icu-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
intervals-icu-pp-cli feedback --stdin < notes.txt
intervals-icu-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/intervals-icu-pp-cli/feedback.jsonl`. They are never POSTed unless `INTERVALS_ICU_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `INTERVALS_ICU_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
intervals-icu-pp-cli profile save briefing --json
intervals-icu-pp-cli --profile briefing activity get mock-value
intervals-icu-pp-cli profile list --json
intervals-icu-pp-cli profile show briefing
intervals-icu-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `intervals-icu-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/intervals-icu/cmd/intervals-icu-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add intervals-icu-pp-mcp -- intervals-icu-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which intervals-icu-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   intervals-icu-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `intervals-icu-pp-cli <command> --help`.
