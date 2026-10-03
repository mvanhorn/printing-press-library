---
name: pp-wheelog
description: "Find and compare recorded accessibility facts with gaps and conflicting reports visible. Trigger phrases: `find WheeLog accessibility evidence`, `compare recorded toilet equipment`, `inspect a WheeLog public spot`, `check my saved accessibility shortlist`, `use wheelog`, `run wheelog-pp-cli`."
author: "Jet Sng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - wheelog-pp-cli
    install:
      - kind: go
        bins: [wheelog-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/wheelog/cmd/wheelog-pp-cli
---

# WheeLog — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `wheelog-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install wheelog --cli-only
   ```
2. Verify: `wheelog-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/wheelog/cmd/wheelog-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search public WheeLog spots, inspect category-specific equipment questions, and compare a bounded shortlist. Save normalized public-place evidence for offline lookup, recheck triage, straight-line proximity and exact observation changes.

## When to Use This CLI

Use WheeLog for public crowdsourced wheelchair-accessibility spot evidence, exact source question comparisons and a small offline public-facility shortlist. Japan is the initial travel use case; source keywords can cover other countries. Preserve report counts, unknowns and source record dates when describing results.

## Anti-triggers

Do not use this CLI for:
- Guaranteed accessible routes or personal wheelchair suitability.
- Measured dimensions, live opening conditions or per-report observation dates not supplied by the public source.
- Contributor profiles, personal TrackLogs, messages, posts, bookings or account changes.

## Evidence contract

Results retain source IDs, Japanese names, public facility addresses, category question labels, aggregate positive/negative counts and canonical WheeLog URLs. An affirmative report is a contributor report, not a guarantee of accessibility. `reported_affirmative`, `reported_negative`, `conflicting`, `unreported`, `counts_missing`, `inapplicable`, `not_checked` and `unavailable` remain distinct. Use `categories` to choose exact question IDs; a restroom question does not apply to an elevator.

`record_created_at` and `record_updated_at` are source record metadata. `retrieved_at` dates this CLI's observation. Missing update times remain unknown. `--from`/`--to` select record dates in the inclusive `--timezone` calendar window (default `Asia/Tokyo`); output echoes the UTC request. They do not select individual contributor report dates or trip availability.

Search scans at most five source pages and returns at most 50 records. Detail expansion and comparison are capped at five spots; coverage states how many records were listed, checked or unavailable. `--require-question` triggers real detail reads and returns an assessment rather than filtering away gaps. Keywords use the source's matching rules; an empty bounded result is not proof that no accessible facility exists.

Saved-only commands read existing evidence without migrations, table creation or permission changes. A missing shortlist is empty. Saved path aliases are resolved to the guarded target; multiply linked database files are rejected. Windows checks file attributes for link counts; unavailable link-count metadata fails closed. A non-empty WAL/rollback journal or a database change during the read returns `cache_visibility_unavailable`; close other database writers and retry. Auto discovery and inspection try the source first and read saved fallback only after a source failure; a failed cache fallback remains an explicit error. Saved reads use a private snapshot copied from a verified open descriptor (at most 64 MiB); SQL never reopens the selected pathname, and temporary snapshots are removed on completion or error. Refreshing or saving observations still needs a writable database. Source saves, refreshes and removals bind to the same canonical target and verify selected-path identity and a single hard link before opening and around commits on one reserved connection. Writable source-cache paths containing `?` or `#` are rejected before opening; saved-only reads support those names through escaped private-snapshot URIs.

The public contract supplies aggregate equipment answers rather than typed measured widths or slopes, individual report dates, current opening conditions or accessible routes. Read supplementary notes on the canonical source page. Contributor profiles, raw narratives, photos, comments and personal TrackLogs are excluded before cache and output.

`auto` prefers a fresh source read and labels saved fallback; `live` requires source requests; `local` uses saved evidence. `shortlist list` is always local. `categories` computes the recorded source catalog and rejects live mode. Save up to 50 selected public spots, retaining only the latest two normalized observations per spot. Retrieval-only differences do not count as source changes. Straight-line distances cover only saved facilities and do not establish a wheelchair route; the supplied origin is not stored.

## Unique Capabilities

These workflows combine verified public spot evidence with a bounded saved shortlist.

### Recorded evidence
- **`spots compare`** — Align requested source question IDs and report supporting, opposing, mixed, unknown and inapplicable evidence.

  _Choose this when comparing an explicit shortlist against source question evidence._

  ```bash
  wheelog-pp-cli spots compare 166345 166344 --require-question 102 --agent
  ```
- **`spots search`** — Expand a bounded keyword shortlist into actual detail evidence for requested questions.

  _Choose this when candidate search results need actual question counts before triage._

  ```bash
  wheelog-pp-cli spots search 成田空港 --category toilet --require-question 102 --limit 3 --agent
  ```

### Saved shortlist
- **`shortlist changes`** — Show exact changes between two allowlisted source observations for selected public places.

  _Choose this to inspect source evidence changes without asserting physical changes._

  ```bash
  wheelog-pp-cli shortlist changes --data-source local --agent
  ```
- **`shortlist list`** — Prioritize old, unknown or conflicting evidence in a saved shortlist with explicit reasons.

  _Choose this for saved-list maintenance and missing or conflicting question evidence._

  ```bash
  wheelog-pp-cli shortlist list --audit --require-question 102 --max-record-age 180d --agent
  ```
- **`shortlist list`** — Rank saved public facilities by straight-line distance from an explicit origin.

  _Choose this for offline proximity within the saved shortlist, with no route promise._

  ```bash
  wheelog-pp-cli shortlist list --origin 35.7742,140.3879 --radius-m 500 --category toilet --agent
  ```

## Discovery Signals

The public web application's anonymous search and detail requests were observed on 2026-10-03 and replayed over ordinary HTTPS. The supported source surface is POST-based read-only RPC with an explicit semantic success envelope. It requires no credentials, cookies or browser runtime. The CLI allowlists public-place fields before storage or output and stops with a contract error if the source envelope changes.

## Command Reference

**spots** — Public crowdsourced facility accessibility evidence

- `wheelog-pp-cli spots inspect` — Inspect allowlisted public accessibility question reports for a source spot ID.
- `wheelog-pp-cli spots search` — Search public spot records by source keyword, category, and evidence record dates.
- `wheelog-pp-cli spots compare` — Compare up to five selected IDs against exact source question IDs.
- `wheelog-pp-cli categories` — List exact category values and question-ID ranges.
- `wheelog-pp-cli shortlist save <id...>` — Save normalized public facility observations locally; no contributor or account writes.
- `wheelog-pp-cli shortlist list` — Read saved evidence, `--audit` recheck reasons or `--origin` straight-line distances.
- `wheelog-pp-cli shortlist changes` — Refresh a bounded selected set or compare its last saved transition with `--data-source local`.
- `wheelog-pp-cli shortlist remove <id>` — Remove local saved membership and evidence.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
wheelog-pp-cli which "recorded toilet equipment"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Dated restroom records

```bash
wheelog-pp-cli spots search 成田空港 --category toilet --from 2026-10-02 --to 2026-10-02 --timezone Asia/Tokyo --limit 3 --agent
```

Request the explicit Japanese record-date window; output echoes the UTC backend interval.

### Requested equipment comparison

```bash
wheelog-pp-cli spots compare 166345 166344 --require-question 102 --agent
```

Keep inapplicable, unreported, conflicting and unavailable evidence distinct.

### Small inspection output

```bash
wheelog-pp-cli spots inspect 166345 --agent --select id,name,questions
```

Return only selected public spot and question facts.

### Offline shortlist rechecks

```bash
wheelog-pp-cli shortlist list --audit --require-question 102 --data-source local --agent
```

Prioritize local evidence gaps without network requests.

## Auth Setup

The supported public spot surface works anonymously over ordinary HTTPS. No account, API key, cookies or resident browser is needed.

Run `wheelog-pp-cli doctor` to verify setup.

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
  wheelog-pp-cli spots search 成田空港 --agent --select results.id,results.name,coverage
  ```
- **Previewable** — `--dry-run` summarizes the intended action without executing the command
- **Non-interactive** — never prompts; inputs use flags or positional arguments
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `WHEELOG_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WHEELOG_CONFIG_DIR`, `WHEELOG_DATA_DIR`, `WHEELOG_STATE_DIR`, `WHEELOG_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WHEELOG_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `data.db` with saved public-facility evidence and optional learning rows. `state` contains invocation state and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Run `wheelog-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "wheelog": {
        "command": "wheelog-pp-mcp",
        "env": {
          "WHEELOG_HOME": "/srv/wheelog"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WHEELOG_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WHEELOG_HOME`, or `doctor` will not find saved evidence or state left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data — the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /tmp/wheelog-question.txt)
wheelog-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "wheelog-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `wheelog-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `wheelog-pp-cli learnings candidates` lists the full open set.

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
QUERY=$(cat /tmp/wheelog-question.txt)
wheelog-pp-cli teach --query "$QUERY" --resource-type spots --resource 166345 --resource 166344
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

Create `/tmp/wheelog-playbook.json` and `/tmp/wheelog-playbook-notes.md` with file-writing tools before running these examples. Use the JSON shape described below and privacy-scrubbed notes. The question file is created in Step 1.

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /tmp/wheelog-question.txt)
wheelog-pp-cli teach \
  --query "$QUERY" \
  --resource-type spots --resource 166345 \
  --playbook-file /tmp/wheelog-playbook.json \
  --playbook-notes-file /tmp/wheelog-playbook-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /tmp/wheelog-question.txt)
wheelog-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file /tmp/wheelog-playbook.json \
  --notes-file /tmp/wheelog-playbook-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /tmp/wheelog-question.txt)
NOTE=$(cat /path/to/note.txt)
wheelog-pp-cli playbook amend \
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

`wheelog-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `WHEELOG_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
wheelog-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
wheelog-pp-cli feedback --stdin < notes.txt
wheelog-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WHEELOG_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WHEELOG_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
wheelog-pp-cli profile save briefing --json
wheelog-pp-cli --profile briefing spots search
wheelog-pp-cli profile list --json
wheelog-pp-cli profile show briefing
wheelog-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `wheelog-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/wheelog/cmd/wheelog-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add wheelog-pp-mcp -- wheelog-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which wheelog-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   wheelog-pp-cli spots inspect 166345 --agent
   ```
4. If ambiguous, drill into subcommand help: `wheelog-pp-cli spots inspect --help`.
