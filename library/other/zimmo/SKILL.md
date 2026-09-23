---
name: pp-zimmo
description: "Belgian real estate from Zimmo with exact addresses, sold comps, commune prices per m² and a local store for deal sourcing. Trigger phrases: `search zimmo`, `sold prices near this address`, `price per m2 in Ixelles`, `find PEB G houses in Schaerbeek`, `use zimmo`."
author: "sambassio"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - zimmo-pp-cli
    install:
      - kind: go
        bins: [zimmo-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/zimmo/cmd/zimmo-pp-cli
---

# Zimmo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `zimmo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install zimmo --cli-only
   ```
2. Verify: `zimmo-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/zimmo/cmd/zimmo-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search Zimmo's listings, including sold and rented ones, from the terminal with no account. Listings you find are stored locally, which powers comps, underpriced, peb-trap, yield and motivated, and same-as matches them against the immoweb-pp-cli and immovlan-pp-cli stores.

## When to Use This CLI

Use this CLI for Belgian property search, valuation and deal sourcing from Zimmo: listings for sale or rent, sold comparables, commune price per m², agencies, and daily watches. It pairs with immoweb-pp-cli and immovlan-pp-cli for cross-portal work.

## Anti-triggers

Do not use this CLI for:
- Contacting agencies or submitting forms on Zimmo
- Managing a Zimmo account, favourites or saved searches on the website
- Real-estate markets outside Belgium

## Unique Capabilities

Commands built on the local store and on fields only Zimmo publishes (exact address, EPC kWh, rent, sold listings).

### Deal sourcing
- **`enrich`** — Refresh stored listings from Zimmo, resumably: status changes, price history, EPC, flood and planning flags, GPS.

  _Run it before peb-trap or motivated so their filters see detail fields._

  ```bash
  zimmo-pp-cli enrich --missing epc,flood --top 200
  ```
- **`peb-trap`** — Energy-poor stored listings (EPC F/G or high kWh/m²), rented ones first with their rent, showing Zimmo's renovation-obligation flag.

  _Surfaces owners under pressure to sell._

  ```bash
  zimmo-pp-cli peb-trap --postcode 1030 --max-price 450000
  ```
- **`motivated`** — Rank listings by seller pressure: days on market, total price cut, re-listing, EPC.

  _Finds negotiable deals first._

  ```bash
  zimmo-pp-cli motivated --postcode 1030 --min-days 120
  ```

### Cross-portal
- **`same-as`** — Find the Immoweb or Immovlan twin of a Zimmo listing and the price gap between portals.

  _Dedupe leads across Belgian portals without refetching anything._

  ```bash
  zimmo-pp-cli same-as --all --unmatched --agent
  ```

### Valuation
- **`comps`** — Sold or rented listings around one property with median €/m² and the subject's gap to it.

  _Median sold €/m² around the subject and its gap to it; Zimmo lists few sold properties, so the result widens to the postcode when thin._

  ```bash
  zimmo-pp-cli comps LAISZ --radius 800m --months 24 --agent
  ```
- **`underpriced`** — Rank stored for-sale houses and apartments by discount to their commune's €/m² (viager, whole buildings and service flats skipped).

  _Turns thousands of listings into a short list of apparent bargains._

  ```bash
  zimmo-pp-cli underpriced --postcode 1050 --type apartment --below 15
  ```
- **`yield`** — Gross rental yield of sale listings: the published rent when let, otherwise rentals of similar surface in the same postcode and type.

  _Buy-to-let screening without a spreadsheet._

  ```bash
  zimmo-pp-cli yield --all --postcode 1060 --min 5
  ```

## Limits to know

- Sold data is sparse: Zimmo only shows listings agencies mark as sold, so `comps` may widen to the postcode or return few rows; its sold date is Zimmo's last update of the listing.
- `peb-trap`, `underpriced`, `motivated`, `yield --all`, `same-as`, `drops` and `dump` read the local store: run `find` or `watch run` first.
- `same-as` needs the immoweb-pp-cli / immovlan-pp-cli stores on the same machine.

## Command Reference

**Listings, prices and alerts (hand-written, local store)**

- `zimmo-pp-cli find` — Search listings (sale, rent, sold, rented, take-over) by commune, postcode, type, price, bedrooms, surface, EPC and text; stores every result
- `zimmo-pp-cli show <code>` — One listing in full: exact address, GPS, EPC kWh, flood/planning flags, rent, price history, agency
- `zimmo-pp-cli photos <code>` — List or download a listing's photos
- `zimmo-pp-cli prices` — Commune price per m² by type with the monthly history
- `zimmo-pp-cli agency <uuid|code>` — Agency contact, VAT, review score, listings count
- `zimmo-pp-cli watch save|run|list|rm` — Saved searches: new, cheaper and gone listings since the last run
- `zimmo-pp-cli drops` — Price cuts on stored listings (Zimmo price history + your observations)
- `zimmo-pp-cli shortlist [code]` — Star listings with a note, or list starred ones
- `zimmo-pp-cli dump` — Export stored listings as JSON, CSV or GeoJSON

Listings reach the local store only through `find`, `show`, `watch run` and `enrich`; framework `sync` loads places only.

**dealers** — Get and search real-estate agencies

- `zimmo-pp-cli dealers get` — Get one real-estate agency (dealer) by UUID
- `zimmo-pp-cli dealers search` — Search agencies (filter by category, placeId; sort by distance)

**geocode** — Geocode a Belgian address

- `zimmo-pp-cli geocode` — Geocode a Belgian address to coordinates and place ids

**listings** — Get and search listings (raw API shape)

- `zimmo-pp-cli listings get` — Get one listing by Zimmo code (e.g. LAISZ) or listing UUID
- `zimmo-pp-cli listings property-count` — Total number of active properties on Zimmo
- `zimmo-pp-cli listings search` — Search listings with a Zimmo filter object (status, placeId, postalCode, category, price, bedrooms, energyLabel, text)

**locality-price** — Commune price per m² (raw API shape)

- `zimmo-pp-cli locality-price <placeId>` — Price per m² for a commune (by property type) with monthly history

**places** — Resolve postcodes and commune names to place ids

- `zimmo-pp-cli places` — Resolve a postcode and/or commune name to Zimmo place ids

**sub-locality-price** — Sub-locality price per m² (raw API shape)

- `zimmo-pp-cli sub-locality-price <placeId>` — Price per m² for a sub-locality with monthly history

**sublocalities** — Search sub-localities by keyword

- `zimmo-pp-cli sublocalities` — Search sub-localities (deelgemeenten) by keyword


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
zimmo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Cheap flats in Ixelles

```bash
zimmo-pp-cli find --postcode 1050 --type apartment --max-price 300000 --agent --select listings.zimmo_code,listings.price,listings.surface_m2,listings.epc,listings.address
```

Narrow a large listing payload to the fields that matter.

### Sold comps for a lead

```bash
zimmo-pp-cli comps LAISZ --radius 800m --months 24
```

Sold listings nearby with median price per m²; widens to the postcode when the radius is thin.

### Daily PEB hunt

```bash
zimmo-pp-cli peb-trap --postcode 1030 --max-price 450000 --json
```

Reads the local store: run `zimmo-pp-cli watch run --all` first; a one-off `zimmo-pp-cli find --epc F,G` also works. Energy-poor listings, rented ones first with their current rent.

### Daily alert

```bash
zimmo-pp-cli watch run --all --json
```

New, cheaper and gone listings for every saved search since the last run.

## Auth Setup

No account or key. The CLI mints Zimmo's anonymous web token automatically (valid 24 hours) and caches it.

Run `zimmo-pp-cli doctor` to verify setup.

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
  zimmo-pp-cli show LAISZ --agent --select address,price,epc
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — commands that read listings (peb-trap, underpriced, motivated, drops, dump, same-as) use the local SQLite store filled by find/show/watch run; `search` queries Zimmo live by default and the stored listings with `--data-source local`
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

- Use `--home <dir>` for one invocation, or set `ZIMMO_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ZIMMO_CONFIG_DIR`, `ZIMMO_DATA_DIR`, `ZIMMO_STATE_DIR`, `ZIMMO_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ZIMMO_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `data.db` (stored listings, saved searches, shortlist). `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- No credential is stored: Zimmo's anonymous web token is cached as `anonymous-token.json` in the cache dir (24h). `ZIMMO_TOKEN` overrides it.
- Run `zimmo-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "zimmo": {
        "command": "zimmo-pp-mcp",
        "env": {
          "ZIMMO_HOME": "/srv/zimmo"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ZIMMO_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ZIMMO_HOME`, or `doctor` will not find credentials left under the former root.

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
zimmo-pp-cli recall "$QUERY" --agent
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
      "next_action": ["<trial command>", "zimmo-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `zimmo-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `zimmo-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., an "elsene" teach satisfying an "ixelles" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Bruxelles" → the city 1000 or the Brussels region). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Teach the mapping after answering; `sync` only loads places.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall — argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
zimmo-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "elsene") satisfies future queries under another alias (e.g., "ixelles", "1050") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
zimmo-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
zimmo-pp-cli teach-playbook \
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
zimmo-pp-cli playbook amend \
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

`zimmo-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `ZIMMO_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
zimmo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
zimmo-pp-cli feedback --stdin < notes.txt
zimmo-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ZIMMO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ZIMMO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
zimmo-pp-cli profile save briefing --json
zimmo-pp-cli --profile briefing show LAISZ
zimmo-pp-cli profile list --json
zimmo-pp-cli profile show briefing
zimmo-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Other error (hand-written commands return Zimmo API errors as 1) |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Access refused (generated commands on 401/403) |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `zimmo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/zimmo/cmd/zimmo-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add zimmo-pp-mcp -- zimmo-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which zimmo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   zimmo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `zimmo-pp-cli <command> --help`.
