---
name: pp-garmin
description: "Your whole Garmin history in a local database, not one 28-day page at a time. Trigger phrases: `how did I sleep this month`, `sleep trend for the last 90 days`, `training load and time in heart rate zones`, `vo2 max trend`, `fill the garmin archive`, `use garmin connect`, `run garmin`."
author: "Prashant Kamani"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - garmin-pp-cli
    install:
      - kind: go
        bins: [garmin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/garmin/cmd/garmin-pp-cli
---

# Garmin Connect — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `garmin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install garmin --cli-only
   ```
2. Verify: `garmin-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/garmin/cmd/garmin-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Garmin Connect caps every daily-stats request at 28 calendar days and issues no personal API token, which is why its ecosystem is a handful of Python libraries rather than a tool an agent can call. `history` walks the whole account into a local SQLite archive, oldest day first, and `insights sleep` and `insights training` answer months-long sleep, training-load and heart-rate-zone questions from that archive with JSON on stdout. `auth login` keeps each household account in its own home and refuses to store a token that belongs to somebody else.

## When to Use This CLI

Reach for this CLI for questions about the owner's own Garmin Connect history: sleep and training trends, VO2 max, resting heart rate, step and intensity-minute totals, heart-rate zones, and the activity feed. It earns its keep on questions that span weeks or years, because those are answered from the local archive instead of from Garmin's 28-day pages. Do not reach for it before the archive is filled: an unfilled archive answers every trend question with nulls and a hint naming `history`, which is an honest answer but not the one the user asked for.

## Anti-triggers

Do not use this CLI for:
- Writing anything back to Garmin Connect. Every command here reads.
- Another household member's account from this home. Each account needs its own login in its own home.
- A live device readout. A metric appears here only after the watch or bike computer has synced to Garmin Connect.
- Strava, Apple Health, Whoop or Oura data, which live in their own services.

## Unique Capabilities

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

## Command Reference

**account** — Bootstrap and identity: social profile, unit settings, and the account email a login is checked against.

- `garmin-pp-cli account personal-information` — Returns the identity block for the signed-in account, including the account email.
- `garmin-pp-cli account settings` — Returns the account's settings.
- `garmin-pp-cli account social-profile` — Returns the account's public profile.

**activities** — The activity feed, per-activity detail and splits, lifetime breakdowns, and original file downloads.

- `garmin-pp-cli activities breakdown` — Server-side aggregation of every activity on the account into totals per parent activity type, by duration or distance.
- `garmin-pp-cli activities download-original` — Fetches the original recorded file for one activity as a ZIP containing the device FIT file.
- `garmin-pp-cli activities get` — Everything Garmin holds about one activity at summary level: type, timing, distance and elevation.
- `garmin-pp-cli activities hr-time-in-zones` — Seconds spent in each configured heart-rate zone during one activity.
- `garmin-pp-cli activities list` — The account's activity feed, one row per recorded activity, newest first.
- `garmin-pp-cli activities splits` — The lap or split breakdown of a single activity, each with its own distance, duration and averages.

**fitness** — Fitness level over time: VO2 max, max-met values, and Garmin's fitness-age estimate.

- `garmin-pp-cli fitness age` — Garmin's fitness-age estimate and the components it was computed from for a single date.
- `garmin-pp-cli fitness max-metrics` — Fitness-level trend: VO2 max, generic and cycling, plus Garmin's max-met value, one row per date.

**heart_rate** — Configured heart-rate zones, daily heart-rate detail, and per-activity time in zone.

- `garmin-pp-cli heart-rate daily` — The intraday heart-rate series for a single date plus the resting, minimum and maximum values Garmin derived from it.
- `garmin-pp-cli heart-rate daily-alt` — Identical payload to `daily`, on the wellness-service path that takes the date as a query parameter.
- `garmin-pp-cli heart-rate zones` — The zone boundaries the account has configured, per sport.

**sleep** — Sleep score trends, server-aggregated nightly summaries, and full per-night detail.

- `garmin-pp-cli sleep night` — Everything Garmin recorded for a single night: stage minutes, sleep windows, restlessness and the sleep-score breakdown.
- `garmin-pp-cli sleep night-alt` — Identical payload to `night`, on the sleep-service path that takes the date as a query parameter.
- `garmin-pp-cli sleep score-stats` — The sleep score trend: one row per calendar date with the score value and qualifier.
- `garmin-pp-cli sleep stats` — One row per night between `start` and `end`, aggregated by Garmin.

**steps** — Daily and weekly step totals against the account's goal.

- `garmin-pp-cli steps daily` — One row per calendar date with total steps, the step goal and the distance walked.
- `garmin-pp-cli steps weekly` — Weekly step buckets instead of daily ones, about a year of them in a single request.

**training** — Training status over a window and the daily training-readiness score.

- `garmin-pp-cli training readiness` — Garmin's training-readiness score for a single date, with the inputs it was built from.
- `garmin-pp-cli training status` — Training status with acute and chronic load, for the days ending on the given date.

**wellness** — Daily wellness roll-ups: the day summary, intensity minutes, resting heart rate, and hydration.

- `garmin-pp-cli wellness daily-summary` — The single-day roll-up: steps, floors, intensity minutes, calories, resting heart rate and stress.
- `garmin-pp-cli wellness hydration` — Water intake logged for a single date against the day's goal.
- `garmin-pp-cli wellness hydration-alt` — The hydration payload plus the activity sweat-loss and goal-adjustment fields.
- `garmin-pp-cli wellness intensity-minutes-weekly` — Weekly buckets of moderate and vigorous intensity minutes against the account's weekly goal.
- `garmin-pp-cli wellness metrics-daily` — A single named metric as a daily time series over a date range, selected by the numeric metric id.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
garmin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

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

## Auth Setup

Garmin issues no personal API key, so `garmin-pp-cli auth login --email you@example.com` is the way in: it signs the browser out of Garmin, opens Garmin's own sign-in page, catches the one-time ticket on a loopback port on 127.0.0.1, and then asks Garmin which account it just authenticated. If that is not the address you passed, nothing is written to disk. Your password never reaches this CLI, and the refresh token keeps the session alive afterwards without another browser visit. A token supplied in GARMIN_ACCESS_TOKEN or GARMIN_TOKEN is used as-is instead: never refreshed, and not this home's stored chain. One Garmin account per home; `auth status --verify` asks Garmin which one this home holds. `garmin-pp-cli auth logout` clears that home's stored chain.

Run `garmin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json` — one JSON document on stdout (sync progress events go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  garmin-pp-cli activities list --agent --select activityId,activityName,activityType
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `GARMIN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `GARMIN_CONFIG_DIR`, `GARMIN_DATA_DIR`, `GARMIN_STATE_DIR`, `GARMIN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `GARMIN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `garmin-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `GARMIN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `GARMIN_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
garmin-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "garmin-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `garmin-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `garmin-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `garmin-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
garmin-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
garmin-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
garmin-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
garmin-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between seasons, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The year-specific or entity-specific answer to the user's question. That's the response, not a learning.
- Per-team / per-athlete / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`garmin-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `GARMIN_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
garmin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
garmin-pp-cli feedback --stdin < notes.txt
garmin-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `GARMIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GARMIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
garmin-pp-cli profile save briefing --json
garmin-pp-cli --profile briefing activities list
garmin-pp-cli profile list --json
garmin-pp-cli profile show briefing
garmin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `garmin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/health/garmin/cmd/garmin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add garmin-pp-mcp -- garmin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which garmin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   garmin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `garmin-pp-cli <command> --help`.
