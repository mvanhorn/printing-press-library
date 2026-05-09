---
name: pp-myfitnesspal
description: "Pull every meal you ever logged out of MyFitnessPal — per-food CSV, agent-shaped trends, and a local SQLite store. Trigger phrases: `what did I eat this week`, `export my food diary`, `find every time I logged X`, `top foods driving my protein`, `am I hitting my calorie streak`, `use myfitnesspal`, `run myfitnesspal`."
author: "Nick Scarabosio"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - myfitnesspal-pp-cli
    install:
      - kind: go
        bins: [myfitnesspal-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/myfitnesspal/cmd/myfitnesspal-pp-cli
---

# MyFitnessPal — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `myfitnesspal-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install myfitnesspal --cli-only
   ```
2. Verify: `myfitnesspal-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/myfitnesspal/cmd/myfitnesspal-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

MyFitnessPal closed their API, gates per-food export behind premium, and ships per-meal rows even there. This CLI imports your browser session, syncs your diary to local SQLite, and answers questions the official UI never can: which 5 foods drove 80% of your protein this quarter, what's your weekly weight slope vs your deficit, what changed since last sync. Built with agent-native output (`--json`, `--select`, `context`) so Claude can reason over your last 14 days in one tool call.

## When to Use This CLI

Reach for this CLI when an agent needs to reason over a user's actual eating, exercise, or weight history — especially across windows longer than today. The local SQLite store + agent-shaped `context` command means one tool call replaces the back-and-forth of pasting yesterday's diary into the chat. Use it for nutrition coaching, deficit calibration, macro analysis, recall questions ('every time I logged X'), and anywhere the official MFP UI's per-meal rollup blocks the question.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`export csv`** — Export your food diary to CSV with one row per logged food, not per meal. Premium MFP only ships per-meal CSVs.

  _Reach for this when an agent needs the user's full eating history at food granularity for analysis, training, or long-form coaching memory._

  ```bash
  myfitnesspal-pp-cli export csv --from 2026-01-01 --to 2026-05-08 --out diary.csv
  ```
- **`analytics top-foods`** — Pareto query: which N foods drove X% of your protein/carbs/fat/fiber/sugar/calories over a window?

  _Use when an agent is helping the user understand what's actually driving a macro target, not what they think is._

  ```bash
  myfitnesspal-pp-cli analytics top-foods --nutrient protein --days 60 --cumulative-percent 80 --json
  ```
- **`find`** — Full-text search across every diary entry and food in the local store. Returns date, meal, servings, calories per match.

  _Use when an agent needs to recall every time the user logged a specific food without scrolling through months of diary._

  ```bash
  myfitnesspal-pp-cli find --food "Chipotle Bowl" --from 2026-01-01 --json
  ```
- **`analytics streak`** — Longest run of consecutive days where calorie totals fall within ±tolerance of your goal.

  _Use when the user asks how their adherence is trending — answer arrives without subjective interpretation._

  ```bash
  myfitnesspal-pp-cli analytics streak --days 60 --tolerance 0.05 --json
  ```

### Agent-native plumbing
- **`context`** — Single-call snapshot of the last N days: diary totals, weight trend, current goals, recent foods, macro deltas — sized for an agent context window.

  _First call any agent should make before reasoning about a user's nutrition — gives the full picture in one shot._

  ```bash
  myfitnesspal-pp-cli context --days 14 --json
  ```

## Command Reference

**api-user** — Authenticated user record on the v2 API (preferences, paid subs, profiles).

- `myfitnesspal-pp-cli api-user` — Get the v2 user record (units, goals preferences, paid subs, profiles).

**diary** — Daily food diary (per-meal entries with full nutrient panel).

- `myfitnesspal-pp-cli diary get-day` — Get one day's food diary as scraped HTML (legacy surface python-myfitnesspal uses).
- `myfitnesspal-pp-cli diary load-recent` — Load the recent-foods quick-pick list for a meal.

**exercise** — Cardio and strength exercises logged on a given day.

- `myfitnesspal-pp-cli exercise` — Get one day's exercise log (cardio + strength) as scraped HTML.

**food** — Search the public food database, view food details, log custom foods.

- `myfitnesspal-pp-cli food details` — Get full nutrient panel for a single food by MFP food id.
- `myfitnesspal-pp-cli food search` — Search the food database.
- `myfitnesspal-pp-cli food suggested-servings` — Get common serving-size suggestions for a food (powers the '1 cup / 100g / medium' picker).

**goals** — Daily calorie / macro / water / weight goals.

- `myfitnesspal-pp-cli goals` — Get your current daily goals (calorie target, macro split, water target) as scraped HTML.

**measurement** — Weight, body fat, and other body measurements (time series).

- `myfitnesspal-pp-cli measurement get-range` — Get a date range of values for one measurement type as scraped HTML.
- `myfitnesspal-pp-cli measurement types` — List the measurement types defined for your account (Weight, BodyFat, Neck, Waist, Hips, plus custom).

**note** — Free-text notes attached to a day's food or exercise diary.

- `myfitnesspal-pp-cli note` — Get the food note for a single day.

**reports** — Aggregated time-series reports (any nutrient or weight as a date->value series).

- `myfitnesspal-pp-cli reports` — Get a time-series report (e.g. nutrition/Net%20Calories/30 returns the last 30 days of net calories).

**user** — Authenticated user account info, units, and preferences.

- `myfitnesspal-pp-cli user auth-token` — Bootstrap a v2 bearer token from your session cookies.
- `myfitnesspal-pp-cli user top-foods-server` — Get your top-logged foods over a date range, computed server-side (powers the 'your most-eaten' insights).

**water** — Daily water intake tracking.

- `myfitnesspal-pp-cli water` — Get water intake for a single day.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
myfitnesspal-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Build agent context for a coaching question

```bash
myfitnesspal-pp-cli context --days 30 --json --select diary_totals,weight_trend,goals
```

One tool call gives the agent everything it needs to answer 'should I cut more this week?' without back-and-forth pasting.

### Find every time you logged a food

```bash
myfitnesspal-pp-cli find --food "Greek yogurt" --from 2026-01-01 --json --select date,meal,servings,calories
```

Local FTS over your synced diary; instant answer to recall questions the MFP web UI requires scrolling for.

### Per-food CSV for a quarter

```bash
myfitnesspal-pp-cli export csv --from 2026-01-01 --to 2026-03-31 --out q1-2026.csv
```

One row per food entry with full nutrients — the export premium MFP doesn't deliver.

### Top foods driving a macro

```bash
myfitnesspal-pp-cli analytics top-foods --nutrient protein --days 60 --cumulative-percent 0.8 --json
```

Pareto query over the local diary: which foods drove most of your protein over the last 60 days.

### Adherence streak

```bash
myfitnesspal-pp-cli analytics streak --days 60 --tolerance 0.05 --json
```

Longest run of days inside ±5% of your calorie goal. Goal value is read from the most recent `goal_snapshot` synced into the local store; pass `--goal-calories <value>` to override.

## Auth Setup

MyFitnessPal closed their public API. This CLI uses your logged-in browser session — log in to myfitnesspal.com in Chrome, then run `myfitnesspal-pp-cli auth login --chrome`. Cookies are read from the .myfitnesspal.com domain. Sessions usually last 7-30 days; when they expire, log in again in Chrome and re-run `auth login --chrome`.

Run `myfitnesspal-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  myfitnesspal-pp-cli api-user --user-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
myfitnesspal-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
myfitnesspal-pp-cli feedback --stdin < notes.txt
myfitnesspal-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.myfitnesspal-pp-cli/feedback.jsonl`. They are never POSTed unless `MYFITNESSPAL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MYFITNESSPAL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
myfitnesspal-pp-cli profile save briefing --json
myfitnesspal-pp-cli --profile briefing api-user --user-id 550e8400-e29b-41d4-a716-446655440000
myfitnesspal-pp-cli profile list --json
myfitnesspal-pp-cli profile show briefing
myfitnesspal-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `myfitnesspal-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/myfitnesspal/cmd/myfitnesspal-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add myfitnesspal-pp-mcp -- myfitnesspal-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which myfitnesspal-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   myfitnesspal-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `myfitnesspal-pp-cli <command> --help`.
