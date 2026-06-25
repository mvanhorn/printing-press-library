---
name: pp-withings
description: "Every Withings metric in one offline-first, agent-native CLI — with a local SQLite mirror and recomposition, recovery Trigger phrases: `check my withings`, `how is my weight trending`, `am I recovering or overtraining`, `sleep debt this week`, `BP report for my doctor`, `use withings`, `run withings`."
author: "Greg Stellato"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - withings-pp-cli
    install:
      - kind: go
        bins: [withings-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/withings/cmd/withings-pp-cli
---

# Withings — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `withings-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install withings --cli-only
   ```
2. Verify: `withings-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/withings/cmd/withings-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Pulls your weight/body-composition, activity, sleep, heart/ECG and workouts from the official Withings API into a local SQLite store you can search, query with SQL, and export as JSON or CSV. On top of that it adds local-join analytics — `recomp`, `recovery`, `bp-report`, `sleep debt`, `digest`, `correlate` — that no single API call can produce.

## When to Use This CLI

Use this CLI when you want programmatic, offline-queryable access to your own Withings health data and analytics across metrics over time — body recomposition, training recovery, sleep debt, BP/AFib reports for a clinician, or a structured health digest to feed an LLM. It is ideal for agents answering 'how is my health trending' and for scripted exports.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to push data to Garmin/TrainerRoad/Strava — use withings-sync for one-way fitness-platform sync.
- Do not use it for medical diagnosis; it reports your recorded measurements and is not a clinical device.
- Do not use it to access another person's Withings data — it operates only on the authorized account.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local-join analytics
- **`recomp`** — See whether you're actually recomposing — fat mass down while lean mass holds — on a de-noised rolling-average weight, not scale-weight whiplash.

  _Reach for this instead of raw measures when the question is 'is my cut/bulk working', not 'what did I weigh on Tuesday'._

  ```bash
  withings-pp-cli recomp --since 90d --agent
  ```
- **`recovery`** — Weigh recent workout HR-zone load against your recovery markers (resting HR, HRV, sleep score) to catch overtraining before it catches you.

  _Use when deciding whether to push or back off; it correlates how hard you trained with how recovered you are._

  ```bash
  withings-pp-cli recovery --since 14d --agent
  ```
- **`sleep debt`** — Cumulative sleep deficit against your target over a rolling window — the number the per-night summary never adds up for you.

  _Use for sleep-deficit accounting over time; for one night use 'sleep summary'._

  ```bash
  withings-pp-cli sleep debt --window 14d --agent
  ```
- **`correlate`** — Pearson + best-lag correlation between any two daily metrics (weight vs sleep score, steps vs resting HR).

  _The flexible fallback when no curated readout fits the metric pair you care about._

  ```bash
  withings-pp-cli correlate weight sleep_score --since 90d --agent
  ```

### Clinician + agent reports
- **`bp-report`** — A dated blood-pressure + AFib table with your own annotations (medication changes, symptoms) — the clean history to hand a cardiologist.

  _Use for a doctor-ready BP/AFib history; prefer it over raw 'heart list' when you need the annotated report._

  ```bash
  withings-pp-cli bp-report --since 90d --agent
  ```
- **`digest`** — One structured 'what changed since <time> across all my metrics' snapshot — built for piping into an agent.

  _The go-to for LLM health Q&A: one cursor-driven structured pull instead of N endpoint calls._

  ```bash
  withings-pp-cli digest --since 24h --agent
  ```

## Command Reference

**activity** — Daily and intraday activity: steps, distance, calories, heart-rate zones

- `withings-pp-cli activity get` — Daily activity aggregates (getactivity).
- `withings-pp-cli activity intraday` — High-resolution intraday activity series (getintradayactivity).

**devices** — Linked Withings devices

- `withings-pp-cli devices` — List linked devices (getdevice): model, battery, last sync.

**goals** — User goals: steps, sleep, weight

- `withings-pp-cli goals` — Get user goals (getgoals).

**heart** — ECG/AFib recordings and blood-pressure readings

- `withings-pp-cli heart ecg` — Get a single raw ECG signal by signalid (get).
- `withings-pp-cli heart list` — List heart recordings (list): ECG signalids, AFib classification, BP.

**measure** — Body measurements: weight, body composition, blood pressure, SpO2, temperature, ECG intervals

- `withings-pp-cli measure` — Get body measurements (getmeas). Filter by type codes and date range.

**notify** — Webhook subscriptions (Notify)

- `withings-pp-cli notify get` — Get one subscription (get).
- `withings-pp-cli notify list` — List webhook subscriptions (list).
- `withings-pp-cli notify revoke` — Revoke a webhook subscription (revoke).
- `withings-pp-cli notify subscribe` — Subscribe a webhook (subscribe).

**sleep** — Sleep stage series and per-night summaries

- `withings-pp-cli sleep series` — High-frequency sleep stage series (get).
- `withings-pp-cli sleep summary` — Per-night sleep summary (getsummary): durations, HR, RR, snoring, AHI, sleep score.

**workouts** — Workout sessions with HR zones, distance, calories

- `withings-pp-cli workouts` — List workout sessions (getworkouts).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
withings-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Mirror everything, then ask offline

```bash
withings-pp-cli sync --resources measure,activity,sleep,workouts,heart --since 180d
```

One incremental sync populates the local store for fast offline analytics.

### Agent-native health digest

```bash
withings-pp-cli digest --since 24h --agent --select weight,sleep_score,resting_hr,new_afib
```

Narrowed structured output an agent can consume without parsing the full envelope.

### Doctor-ready BP/AFib history

```bash
withings-pp-cli bp-report --since 90d --note 2026-06-03=started-amlodipine --csv
```

A dated BP + AFib table with your medication-change annotation, exported as CSV.

### Is my cut working?

```bash
withings-pp-cli recomp --since 90d
```

De-noised weight trend plus fat-down/muscle-held verdict over the window.

## Auth Setup

Withings uses OAuth2. Register a free app at the Withings developer portal, run `withings-pp-cli auth login` once to authorize in your browser, and the CLI stores and auto-rotates your refresh token (Withings refresh tokens are single-use, so the CLI persists each new one for you).

Run `withings-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  withings-pp-cli activity get --agent --select id,name,status
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
withings-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
withings-pp-cli feedback --stdin < notes.txt
withings-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/withings-pp-cli/feedback.jsonl`. They are never POSTed unless `WITHINGS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WITHINGS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
withings-pp-cli profile save briefing --json
withings-pp-cli --profile briefing activity get
withings-pp-cli profile list --json
withings-pp-cli profile show briefing
withings-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `withings-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/withings/cmd/withings-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add withings-pp-mcp -- withings-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which withings-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   withings-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `withings-pp-cli <command> --help`.
