---
name: pp-immoweb
description: "Search Immoweb from the terminal with every site filter, then get the answers the site can't give: price cuts, days on market, commune medians and rental yield. Trigger phrases: `search immoweb`, `find an apartment to rent in Ixelles`, `is this house cheap for the area`, `which listings dropped their price`, `median rent in Ixelles`, `compare communes Ixelles and Saint-Gilles`, `what's new in my saved search`, `rental yield in Liège`, `use immoweb`, `run immoweb-pp-cli`."
author: "sambassio"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - immoweb-pp-cli
    install:
      - kind: go
        bins: [immoweb-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-cli
---

# Immoweb — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `immoweb-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install immoweb --cli-only
   ```
2. Verify: `immoweb-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Talks to the same JSON endpoints the immoweb.be website uses, with no login and no browser, and exposes every search filter as a flag. Everything you look at lands in a local SQLite history, so `watch` shows what is new, cheaper or gone since last time, `deal` judges one listing against its neighbours, and `market` rebuilds the commune price statistics Immoweb no longer publishes.

## When to Use This CLI

Use this CLI for Belgian property searches on Immoweb: finding houses, apartments, land or rentals by commune and criteria, reading one listing's full details, tracking saved searches over time, and answering price questions (is this listing cheap, what did it cost before, what yield does a commune give) from the locally recorded history. It needs no Immoweb account.

## Anti-triggers

Do not use this CLI for:
- Do not use it to contact agencies, book visits or post listings; those are Immoweb account or write actions it does not perform.
- Do not use it for property outside Belgium or for other portals such as Zimmo, Immovlan or Biddit auctions.
- Do not use it for official valuations, notary or registration-duty calculations; its figures are asking-price statistics, not appraisals.
- Do not use it to read the user's Immoweb favourites or account saved searches; it keeps its own local saved searches instead.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Market intelligence
- **`market`** — Median asking price, €/m², price-cut share and recent disappearances for one or several communes, broken down by bedrooms, EPC, type or postcode. Student rooms and per-room lets are left out. Pulls missing areas on first use (--no-pull to stay offline).

  _Reach for this when an agent needs neighbourhood price levels or wants to compare communes before searching._

  ```bash
  immoweb-pp-cli market ixelles saint-gilles --type apartment --deal rent --by bedrooms --agent
  ```
- **`yield`** — Estimates gross rental yield for a sale listing or a whole commune from comparable rents, and refuses when there are too few comparables. Pulls missing areas on first use (--no-pull to stay offline).

  _Use for investor questions about rental return before looking at individual listings._

  ```bash
  immoweb-pp-cli yield --commune liege --type apartment --agent
  ```

### Decision support
- **`deal`** — Tells you whether one listing is cheap or expensive: €/m² percentile against comparable listings, days on market, price-cut history, demand and EPC gap. Pulls missing areas on first use (--no-pull to stay offline).

  _Use before advising on an offer or deciding whether a specific listing is worth a visit._

  ```bash
  immoweb-pp-cli deal 21828249 --agent
  ```
- **`triage`** — Ranks the current matches of a saved search (or ad-hoc filters) by a transparent score: EUR/m2 percentile within the same postal code, freshness, price cut, private seller and EPC (with --enrich) so you know who to call first.

  _Use when a search returns too many listings and the user needs a prioritised short list with reasons._

  ```bash
  immoweb-pp-cli triage --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2 --agent
  ```

### Local state that compounds
- **`drops`** — Lists stored listings whose asking price fell (from prices recorded by find/watch/pull and Immoweb's old-price field), with first and latest price, number of cuts, total % cut and days listed.

  _Use to find motivated sellers or negotiation leverage across everything the user has tracked._

  ```bash
  immoweb-pp-cli drops --commune liege --since 30d --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 12 API entries from 12 total network entries
- Protocols: rest_json (75% confidence)

## Command Reference

**listings** — Immoweb classifieds: search, count, map results, full details and similar listings

- `immoweb-pp-cli listings count` — Exact number of listings matching the filters (search pages cap at 9,969)
- `immoweb-pp-cli listings get` — Full details of one listing: EPC, cadastral income, surfaces, heating, agency contact, photos, views and bookmarks
- `immoweb-pp-cli listings map` — Up to 200 listings per call with GPS coordinates (the bulk path used by the Immoweb map view)
- `immoweb-pp-cli listings search` — Search listings with every Immoweb filter (30 per page, raw Immoweb result objects)
- `immoweb-pp-cli listings similar` — Listings Immoweb considers similar to a given listing

**locations** — Resolve commune names and postal codes to Immoweb location filters

- `immoweb-pp-cli locations` — Look up a commune, locality or postal code; returns the postalCodes value to pass to search filters

**Search, track and analyse**

- **`immoweb-pp-cli find`** - Live search with friendly filters (`--type`, `--deal`, `--commune`/`--postcode`/`--province`, price, bedrooms, surfaces, `--epc`, `--garden`...), `--url` to start from a pasted Immoweb search, `--pages`, `--private-only`, `--hide-under-option`, `--near lat,lng --radius-km`. Results are stored locally.
- **`immoweb-pp-cli show <id|url>`** - Readable listing card: price, EUR/m2, EPC, cadastral income, surfaces, days online, demand, agency contact, recorded price history.
- **`immoweb-pp-cli photos <id|url>`** - Download photos (`--dir`, `--size small|medium|large|xl`) or only list their URLs (`--list`).
- **`immoweb-pp-cli saved add|list|remove`** - Named local searches (same filters as `find`, or `--url`).
- **`immoweb-pp-cli watch <saved>`** / **`watch --all`** - New listings, price changes (with the old price) and listings gone since the last run. The first run records a silent baseline.
- **`immoweb-pp-cli pull`** - Harvest every listing of an area into the local store (`--commune`/`--postcode`, `--type`, `--deal`); splits the area into price bands of up to 200 listings so it needs only a few requests, and marks listings that disappeared as gone.
- **`immoweb-pp-cli hide <id...>`** (`--undo`) - Dismiss listings so `find`, `watch` and `triage` skip them.
- **`immoweb-pp-cli shortlist add|list|remove`** - Local shortlist with notes.
- **`immoweb-pp-cli dump`** - Export stored listings as `--format csv|geojson|jsonl` (a JSON array with `--json`).
- **`market`, `deal`, `triage`, `drops`, `yield`** - see Unique Features.

`find`, `show`, `watch`, `pull` and the automatic pulls of `market`/`deal`/`yield`/`triage` fill the local store; `drops`, `dump`, `search` and `--no-pull` runs read it. Do not use the generic `sync` command to fill the store: it has no area filter and would page through listings for all of Belgium. Use `pull` instead.

## Known behaviours

- The 19 Brussels communes resolve to their own postal codes (Ixelles = 1050), because Immoweb's "(all localities)" grouping adds neighbouring codes (Ixelles would include 1000, the City of Brussels). Elsewhere a commune follows Immoweb's grouping: check it with `immoweb-pp-cli locations --query <name>`, and use `--postcode` or `market --by postcode` for a clean split.
- Student rooms (kots) and per-room lets are left out of `market`, `yield` and `triage`, and `deal` compares rooms only with rooms; `--include-rooms` keeps them.
- The first `market`, `deal` or `yield` on an area pulls it from Immoweb (a few seconds, up to ~20 s for large communes on a slow connection). The pull is reused for 24 h (`--refresh-after`); `--no-pull` or `--data-source local` stay offline.
- Days on market come from Immoweb's publication date, which only the listing detail carries: they appear after `show`, `deal` or `triage --enrich`. Without it, `market` leaves the median out and says why, and `triage` scores freshness on the last update date.
- Listings without a published price ("price on request") cannot be reached through price bands; `pull` reports them as `without_price`.
- Immoweb soft-throttles by returning empty pages; the CLI turns that into exit code 7 instead of an empty result. Wait a minute and retry.
- `triage` leaves out hidden listings and listings under option (`--include-under-option` keeps them).


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
immoweb-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Who to call first for a rental search

```bash
immoweb-pp-cli triage --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2 --agent
```

Ranks current matches with the per-factor breakdown (EUR/m2 vs the same postal code, freshness, cut, private seller); add --enrich 5 to score EPC for the top five.

### Is this house fairly priced?

```bash
immoweb-pp-cli deal 21828249 --agent
```

Returns the €/m² percentile against comparables, days on market and any recorded price cuts.

### Key facts from a listing, agent-sized

```bash
immoweb-pp-cli listings get 21828249 --agent --select classified.price.mainValue,classified.property.netHabitableSurface,classified.transaction.certificates.epcScore,classified.statistics.viewCount
```

The raw detail JSON is about 20 KB; --select keeps only the dotted fields the agent needs.

### Compare commune rents

```bash
immoweb-pp-cli market ixelles saint-gilles etterbeek --type apartment --deal rent --by bedrooms --agent
```

Median rent and €/m² per bedroom count, side by side, with sample sizes.

### Recent price cuts in what you have tracked

```bash
immoweb-pp-cli drops --since 30d --agent
```

Reads the local store only: cuts come from prices recorded by find, watch and pull plus Immoweb's own old-price field, so pull an area first (immoweb-pp-cli pull --commune liege --type apartment --deal sale).

## Auth Setup

No authentication required.

Run `immoweb-pp-cli doctor` to verify setup.

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
  immoweb-pp-cli listings get 21828249 --agent --select classified
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — `drops`, `dump`, `search` and `--no-pull` runs read the local SQLite store filled by `find`, `show`, `watch` and `pull` (never `sync`, which has no area filter)
- **Non-interactive** — never prompts, every input is a flag
- **Read-only toward Immoweb** — it never contacts agencies or changes anything on immoweb.be; its only writes are the local store (saved searches, hidden listings, shortlist) and downloaded photos

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local (for `market`/`deal`/`yield`, pass `--data-source local` rather than `--no-pull` when you need the envelope to say local). A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `IMMOWEB_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `IMMOWEB_CONFIG_DIR`, `IMMOWEB_DATA_DIR`, `IMMOWEB_STATE_DIR`, `IMMOWEB_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `IMMOWEB_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `data.db` (listings, price history, saved searches). `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Run `immoweb-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "immoweb": {
        "command": "immoweb-pp-mcp",
        "env": {
          "IMMOWEB_HOME": "/srv/immoweb"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `IMMOWEB_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `IMMOWEB_HOME`, or `doctor` will not find the local store left under the former root.

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
immoweb-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "immoweb-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `immoweb-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `immoweb-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `immoweb-pp-cli pull --commune <name> --type <type> --deal <sale|rent>` to add listings for that area.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
immoweb-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
immoweb-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
immoweb-pp-cli teach-playbook \
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
immoweb-pp-cli playbook amend \
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

`immoweb-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `IMMOWEB_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
immoweb-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
immoweb-pp-cli feedback --stdin < notes.txt
immoweb-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `IMMOWEB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IMMOWEB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
immoweb-pp-cli profile save briefing --json
immoweb-pp-cli --profile briefing listings get 21828249
immoweb-pp-cli profile list --json
immoweb-pp-cli profile show briefing
immoweb-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Unexpected failure |
| 2 | Usage error (wrong arguments) |
| 3 | Not found (listing removed or sold, unknown saved search) |
| 4 | Immoweb refused the request (HTTP 401/403, usually bot protection; wait and retry) |
| 5 | API error (upstream issue) |
| 7 | Rate limited (soft throttling; wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `immoweb-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/immoweb/cmd/immoweb-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add immoweb-pp-mcp -- immoweb-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which immoweb-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   immoweb-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `immoweb-pp-cli <command> --help`.
