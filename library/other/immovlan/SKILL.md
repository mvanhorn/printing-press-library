---
name: pp-immovlan
description: "Search Immovlan from the terminal with every site filter, keep a local history, and cross it with Immoweb: exclusives, re-listings, rented PEB F/G traps and division candidates. Trigger phrases: `search immovlan`, `what is for sale on immovlan in schaerbeek`, `PEB F houses on immovlan`, `is this immovlan listing also on immoweb`, `immovlan exclusives`, `use immovlan`, `run immovlan`."
author: "sambassio"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - immovlan-pp-cli
    install:
      - kind: go
        bins: [immovlan-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/immovlan/cmd/immovlan-pp-cli
---

# Immovlan — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `immovlan-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install immovlan --cli-only
   ```
2. Verify: `immovlan-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/immovlan/cmd/immovlan-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Reads the same server-rendered pages the immovlan.be site serves, with no login, and stores every result in SQLite. Field names mirror immoweb-pp-cli so a sourcing job can merge both portals, and same-as tells you which listings exist only on Immovlan. enrich fills the detail fields the search cards never show, and peb-trap, split-candidates and relisted answer the questions a property trader asks before calling an agency.

## When to Use This CLI

Reach for this CLI when a task needs Belgian listings from Immovlan: searching a commune with price, bedroom and PEB filters, reading a listing's full details, tracking what is new or cheaper, or crossing Immovlan with Immoweb for a property trader's sourcing. It is the second portal next to immoweb-pp-cli in the brussels-peb-hunter job.

## Anti-triggers

Do not use this CLI for:
- Do not use it to contact sellers or agencies, place bids, or fill site forms (Turnstile-protected).
- Do not use it for Immoweb, Zimmo or Biddit data; use immoweb-pp-cli or the Biddit feed in the sourcing job.
- Do not use it for sold prices or market medians; Immovlan shows asking prices only.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-portal intelligence
- **`same-as`** — Tell whether a stored Immovlan listing is also on Immoweb, at what price, or is Immovlan-only.

  _Use it to dedupe a cross-portal sourcing run or to find Immovlan exclusives before calling an agency. Requires immoweb-pp-cli's local store (`--immoweb-db` or `$IMMOWEB_PP_DB`); run `enrich --missing street` first so addresses can match. Every row carries flat `immoweb_id` (0 when Immovlan-only), `immoweb_url`, `immoweb_price`, `delta_pct`, `match_status` (address / price_surface_agency / none) and `immoweb_gone_status`; the nested `immoweb` object exists on matched rows only and is dropped by `--agent` compaction, so agents read the flat fields._

  ```bash
  immovlan-pp-cli same-as --all --unmatched --postcode 1030 --agent
  ```
- **`agencies`** — Which agencies list in a zone, their PEB F/G share, private-seller share and the CRM software behind each feed.

  _Use it to see which feeds are syndicated (likely also on Immoweb) versus Rossel-exclusive. The software column stays `?` until `enrich` has read the listing pages._

  ```bash
  immovlan-pp-cli agencies --postcode 1030 --epc F,G --agent
  ```

### Motivated-seller signals
- **`peb-trap`** — List PEB F or G properties that are currently rented: owners who can no longer re-let in Brussels since 2026.

  _Use it when the question is which sellers are structurally motivated, not merely cheap. Needs `enrich --missing epc,rented` first; `--include-unknown` also lists F/G rows not yet enriched._

  ```bash
  immovlan-pp-cli peb-trap --postcode 1030,1210 --max-price 2500000 --agent
  ```
- **`split-candidates`** — Houses large enough to divide, ranked by €/m² within their postcode, with land, year, PEB and rented flags.

  _Use it for a trader who divides houses into apartments; find cannot filter by surface._

  ```bash
  immovlan-pp-cli split-candidates --min-surface 200 --postcode 1030 --agent
  ```

### Local state that compounds
- **`enrich`** — Fill missing detail fields (PEB letter, street, geo, rented, cadastral income, agency software) for many stored listings in one resumable pass.

  _Run it after find and before peb-trap, same-as or split-candidates so their inputs exist._

  ```bash
  immovlan-pp-cli enrich --missing epc,rented --top 150 --agent
  ```
- **`relisted`** — Listings that came back under a new reference, with their true first-seen date and price at each reference.

  _Use it to expose a fake Nouveau before negotiating on ancienneté._

  ```bash
  immovlan-pp-cli relisted --since 90d --postcode 1030 --agent
  ```

## Command Reference

**listings** — Immovlan listings: server-rendered search pages (20 cards per page) and detail pages

- `immovlan-pp-cli listings get` — Fetch one listing page by reference (e.g. vbe69761); Immovlan redirects to the canonical URL. Returns the schema.
- `immovlan-pp-cli listings search` — Search listings; returns the detail links of one result page (20 per page).

**Sourcing workflow** (hand-written, all read the local store `data.db` after `find`)

- `immovlan-pp-cli find` — Search by type, deal, postcode/commune, price band, PEB band and sort; stores every card locally. Cards carry no street: run `enrich` for addresses.
- `immovlan-pp-cli show <ref>` — One listing's detail (PEB letter, surface, price, publication date, rented, cadastral income, agency or private).
- `immovlan-pp-cli photos <ref> --dir <dir>` — Download a listing's photos (CLI only, not exposed over MCP; existing files are never overwritten). Over MCP, read `pictures` from `show`.
- `immovlan-pp-cli saved add|list|remove` — Named searches; `watch <name>` re-runs one and reports new, price-changed and gone listings since the previous run.
- `immovlan-pp-cli drops` — Price drops recorded across runs.
- `immovlan-pp-cli hide <ref>` / `shortlist <ref>` — Local triage flags applied to every listing table.
- `immovlan-pp-cli dump --format csv|jsonl|geojson` — Export the store; `locations --query <commune>` lists the postcodes behind a commune name.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
immovlan-pp-cli which "rented PEB F houses"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Bad-PEB houses for sale in two communes, newest first

```bash
immovlan-pp-cli find --type maison --deal sale --postcode 1030,1210 --epc F,G --pages 5 --agent --select id,url,price,surface_m2,price_per_m2,epc,locality
```

One line per listing with the fields the sourcing job needs; --select keeps the payload small.

### Rented PEB traps after enrichment

```bash
immovlan-pp-cli enrich --missing epc,rented --top 200 --agent && immovlan-pp-cli peb-trap --postcode 1030 --agent
```

enrich reads the detail pages once; peb-trap then filters locally.

### Immovlan exclusives

```bash
immovlan-pp-cli same-as --all --unmatched --postcode 1030 --agent
```

Joins with immoweb-pp-cli's store; unmatched rows are listings only Immovlan carries.

### What changed since the last run of a saved search

```bash
immovlan-pp-cli saved add sch-fg --type maison,appartement --deal sale --postcode 1030 && immovlan-pp-cli watch sch-fg --agent
```

First run is a baseline; later runs report new, price-changed and gone listings since the previous run.

### Notary sales on Immovlan

```bash
immovlan-pp-cli find --deal public-sale --postcode 1030,1080 --agent
```

Immovlan's own public-sale feed, next to Biddit.

## Auth Setup

No authentication required.

Run `immovlan-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, and `--deliver` paths:

- `--json` — one JSON document on stdout (progress notes go to stderr)
- `--compact` — keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain` — tabular rows (collection envelopes unwrap to the row array)
- `--quiet` — one identity value per row, no envelope

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  immovlan-pp-cli listings get vbe69761 --agent --select url,name,datePosted
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **One JSON shape** — `find`, `watch`, `drops`, `relisted`, `peb-trap`, `split-candidates`, `agencies` and `same-as` return an object with `results` plus counters and an optional `note` (an empty store is a note, not an error); `dump --agent` returns the bare array. `watch` price changes carry `old_price`, `new_price`, `change_pct`.
- **Offline-friendly** — `find`, `watch` and `enrich` store every page in SQLite; `drops`, `peb-trap`, `split-candidates`, `relisted`, `agencies`, `same-as`, `dump` and `shortlist` read it offline; `show --data-source local` prints the stored row
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

- Use `--home <dir>` for one invocation, or set `IMMOVLAN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `IMMOVLAN_CONFIG_DIR`, `IMMOVLAN_DATA_DIR`, `IMMOVLAN_STATE_DIR`, `IMMOVLAN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `IMMOVLAN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `data.db` (the listing store); this CLI has no credentials, cookies or auth files. `state` contains `teach.log` and the learn journal. `cache` contains regenerable HTTP/cache files.
- Run `immovlan-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "immovlan": {
        "command": "immovlan-pp-mcp",
        "env": {
          "IMMOVLAN_HOME": "/srv/immovlan"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `IMMOVLAN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `IMMOVLAN_HOME`, or `doctor` will not find the store left under the former root.

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
immovlan-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "immovlan-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `immovlan-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `immovlan-pp-cli learnings candidates` lists the full open set.

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
immovlan-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
immovlan-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
immovlan-pp-cli teach-playbook \
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
immovlan-pp-cli playbook amend \
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

`immovlan-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `IMMOVLAN_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
immovlan-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
immovlan-pp-cli feedback --stdin < notes.txt
immovlan-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `IMMOVLAN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `IMMOVLAN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
immovlan-pp-cli profile save briefing --json
immovlan-pp-cli --profile briefing listings get vbe69761
immovlan-pp-cli profile list --json
immovlan-pp-cli profile show briefing
immovlan-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Blocked (HTTP 401/403 on the generic `listings` commands or `doctor`: bot wall, geography or rate block; no credentials exist) |
| 5 | API error (also a 403 or bot challenge met by `find`, `show`, `watch`, `enrich`) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `immovlan-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/immovlan/cmd/immovlan-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add immovlan-pp-mcp -- immovlan-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which immovlan-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   immovlan-pp-cli peb-trap --postcode 1030 --agent
   ```
4. If ambiguous, drill into subcommand help: `immovlan-pp-cli find --help`.
