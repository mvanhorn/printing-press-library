---
name: pp-ticketmaster
description: "Every Discovery v2 endpoint plus offline search, multi-venue watchlists, residency dedup, and on-sale tracking no API call exposes. Trigger phrases: `what concerts in <city> this weekend`, `what's playing at <venue>`, `where is <artist> playing`, `presale watch`, `ticketmaster events`, `use ticketmaster`, `run ticketmaster`."
author: "Omar Shahine"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ticketmaster-pp-cli
    install:
      - kind: go
        bins: [ticketmaster-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/cmd/ticketmaster-pp-cli
---

# Ticketmaster — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ticketmaster-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ticketmaster --cli-only
   ```
2. Verify: `ticketmaster-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/cmd/ticketmaster-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

ticketmaster-pp-cli is the first single-binary CLI for the Ticketmaster Discovery API. It absorbs the full read-only surface (events, venues, attractions, classifications, suggest) and adds a local SQLite store with FTS search, named watchlists, residency collapse, tour-view with on-sale flags, and markdown briefs — the workflows real users built scripts to handle.

## When to Use This CLI

Reach for this CLI when a user asks 'what's on at <venue/metro>', 'where is <artist> playing', or 'what concerts this weekend'. Best for repeat queries against curated venue/artist watchlists (offline FTS shines here), residency-heavy venues (opera, Broadway, comedy), and agent contexts where compact JSON output keeps token usage low. Skip when checkout/purchase is needed — that requires the Commerce API, which this CLI does not cover.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local-store aggregations
- **`events upcoming`** — Fan out across a venue ID file or list and return one merged, deduplicated, date-sorted event list — the watchlist primitive behind any curated 'what's on at my venues' workflow.

  _When the user has a curated list of venues they care about and wants one merged feed; replaces hand-rolled per-venue fan-out scripts._

  ```bash
  ticketmaster-pp-cli events upcoming --venue-ids KovZ917Ahkk,KovZpZAFkvEA --days 60 --json
  ```
- **`events residency`** — Collapse runs of same-name + same-venue events into one row per residency with first_date, last_date, night_count, and id_list — so a 16-night opera season shows as one entry, not 16.

  _When listing upcoming events would otherwise show many near-duplicate rows for Broadway tours, opera seasons, or comedy residencies._

  ```bash
  ticketmaster-pp-cli events residency --window 28 --json
  ```
- **`events by-classification`** — Local join of events × classifications, grouped by segment and genre, with event count and three example events per leaf — the bucketed view newsletter authors and local-scene trackers reach for.

  _When summarizing 'what's on this month' broken down by music vs theatre vs comedy vs sports._

  ```bash
  ticketmaster-pp-cli events by-classification --dma 383 --window 60 --json
  ```
- **`events watchlist`** — Save, list, run, and remove named filter sets (venue IDs, attraction IDs, segments, DMA IDs) that persist across runs in the local SQLite store — the generic primitive any curated 'my venues' workflow composes from.

  _When the same curated venue/artist/genre filter recurs across many queries._

  ```bash
  ticketmaster-pp-cli events watchlist save seattle --venue-ids KovZ917Ahkk,KovZpZAFkvEA,KovZpZA1klkA
  ```
- **`events price-bands`** — Bucket events by priceRanges.min into <$50 / $50-100 / $100-200 / $200+ bands and report count + sample events per band, grouped by classification.

  _When the user wants to know where the affordable shows are this month, or how a venue's pricing skews._

  ```bash
  ticketmaster-pp-cli events price-bands --dma 383 --window 30 --json
  ```

### Tour & on-sale tracking
- **`events tour`** — For a given attraction (artist/team/touring show), return every upcoming event sorted by date, with city, venue, on-sale status, and a flag for events going on-sale within 7 days.

  _When tracking an artist across cities or watching for presale windows._

  ```bash
  ticketmaster-pp-cli events tour KovZ917Ahkk --on-sale-window 7 --json
  ```
- **`events on-sale-soon`** — Local query for events whose public on-sale falls in the next N days, sorted ascending — the canonical 'presale watch' view that no API endpoint provides.

  _When the user wants to be alerted to upcoming on-sale dates without polling each artist manually._

  ```bash
  ticketmaster-pp-cli events on-sale-soon --window 7 --classification rock --json
  ```

### Agent-native plumbing
- **`events dedup`** — Read an event JSON array from stdin or the local store, apply a deduplication strategy (name+venue+date, or tour-leg), and write the deduped stream to stdout — composes with any upstream command.

  _When merging results from multiple queries or sources and the duplicates need to be removed before agent processing._

  ```bash
  ticketmaster-pp-cli events list --keyword phish --json | ticketmaster-pp-cli events dedup --strategy tour-leg
  ```
- **`events brief`** — Render a markdown 'what's on' report grouped by night → venue → events with classification labels and price bands, suitable for newsletter, Obsidian, iMessage, or agent context.

  _When the user needs a paste-ready event summary for a chat thread, newsletter, or LLM context._

  ```bash
  ticketmaster-pp-cli events brief --dma 383 --window 7
  ```

## Command Reference

**attractions** — Manage attractions

- `ticketmaster-pp-cli attractions find` — Find attractions (artists, sports, packages, plays and so on) and filter your search by name, and much more.
- `ticketmaster-pp-cli attractions get` — Get details for a specific attraction using the unique identifier for the attraction.

**classifications** — Manage classifications

- `ticketmaster-pp-cli classifications get` — Get details for a specific segment, genre, or sub-genre using its unique identifier.
- `ticketmaster-pp-cli classifications get-genre` — Get details for a specific genre using its unique identifier.
- `ticketmaster-pp-cli classifications get-segment` — Get details for a specific segment using its unique identifier.
- `ticketmaster-pp-cli classifications get-subgenre` — Get details for a specific sub-genre using its unique identifier.
- `ticketmaster-pp-cli classifications list` — Find classifications and filter your search by name, and much more.

**events** — Manage events

- `ticketmaster-pp-cli events get` — Get details for a specific event using the unique identifier for the event.
- `ticketmaster-pp-cli events list` — Find events and filter your search by location, date, availability, and much more.

**suggest** — Manage suggest

- `ticketmaster-pp-cli suggest` — Find search suggestions and filter your suggestions by location, source, etc.

**venues** — Manage venues

- `ticketmaster-pp-cli venues get` — Get details for a specific venue using the unique identifier for the venue.
- `ticketmaster-pp-cli venues list` — Find venues and filter your search by name, and much more.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ticketmaster-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Seattle watchlist composition (no Seattle-specific code in the CLI)

```bash
ticketmaster-pp-cli events watchlist save seattle --venue-ids KovZ917Ahkk,KovZpZAFkvEA,KovZpZA1klkA,KovZpZAFEdeA,KovZpZAFkv1A
```

Save a named watchlist of Seattle venue IDs. Then run "events watchlist run seattle --days 60 --json" to apply the filter across upcoming events; pipe through "events dedup --strategy name-venue-date" for the final cleaned feed. Replaces a 595-line bash script.

### Track an artist across all upcoming dates with on-sale flags

```bash
ticketmaster-pp-cli events tour KovZ917Ahkk --on-sale-window 7 --json --select 'name,dates.start.localDate,_embedded.venues[0].name,_embedded.venues[0].city.name,sales.public.startDateTime'
```

Returns every upcoming tour stop with city, venue, and a flag for stops going on-sale within a week. The --select narrows the deeply-nested Discovery payload to just what an agent needs.

### Weekend brief for a metro

```bash
ticketmaster-pp-cli events brief --dma 383 --window 3 --classification music
```

Render a markdown brief of the next 3 days of music events in Seattle-Tacoma (DMA 383) - paste-ready for Obsidian or an iMessage thread.

### On-sale watch for rock shows

```bash
ticketmaster-pp-cli events on-sale-soon --window 14 --classification rock --json
```

Two-week-out scan for rock events going on public sale; pipe to an alerting script.

## Auth Setup

Authentication is a single Ticketmaster Discovery API consumer key, passed as the `apikey` query parameter on every request. Register at https://developer-acct.ticketmaster.com and copy the Consumer Key from your My Apps dashboard. Set TICKETMASTER_API_KEY in your shell environment. The free tier allows 5000 requests/day at 5/second.

Run `ticketmaster-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ticketmaster-pp-cli attractions get mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `TICKETMASTER_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TICKETMASTER_CONFIG_DIR`, `TICKETMASTER_DATA_DIR`, `TICKETMASTER_STATE_DIR`, `TICKETMASTER_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TICKETMASTER_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `ticketmaster-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "ticketmaster": {
        "command": "ticketmaster-pp-mcp",
        "env": {
          "TICKETMASTER_HOME": "/srv/ticketmaster"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TICKETMASTER_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TICKETMASTER_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
ticketmaster-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "ticketmaster-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `ticketmaster-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `ticketmaster-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `ticketmaster-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
ticketmaster-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
ticketmaster-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
ticketmaster-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
ticketmaster-pp-cli playbook amend \
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

`ticketmaster-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `TICKETMASTER_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ticketmaster-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ticketmaster-pp-cli feedback --stdin < notes.txt
ticketmaster-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TICKETMASTER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TICKETMASTER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ticketmaster-pp-cli profile save briefing --json
ticketmaster-pp-cli --profile briefing attractions get mock-value
ticketmaster-pp-cli profile list --json
ticketmaster-pp-cli profile show briefing
ticketmaster-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ticketmaster-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketmaster/cmd/ticketmaster-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ticketmaster-pp-mcp -- ticketmaster-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ticketmaster-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ticketmaster-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ticketmaster-pp-cli <command> --help`.
