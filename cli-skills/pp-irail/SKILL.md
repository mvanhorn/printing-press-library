---
name: pp-irail
description: "Every Belgian rail lookup the existing tools offer, plus transfer-risk, delay history and accessibility data that none of them have. Trigger phrases: `next train from Brussels-Central`, `when is my train to Ghent`, `is my train delayed`, `are there any rail disruptions in Belgium`, `plan a train journey from Leuven to Antwerp`, `will I make my transfer`, `use irail`, `run irail`."
author: "Olivier"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - irail-pp-cli
    install:
      - kind: go
        bins: [irail-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/irail/cmd/irail-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/irail/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# iRail — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `irail-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install irail --cli-only
   ```
2. Verify: `irail-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/irail/cmd/irail-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

iRail exposes live NMBS/SNCB departures, journey planning and disruptions for free with no API key. This CLI adds what every other client throws away: it records observations locally so punctuality can answer whether a train is chronically late, joins the open stations dataset so transfer-risk knows the real minimum transfer time at each station, and surfaces station accessibility data the API never returns. The analysis commands emit typed JSON - delays as numbers, cancellations as booleans - while the raw endpoint commands pass iRail's payload through unchanged.

## When to Use This CLI

Use this CLI for anything involving Belgian passenger rail: when the next train leaves, how to get from one station to another, whether a journey is disrupted, and how reliable a route has been over time. It is the right tool when you need typed, scriptable output rather than a web page, and the only tool that can answer historical punctuality or transfer-risk questions. It also serves accessibility questions about Belgian stations that the live API cannot answer.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to buy tickets, reserve seats or check fares; iRail is a read-only timetable API with no booking surface.
- Do not use it for rail networks outside Belgium; coverage is NMBS/SNCB plus a small number of cross-border stations.
- Do not use it for live vehicle GPS positions; the API reports scheduled and delayed times, not continuous tracking.
- Do not use it to query personal NMBS account data such as subscriptions or order history; that lives behind belgiantrain.be, which this CLI does not touch.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Live data joined with open datasets
- **`transfer-risk`** — Tells you whether the transfers in a journey still hold once today's delays are applied.

  _Reach for this instead of a plain route lookup whenever a journey has a transfer and delays are already in play._

  ```bash
  irail-pp-cli transfer-risk --from Oostende --to Hasselt --agent
  ```
- **`disruptions route`** — Filters the national disruption feed down to the stations your journey actually passes through.

  _Use this when the national list is too noisy to answer whether one specific trip is affected._

  ```bash
  irail-pp-cli disruptions route --from Ghent-Sint-Pieters --to Brussels-Central --agent
  ```
- **`stations facilities`** — Reports step-free access, elevators, ramps, lockers, bike parking and ticket-desk hours for a station.

  _Use this for accessibility and amenity questions that the live rail API simply cannot answer._

  ```bash
  irail-pp-cli stations facilities --station Ghent-Sint-Pieters --agent
  ```

### Local state that compounds
- **`punctuality`** — Shows how reliable a train or route has actually been, from delay observations recorded on your machine.

  _Use this for questions about the past such as chronic lateness; it never calls the API._

  ```bash
  irail-pp-cli punctuality --from Ghent-Sint-Pieters --to Brussels-Central --board-type route --agent
  ```
- **`observe`** — Records what the board says right now into local SQLite, building the history other commands read.

  _Run this on a schedule; it is what makes punctuality and changes able to answer anything._

  ```bash
  irail-pp-cli observe --station Brussels-Central
  ```
- **`changes`** — Reports new delays, cancellations and platform changes since the last time you looked.

  _Use this for deltas during a commute rather than re-reading a whole board._

  ```bash
  irail-pp-cli changes --station Brussels-Central --agent
  ```

### Time reasoning done properly
- **`leave-by`** — Answers the last train you can take and still arrive before a deadline, accounting for current delays.

  _Use this when the arrival deadline is fixed and the departure time is the unknown._

  ```bash
  irail-pp-cli leave-by --from Leuven --to Brussels-Central --arrive-by 09:00 --agent
  ```

## Command Reference

**board** — Live departure and arrival boards for a station

- `irail-pp-cli board` — Live departures (or arrivals) at a station, with delay, platform and occupancy

**disruptions** — Network-wide disruptions and planned engineering works

- `irail-pp-cli disruptions` — Current disruptions and planned works across the whole network

**logs** — Recent iRail API request log entries

- `irail-pp-cli logs` — Last request log entries. Note: upstream currently returns an empty list; bulk archives live at gtfs.irail.be/logs/

**route** — Journey planning between two stations, with transfers and live delay

- `irail-pp-cli route` — Plan a journey between two stations, including transfers and per-leg delay

**stations** — Belgian and cross-border stations served by NMBS/SNCB

- `irail-pp-cli stations` — List every station iRail knows about (716 incl. cross-border)

**train** — Individual trains: full stop trace and physical composition

- `irail-pp-cli train composition` — Physical make-up of a train: segments, units, and per-carriage facilities
- `irail-pp-cli train get` — Every stop of one train with live delay. Note: iRail ignores the date parameter


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
irail-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Next departures, narrowed for an agent

```bash
irail-pp-cli board --station Brussels-Central --agent --select vehicle,delay,platform,canceled
```

A full board is roughly 34 KB of JSON; selecting four fields keeps the response small enough to reason over without burning context.

### Will my transfer survive today's delays

```bash
irail-pp-cli transfer-risk --from Oostende --to Hasselt --agent
```

Joins live per-leg delay against each station's official minimum transfer time and flags transfers that no longer hold.

### Only the disruptions that affect my commute

```bash
irail-pp-cli disruptions route --from Ghent-Sint-Pieters --to Brussels-Central --agent
```

Filters the national feed, which was 32 entries on a normal day, down to the stations this journey passes through.

### Last train that still gets me there by nine

```bash
irail-pp-cli leave-by --from Leuven --to Brussels-Central --arrive-by 09:00 --agent
```

Plans backwards from the deadline and applies current delays plus a safety margin.

### Is this station step-free

```bash
irail-pp-cli stations facilities --station Ghent-Sint-Pieters --agent
```

Reads the open facilities dataset for elevators, ramps and wheelchair access, none of which the rail API returns.

## Auth Setup

No credentials are required: the iRail API is open and unauthenticated, so every command works immediately after install. Two operational limits matter instead of a key. iRail allows 3 requests per second per IP with 5 burst, returning HTTP 429 beyond that, so this CLI ships an adaptive rate limiter. iRail also blocks source IPs that send no User-Agent without prior warning, so every request carries an identifying User-Agent automatically.

Run `irail-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  irail-pp-cli board --station Ghent-Sint-Pieters --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `IRAIL_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `IRAIL_CONFIG_DIR`, `IRAIL_DATA_DIR`, `IRAIL_STATE_DIR`, `IRAIL_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `IRAIL_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `irail-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "irail": {
        "command": "irail-pp-mcp",
        "env": {
          "IRAIL_HOME": "/srv/irail"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `IRAIL_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `IRAIL_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
irail-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "irail-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `irail-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `irail-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `irail-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
irail-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
irail-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
irail-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
irail-pp-cli playbook amend \
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

`irail-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `IRAIL_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
irail-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
irail-pp-cli feedback --stdin < notes.txt
irail-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `IRAIL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IRAIL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
irail-pp-cli profile save briefing --json
irail-pp-cli --profile briefing board --station Ghent-Sint-Pieters
irail-pp-cli profile list --json
irail-pp-cli profile show briefing
irail-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `irail-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/irail/cmd/irail-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add irail-pp-mcp -- irail-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which irail-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   irail-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `irail-pp-cli <command> --help`.
