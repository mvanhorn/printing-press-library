---
name: pp-wework
description: "Find and book WeWork day-desks from the terminal — cities, buildings, availability, price, and your bookings — with a local store for instant, agent-native queries. Trigger phrases: `find a wework desk`, `book a wework desk`, `wework desks in austin`, `my wework bookings`, `use wework`, `run wework`."
author: "Paul Byrne"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - wework-pp-cli
---

# WeWork — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `wework-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install wework --cli-only
   ```
2. Verify: `wework-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/wework/cmd/wework-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

WeWork's desk booking is a map-and-modal web app with no CLI and no public API. This CLI reverse-engineers the members-portal read API into scriptable commands: resolve any city, search bookable desks by city + date with credits/price/seat availability, browse buildings and amenities, and list your bookings. Cities and locations sync into a local SQLite store so repeat queries are instant and work offline.

## When to Use This CLI

Use this CLI to script or agent-drive WeWork day-desk discovery: check whether a desk is available in a given city on a given date, compare prices/credits across buildings, resolve city and building UUIDs, and review your upcoming bookings — all without the web map UI.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Full-text search across every synced WeWork city and building, offline and instant.

  _Reach for this to resolve a place name to concrete city/building UUIDs without hitting the network._

  ```bash
  wework-pp-cli search "austin" --agent
  ```
- **`desks`** — Sort desks by credits or price and filter to only those with open seats.

  _Answers 'what is the cheapest available desk here tomorrow' in one call._

  ```bash
  wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --sort credits --available-only
  ```

### Agent-native plumbing
- **`auth session-import` / `auth refresh` / `auth push` / `auth handoff`** — Bootstrap, remotely transfer, and sustain a rotating Auth0 session on a headless booking host without passing secrets in command arguments.

  _Use `push` when this machine already has a complete session; use `handoff` for the manual capture fallback._

  ```bash
  wework-pp-cli auth push --ssh-target user@booking-host
  wework-pp-cli auth handoff --ssh-target user@booking-host
  wework-pp-cli auth refresh --force --json
  ```
- **`desks`** — Search bookable desks by city name and date, deriving the map bounding box the API requires from cached city geo.

  _The headline command: turns a plain city name + validated YYYY-MM-DD date into structured desk availability with price, seats, and truthful live-source metadata._

  ```bash
  wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --agent
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**common-booking** — Manage common booking

- `wework-pp-cli common-booking book-desk` — Creates a real desk reservation and charges the member's saved payment account.
- `wework-pp-cli common-booking cancel-booking` — Cancels an existing booking. Verified against a live cancellation on 2026-08-12 (full $47 refund confirmed).
- `wework-pp-cli common-booking get-booking-details` — [BEST-EFFORT/UNVERIFIED] Get booking details
- `wework-pp-cli common-booking list-bookings` — List my upcoming desk bookings
- `wework-pp-cli common-booking quote-booking` — Returns the price breakdown (grandTotal, subTotal, taxes, lineItems) for a prospective desk booking.

**spaces** — Manage spaces

- `wework-pp-cli spaces list-locations` — List WeWork buildings in a city with availability
- `wework-pp-cli spaces search-desks` — The primary desk-search endpoint.

**wework-yardi** — Manage wework yardi

- `wework-pp-cli wework-yardi get-city-details` — Get details for a city
- `wework-pp-cli wework-yardi list-amenities` — List amenities for locations
- `wework-pp-cli wework-yardi list-cities` — Returns every city where WeWork desks can be booked (~842), each with its market geo (latitude/longitude)
- `wework-pp-cli wework-yardi list-locations-by-geo` — List WeWork buildings within a geo bounding box


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
wework-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Resolve a city without matching a nearby street address

```bash
wework-pp-cli cities --filter "Austin, TX" --data-source live --agent
```

The friendly `cities`, `locations`, `desks`, and `bookings` aliases are live-only. Use `--data-source auto` or `--data-source live`; `local` is rejected before any request. City filtering matches identity fields (city/market, state, and country), not arbitrary nearby-location addresses. `locations` and `desks` reject malformed dates before contacting WeWork.

### Bootstrap renewable auth on a remote booking host

```bash
wework-pp-cli auth login --chrome
wework-pp-cli auth push --ssh-target user@booking-host
```

`auth push` sends the complete stored bundle only through SSH stdin, imports it remotely, force-refreshes it, and fails unless the booking host verifies as renewable and headless-ready. Never put token values in command arguments, examples, or agent output. The forced rotation makes the booking host the sole refresh-token-family owner, so stop using the source session. If local Chrome import is unavailable, use `auth handoff --ssh-target user@booking-host` for the printed manual capture workflow.

### Cheapest available desk in a city

```bash
wework-pp-cli desks --city "Austin, TX" --date 2026-08-18 --sort credits --available-only --agent
```

Ranks bookable desks by credits and drops any with no open seats.

### Compact desk view for agents

```bash
wework-pp-cli desks --city "New York, NY" --date 2026-08-18 --agent --select desks.location.name,desks.credits,desks.seat.available
```

Returns only high-gravity fields from the deeply-nested desk response to keep agent context small.

### Resolve a place to UUIDs offline

```bash
wework-pp-cli search "barton springs" --json
```

Finds the matching city/building from the local store without a network call.

## Auth Setup

WeWork uses an Auth0 access token, rotating refresh token, account UUID, and member type. On a machine with an existing Chrome login, prefer `wework-pp-cli auth login --chrome`: it reads a private snapshot of the real `members.wework.com` LevelDB records, imports the complete four-value bundle without opening/debugging a browser, and validates the WeWork issuer/client. Otherwise capture the complete bundle shown by `wework-pp-cli auth session-import --help`, pipe it to `auth session-import --stdin`, and confirm `headless_ready=true` plus `renewable=true` with `auth whoami --json`. The former `auth import` spelling remains a compatibility alias. Import requires all four values unless the operator explicitly selects `--allow-partial` to repair an existing setup. Normal commands refresh automatically near expiry, serialize refreshes across concurrent processes, and stop before the API request if renewal fails; `auth refresh --force` verifies the chain immediately. A clean host can bootstrap from `WEWORK_TOKEN`, `WEWORK_REFRESH_TOKEN`, `WEWORK_UUID`, and `WEWORK_MEMBER_TYPE`; the private credential file owns the rotated replacements.

For a separate computer, first prefer `wework-pp-cli auth push --ssh-target user@booking-host` when a complete session is already stored locally. It transfers the bundle only through SSH stdin, force-refreshes remotely, and verifies remote renewability; after that rotation, the remote host exclusively owns the token family. Otherwise use `auth handoff --ssh-target user@booking-host` for the printed login, capture, and stdin-transfer steps. The booking host stays windowless. WeWork has not enabled device authorization or a CLI callback, so do not promise a one-click phone callback. `auth login` without flags is browser-free and prints the recommended import/handoff setup; `--chrome` is the windowless local-session importer and `--cdp` remains an explicit last-resort browser fallback.

Run `wework-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  wework-pp-cli common-booking get-booking-details --agent
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
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

Friendly live aliases report `.meta.source = "live"`. Their `--csv` output contains one row per city, location, or desk; `--quiet` suppresses successful output.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `WEWORK_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `WEWORK_CONFIG_DIR`, `WEWORK_DATA_DIR`, `WEWORK_STATE_DIR`, `WEWORK_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `WEWORK_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `wework-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "wework": {
        "command": "wework-pp-mcp",
        "env": {
          "WEWORK_HOME": "/srv/wework"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `WEWORK_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `WEWORK_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
wework-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "wework-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `wework-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `wework-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `wework-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
wework-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
wework-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
wework-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
wework-pp-cli playbook amend \
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

`wework-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `WEWORK_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
wework-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
wework-pp-cli feedback --stdin < notes.txt
wework-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `WEWORK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `WEWORK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
wework-pp-cli profile save briefing --json
wework-pp-cli --profile briefing common-booking get-booking-details
wework-pp-cli profile list --json
wework-pp-cli profile show briefing
wework-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `wework-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add wework-pp-mcp -- wework-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which wework-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   wework-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `wework-pp-cli <command> --help`.
