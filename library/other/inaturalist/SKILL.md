---
name: pp-inaturalist
description: "Explore iNaturalist's full API, plus privacy-safe field briefings and identification progress that ordinary endpoint wrappers miss. Trigger phrases: `what wildlife is nearby`, `make a nature scavenger hunt`, `did my iNaturalist observations get identified`, `compare wildlife seasons nearby`, `use iNaturalist`, `run iNaturalist`."
author: "avanderheyde"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - inaturalist-pp-cli
    install:
      - kind: go
        bins: [inaturalist-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/inaturalist/cmd/inaturalist-pp-cli
---

# iNaturalist — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `inaturalist-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install inaturalist --cli-only
   ```
2. Verify: `inaturalist-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/inaturalist/cmd/inaturalist-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use nearby highlights and seasonal-shift for transparent, bounded biodiversity briefings. Create factual scavenger hunts from real taxa, and use identification commands to track whether observations gained community IDs without exposing locations.

## When to Use This CLI

Use this CLI for bounded iNaturalist discovery, taxon research, safe nearby field briefings, and observation-identification progress. Prefer it when raw endpoint data needs agent-shaped output or a local history, and use its exact privacy labels rather than trying to infer a more precise location.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to recover, infer, or share obscured/private observation coordinates.
- Do not use this CLI for bulk data or media scraping; use iNaturalist exports or GBIF datasets.
- Do not use this CLI to create observations or other remote changes without explicit review and confirmation.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Privacy-safe field briefings
- **`nearby highlights`** — Get a transparent, recent wildlife briefing for an area without exposing observation coordinates.

  _Use this when an agent needs an explanation-backed local wildlife overview rather than a raw observation list._

  ```bash
  inaturalist-pp-cli nearby highlights --lat 37.7749 --lng -122.4194 --radius 5 --agent
  ```
- **`hunt create`** — Create a factual, balanced nature scavenger-hunt checklist from taxa actually observed nearby.

  _Use this to turn local biodiversity evidence into a safe field activity with traceable taxa._

  ```bash
  inaturalist-pp-cli hunt create --place-id 97394 --iconic-taxa Aves,Plantae --agent
  ```
- **`nearby seasonal-shift`** — Compare two field windows and surface taxa that newly appeared, returned, or changed materially.

  _Use this to answer how local wildlife changed between explicit time windows._

  ```bash
  inaturalist-pp-cli nearby seasonal-shift --place-id 97394 --recent-days 30 --baseline-days 30 --agent
  ```

### Identification progress
- **`observations id-status`** — See which of an observer's recent observations are identified, need IDs, disagree, or have no taxon.

  _Use this for a current identification-progress answer instead of manually filtering raw observations._

  ```bash
  inaturalist-pp-cli observations id-status --user inaturalist --since 30d --agent
  ```
- **`observations id-changes`** — Report observations that became identified, changed, withdrew, or still need IDs since a previous privacy-safe sync.

  _Use this to find identification progress since the last check without fabricating history._

  ```bash
  inaturalist-pp-cli observations id-changes --user inaturalist --since 30d --agent
  ```

## Command Reference

**annotations** — Create, delete, and vote

- `inaturalist-pp-cli annotations create` — Create an annotation
- `inaturalist-pp-cli annotations delete` — Delete an annotation

**colored-heatmap** — Manage colored heatmap


**comments** — Create, update, and delete

- `inaturalist-pp-cli comments create` — Create a comment
- `inaturalist-pp-cli comments delete` — Delete a comment
- `inaturalist-pp-cli comments update` — Update a comment

**controlled-terms** — Search and fetch

- `inaturalist-pp-cli controlled-terms list` — List all attribute controlled terms
- `inaturalist-pp-cli controlled-terms list-controlledterms` — Returns attribute controlled terms relevant to a taxon

**flags** — Create, update, and delete flags

- `inaturalist-pp-cli flags create` — Create a flag.
- `inaturalist-pp-cli flags delete` — Delete a flag
- `inaturalist-pp-cli flags update` — Update a flag. Generally only used to resolve the flag.

**grid** — Manage grid


**heatmap** — Manage heatmap


**identifications** — Create, update, and delete

- `inaturalist-pp-cli identifications create` — Create an identification
- `inaturalist-pp-cli identifications delete` — Delete an identification.
- `inaturalist-pp-cli identifications get` — Given an ID, or an array of IDs in comma-delimited format, returns corresponding identifications.
- `inaturalist-pp-cli identifications list` — Given zero to many of following parameters, returns identifications matching the search criteria
- `inaturalist-pp-cli identifications list-categories` — Given zero to many of following parameters
- `inaturalist-pp-cli identifications list-identifiers` — Given zero to many of following parameters
- `inaturalist-pp-cli identifications list-observers` — Given zero to many of following parameters
- `inaturalist-pp-cli identifications list-recenttaxa` — Returns an array of objects each containing an identification and a taxon.
- `inaturalist-pp-cli identifications list-similarspecies` — Returns species attached to IDs of observations of this taxon, or attached to observations identified as this species
- `inaturalist-pp-cli identifications list-speciescounts` — Given zero to many of following parameters
- `inaturalist-pp-cli identifications update` — Update an identification.

**messages** — Create, fetch, delete

- `inaturalist-pp-cli messages create` — Create and deliver a new message to another user
- `inaturalist-pp-cli messages delete` — This will all of the authenticated user's copies of the messages in tha thread to which the specified message belongs.
- `inaturalist-pp-cli messages get` — Retrieves all messages in the thread the specified message belongs to and marks them all as read.
- `inaturalist-pp-cli messages list` — Retrieve messages for the authenticated user. This does not mark them as read.
- `inaturalist-pp-cli messages list-unread` — Gets a count of messages the authenticated user has not read

**observation-field-values** — Create, update, and delete

- `inaturalist-pp-cli observation-field-values create` — Create an observation field value
- `inaturalist-pp-cli observation-field-values delete` — Delete an observation field value
- `inaturalist-pp-cli observation-field-values update` — Update an observation field value

**observation-photos** — Create and delete

- `inaturalist-pp-cli observation-photos create` — Create an observation photo
- `inaturalist-pp-cli observation-photos delete` — Delete an observation photo
- `inaturalist-pp-cli observation-photos update` — Update an observation photo

**observations** — CRUD, search, faving, quality metrics, stats, and more

- `inaturalist-pp-cli observations create` — Create an observation
- `inaturalist-pp-cli observations delete` — Delete an observation
- `inaturalist-pp-cli observations get` — Given an ID, or an array of IDs in comma-delimited format, returns corresponding observations.
- `inaturalist-pp-cli observations list` — Given zero to many of following parameters, returns observations matching the search criteria.
- `inaturalist-pp-cli observations list-deleted` — Given a starting date
- `inaturalist-pp-cli observations list-histogram` — Given zero to many of following parameters, returns histogram data about observations matching the search criteria
- `inaturalist-pp-cli observations list-identifiers` — Given zero to many of following parameters
- `inaturalist-pp-cli observations list-observers` — Given zero to many of following parameters
- `inaturalist-pp-cli observations list-popularfieldvalues` — Given zero to many of following parameters, returns an array of relevant controlled terms values and a monthly histogram
- `inaturalist-pp-cli observations list-speciescounts` — Given zero to many of following parameters
- `inaturalist-pp-cli observations list-updates` — Given zero to many of following parameters
- `inaturalist-pp-cli observations update` — Update an observation

**photos** — Manage photos

- `inaturalist-pp-cli photos` — Create a photo

**places** — Search and fetch

- `inaturalist-pp-cli places get` — Given an ID, or an array of IDs in comma-delimited format, returns corresponding places.
- `inaturalist-pp-cli places list` — Given an string, returns places with names starting with the search term.
- `inaturalist-pp-cli places list-nearby` — Given an bounding box, and an optional name query

**points** — Manage points


**posts** — Fetch site and project posts

- `inaturalist-pp-cli posts create` — Create a post
- `inaturalist-pp-cli posts delete` — Delete a post
- `inaturalist-pp-cli posts list` — Return journal posts from the iNaturalist site
- `inaturalist-pp-cli posts list-foruser` — Return journal posts from the iNaturalist site.
- `inaturalist-pp-cli posts update` — Update a post

**project-observations** — Create, update, and delete

- `inaturalist-pp-cli project-observations create` — Add an observation to a project
- `inaturalist-pp-cli project-observations delete` — Delete a project observation
- `inaturalist-pp-cli project-observations update` — Update a project observation

**projects** — Search and fetch projects and members

- `inaturalist-pp-cli projects get` — Given an ID, or an array of IDs in comma-delimited format, returns corresponding projects.
- `inaturalist-pp-cli projects list` — Given zero to many of following parameters, returns projects matching the search criteria
- `inaturalist-pp-cli projects list-autocomplete` — Given an string, returns projects with titles starting with the search term

**site_search** — Manage site search

- `inaturalist-pp-cli site-search` — Given zero to many of following parameters, returns object matching the search criteria

**subscriptions** — Manage subscriptions

- `inaturalist-pp-cli subscriptions create` — Toggles current user's subscription to this observation.
- `inaturalist-pp-cli subscriptions create-project` — Toggles current user's subscription to this project.

**taxa** — Search and fetch

- `inaturalist-pp-cli taxa get` — Given an ID, or an array of IDs in comma-delimited format, returns corresponding taxa.
- `inaturalist-pp-cli taxa list` — Given zero to many of following parameters, returns taxa matching the search criteria
- `inaturalist-pp-cli taxa list-autocomplete` — Given an string, returns taxa with names starting with the search term

**taxon-places** — Manage taxon places


**taxon-ranges** — Manage taxon ranges


**users** — Fetch and update

- `inaturalist-pp-cli users create` — Resend an email confirmation
- `inaturalist-pp-cli users get` — Given an ID, returns corresponding user
- `inaturalist-pp-cli users list` — Given an string, returns users with names or logins starting with the search term
- `inaturalist-pp-cli users list-me` — Fetch the logged-in user
- `inaturalist-pp-cli users update` — Update the logged-in user's session
- `inaturalist-pp-cli users update-id` — Update a user

**votes** — Manage votes

- `inaturalist-pp-cli votes create` — Vote on an annotation
- `inaturalist-pp-cli votes create-vote` — Vote on an observation. A vote with an empty `scope` is recorded as a `fave` of the observation.
- `inaturalist-pp-cli votes delete` — Remove a vote from annotation
- `inaturalist-pp-cli votes delete-unvote` — Remove a vote from an observation


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
inaturalist-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Nearby wildlife briefing

```bash
inaturalist-pp-cli nearby highlights --lat 37.7749 --lng -122.4194 --radius 5 --agent --select results.taxon_name,results.reason,results.geoprivacy
```

Return just the privacy-safe taxa, ranking rationale, and privacy state for an explicit area.

### Build a bird and plant hunt

```bash
inaturalist-pp-cli hunt create --place-id 97394 --iconic-taxa Aves,Plantae --agent
```

Make a factual checklist from observed taxa, without observation locations.

### Check identification progress

```bash
inaturalist-pp-cli observations id-status --user inaturalist --since 30d --agent
```

Group recent observations by their current identification state.

### Compare field windows

```bash
inaturalist-pp-cli nearby seasonal-shift --place-id 97394 --recent-days 30 --baseline-days 30 --agent
```

See transparent changes between two bounded observation windows.

## Auth Setup

Public read endpoints work without credentials. Authenticated iNaturalist responses can include private data, so the CLI must never surface or store private location fields in compound workflows; write commands require the official JWT/OAuth flow.

Run `inaturalist-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  inaturalist-pp-cli controlled-terms list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

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

- Use `--home <dir>` for one invocation, or set `INATURALIST_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `INATURALIST_CONFIG_DIR`, `INATURALIST_DATA_DIR`, `INATURALIST_STATE_DIR`, `INATURALIST_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `INATURALIST_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `inaturalist-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "inaturalist": {
        "command": "inaturalist-pp-mcp",
        "env": {
          "INATURALIST_HOME": "/srv/inaturalist"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `INATURALIST_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `INATURALIST_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
inaturalist-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "inaturalist-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `inaturalist-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `inaturalist-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `inaturalist-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
inaturalist-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
inaturalist-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
inaturalist-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
inaturalist-pp-cli playbook amend \
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

`inaturalist-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `INATURALIST_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
inaturalist-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
inaturalist-pp-cli feedback --stdin < notes.txt
inaturalist-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `INATURALIST_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `INATURALIST_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
inaturalist-pp-cli profile save briefing --json
inaturalist-pp-cli --profile briefing controlled-terms list
inaturalist-pp-cli profile list --json
inaturalist-pp-cli profile show briefing
inaturalist-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `inaturalist-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/inaturalist/cmd/inaturalist-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add inaturalist-pp-mcp -- inaturalist-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which inaturalist-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   inaturalist-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `inaturalist-pp-cli <command> --help`.
