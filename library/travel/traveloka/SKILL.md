---
name: pp-traveloka
description: "Research dated flights and hotel stays with source price bases, complete itineraries and comparable quote snapshots. Trigger phrases: `Compare Traveloka flights for these dates`, `Find Traveloka hotel rates for this party`, `Inspect this Traveloka room cancellation policy`, `Compare two Traveloka quote snapshots`, `use Traveloka`, `run Traveloka`."
author: "Jet Sng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - traveloka-pp-cli
    install:
      - kind: go
        bins: [traveloka-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/traveloka/cmd/traveloka-pp-cli
---

# Traveloka — Printing Press CLI

## Local delivery and scoped session

From the delivered Traveloka project directory, build the local binary with Go 1.26.6 or newer and make this directory visible on PATH:

```bash
go build -o traveloka-pp-cli ./cmd/traveloka-pp-cli
export PATH="$PWD:$PATH"
traveloka-pp-cli --version
```

Continue only when the local binary resolves. Consumer access needs the scoped guest-session workflow in Auth Setup below.

The catalog installer reference below applies after a separate public release. For this local delivery, use the built binary above.

## Prerequisites: Install the CLI

This skill drives the `traveloka-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install traveloka --cli-only
   ```
2. Verify: `traveloka-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/traveloka/cmd/traveloka-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Retrieve Traveloka flight and hotel offers with explicit dates, party, market and currency. Compare selected dates and source-backed snapshots while retaining itinerary details, room policies, price units and canonical booking links.

## When to Use This CLI

Use this CLI for explicit-date Traveloka flight and hotel research, detailed room and itinerary inspection, and canonical booking handoff. Use date grids for a small list of acceptable dates and snapshot commands for reproducible comparisons of already retrieved offers.

## Anti-triggers

Do not use this CLI for:
- Booking, paying, cancelling or modifying a reservation
- Account/profile changes or loyalty operations
- CAPTCHA bypass, proxy rotation or synthetic clearance tokens
- Tours, buses, packages or other Traveloka products
- Guaranteed final ticket prices or inferred sold-out inventory

## Unique Capabilities

These five commands compare explicit dates or retrieved Traveloka offers.

### Explicit date comparisons
- **`flights date-grid`** — Compare retrieved trip totals across a bounded list of explicit departure and return dates.

  _Use this when the traveller can choose among specific dates and needs fresh, comparable offers._

  ```bash
  traveloka-pp-cli flights date-grid --origin SIN --destination CGK --depart-dates 2026-11-20,2026-11-21 --limit 2 --agent
  ```
- **`hotels date-grid`** — Compare source stay totals for one property across equal-length stays with fixed occupancy.

  _Use this to compare specific alternative stays without assuming identical rooms are available._

  ```bash
  traveloka-pp-cli hotels date-grid --property-id 9000000001714 --stays 2027-01-06:2027-01-08,2027-01-13:2027-01-15 --adults 2 --rooms 1 --limit 2 --agent
  ```

### Retrieved offer comparisons
- **`flights shortlist`** — Keep retrieved flight offers that trade price against stops and elapsed time.

  _Use this to reduce an existing search to source-backed trade-offs without a subjective score. Offers with missing price, stops or duration remain in unknown_dimensions._

  ```bash
  traveloka-pp-cli flights shortlist --snapshot /private/tmp/traveloka-flight.json --agent
  ```
- **`hotels flexibility`** — See the stay-price difference between comparable cancellable and nonrefundable room rates.

  _Use this when choosing a rate plan for the same room and stay. Unknown or unmatched policies remain unpaired; zero comparable cancellation pairs is a valid result._

  ```bash
  traveloka-pp-cli hotels flexibility --snapshot /private/tmp/traveloka-rooms.json --agent
  ```

### Retrieval history
- **`quotes diff`** — Compare matched-offer price and policy changes between two explicit retrieval snapshots.

  _Use this to explain changes between two actual searches without inferring sold-out inventory._

  ```bash
  traveloka-pp-cli quotes diff --before /private/tmp/traveloka-flight-before.json --after /private/tmp/traveloka-flight-after.json --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 15 API entries from 15 total network entries
- Protocols: rest_json (75% confidence)
- Auth signals: browser-clearance-session
- The delivered source operations are the eight read-only airport, flight and hotel endpoints listed above; shopper commands orchestrate these calls.
- Advanced source commands require explicit operation-specific JSON through `--data` or a bounded `--data-file` object (maximum 2 MiB). These inputs contain public search fields and fresh source IDs; cookies and request-token envelopes remain in the private session. `--data-file` cannot be combined with `--data` or `--stdin`.

## Command Reference

**airport** — Advanced read-only Traveloka airport source operations. Clean resolve/flights/hotels commands orchestrate the shopper workflows.

- `traveloka-pp-cli airport` — Resolve ranked airport/city matches; prefer the resolve command.

**flight** — Advanced read-only Traveloka flight source operations. Clean resolve/flights/hotels commands orchestrate the shopper workflows.

- `traveloka-pp-cli flight initial` — Start a dated flight search; prefer flights search for the complete workflow.
- `traveloka-pp-cli flight poll` — Read incremental flight inventory or selected-outbound return options.
- `traveloka-pp-cli flight prefetch` — Read source-confirmed prices for selected flight journeys; search preparation only.

**hotel** — Advanced read-only Traveloka hotel source operations. Clean resolve/flights/hotels commands orchestrate the shopper workflows.

- `traveloka-pp-cli hotel catalog` — Read dated property inventory; prefer hotels search.
- `traveloka-pp-cli hotel features` — Read hotel autocomplete feature metadata.
- `traveloka-pp-cli hotel lookup` — Resolve ranked hotel destinations and properties; prefer resolve.
- `traveloka-pp-cli hotel rooms` — Read dated room/rate plans; prefer hotels rooms.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
traveloka-pp-cli which "compare flight dates"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Complete return flights

```bash
traveloka-pp-cli flights search --origin SIN --destination CGK --depart 2026-11-20 --return-date 2026-11-27 --adults 1 --limit 3 --agent --select offers,query,retrieved_at
```

Return both itineraries with authoritative combined totals and the matched shopper context.

### Explicit flight dates

```bash
traveloka-pp-cli flights date-grid --origin SIN --destination CGK --depart-dates 2026-11-20,2026-11-21 --limit 2 --agent
```

Run a bounded date comparison with an outcome for every requested cell.

### Hotel stay alternatives

```bash
traveloka-pp-cli hotels date-grid --property-id 9000000001714 --stays 2027-01-06:2027-01-08,2027-01-13:2027-01-15 --adults 2 --rooms 1 --limit 2 --agent
```

Keep occupancy fixed while retaining returned room and rate identities.

### Same-room cancellation price difference

```bash
traveloka-pp-cli hotels flexibility --snapshot /private/tmp/traveloka-rooms.json --agent
```

Pair explicit policies only when room, stay, meal, occupancy and payment match. Unknown or unmatched rates remain unpaired, and no comparable pairs is a valid result.

## Auth Setup

Consumer searches require a legitimate anonymous Traveloka browser session. Run auth capture --launch --timeout 2m with an already installed browser-use backend, or auth import-session with Traveloka-only cookie and captured-request JSON files. auth capture without --launch prints its plan. Capture closes its task guest browser before POST airport validation; ordinary searches replay directly over HTTP. Select the private mode-0600 session file with TRAVELOKA_SESSION_FILE or --session-file. Protection/expiry requires explicit normal-browser refresh. Airport validation proves that endpoint at that time; actual flight/hotel access is checked by the intended query. No partner API key, account change, CAPTCHA solving or global installation is performed.

Run `traveloka-pp-cli doctor` to verify setup.

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
  traveloka-pp-cli resolve --query Singapore --kind airport --agent --select locations
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `TRAVELOKA_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TRAVELOKA_CONFIG_DIR`, `TRAVELOKA_DATA_DIR`, `TRAVELOKA_STATE_DIR`, `TRAVELOKA_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TRAVELOKA_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Traveloka cookie and captured-request secrets live in the private mode-0600 session JSON, defaulting to `traveloka-session.json` under the resolved state dir. Select another file with `--session-file` or `TRAVELOKA_SESSION_FILE`. Generic credential compatibility uses `credentials.toml` under the data dir; legacy `config.toml` secrets leave that file on the first generic auth write.
- Run `traveloka-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "traveloka": {
        "command": "traveloka-pp-mcp",
        "env": {
          "TRAVELOKA_HOME": "/srv/traveloka"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TRAVELOKA_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TRAVELOKA_HOME`, or `doctor` will not find credentials left under the former root.

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
traveloka-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "traveloka-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `traveloka-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm SIN` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject SIN` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `traveloka-pp-cli learnings candidates` lists the full open set.

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
traveloka-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
traveloka-pp-cli teach \
  --query "$QUERY" \
  --resource-type <type> \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
traveloka-pp-cli teach-playbook \
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
traveloka-pp-cli playbook amend \
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

`traveloka-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `TRAVELOKA_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
traveloka-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
traveloka-pp-cli feedback --stdin < notes.txt
traveloka-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TRAVELOKA_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TRAVELOKA_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Commands that render results accept the global `--deliver` flag. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
traveloka-pp-cli profile save briefing --json
traveloka-pp-cli --profile briefing resolve --query Singapore --kind airport
traveloka-pp-cli profile list --json
traveloka-pp-cli profile show briefing
traveloka-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required or source access blocked |
| 5 | API error (upstream issue) |
| 6 | Local-file error |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `traveloka-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Build `cmd/traveloka-pp-mcp` from this directory and configure the resulting local executable as a stdio MCP server. Set `TRAVELOKA_SESSION_FILE` to the private imported session path in the host's environment. Its tools mirror the current Cobra command tree. Session capture launches a visible browser only with explicit `--launch`; perform that setup in the CLI before read-only research through MCP.

## Direct Use

1. Check if installed: `which traveloka-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   traveloka-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `traveloka-pp-cli flights search --help`.
