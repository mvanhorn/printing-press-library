---
name: pp-japan-theme-parks
description: "Compare park dates and ride waits for Tokyo Disney Resort, Universal Studios Japan and Fuji-Q from source-backed data. Trigger phrases: `is Tokyo DisneySea sold out on`, `Tokyo Disney ticket price for`, `USJ opening hours this month`, `DisneySea wait times now`, `USJ wait times now`, `use japan-theme-parks`, `run japan-theme-parks`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - japan-theme-parks-pp-cli
    install:
      - kind: go
        bins: [japan-theme-parks-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/cmd/japan-theme-parks-pp-cli
---

# Japan Theme Parks — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `japan-theme-parks-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install japan-theme-parks --cli-only
   ```
2. Verify: `japan-theme-parks-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.9 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/cmd/japan-theme-parks-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

`dates` shows Tokyo Disney Resort ticket sale status and price per ticket type with official TDR hours and third-party (ThemeParks.wiki) USJ and Fuji-Q hours for each date. `waits`, `snapshot` and `typical` turn Queue-Times live waits into a local history with honest sample sizes. Read-only: it never buys tickets.

## When to Use This CLI

Use this CLI to choose a Tokyo Disneyland or Tokyo DisneySea date from official per-ticket sale status, price and hours; to see Universal Studios Japan and Fuji-Q hours for about the next month (third-party ThemeParks.wiki data); and to compare live Queue-Times waits with waits you recorded yourself.

## Anti-triggers

Do not use this CLI for:
- Buying, holding or adding tickets to a cart
- USJ dated Studio Pass price or Express Pass availability (not public outside the queued web store)
- Crowd forecasts for future dates

## Unique Capabilities

### Pick the date
- **`dates`** — See, for each date, whether Tokyo Disneyland and DisneySea tickets are on sale, few left, sold out or not yet on sale, what each ticket costs, and the hours: official for TDL/TDS (published months only), third-party ThemeParks.wiki for USJ and Fuji-Q (about one month ahead).

  _Use it before recommending a park date; it gives sale status, price and hours with source fetch times instead of guesses. Beyond the sources' horizons it returns null with a reason._

  ```bash
  japan-theme-parks-pp-cli dates --from 2026-11-13 --to 2026-11-15 --agent
  ```

### Pick the rides
- **`typical`** — See median and p75 waits per ride by weekday and hour from your own recorded snapshots, with sample counts and date span.

  _Use it to compare weekdays or hours for specific rides without inventing a crowd forecast. It is empty until snapshot has run on a schedule; thin cells show insufficient._

  ```bash
  japan-theme-parks-pp-cli typical --park 275 --weekday sat --hour 10
  ```
- **`snapshot`** — Record the current waits for the Japan parks into a local history that later commands read.

  _Run it from your own scheduler (see the cron example in the README) so typical and waits have history._

  ```bash
  japan-theme-parks-pp-cli snapshot --park 274,275,284
  ```
- **`waits`** — On the park day, see live waits next to the usual wait for this weekday and hour from your own snapshots, with sample counts.

  _Use it on the park day to see which rides are below their usual wait; the usual-wait columns appear only after snapshot has recorded history for this weekday and hour._

  ```bash
  japan-theme-parks-pp-cli waits 275 --open-only --agent
  ```

## Command Reference

**parks**

- `japan-theme-parks-pp-cli parks` — The five Japan parks with JA/EN names, Queue-Times id, timezone, official site, crowd-calendar link and which sources cover hours and tickets.

**source** — Raw Queue-Times JSON (Powered by Queue-Times.com, https://queue-times.com/). Prefer parks, waits, dates, snapshot and typical for normalized output.

- `japan-theme-parks-pp-cli source parks` — List every Queue-Times park group and park (raw; prefer the parks command for Japan parks).
- `japan-theme-parks-pp-cli source queue-times` — Raw live queue times for one Queue-Times park id (prefer the waits command).


**dates**


- `japan-theme-parks-pp-cli dates` — See, for each date, whether Tokyo Disneyland and DisneySea tickets are on sale, few left, sold out or not yet on sale, what each ticket costs, and the opening hours for TDL, TDS, USJ and Fuji-Q.


**snapshot**


- `japan-theme-parks-pp-cli snapshot` — Record the current waits for the Japan parks into a local history that later commands read.


**typical**


- `japan-theme-parks-pp-cli typical` — See median and p75 waits per ride by weekday and hour from your own recorded snapshots, with sample counts and date span.


**waits**


- `japan-theme-parks-pp-cli waits` — On the park day, see live waits next to the usual wait for this weekday and hour from your own snapshots, with sample counts.




### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
japan-theme-parks-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Which DisneySea dates are still on sale

```bash
japan-theme-parks-pp-cli dates --park tds --from 2026-11-01 --to 2026-11-30 --status on-sale,few-left --agent --select results.date,results.tickets
```

Keep only DisneySea dates with a ticket on sale or few left; months TDR has not published yet return no rows.

### Record waits from cron

```bash
japan-theme-parks-pp-cli snapshot --park 274,275,284,337,285
```

Append one snapshot per park to the local history.

### Usual Saturday morning waits

```bash
japan-theme-parks-pp-cli typical --park 275 --weekday sat --hour 10
```

Median and p75 per ride with sample count and date span. Needs earlier snapshot runs; cells under --min-samples (default 6) show insufficient.

## Source credits and honesty rules

- When you show waits, say "Powered by Queue-Times.com" and link https://queue-times.com/.
- When you show USJ or Fuji-Q hours, say they come from ThemeParks.wiki (third-party, not official), say "Powered by ThemeParks.wiki" and link https://themeparks.wiki/. Do not redistribute or bulk-export that schedule.
- `null` values come with a reason field (`unknown_reason`, `tickets_unknown_reason`). Repeat the reason; do not fill in a guess.
- `sale_opens_at` with `observed: false` is the published TDR rule, not an observation.
- `typical` numbers are your local recorded samples, not a forecast. Always give `samples`, `distinct_days` and the first/last date.

## Auth Setup

No authentication required.

Run `japan-theme-parks-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, and `--deliver` paths:

- `--json` — one JSON document on stdout (warnings and hints go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  japan-theme-parks-pp-cli dates 2026-11-14 --park tds --agent --select results.date,results.one_day_passport_status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Local history** — `typical` and the usual-wait columns of `waits` read only the SQLite history that `snapshot` writes
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {
    "source": "live | local | computed | dry-run",
    "generated_at": "2026-10-09T02:33:36Z",
    "timezone": "Asia/Tokyo",
    "sources": [{"name": "queue-times.com", "url": "...", "fetched_at": "...", "attribution": "Powered by Queue-Times.com", "attribution_url": "https://queue-times.com/"}],
    "fetch_failures": [{"source": "api.themeparks.wiki", "error": "..."}],
    "notes": ["..."]
  },
  "results": "<command data>"
}
```

Parse `.results` for data, `.meta.source` to know whether it is live, local or computed, and `.meta.sources[]` for the fetch time and the credit to show. A non-empty `fetch_failures` means part of the data is missing (the matching fields are null with a reason). A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `JAPAN_THEME_PARKS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `JAPAN_THEME_PARKS_CONFIG_DIR`, `JAPAN_THEME_PARKS_DATA_DIR`, `JAPAN_THEME_PARKS_STATE_DIR`, `JAPAN_THEME_PARKS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `JAPAN_THEME_PARKS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains durable local data such as `data.db`. `state` contains the learning journal and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Run `japan-theme-parks-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "japan-theme-parks": {
        "command": "japan-theme-parks-pp-mcp",
        "env": {
          "JAPAN_THEME_PARKS_HOME": "/srv/japan-theme-parks"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `JAPAN_THEME_PARKS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `JAPAN_THEME_PARKS_HOME`, or `doctor` will not find files left under the former root.

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
japan-theme-parks-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "japan-theme-parks-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `japan-theme-parks-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `japan-theme-parks-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
japan-theme-parks-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
japan-theme-parks-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
japan-theme-parks-pp-cli teach-playbook \
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
japan-theme-parks-pp-cli playbook amend \
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

`japan-theme-parks-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `JAPAN_THEME_PARKS_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
japan-theme-parks-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
japan-theme-parks-pp-cli feedback --stdin < notes.txt
japan-theme-parks-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `JAPAN_THEME_PARKS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `JAPAN_THEME_PARKS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
japan-theme-parks-pp-cli profile save briefing --json
japan-theme-parks-pp-cli --profile briefing source parks
japan-theme-parks-pp-cli profile list --json
japan-theme-parks-pp-cli profile show briefing
japan-theme-parks-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited or blocked by a firewall or bot challenge (back off; do not retry immediately) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `japan-theme-parks-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/cmd/japan-theme-parks-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add japan-theme-parks-pp-mcp -- japan-theme-parks-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which japan-theme-parks-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   japan-theme-parks-pp-cli dates 2026-11-14 --park tds --agent
   ```
4. If ambiguous, drill into command help, for example `japan-theme-parks-pp-cli waits --help`.
