---
name: pp-seats-aero
description: "Every Seats.aero Partner API endpoint, plus a local award-availability store that tells you what is new, what is still live, and where your miles reach nonstop. Trigger phrases: `find award seats to Tokyo in business`, `what award availability is new since yesterday`, `where can my miles take me nonstop from JFK`, `is this award seat still available before I book`, `show me the award calendar for united to Europe`, `use seats-aero`, `run seats-aero`."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - seats-aero-pp-cli
    install:
      - kind: go
        bins: [seats-aero-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/seats-aero/cmd/seats-aero-pp-cli
---

# Seats.aero — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `seats-aero-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install seats-aero --cli-only
   ```
2. Verify: `seats-aero-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/seats-aero/cmd/seats-aero-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

seats-aero-pp-cli wraps all seven Partner API endpoints (cached search, bulk availability, trips, routes, destinations, refresh, live) with agent-native output, then syncs routes and availability into typed SQLite tables. That local store powers new-since, calendar, direct-scan, reach and a quota-guarded recheck, none of which a thin wrapper or the web app can do.

## When to Use This CLI

Use seats-aero-pp-cli whenever an agent needs award-flight availability by mileage program: cheapest awards on a route, a program's calendar, nonstop reach from an airport, or verifying a cached seat before booking. It is the right tool for miles-and-points redemption research and for building a local award-availability corpus that can be queried offline.

## Anti-triggers

Do not use this CLI for:
- Do not use it to book or ticket a flight; it reports availability only and never books.
- Do not use it for cash fares, hotel awards, points valuations, or transfer-partner math; the Partner API carries none of that.
- Do not use it to set push alerts; Seats.aero alerts are a web-app feature, use new-since on a schedule instead.
- Do not expect live search with a Pro key; that endpoint is commercial-agreement only.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`new-since`** — See which award seats appeared on a route since you last looked, from your synced local data.

  _Reach for this when the user asks what changed or what is new on a route, instead of re-running a full search and diffing by eye._

  ```bash
  seats-aero-pp-cli new-since --origin JFK --destination NRT --cabin business --since 24h --agent
  ```
- **`direct-scan`** — Find direct-flight award seats under a mileage ceiling across every synced program at once.

  _Pick this over the awards search when the user wants nonstop-only results across programs from already-synced data with zero API calls._

  ```bash
  seats-aero-pp-cli direct-scan --origin JFK --destination NRT --cabin business --max-mileage 90000 --sources united,virginatlantic,aeroplan --agent
  ```
- **`calendar`** — Turn one route's synced availability into a date-by-cabin matrix you can scan at a glance.

  _Use this when the user asks which dates have business or first availability on a route; it answers from the local store in one shot._

  ```bash
  seats-aero-pp-cli calendar --origin JFK --destination NRT --source united --start 2026-10-01 --end 2026-12-31 --agent
  ```

### Quota-aware plumbing
- **`recheck`** — Re-verify aging award rows are still live right before booking. It performs one live quota probe per run (1 daily call), and --apply is refused when quota is unknown unless --ignore-quota is passed.

  _Print-only by default: lists the aging rows it would refresh, their age, and the remaining daily quota (one probe call). Add --apply to spend refresh credits; --apply is refused when the quota is unknown unless --ignore-quota is passed._

  ```bash
  seats-aero-pp-cli recheck --origin JFK --destination NRT --cabin business --max-mileage 90000 --agent
  ```
- **`reach`** — Discover where your miles can take you nonstop from one airport, ranked by cost and cross-checked against real dated seats.

  _Reach for this when the user has miles but no fixed destination. It has no local mode, and results is an object {origin, cabin, max_mileage, source, destinations[]} rather than a bare array. --confirm-live makes up to 10 extra live /search calls (opt-in; never under the test harness)._

  ```bash
  seats-aero-pp-cli reach --origin JFK --cabin business --max-mileage 90000 --top 10 --agent
  ```

## Command Reference

## Upgrading from 2026.8.1

`seats-aero-partner-search` was renamed to `awards`; the old command remains a hidden, deprecated alias for one release. Its `--cabin` flag is now `--cabins` and accepts comma-separated values. Existing stores are migrated by the first read-write command, such as `sync`; `doctor` is read-only and reports `migration_pending` without changing the store.

The credential environment variable is now `SEATS_AERO_API_KEY`. `SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION` and the TOML key `aero_partner_partner_authorization` are still honoured; `doctor` reports the selected `credentials_location`.

The store is migrated in place on the first read-write command (`sync` or `doctor`), while the legacy `seats_aero_partner_search` table is left untouched. `sync --concurrency` now defaults to `1` (was `4`), and `--timeout` now defaults to `1m` (was `30s`). The MCP tool is now `awards_cached-search`.

**availability** — Manage availability

- `seats-aero-pp-cli availability` — Retrieve a large amount of availability objects from one specific mileage program.

**awards** — Manage awards

- `seats-aero-pp-cli awards` — Search Seats.aero cached award availability between one or more origin and destination airports, across one or more mileage programs.

**destinations** — Manage destinations

- `seats-aero-pp-cli destinations` — Returns the airports reachable from (or to) a single airport

**live** — Manage live

- `seats-aero-pp-cli live` — Commercial-agreement API keys only; Pro keys receive 403 -- do not retry. 5-15 s latency.

**refresh** — Manage refresh

- `seats-aero-pp-cli refresh` — Use this endpoint to refresh old cached data; credit-metered; the response quota block shows remaining refreshes; Pro keys only.

**reach** — Discover nonstop destinations within a mileage ceiling

- `seats-aero-pp-cli reach` — Has no local mode, and `results` is an object `{origin, cabin, max_mileage, source, destinations[]}` rather than a bare array. `--confirm-live` makes up to 10 extra live `/search` calls (opt-in; never under the test harness).

**recheck** — Re-verify aging award rows before booking

- `seats-aero-pp-cli recheck` — Print-only by default; performs one live quota probe per run (1 daily call). `--apply` is refused when the quota is unknown unless `--ignore-quota` is passed.

**routes** — Manage routes

- `seats-aero-pp-cli routes` — Get all origin-destination routes tracked for a mileage program.

**trips** — Manage trips

- `seats-aero-pp-cli trips <id>` — Retrieve flight-level information from an Availability object by its revalidation/trip ID from search or availability results.


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `SEATS_AERO_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

`sync` and `sync --resources availability` cannot populate availability because the endpoint requires `source`. Use `seats-aero-pp-cli sync --resources availability --resource-param availability:source=<program> --since 7d` once per program. The `routes` resource needs no parameter.

Covered paths:

- `seats-aero-pp-cli availability`
- `seats-aero-pp-cli availability get`
- `seats-aero-pp-cli availability list`
- `seats-aero-pp-cli availability search`
- `seats-aero-pp-cli awards`
- `seats-aero-pp-cli awards get`
- `seats-aero-pp-cli awards list`
- `seats-aero-pp-cli awards search`
- `seats-aero-pp-cli destinations`
- `seats-aero-pp-cli destinations get`
- `seats-aero-pp-cli destinations list`
- `seats-aero-pp-cli destinations search`
- `seats-aero-pp-cli routes`
- `seats-aero-pp-cli routes get`
- `seats-aero-pp-cli routes list`
- `seats-aero-pp-cli routes search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
seats-aero-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Cheapest business awards across all programs, narrowed for an agent

```bash
seats-aero-pp-cli awards --origin-airport SFO --destination-airport NRT --cabins business --order-by lowest_mileage --take 10 --agent --select data.Date,data.Route.Source,data.JMileageCost,data.JDirect
```

One cached search across every program, with --select trimming the wide availability rows to the four fields that matter.

### What appeared since yesterday

```bash
seats-aero-pp-cli new-since --origin JFK --destination NRT --cabin business --since 24h --agent
```

Diffs the local availability table on first_seen_at, so only genuinely new rows come back.

### Verify before you book, without spending credits

```bash
seats-aero-pp-cli recheck --origin JFK --destination NRT --cabin business --max-mileage 90000 --agent
```

Print-only by default: lists the aging rows it would refresh, their age, and the remaining daily quota (one probe call). Add --apply to spend refresh credits.

### Direct-only under 90k miles across three programs

```bash
seats-aero-pp-cli direct-scan --origin JFK --destination NRT --cabin business --max-mileage 90000 --sources united,virginatlantic,aeroplan --agent
```

A cross-program join over synced data that the one-program-per-call bulk endpoint cannot answer.

### Where can 90k miles take me nonstop

```bash
seats-aero-pp-cli reach --origin JFK --cabin business --max-mileage 90000 --top 10 --agent
```

Fans out via destinations then confirms each candidate against dated seats. reach has no local mode, and results is an object {origin, cabin, max_mileage, source, destinations[]} rather than a bare array. --confirm-live makes up to 10 extra live /search calls (opt-in; never under the test harness).

## Auth Setup

Seats.aero Pro subscribers create a Partner API key under Settings, then export it as SEATS_AERO_API_KEY. Keys are Pro-tier (1,000 calls per day, resets at midnight UTC; the X-RateLimit-Remaining header tracks it) or commercial-agreement keys. Pro keys cannot call live search and commercial keys cannot call refresh; doctor reports the remaining daily calls and which tier your key appears to be.

Run `seats-aero-pp-cli doctor` to verify setup.

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
  seats-aero-pp-cli destinations --agent --select airport,business,economy
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit confirmation** — `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SEATS_AERO_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SEATS_AERO_CONFIG_DIR`, `SEATS_AERO_DATA_DIR`, `SEATS_AERO_STATE_DIR`, `SEATS_AERO_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SEATS_AERO_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `seats-aero-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "seats-aero": {
        "command": "seats-aero-pp-mcp",
        "env": {
          "SEATS_AERO_HOME": "/srv/seats-aero"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SEATS_AERO_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SEATS_AERO_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
seats-aero-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "seats-aero-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `seats-aero-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `seats-aero-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `seats-aero-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
seats-aero-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
seats-aero-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
seats-aero-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
seats-aero-pp-cli playbook amend \
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

`seats-aero-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SEATS_AERO_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
seats-aero-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
seats-aero-pp-cli feedback --stdin < notes.txt
seats-aero-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SEATS_AERO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SEATS_AERO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

Outputs from `doctor`, `agent-context`, `learnings`, and `feedback` can contain local paths or free text and should not be delivered to third-party webhooks.

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
seats-aero-pp-cli profile save briefing --json
seats-aero-pp-cli --profile briefing destinations
seats-aero-pp-cli profile list --json
seats-aero-pp-cli profile show briefing
seats-aero-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `seats-aero-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/seats-aero/cmd/seats-aero-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add seats-aero-pp-mcp -- seats-aero-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which seats-aero-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   seats-aero-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `seats-aero-pp-cli <command> --help`.
