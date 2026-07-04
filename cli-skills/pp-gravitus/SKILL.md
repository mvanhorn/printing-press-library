---
name: pp-gravitus
description: "The only CLI that syncs your Gravitus strength data into your training dashboard. Trigger phrases: `sync my Gravitus workouts`, `load my lifting data into the dashboard`, `find my plateau lifts`, `export my Gravitus data`, `check my personal records`, `use gravitus`, `run gravitus-pp-cli`."
author: "mvanhorn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - gravitus-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/other/gravitus/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Gravitus — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `gravitus-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install gravitus --cli-only
   ```
2. Verify: `gravitus-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/gravitus/cmd/gravitus-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use gravitus-pp-cli as the data pipeline for any training dashboard or analysis tool that needs Gravitus workout data. It is the only reliable way to authenticate with Gravitus, paginate full workout history, and write structured LiftingSession records into SQLite.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Dashboard pipeline
- **`sync`** — Sync all Gravitus workouts into your training dashboard's SQLite database — writes LiftingSession records in the exact Prisma schema format with auth, pagination, and incremental support.

  _Use to populate the training dashboard's lifting data — the only reliable way to authenticate and paginate all workout history into dev.db._

  ```bash
  gravitus-pp-cli gravitus-sync --dashboard-db ./prisma/dev.db
  ```
- **`export`** — Export your complete Gravitus training history to CSV or JSON — the first and only way to get your data out of Gravitus.

  _Use when a coach, analyst, or AI agent needs the full training history outside the app._

  ```bash
  gravitus-pp-cli export --format csv --output training_history.csv
  ```

### Analytics
- **`exercises plateau`** — Identifies exercises where estimated 1RM hasn't improved in N weeks — alert-style output for the dashboard coaching panel.

  _Use before a program change — gives evidence-based list of which lifts need intervention._

  ```bash
  gravitus-pp-cli exercises plateau --weeks 6 --agent
  ```
- **`stats volume`** — Weekly total lifting volume (lbs) aggregated from all synced sessions — the same metric the dashboard LiftingSection displays.

  _Use to feed the dashboard's volume trend chart or check load progression over a training block._

  ```bash
  gravitus-pp-cli stats volume --weeks 12 --agent
  ```
- **`exercises prs`** — All-time PRs across every exercise, extracted from PR markers on workout pages.

  _Use to display the personal records panel in the dashboard or track PR cadence._

  ```bash
  gravitus-pp-cli exercises prs --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**accounts** — Authentication — login and session management

- `gravitus-pp-cli accounts` — Fetch login page to retrieve CSRF token

**exercises** — Exercise history, personal records, and volume trends

- `gravitus-pp-cli exercises <exercise_slug>` — Fetch exercise history with PR timeline and volume data

**users** — User profile and paginated workout history

- `gravitus-pp-cli users <user_id>` — Fetch user profile and paginated workout history list

**workouts** — Workout sessions with exercises, sets, reps, weight, and PRs

- `gravitus-pp-cli workouts <workout_id>` — Fetch full workout detail — exercises, sets, reps, weight, personal records


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
gravitus-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Full initial sync into dashboard

```bash
gravitus-pp-cli gravitus-sync --dashboard-db ./prisma/dev.db
```

Pulls all workout pages with pagination and writes every session to dev.db

### Incremental sync (run daily)

```bash
gravitus-pp-cli gravitus-sync --incremental --dashboard-db ./prisma/dev.db
```

Only fetches workouts not already in dev.db — fast and safe to run repeatedly

### Find plateaued lifts for coaching panel

```bash
gravitus-pp-cli exercises plateau --weeks 6 --agent --select exercise,last_pr_date,weeks_stalled
```

Returns structured JSON of stalled lifts for the dashboard coaching panel

### Export full history to CSV

```bash
gravitus-pp-cli export --format csv --output training_history.csv
```

Dumps every workout and set to CSV — useful for offline analysis or backup

### Weekly volume for charting

```bash
gravitus-pp-cli stats volume --weeks 12 --agent
```

Returns 12 weeks of volume totals as JSON — feed directly to a Recharts bar chart

## Auth Setup

Gravitus uses Django session auth. Run `gravitus-pp-cli auth login-password` with your email and password — the CLI handles the CSRF token exchange and stores your session cookie in the config file. Re-run `auth login` whenever the session expires (typically every few weeks).

Run `gravitus-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  gravitus-pp-cli exercises mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
gravitus-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
gravitus-pp-cli feedback --stdin < notes.txt
gravitus-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.gravitus-pp-cli/feedback.jsonl`. They are never POSTed unless `GRAVITUS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GRAVITUS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
gravitus-pp-cli profile save briefing --json
gravitus-pp-cli --profile briefing exercises mock-value
gravitus-pp-cli profile list --json
gravitus-pp-cli profile show briefing
gravitus-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `gravitus-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add gravitus-pp-mcp -- gravitus-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which gravitus-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   gravitus-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `gravitus-pp-cli <command> --help`.
