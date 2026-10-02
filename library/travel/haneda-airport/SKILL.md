---
name: pp-haneda-airport
description: "Query first-party Haneda flight boards with precise dates, codeshares and terminal handoffs. Trigger phrases: `Haneda flight status`, `Haneda arrivals and departures`, `Haneda terminal and gate`, `Haneda codeshare lookup`, `use haneda-airport`, `run haneda-airport`."
author: "zjsng"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - haneda-airport-pp-cli
    install:
      - kind: go
        bins: [haneda-airport-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/cmd/haneda-airport-pp-cli
---

# Haneda Airport — Printing Press CLI

For this unpublished local build, use its staged `build/stage/bin/haneda-airport-pp-cli` binary after building from source. Public library installation instructions below apply once published.

## Prerequisites: Install the CLI

This skill drives the `haneda-airport-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install haneda-airport --cli-only
   ```
2. Verify: `haneda-airport-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/cmd/haneda-airport-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read domestic and international arrivals and departures, inspect source disruptions and published schedules, and compare your saved observations. Explicit source timestamps and unknown fields keep terminal planning grounded in what the airport actually reports.

## When to Use This CLI

Use for dated Haneda public flight status, codeshare lookup, delays/cancellations, terminal planning and published schedule inspection. Use saved snapshots for explicit offline observations and change comparison.

## Anti-triggers

Do not use this CLI for:
- No seat or fare inventory, booking, payment or account operations.
- No guaranteed connections or inferred operating-carrier/actual-time claims.

## Unique Capabilities

Additional capabilities combine the source's flight facts with local observations.

### Flight planning and observations
- **`snapshot save`** — Preserve timestamped scoped boards locally.

  _Preserve timestamped scoped boards locally._

  ```bash
  haneda-airport-pp-cli snapshot save --file /tmp/haneda-board.json --kind international --direction departure
  ```
- **`snapshot search`** — Filter a saved board without network access.

  _Filter a saved board without network access._

  ```bash
  haneda-airport-pp-cli snapshot search --file /tmp/haneda-board.json --flight NH849 --limit 5
  ```
- **`snapshot diff`** — See changed status, time and terminal facilities.

  _See changed status, time and terminal facilities._

  ```bash
  haneda-airport-pp-cli snapshot diff --before /tmp/haneda-before.json --after /tmp/haneda-after.json
  ```
- **`plan`** — Find the source terminal, gates and check-in handoffs.

  _Find the source terminal, gates and check-in handoffs._

  ```bash
  haneda-airport-pp-cli plan NH849 --kind international --agent
  ```
- **`flights rollover`** — Inspect adjacent service days and cross-midnight changes.

  _Inspect adjacent service days and cross-midnight changes._

  ```bash
  haneda-airport-pp-cli flights rollover --kind international --direction departure --limit 5
  ```

## Discovery Signals

Native Chrome exposed the public workflows and assets. Anonymous HTTP replay confirmed the request contracts; the enriched capture records those replay exchanges.
- Replay coverage: 14 public API exchanges across 10 endpoint shapes
- Protocols: rest_json (75% confidence)
- Candidate command ideas: create_search — Derived from observed POST /en/app/api/v2/flight/search traffic.; list_city_list_search.json — Derived from observed GET /site_resource/flight/data/dms/city_list_search.json traffic.; list_company_list_search.json — Derived from observed GET /site_resource/flight/data/dms/company_list_search.json traffic.; list_flight_status.json — Derived from observed GET /en/app_resource/flight/flightStatus/flight_status.json traffic.; list_hdacfasc.json — Derived from observed GET /app_resource/flight/data/dms/hdacfasc.json traffic.; list_hdacfdsc.json — Derived from observed GET /app_resource/flight/data/dms/hdacfdsc.json traffic.

## Cookbook

Use the following recipes for a small board, arrivals, disruptions and published schedules.

## Recipes

### Small flight board

```bash
haneda-airport-pp-cli flights search --kind international --limit 5 --agent --select flights.id,flights.source_primary_flight,flights.scheduled_at,flights.status,sources.reported_at,observed_at
```

Keep useful flight facts and source time in compact agent output.

### Domestic arrivals

```bash
haneda-airport-pp-cli flights search --kind domestic --direction arrival --destination CTS --limit 5 --agent
```

Airport and source city aliases are resolved without changing codeshare identity.

### Disruptions

```bash
haneda-airport-pp-cli flights disruptions --kind international --direction both --limit 10 --agent
```

Separate airport summary counts from the returned delayed/canceled/diverted flight groups.

### Monthly schedule

```bash
haneda-airport-pp-cli schedule search --kind international --flight NH849 --limit 5 --agent
```

Inspect published periods and operating weekdays with explicit feed coverage.

### Codeshare detail

```bash
haneda-airport-pp-cli flights detail UA8003 --kind international --direction departure --limit 5 --agent
```

Resolve a marketing number through the source-primary-only endpoint and bounded board fallback; retain the original listed group.

### Padded terminal lookup

```bash
haneda-airport-pp-cli plan NH0849 --kind international --direction departure --agent
```

Find the same primary group with a zero-padded number and use source terminal/facility handoffs.

### Midnight and adjacent days

```bash
haneda-airport-pp-cli flights rollover --kind international --direction both --limit 5 --agent
```

Inspect explicit service and revised dates. Empty output means this observation contained no matching rollover evidence.

### Preserve an observation

```bash
haneda-airport-pp-cli snapshot save --kind international --direction both --file ./haneda-before.json --agent
```

Write a complete local observation; choose a new filename if it already exists. No provider state is changed.

### Search a saved group

```bash
haneda-airport-pp-cli snapshot search --file ./haneda-before.json --flight UA8003 --limit 5 --agent
```

After saving the preceding observation, resolve its marketing group offline with its original time and age.

### Compare observations

```bash
haneda-airport-pp-cli snapshot diff --before ./haneda-before.json --after ./haneda-after.json --limit 5 --agent
```

First save a second board to haneda-after.json with the same date/kind/direction. No changes is a valid result; disappearance is not inferred cancellation.

### Resolve an airline

```bash
haneda-airport-pp-cli catalog airlines --kind international --query ANA --limit 5 --agent
```

Retain provider airline identifiers, IATA flight prefixes, Japanese names and canonical airline URLs.

## Command Reference

| Command | Purpose |
| --- | --- |
| `flights search` | Live domestic/international arrivals and departures; local flight, airline, airport/city, status and terminal filters |
| `flights detail [flight-or-id]` | Resolve a listed number or board ID to its source group, full facilities and canonical detail link |
| `flights disruptions` | Source-declared delayed, canceled and diverted services, plus a separately scoped airport summary |
| `flights rollover` | Adjacent service days and explicit revised dates crossing midnight |
| `plan [flight-or-id]` | Flight facilities, terminal floor, inter-terminal transfer and connection-guide handoffs |
| `catalog airports` | English/Japanese names, airport codes and distinct source city values |
| `catalog airlines` | English/Japanese names, source airline codes, flight prefixes and airline URLs |
| `schedule search` | Published feed windows, service periods and weekdays; accepts full listed flight numbers |
| `snapshot save` | Fetch a complete unfiltered board and atomically save an observation |
| `snapshot search` | Filter one saved observation without network requests |
| `snapshot diff` | Compare two complete observations with identical source/date/kind/direction coverage |

Use `--kind domestic|international|all` and `--direction departure|arrival|both`. Defaults are international departures; detail and plan search both directions. `--destination` identifies the other airport: destination for departures, origin for arrivals. `--airline` matches any listed airline, including marketing codeshares; `--flight` accepts full numbers such as NH849 or UA8003. Detail and plan also resolve zero padding such as NH0849. `--status` accepts a source category or exact source status text, a comma-separated list, or `all` / `unknown`. Empty CSV tokens are errors. `--terminal T1|T2|T3` filters known terminals.

All result pages default to 20 groups, with `--limit 1..200` and `--offset 0..20000`. `next_offset` refers to the locally filtered result. Re-running a live page fetches a new observation, so totals may change between pages. Use a saved snapshot for stable offline paging.

## Flight evidence and dates

`--date YYYY-MM-DD` is a JST service-day query and defaults to today for live boards. The source supports domestic dates from today and international dates from yesterday, through three calendar months. An `all` query uses the domestic start. The provider may include flights from adjacent service days; their `service_date` and stable `hnd:kind:direction:YYYYMMDD:primary` IDs retain those days. Use `--service-day-only` to narrow to the requested day, or `flights rollover` to inspect rollover evidence.

Each `listed_flights` array retains source order. Its first number is `source_primary_flight`; the source does not establish that this is the operating carrier, so `operating_flight` stays null. Marketing codeshares match the same group without replacing its identity. Detail tries the source's primary-only exact endpoint, then performs a bounded board lookup when a codeshare or padded number needs resolution. `coverage.query_mode` and `coverage.flight_lookup` disclose the lookup scope.

`scheduled_at` and `revised_at` are distinct RFC3339 timestamps in JST. Revised times use the provider's explicit changed date, including earlier calendar days; `time_change_minutes` can be negative. The provider does not label a changed time as estimated versus actual, so `actual_at` stays null. Missing values remain null or empty arrays. `unknown_fields` identifies unavailable operator/actual time, airport, schedule/revision, terminal, boarding-gate and check-in-counter facts, plus unknown status/category or terminal mapping; a blank category with visible status text is still a known status. Gates, check-in counters, security checkpoints and arrival exits have separate fields. An arrival exit is not an aircraft boarding gate. Full `facilities` entries carry available provider map/detail links when requested with `--include-facilities` or via detail/plan.

`observed_at` is the local fetch time. Board `sources.reported_at` is the provider response time, not proof of when an individual flight last changed; board `source_updated_at` stays null. Monthly schedule `sources.updated_at` preserves its published `last_upd`. The disruption summary has its own `gettingAt` and coverage, so its counts need not equal a dated or filtered board. Airline information can arrive late; use the canonical flight/airline links for current operational guidance.

Monthly feeds expose raw `period_start` / `period_end`, each service's operating period, and weekdays. An optional exact `--date` must lie inside the feed's published window and applies period/weekday rules. Haneda clock times use JST. Remote international airport times remain unassigned to a timezone/day when the feed does not establish them. Schedule lookup uses a listed flight number; board IDs belong to `flights detail`.

## Saved observations

`snapshot save` records every group in the selected date/kind/direction scope with source totals and observation time. It refuses truncated scans. The default destination is a timestamped JSON file under the CLI cache's `haneda-snapshots` directory; `--file` chooses an explicit path, and replacing an existing regular file requires `--overwrite`. Files are capped at 8 MiB. Search uses `--file` or the latest cached observation. Explicit kind/direction requests must fit its coverage; explicit `--date` must match its source request date and selects only that service day. Uncovered requests are errors, while a covered query may return zero matching groups. Default search still preserves adjacent service-day rows. Snapshot data is explicitly local; it is never an implicit live fallback. Ages over five minutes set `stale_snapshot` by default and retain the original timestamps. `snapshot search --max-age 30m` chooses another positive age threshold.

Diff uses explicit `--before` / `--after`, or the newest compatible cached pair with matching origin/date/kind/direction. A newest unpaired scope does not hide an older compatible pair. Automatic selection examines at most 64 MiB. If a valid compatible pair has already been found, a scan stop caused by the budget or an unreadable/malformed older file preserves that comparison with `cache_selection_complete:false` and `cache_selection_notes`; a newer pair may remain unexamined. Choose explicit paths for exact pair selection. Without a known pair, a stopped scan is an error. Full provider facility entries and map handoffs participate in material changes. Missing baselines return empty results with `baseline_sufficient:false`. Explicitly selected malformed snapshots, invalid timestamps, missing source scopes, count mismatches and incomplete observations are errors; the bounded automatic selection exception above applies only to older files after a fully validated pair is already known. A disappeared group is `no_longer_reported`, never inferred canceled. Only the source's status establishes a reported cancellation.

## Request bounds

Domain commands have a 30-second hard deadline, no automatic retries, at most 20 requests, a 4 MiB response cap and a 16 MiB aggregate cap. The adaptive source limiter defaults to two requests per second. Board/catalog metadata is reused only within one command; live boards are freshly fetched. `budget` reports the actual request and response-byte counts. Typical single-kind boards use three requests; all kinds/both directions use eight. Detail fallback can add up to four board requests. Source failures and rate limits are errors rather than empty successes.

`--max-scan-records` defaults to 5000 and permits 1..20000. A reached cap is explicit in `scan_cap_hit`; filtered totals then cover only examined records. A failed component aborts an `all`/`both` scope. Airport status, schedules and terminal facilities do not establish seat inventory, fares, reservations or guaranteed connections.


**Raw source diagnostics** — The hidden `source` group preserves read-only provider shapes and is excluded from MCP tools. Domain commands above provide normalized planning facts.

- `haneda-airport-pp-cli source airlines-domestic` — Read the raw first-party airlines-domestic JSON source.
- `haneda-airport-pp-cli source airlines-international` — Read the raw first-party airlines-international JSON source.
- `haneda-airport-pp-cli source airports-domestic` — Read the raw first-party airports-domestic JSON source.
- `haneda-airport-pp-cli source airports-international` — Read the raw first-party airports-international JSON source.
- `haneda-airport-pp-cli source board` — Read the raw first-party board JSON source.
- `haneda-airport-pp-cli source disruption-summary` — Read the raw first-party disruption-summary JSON source.
- `haneda-airport-pp-cli source schedule-domestic-arrivals` — Read the raw first-party schedule-domestic-arrivals JSON source.
- `haneda-airport-pp-cli source schedule-domestic-departures` — Read the raw first-party schedule-domestic-departures JSON source.
- `haneda-airport-pp-cli source schedule-international-arrivals` — Read the raw first-party schedule-international-arrivals JSON source.
- `haneda-airport-pp-cli source schedule-international-departures` — Read the raw first-party schedule-international-departures JSON source.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
haneda-airport-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Auth Setup

The supported source paths are anonymous. No API key, login, reservation or browser runtime is required.

Run `haneda-airport-pp-cli doctor` to verify setup.

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
  haneda-airport-pp-cli source airlines-domestic --agent --select en,ja,ko
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline observations** — use `snapshot search` and `snapshot diff` for saved board files; live domain commands reject `--data-source local`
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local" | "computed", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `HANEDA_AIRPORT_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `HANEDA_AIRPORT_CONFIG_DIR`, `HANEDA_AIRPORT_DATA_DIR`, `HANEDA_AIRPORT_STATE_DIR`, `HANEDA_AIRPORT_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `HANEDA_AIRPORT_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings and profiles. `data` contains the local `data.db` learning store. `state` contains persisted runtime state and `teach.log`. `cache/haneda-snapshots` contains explicitly saved observations; snapshot commands also accept explicit file paths.
- Run `haneda-airport-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "haneda-airport": {
        "command": "haneda-airport-pp-mcp",
        "env": {
          "HANEDA_AIRPORT_HOME": "/srv/haneda-airport"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `HANEDA_AIRPORT_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `HANEDA_AIRPORT_HOME`, or `doctor` will not find local state left under the former root.

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
haneda-airport-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "haneda-airport-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `haneda-airport-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `haneda-airport-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `haneda-airport-pp-cli sync --resources source,source-hdacfasc-json,source-hdacfdsc-json` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
haneda-airport-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
haneda-airport-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
haneda-airport-pp-cli teach-playbook \
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
haneda-airport-pp-cli playbook amend \
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

`haneda-airport-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `HANEDA_AIRPORT_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
haneda-airport-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
haneda-airport-pp-cli feedback --stdin < notes.txt
haneda-airport-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `HANEDA_AIRPORT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `HANEDA_AIRPORT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
haneda-airport-pp-cli profile save briefing --json
haneda-airport-pp-cli --profile briefing source airlines-domestic
haneda-airport-pp-cli profile list --json
haneda-airport-pp-cli profile show briefing
haneda-airport-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `haneda-airport-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/cmd/haneda-airport-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add haneda-airport-pp-mcp -- haneda-airport-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which haneda-airport-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   haneda-airport-pp-cli flights search --kind international --limit 5 --agent
   ```
4. If ambiguous, drill into subcommand help: `haneda-airport-pp-cli flights search --help`.
