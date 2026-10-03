---
name: pp-snowjapan
description: "Compare Japan ski areas with terrain facts, dated reports and explicit historical-season evidence. Trigger phrases: `compare Japan ski areas`, `check SnowJapan historical season dates`, `find ski resorts in Nagano`, `read a dated Hakuba snow observation`, `use SnowJapan`, `run snowjapan-pp-cli`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - snowjapan-pp-cli
    install:
      - kind: go
        bins: [snowjapan-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/snowjapan/cmd/snowjapan-pp-cli
---

# SnowJapan — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `snowjapan-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install snowjapan --cli-only
   ```
2. Verify: `snowjapan-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/snowjapan/cmd/snowjapan-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search and inspect public SnowJapan resort facts, compare a small shortlist, and read dated regional snow observations. Saved facts support tradeoff comparisons, municipality portfolios, historical span intersections and transparent evidence gaps.

## When to Use This CLI

Use for Japan ski-area discovery and factual shortlist comparisons, dated regional snow observations, and confirmed historical season evidence. Explicitly sync facts for offline tradeoffs, municipality portfolios and evidence-gap checks.

## Anti-triggers

Do not use this CLI for:
- Booking or buying lift tickets
- Live open-lift status or safety guarantees
- Predicting upcoming dates from past winters
- Republishing editorial reports

## Unique Capabilities

These local computations use explicitly saved SnowJapan facts.

### Resort decisions
- **`plan frontier`** — Show nondominated resort choices for explicitly selected statistics.

  _When a traveler wants to expose numeric tradeoffs across a shortlist._

  ```bash
  snowjapan-pp-cli plan frontier --prefecture Nagano --maximize vertical,courses,longest --limit 10 --agent
  ```
- **`plan towns`** — Compare source-defined towns by known resort options and statistical ranges.

  _When deciding which municipalities warrant further resort inspection._

  ```bash
  snowjapan-pp-cli plan towns --prefecture Nagano --season 2025-2026 --limit 10 --agent
  ```

### Historical evidence
- **`plan windows`** — Intersect a past trip window with recorded first/last season spans.

  _When exact historical date boundaries matter._

  ```bash
  snowjapan-pp-cli plan windows --season 2025-2026 --from 2026-03-28 --to 2026-04-05 --resorts able-hakuba-goryu --agent
  ```
- **`plan coverage`** — Find listed resorts with missing or ambiguous historical endpoint evidence.

  _When plans need explicit unknowns for a changed shortlist._

  ```bash
  snowjapan-pp-cli plan coverage --season 2025-2026 --prefecture Nagano --limit 20 --agent
  ```

### Saved observations
- **`plan changes`** — See factual field changes between the latest two locally captured observations.

  _After an explicit capture or sync, to inspect source edits without continuous monitoring._

  ```bash
  snowjapan-pp-cli plan changes --resorts able-hakuba-goryu --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 6 API entries from 6 total network entries
- Protocols: ssr_embedded_data (100% confidence)
- Auth signals: none
- Generation hints: Direct HTTP runtime; custom factual chart projection required; no browser transport or credentials.
- Caveats: custom_html_projection: Source HTML/JS-shaped data requires a bounded non-executing parser; ordinary generated HTML metadata is insufficient.

## Command Reference

**reports** — Dated regional snow observations at reporter base/town level and canonical report links.

- `snowjapan-pp-cli reports get` — Read one dated report's base-level snow figures; new snow means since the previous report.
- `snowjapan-pp-cli reports list` — List latest regional report metadata from the public homepage without report narratives.

**resorts** — Factual active-area directory and individual resort records; installed lifts are not current lift operations.

- `snowjapan-pp-cli resorts get` — Inspect one exact canonical resort path with Japanese name, ability, lift and update-status facts.
- `snowjapan-pp-cli resorts list` — List source-owned national factual chart records through the bounded chart adapter.
- `snowjapan-pp-cli resorts search` — Filter names, towns, prefectures, vertical and installed-lift counts.
- `snowjapan-pp-cli resorts compare <first> <second> [third] [fourth]` — Compare two to four exact resort IDs or unique slugs.
- `snowjapan-pp-cli sync` — Explicitly save source facts; use the exact requested winter parameter.
- `snowjapan-pp-cli search <term>` — Search saved factual rows by resource type.

**seasons** — Confirmed first and last dates of completed winters, not continuous operation or upcoming forecasts.

- `snowjapan-pp-cli seasons list` — List recorded historical season chart rows through the bounded chart adapter.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
snowjapan-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Cookbook

Local planners require an explicit directory capture and, for season queries, an explicit capture of that exact winter:

```bash
snowjapan-pp-cli sync --resources resorts
snowjapan-pp-cli sync --resources seasons --resource-param seasons:season=2025-2026
snowjapan-pp-cli plan frontier --prefecture Nagano --maximize vertical,courses,longest --limit 5 --agent
snowjapan-pp-cli plan coverage --season 2025-2026 --resorts able-hakuba-goryu,hakuba-happo-one --agent
```

A winter that has not been captured returns `season_not_captured`; the CLI does not infer missing source evidence from an empty local cache. Every full season sync replaces that winter's captured population, keeping other winters separate.

For detailed saved-fact changes, capture the same exact resort twice on separate observations, then compare its latest two compatible projections:

```bash
snowjapan-pp-cli sync --resources resorts --resorts nagano-prefecture/hakuba-village/able-hakuba-goryu
snowjapan-pp-cli plan changes --resorts able-hakuba-goryu --agent
```

With no compatible pair, output has `missing_baseline`. Repeating the explicit detailed capture supplies a second observation; unchanged facts produce an empty change list. The comparison selects the most recently saved compatible pair across catalog and detail projections, so a newer catalog pair is not hidden by older detail observations.

Offline `resorts get` and `reports get` require an exact detail capture. A list-only projection returns `detail_not_captured`, including a capture command, and cannot serve as automatic network fallback. To save a report’s numeric observations (up to four exact dated IDs):

```bash
snowjapan-pp-cli sync --resources reports --reports hakuba-now-1st-october-2026
snowjapan-pp-cli reports get hakuba-now-1st-october-2026 --data-source local --agent
```

Ordinary report sync saves list metadata. A later list sync can replace the mirror row; repeat the exact detail capture before offline inspection. Freshness hints use the actual saved observation times; a fresh partial capture does not refresh unrelated older records.

Close any active database writer before reading local facts. Existing WAL/SHM/journal files make local reads fail with a retry instruction. Reads and captures resolve symlink targets, pin their SQL connection and verify database identity; hard-linked databases and URI-sensitive literal filenames are rejected to avoid ambiguous journals or the wrong file. Sync writes use the canonical database path, including for supported symlink aliases. External replacement of the database file during a write is unsupported; detected retargeting or identity changes fail the capture.

## Source scope and evidence

Source fact rows preserve canonical URLs and observation times. Computed planners expose dataset source URLs and observed-time ranges in `.meta`; town summaries do not have individual resort permalinks. Resort statistics describe installed facilities, without live lift-operation claims. Detail `information_status` and `planned_window` are source labels; unconfirmed upcoming dates stay unconfirmed. A source update timestamp is not proof that every field was recently verified.

Historical winter rows record first and last dates and their inclusive calendar span. They do not establish uninterrupted daily operation. Separate access-base records, or conflicting source municipality rows, can share one resort URL; these remain separate rows and joins report ambiguity. Rows whose published dates fall outside the selected winter stay visible with `dates_outside_requested_winter`; planners count them as inconsistent evidence and never confirm a window from them. Town portfolios keep missing, ambiguous and inconsistent endpoint counts separate and do not sum shared terrain.

Dated report numbers describe the reporter's base/town. New snow means since the previous report, which may be more than 24 hours. Missing figures stay null; published zero stays zero. Report prose is not reproduced. October 2026 observations are preseason measurements.

Nationwide discovery supports resort name, municipality, prefecture and numeric facts. Popular-region membership is unavailable in the replayable source, so it is not a search filter. No forecasts, reservations, lift-ticket sales or safety assessments are provided. Requests, response bodies and scanned records are bounded; result limits are 1–200. HTTP and schema failures return errors, and network fallback is explicitly labeled as dated local data.

## Recipes

### Bound the shortlist

```bash
snowjapan-pp-cli resorts search --prefecture Nagano --limit 10 --agent --select results.name,results.vertical_m,results.source_url
```

Request a small factual projection.

### Inspect a dated observation

```bash
snowjapan-pp-cli reports get hakuba-now-1st-october-2026 --agent
```

Snowfall is measured at base/town since the prior report.

### Audit historical gaps

```bash
snowjapan-pp-cli plan coverage --season 2025-2026 --prefecture Nagano --limit 20 --agent
```

After syncing the directory and requested winter, missing and ambiguous endpoint records stay distinct; neither establishes closure.

### Compare past date boundaries

```bash
snowjapan-pp-cli plan windows --season 2025-2026 --from 2026-03-28 --to 2026-04-05 --resorts able-hakuba-goryu --agent
```

An overlap is a calendar span, with continuous operation unknown.

## Auth Setup

Public read-only HTML and provider-published charts; no account, API key or running browser is needed.

Run `snowjapan-pp-cli doctor` to verify setup.

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
  snowjapan-pp-cli reports list --agent --select results.id,results.region,results.report_date
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

- Use `--home <dir>` for one invocation, or set `SNOWJAPAN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SNOWJAPAN_CONFIG_DIR`, `SNOWJAPAN_DATA_DIR`, `SNOWJAPAN_STATE_DIR`, `SNOWJAPAN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SNOWJAPAN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains `config.json` and profiles. `data` contains the factual `data.db` mirror and local learning state. `state` contains persisted query state and `teach.log`. `cache` contains regenerable cache files.
- SnowJapan source access uses no credentials or cookies. Keep the companion CLI installed alongside the MCP server; factual tools use the same CLI adapters.
- Run `snowjapan-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "snowjapan": {
        "command": "snowjapan-pp-mcp",
        "env": {
          "SNOWJAPAN_HOME": "/srv/snowjapan"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SNOWJAPAN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SNOWJAPAN_HOME`, or `doctor` will not find credentials left under the former root.

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
snowjapan-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "snowjapan-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `snowjapan-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `snowjapan-pp-cli learnings candidates` lists the full open set.

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
snowjapan-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
snowjapan-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
snowjapan-pp-cli teach-playbook \
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
snowjapan-pp-cli playbook amend \
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

`snowjapan-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SNOWJAPAN_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
snowjapan-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
snowjapan-pp-cli feedback --stdin < notes.txt
snowjapan-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SNOWJAPAN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SNOWJAPAN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
snowjapan-pp-cli profile save briefing --json
snowjapan-pp-cli --profile briefing reports list
snowjapan-pp-cli profile list --json
snowjapan-pp-cli profile show briefing
snowjapan-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `snowjapan-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/snowjapan/cmd/snowjapan-pp-cli@latest
go install github.com/mvanhorn/printing-press-library/library/travel/snowjapan/cmd/snowjapan-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add snowjapan-pp-mcp -- snowjapan-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which snowjapan-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   snowjapan-pp-cli resorts search --query Hakuba --limit 5 --agent
   ```
4. If ambiguous, drill into subcommand help: `snowjapan-pp-cli resorts search --help`.
