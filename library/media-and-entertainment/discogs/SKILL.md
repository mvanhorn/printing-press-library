---
name: pp-discogs
description: "Every Discogs feature — plus a local price-history mirror the API doesn't keep, so you can watch your wantlist like a limit-order book, spot underpriced records, and track your collection's value over time. Trigger phrases: `check my discogs wantlist for deals`, `what records should I sell`, `value my vinyl collection`, `price suggestions for this release`, `identify this pressing by catalog number`, `is this record undervalued`, `use discogs`, `run discogs`."
author: "ganes-j"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - discogs-pp-cli
    install:
      - kind: go
        bins: [discogs-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/cmd/discogs-pp-cli
---

# Discogs — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `discogs-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install discogs --cli-only
   ```
2. Verify: `discogs-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/cmd/discogs-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

A single Go binary over the full Discogs API — database, collection, wantlist, and marketplace — with an offline SQLite mirror and an MCP server. Because Discogs keeps no price history, this CLI snapshots prices locally, which unlocks fills (wantlist limit orders), undervalued detection, portfolio value over time, and condition-matched comps that no other Discogs tool offers.

## When to Use This CLI

Use this CLI when an agent or script needs to work with Discogs data programmatically: searching the database, managing a collection or wantlist, pricing or listing records on the marketplace, or answering value/deal questions that need price history. It is the right tool when the task benefits from a local mirror, structured JSON output, or repeated queries under the rate limit.

## Anti-triggers

Do not use this CLI for:
- Fetching cover-art images (Discogs image URLs are Cloudflare-gated and unreliable server-side)
- Multi-user OAuth app authentication (this CLI is single-user personal-token only)
- Editing another user's collection or wantlist (only your own account)
- eBay or Gixen auction sniping (this CLI is Discogs-only)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local price history (the moat)
- **`fills`** — Treat each wantlist entry as a standing limit order and surface the ones now selling at or below the max price you set.

  _Reach for this to answer 'which of my wants are buyable right now' without checking hundreds of listings by hand under the 60/min limit._

  ```bash
  discogs-pp-cli fills --agent
  ```
- **`portfolio`** — Chart your collection's value over time with per-record contribution to the change and cost-basis P&L.

  _Use it when the question is 'is my collection up or down, and because of what' rather than 'what is it worth today'._

  ```bash
  discogs-pp-cli portfolio --since 90d --agent
  ```
- **`undervalued`** — Flag releases whose current asking price sits below their own trailing/suggested baseline.

  _Pick this for market-wide mispricing where no preset limit exists; use 'fills' when the trigger is a limit you set._

  ```bash
  discogs-pp-cli undervalued --scope wantlist --agent
  ```
- **`comps`** — The condition-by-condition price picture for one release, with local snapshot history the API can't provide.

  _Use it to price or value a single specific release; use 'pressings' to compare different versions of the same album._

  ```bash
  discogs-pp-cli comps 249504 --agent
  ```
- **`pressings`** — Rank all versions/pressings of a master release by value and liquidity so you know which pressing to buy, want, or sell.

  _Use it to choose among pressings of an album; use 'comps' for the condition breakdown of one specific pressing._

  ```bash
  discogs-pp-cli pressings 96559 --agent
  ```

### Collector decisioning
- **`sell-plan`** — Rank your inventory or collection by net-after-fee proceeds and liquidity so you know what to sell now.

  _Reach for this to decide what to list this week; use 'comps' to price one release._

  ```bash
  discogs-pp-cli sell-plan --source collection --agent
  ```
- **`identify`** — Resolve a physical record's catalog number or barcode to the exact release and show if you own it, want it, and what it's worth.

  _Use it crate-digging in a shop to make a buy/skip call fast; use 'search' for open-ended text queries._

  ```bash
  discogs-pp-cli identify --barcode 0720642442524 --agent
  ```

## Command Reference

**collection** — Browse and manage your Discogs collection (requires a token).

- `discogs-pp-cli collection add` — Add a release to a collection folder.
- `discogs-pp-cli collection create-folder` — Create a new collection folder.
- `discogs-pp-cli collection delete-folder` — Delete a collection folder (must be empty).
- `discogs-pp-cli collection fields` — List the custom fields defined on your collection.
- `discogs-pp-cli collection find` — Find a release's instances in your collection.
- `discogs-pp-cli collection folder` — Get one collection folder (0 = All).
- `discogs-pp-cli collection folders` — List your collection folders.
- `discogs-pp-cli collection items` — List releases in a collection folder (0 = All).
- `discogs-pp-cli collection rate-instance` — Set the rating on a collection instance.
- `discogs-pp-cli collection remove` — Remove a release instance from a folder.
- `discogs-pp-cli collection rename-folder` — Rename a collection folder.
- `discogs-pp-cli collection value` — Get the min/median/max market value of your collection.

**database** — Search Discogs and look up releases, masters, artists, and labels (public; a token raises the rate limit).

- `discogs-pp-cli database artist` — Get an artist by ID.
- `discogs-pp-cli database artist-releases` — List an artist's releases.
- `discogs-pp-cli database community-rating` — Get the community rating for a release.
- `discogs-pp-cli database label` — Get a label by ID.
- `discogs-pp-cli database label-releases` — List releases on a label.
- `discogs-pp-cli database master` — Get a master release (the canonical version group).
- `discogs-pp-cli database master-versions` — List all versions (pressings) of a master release.
- `discogs-pp-cli database rate` — Set your rating (1-5) for a release.
- `discogs-pp-cli database release` — Get a specific release by ID.
- `discogs-pp-cli database search` — Search the Discogs database. Public data; a token raises the rate limit from 25 to 60/min.
- `discogs-pp-cli database unrate` — Delete your rating for a release.
- `discogs-pp-cli database user-rating` — Get a specific user's rating for a release.

**identity** — Your Discogs identity, profile, and contributions (requires a token).

- `discogs-pp-cli identity contributions` — List a user's database contributions.
- `discogs-pp-cli identity profile` — Get a user's public profile.
- `discogs-pp-cli identity submissions` — List a user's database submissions.
- `discogs-pp-cli identity whoami` — Show the authenticated user's identity (username, id).

**inventory** — Request and download CSV exports of your marketplace inventory (requires a token).

- `discogs-pp-cli inventory export` — Request a new CSV export of your inventory.
- `discogs-pp-cli inventory export-download` — Download a finished inventory export CSV.
- `discogs-pp-cli inventory export-get` — Get the status of an inventory export.
- `discogs-pp-cli inventory exports` — List your recent inventory exports.
- `discogs-pp-cli inventory upload-get` — Get the status of an inventory CSV upload.
- `discogs-pp-cli inventory uploads` — List recent inventory CSV uploads.

**lists** — User lists (curated collections of database items).

- `discogs-pp-cli lists get` — Get one list and its items.
- `discogs-pp-cli lists user` — List a user's public lists.

**marketplace** — Marketplace listings, orders, fees, price suggestions, and per-release stats (requires a token).

- `discogs-pp-cli marketplace add-order-message` — Post a message to an order.
- `discogs-pp-cli marketplace create-listing` — Create a new marketplace listing.
- `discogs-pp-cli marketplace delete-listing` — Delete a marketplace listing.
- `discogs-pp-cli marketplace fee` — Calculate the Discogs marketplace fee for a sale price (USD).
- `discogs-pp-cli marketplace fee-currency` — Calculate the marketplace fee for a price in a specific currency.
- `discogs-pp-cli marketplace inventory` — List a seller's marketplace inventory.
- `discogs-pp-cli marketplace listing` — Get one marketplace listing.
- `discogs-pp-cli marketplace order` — Get one marketplace order.
- `discogs-pp-cli marketplace order-messages` — List messages on an order.
- `discogs-pp-cli marketplace orders` — List your marketplace orders (as seller).
- `discogs-pp-cli marketplace price-suggestions` — Get suggested marketplace prices per media condition for a release (seller token).
- `discogs-pp-cli marketplace stats` — Get marketplace stats for a release (lowest price, number for sale).
- `discogs-pp-cli marketplace update-listing` — Edit an existing marketplace listing.
- `discogs-pp-cli marketplace update-order` — Update an order's status or shipping.

**wantlist** — Browse and manage your Discogs wantlist (requires a token).

- `discogs-pp-cli wantlist add` — Add a release to your wantlist.
- `discogs-pp-cli wantlist edit` — Edit the notes/rating on a wantlist item.
- `discogs-pp-cli wantlist list` — List your wantlist.
- `discogs-pp-cli wantlist remove` — Remove a release from your wantlist.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
discogs-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Deals on your wantlist

```bash
discogs-pp-cli fills --agent
```

Lists wantlist items whose lowest marketplace asking has reached the max price you set, with the change since last check.

### Narrow a verbose release payload

```bash
discogs-pp-cli database release 249504 --agent --select id,title,year,labels.name,formats.descriptions,lowest_price
```

A Discogs release is tens of KB; dotted --select paths return only the fields you need so an agent doesn't burn context.

### What to sell this week

```bash
discogs-pp-cli sell-plan --source collection --agent
```

Ranks your records by net-after-fee proceeds and liquidity.

### Identify a record in-hand

```bash
discogs-pp-cli identify --barcode 0720642442524 --agent
```

Resolves the barcode to a release and tells you if you own it, want it, and its current value.

### Value trend over the last quarter

```bash
discogs-pp-cli portfolio --since 90d --agent
```

Charts collection value change and the records that drove it, against cost basis.

## Auth Setup

Database search and lookups are public. Everything tied to a user — collection, wantlist, marketplace, identity — needs a free personal access token from discogs.com/settings/developers, set as DISCOGS_TOKEN. The client sends it as `Authorization: Discogs token=<token>` and always sends a descriptive User-Agent (Discogs returns 403 without one).

Run `discogs-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  discogs-pp-cli database search mock-value --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `DISCOGS_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `DISCOGS_CONFIG_DIR`, `DISCOGS_DATA_DIR`, `DISCOGS_STATE_DIR`, `DISCOGS_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `DISCOGS_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `discogs-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "discogs": {
        "command": "discogs-pp-mcp",
        "env": {
          "DISCOGS_HOME": "/srv/discogs"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `DISCOGS_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `DISCOGS_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
discogs-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "discogs-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `discogs-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `discogs-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `discogs-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
discogs-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
discogs-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
discogs-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
discogs-pp-cli playbook amend \
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

`discogs-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `DISCOGS_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
discogs-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
discogs-pp-cli feedback --stdin < notes.txt
discogs-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `DISCOGS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `DISCOGS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
discogs-pp-cli profile save briefing --json
discogs-pp-cli --profile briefing database search mock-value
discogs-pp-cli profile list --json
discogs-pp-cli profile show briefing
discogs-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `discogs-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/cmd/discogs-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add discogs-pp-mcp -- discogs-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which discogs-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   discogs-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `discogs-pp-cli <command> --help`.
