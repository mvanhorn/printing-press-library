---
name: pp-tiimo
description: "Your Tiimo plan, finally exportable, queryable, and yours. Trigger phrases: `what's on my Tiimo plan today`, `add a task to my to-do list`, `export my Tiimo data`, `how much free time do I have this week`, `which of my routines am I actually keeping`, `use tiimo`, `run tiimo`."
author: "Vincent Colombo"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tiimo-pp-cli
    install:
      - kind: go
        bins: [tiimo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/tiimo/cmd/tiimo-pp-cli
---

# Tiimo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tiimo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tiimo --cli-only
   ```
2. Verify: `tiimo-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/tiimo/cmd/tiimo-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Tiimo has no public API and declines to let you export your data or sync it back to a calendar. This CLI mirrors your own planner into local SQLite and gives you the surfaces the app will not: export to JSON, CSV, or iCalendar, a subscribable calendar feed, and analysis of what you planned versus what actually happened.

## When to Use This CLI

Use this CLI for any task involving a Tiimo user's schedule, routines, or to-do list: reading what is planned, capturing new tasks or activities, checking things off, and above all getting the data out of Tiimo. It is the only way to export Tiimo data or analyze planned-versus-actual behavior, because the product itself offers neither.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for generic calendar work that is not in Tiimo; use the calendar provider's own tooling.
- Do not use it to edit events that came from a linked external calendar - those are read-only in Tiimo and must be changed at the source.
- Do not use it for mood, energy, or focus-timer history; no API surface for those was found.
- Do not use it to reach Tiimo's AI Co-planner; that is a separate service this CLI does not wrap.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Your data, out of the app
- **`export`** — Stream a paginated resource straight to JSON or JSONL. Use backup for a whole-planner snapshot and feed for calendar or spreadsheet formats.

  _Reach for this to pull one resource as a stream; for everything at once use backup, and for a calendar file use feed._

  ```bash
  tiimo-pp-cli export profiles --format json
  ```
- **`feed`** — Generate a subscribable read-only iCalendar file of your Tiimo activities.

  _Use this to make Tiimo activities visible inside any calendar client without giving that client write access._

  ```bash
  tiimo-pp-cli feed --out ~/tiimo.ics --days 30
  ```
- **`backup`** — Write every mirrored record - activities, to-dos, tags, routines, calendars - to one portable JSON snapshot. This is the whole-planner export Tiimo does not offer.

  _This is the command to reach for when the user says they want their Tiimo data out. Use it before any bulk change, or on a schedule._

  ```bash
  tiimo-pp-cli backup --out ~/tiimo-backups
  ```

### Plan versus reality
- **`drift`** — Show which activities consistently start late, overrun, or spend the most time paused.

  _Reach for this when the user asks why their schedule never holds, or which task always blows its estimate._

  ```bash
  tiimo-pp-cli drift --days 30 --agent
  ```
- **`stalls`** — Find the exact checklist step where a multi-step routine tends to break down.

  _Use when a routine keeps failing and the user does not know which step is the blocker._

  ```bash
  tiimo-pp-cli stalls --weeks 8
  ```
- **`adherence`** — Completion rate for each recurring activity over a window.

  _Use to answer how reliably a habit is actually being kept, rather than whether it exists on the calendar._

  ```bash
  tiimo-pp-cli adherence --weeks 4 --agent
  ```

### Timeline intelligence
- **`overlaps`** — List activities that are double-booked against each other.

  _Use before committing to a plan, or when the user suspects their day is over-scheduled._

  ```bash
  tiimo-pp-cli overlaps --from 2026-08-14 --to 2026-08-21
  ```
- **`gaps`** — Find the unscheduled windows in a day that are at least a given length.

  _Use when fitting something new into an existing day without opening the app._

  ```bash
  tiimo-pp-cli gaps --min 30m --from 2026-08-14
  ```
- **`rolling`** — A true rolling multi-day view starting from today.

  _Use for a forward-looking view that does not reset at an arbitrary week boundary._

  ```bash
  tiimo-pp-cli rolling --days 7
  ```
- **`capacity`** — Committed versus free minutes per day, broken down by time-of-day bucket.

  _Use to judge whether a day has real room left before agreeing to add something to it._

  ```bash
  tiimo-pp-cli capacity --from 2026-08-14 --to 2026-08-21 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 15 API entries from 15 total network entries
- Protocols: rest_json (75% confidence)
- Auth signals: oauth2_bearer
- Candidate command ideas: create_activities — Derived from observed POST /api/profiles/{profile_id}/activities traffic.; delete_activities — Derived from observed DELETE /api/profiles/{profile_id}/activities/{activity_id} traffic.; get_activities — Derived from observed GET /api/externalCalendar/profiles/{profile_id}/externalCalendars/{externalcalendar_id}/activities traffic.; get_linkedCalendars — Derived from observed GET /api/externalCalendar/profiles/{profile_id}/linkedCalendars traffic.; get_profiles — Derived from observed GET /api/profiles/{profile_id} traffic.; get_routines — Derived from observed GET /api/profiles/{profile_id}/routines traffic.; get_tags — Derived from observed GET /api/profiles/{profile_id}/tags traffic.; get_todo_task_lists — Derived from observed GET /api/profiles/{profile_id}/todo-task-lists traffic.
- Caveats: empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

## Command Reference

**activities** — Timeline activities - the scheduled items that make up a day

- `tiimo-pp-cli activities create` — Create an activity on the timeline
- `tiimo-pp-cli activities delete` — Delete an activity
- `tiimo-pp-cli activities get` — Get a single activity by ID
- `tiimo-pp-cli activities list` — List activities in a date window, keyed by day
- `tiimo-pp-cli activities update` — Replace an activity. The API rejects PATCH with 405, so send the full object.

**calendars** — External calendars linked into Tiimo

- `tiimo-pp-cli calendars activities` — List imported activities from one linked calendar. These are read-only in Tiimo.
- `tiimo-pp-cli calendars list` — List linked external calendars

**configuration** — Server-side feature configuration

- `tiimo-pp-cli configuration` — Feature flags, locked features, and client configuration

**profiles** — Tiimo profiles (an account can hold several shared profiles)

- `tiimo-pp-cli profiles get` — Get a single profile
- `tiimo-pp-cli profiles list` — List every profile on the account

**routines** — Saved routines

- `tiimo-pp-cli routines <profile_id>` — List routines in a date window

**tags** — User-defined tags applied to activities and tasks

- `tiimo-pp-cli tags <profile_id>` — List tags

**todo_lists** — To-do lists that group tasks

- `tiimo-pp-cli todo-lists <profile_id>` — List to-do lists with their tasks

**todo_tasks** — To-do tasks - the priority-bucketed list, separate from the timeline

- `tiimo-pp-cli todo-tasks create` — Create a to-do task. Returns 200 with the created task.
- `tiimo-pp-cli todo-tasks delete` — Delete a to-do task
- `tiimo-pp-cli todo-tasks get` — Get a single to-do task
- `tiimo-pp-cli todo-tasks list` — List to-do tasks
- `tiimo-pp-cli todo-tasks update` — Update a to-do task. Note the API updates via the COLLECTION path; item-level PUT returns 405.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `TIIMO_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `tiimo-pp-cli profiles`
- `tiimo-pp-cli profiles get`
- `tiimo-pp-cli profiles list`
- `tiimo-pp-cli profiles search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tiimo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Capture a task, or a whole brain dump

```bash
tiimo-pp-cli todo add "book dentist"
```

Capture is the whole point for the target user, so the CLI takes one task per line on stdin. Pipe into it (printf 'call pharmacy\nbook dentist\n' | tiimo-pp-cli todo add --stdin) or redirect a file with < tasks.txt; blank lines are skipped and each line becomes its own task.

### Narrow a deeply nested activity payload for an agent

```bash
tiimo-pp-cli agenda --from 2026-08-14 --to 2026-08-21 --agent --select title,startTime,duration,completedAt,checklist.isCompleted
```

Activities carry dozens of fields including nested checklists and recurrence; selecting a handful keeps the response small enough to reason over.

### Find a free hour this week

```bash
tiimo-pp-cli gaps --min 60m --from 2026-08-14 --to 2026-08-21
```

Returns the unscheduled windows long enough to actually use, without opening the app.

### Publish a calendar feed your other tools can subscribe to

```bash
tiimo-pp-cli feed --out ~/Public/tiimo.ics --days 60
```

Produces the read-only calendar view Tiimo users asked for and were told would not be built.

### See which routine step keeps breaking

```bash
tiimo-pp-cli stalls --weeks 8 --agent
```

Aggregates per-step checklist completion so the failure point is a fact rather than a guess.

## Auth Setup

Tiimo authenticates through OpenID Connect at auth.tiimoapp.com. The simplest path is to set TIIMO_TOKEN to a bearer token for the tiimo_webapi scope; run `tiimo-pp-cli doctor` to confirm the token is accepted. Tokens are short-lived, so store a refresh token when available and let the CLI renew it.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tiimo-pp-cli activities list mock-value --from-date 2026-01-15 --agent --select date,activities
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

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

- Use `--home <dir>` for one invocation, or set `TIIMO_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TIIMO_CONFIG_DIR`, `TIIMO_DATA_DIR`, `TIIMO_STATE_DIR`, `TIIMO_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TIIMO_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `tiimo-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "tiimo": {
        "command": "tiimo-pp-mcp",
        "env": {
          "TIIMO_HOME": "/srv/tiimo"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TIIMO_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TIIMO_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
tiimo-pp-cli recall "<user's question>" --agent
```

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
      "next_action": ["<trial command>", "tiimo-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `tiimo-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `tiimo-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `tiimo-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
tiimo-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
tiimo-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
tiimo-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
tiimo-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
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

`tiimo-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `TIIMO_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tiimo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tiimo-pp-cli feedback --stdin < notes.txt
tiimo-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TIIMO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TIIMO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
tiimo-pp-cli profile save briefing --json
tiimo-pp-cli --profile briefing activities list mock-value --from-date 2026-01-15
tiimo-pp-cli profile list --json
tiimo-pp-cli profile show briefing
tiimo-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `tiimo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/tiimo/cmd/tiimo-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tiimo-pp-mcp -- tiimo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tiimo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tiimo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tiimo-pp-cli <command> --help`.
