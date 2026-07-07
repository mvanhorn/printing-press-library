---
name: pp-prediction-goat
description: "Every Polymarket and Kalshi market in one slim agent-native CLI, with cross-venue topic search and screens no other... Trigger phrases: `what are the odds on`, `polymarket odds for`, `kalshi odds for`, `find prediction markets for`, `what's trending on polymarket`, `what's resolving this week`, `compare polymarket and kalshi on`, `use prediction-goat`, `run prediction-goat`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - prediction-goat-pp-cli
---

# Polymarket + Kalshi - Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `prediction-goat-pp-cli` binary. You must verify the CLI is installed before invoking any command from this skill. If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install prediction-goat --cli-only
   ```
2. Verify: `prediction-goat-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/cmd/prediction-goat-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only by design and by CI lint - the binary structurally cannot trade. `topic <name>` returns every related Polymarket + Kalshi market/event/tag in one ~3KB ranked bundle (vs the official Polymarket CLI's ~250KB firehose). Local SQLite + FTS5 keeps queries instant and free after one sync. Six screens (`trending`, `resolving`, `liquid`, `mispriced`, `movers`, `new`) cover the workflows agents and odds researchers run every week.

## When to Use This CLI

Reach for prediction-goat-pp-cli when an agent needs current prediction-market odds across both Polymarket and Kalshi without trading. The killer commands are `topic`, `compare`, and the six screens - every other command exists so power users can drill into one venue or one market. The CLI is read-only by structural CI guarantee: it cannot place orders, hold a wallet, or sign trades, which makes it safe to embed in agent toolchains.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-venue intelligence
- `topic` - Get every related Polymarket and Kalshi market for a topic in one slim ranked ~3KB bundle - kanye, argentina, chatgpt-5.

  _When an agent needs current odds on a topic, this is the one-call answer across both venues without fanning out to two platform tools and re-ranking by hand._

  ```bash
  prediction-goat-pp-cli topic kanye --json
  ```
- `mispriced` - Find same-outcome markets where Polymarket and Kalshi disagree on implied probability by more than a threshold.

  _The clearest signal that one venue is wrong or one side is mispricing - useful for calibration research, not trading._

  ```bash
  prediction-goat-pp-cli mispriced --threshold 0.05 --json
  ```
- `compare` - Side-by-side YES/NO and implied probability for the same topic across Polymarket and Kalshi.

  _Tells an agent or analyst 'which venue has the better/different number on this outcome' in one read-only call._

  ```bash
  prediction-goat-pp-cli compare 'arizona basketball' --json
  ```
- `markets diff` - Field-by-field structural diff between a specific Polymarket market and a specific Kalshi market.

  _When you already know the two slugs/tickers (e.g. from `topic <theme>`), diff shows you exactly where the venues disagree._

  ```bash
  prediction-goat-pp-cli markets diff <pm-slug> <kalshi-ticker> --json
  ```

### Discovery walks

- `polymarket event-of` - Look up the parent event slug for any Polymarket market slug.

  _The reliable way to anchor into multi-outcome event families without guessing slug suffixes (the gamma frontend appends -467, -595 etc)._

  ```bash
  prediction-goat-pp-cli polymarket event-of will-ghana-win-the-2026-fifa-world-cup
  ```

- `polymarket siblings` - List every sibling market under the parent event of a known market slug.

  _One call returns every team in a World Cup event, every draft slot, every WCF candidate, with prices. The path that works when topic + public-search both miss the long tail._

  ```bash
  prediction-goat-pp-cli polymarket siblings will-ghana-win-the-2026-fifa-world-cup --agent
  ```

- `kalshi-series-search` - Substring search over locally synced Kalshi series tickers and titles.

  _Faster + higher recall than `topic` when you know roughly what you're looking for (e.g. searching WEST surfaces KXNBAWEST, the conference championship series with $29M of volume)._

  ```bash
  prediction-goat-pp-cli kalshi-series-search WEST --agent
  ```

### Screens
- `trending` - Top movers by 24h volume across both venues, ranked.

  _One command answers 'what should I be watching today' without scraping two homepages._

  ```bash
  prediction-goat-pp-cli trending --json --limit 20
  ```
- `resolving` - Markets resolving in the next week/month/days, sorted by liquidity.

  _Tells an agent 'what's about to settle' without re-paging two cursors._

  ```bash
  prediction-goat-pp-cli resolving --week --json
  ```
- `liquid` - Markets above a normalized volume/liquidity floor across both venues.

  _Filters out thin markets that will move on a single 100-dollar bet._

  ```bash
  prediction-goat-pp-cli liquid --min-volume 100000 --json
  ```
- `movers` - Biggest implied-probability deltas over a 24h or 7d window across both venues.

  _Surfaces narrative shifts (price-driven) vs hype shifts (volume-driven from )._

  ```bash
  prediction-goat-pp-cli movers --window 7d --json
  ```
- `new` - Markets created in the last N days across both venues.

  _Newly listed markets are where the alpha and mispricings live._

  ```bash
  prediction-goat-pp-cli new --days 7 --json
  ```

## Command Reference

comments - Comment system and user interactions

- `prediction-goat-pp-cli comments get-by-id` - Get comments by comment id
- `prediction-goat-pp-cli comments get-by-user-address` - Get comments by user address
- `prediction-goat-pp-cli comments list` - List comments

events - Event management and event-related operations

- `prediction-goat-pp-cli events get` - Get event by id
- `prediction-goat-pp-cli events get-by-slug` - Get event by slug
- `prediction-goat-pp-cli events get-creator` - Get event creator by id
- `prediction-goat-pp-cli events list` - List events
- `prediction-goat-pp-cli events list-creators` - List event creators
- `prediction-goat-pp-cli events list-keyset` - Returns events using cursor-based (keyset) pagination for stable, efficient paging through large result sets. Use...
- `prediction-goat-pp-cli events list-pagination` - List events (paginated)
- `prediction-goat-pp-cli events list-sport-results` - List sport events results

markets - Market data and market-related operations

- `prediction-goat-pp-cli markets get` - Get market by id
- `prediction-goat-pp-cli markets get-abridged` - Query abridged markets by information filters
- `prediction-goat-pp-cli markets get-by-slug` - Get market by slug
- `prediction-goat-pp-cli markets get-information` - Query markets by information filters
- `prediction-goat-pp-cli markets list` - List markets
- `prediction-goat-pp-cli markets list-keyset` - Returns markets using cursor-based (keyset) pagination for stable, efficient paging through large result sets. Use...

profiles - User profile management

- `prediction-goat-pp-cli profiles <user_address>` - Get public profile by user address

public-profile - Manage public profile

- `prediction-goat-pp-cli public-profile` - Get public profile by wallet address

public-search - Manage public search

- `prediction-goat-pp-cli public-search` - Search markets, events, and profiles

series - Series management and related operations

- `prediction-goat-pp-cli series get` - Get series by id
- `prediction-goat-pp-cli series list` - List series

series-summary - Manage series summary

- `prediction-goat-pp-cli series-summary get-by-id` - Get series summary by id
- `prediction-goat-pp-cli series-summary get-by-slug` - Get series summary by slug

sports - Sports-related endpoints including teams and game data

- `prediction-goat-pp-cli sports get-market-types` - Get valid sports market types
- `prediction-goat-pp-cli sports get-metadata` - Get sports metadata information

status - Manage status

- `prediction-goat-pp-cli status` - Gamma API Health check

tags - Tag management and related tag operations

- `prediction-goat-pp-cli tags get` - Get tag by id
- `prediction-goat-pp-cli tags get-by-slug` - Get tag by slug
- `prediction-goat-pp-cli tags get-related-by-slug` - Get related tags (relationships) by tag slug
- `prediction-goat-pp-cli tags get-related-to-atag-by-slug` - Get tags related to a tag slug
- `prediction-goat-pp-cli tags list` - List tags

teams - Manage teams

- `prediction-goat-pp-cli teams get` - Get team by id
- `prediction-goat-pp-cli teams list` - List teams


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
prediction-goat-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match - fall back to `--help` or use a narrower query.

## Recipes


### Find every market for a topic across both venues

```bash
prediction-goat-pp-cli topic kanye --json --select markets.title,markets.venue,markets.yesProbability,markets.endDate
```

Slim ranked bundle of every related PM + Kalshi market. `--select` reduces to four fields per row; an agent sees ~1KB instead of the firehose.

### What's settling this week?

```bash
prediction-goat-pp-cli resolving --week --json --select title,venue,endDate,liquidity --limit 10
```

Local SQL filter on end_date < now+7d across both venues, sorted by liquidity descending. `--select` keeps the response tiny.

### Side-by-side odds comparison

```bash
prediction-goat-pp-cli compare 'arizona basketball' --json
```

Resolves the topic to paired markets and renders YES/NO + implied prob for each venue side-by-side. Tells you exactly where PM and Kalshi disagree.

### Enumerate every child market under a Kalshi event

When the event ticker is known (often discovered via `kalshi-series-search` then `kalshi events list --series`), one call returns every child market with live prices:

```bash
prediction-goat-pp-cli kalshi events get KXMENWORLDCUP-26 --with-markets --agent
```

Passes `with_nested_markets=true` to the upstream `/events/{ticker}` endpoint. The response includes a `markets` array with ticker, title, yes_sub_title, status, yes_ask_dollars, no_ask_dollars, volume_24h_fp, and expiration_time for each child. This is the lightweight alternative to a full sync walk when you only need one event's children.

### List every Kalshi event under a series

```bash
prediction-goat-pp-cli kalshi events list --series KXMENWORLDCUP --agent
```

Filters `/events` by `series_ticker`. The `--series` flag forces `--data-source live` since the local store doesn't index by series ticker. Use this before `kalshi events get --with-markets` when you don't know the exact event ticker — series → event → markets in three calls.

### Catch mispricings across venues

```bash
prediction-goat-pp-cli mispriced --threshold 0.05 --json --select pair.pm.title,pair.kalshi.title,delta
```

Returns same-outcome market pairs where implied probabilities diverge by 5+ percentage points. Slim output via `--select`. Untraded Kalshi markets (those carrying the platform default 17c ask with zero volume) are filtered before pairing so the result is actionable divergence, not noise.

### What are the odds X wins event Y? (event-walk recipe)

When you know one outcome's market slug but want every sibling under the same multi-outcome event (e.g. all 48 World Cup teams or all 14 NBA lottery picks), walk from any known slug to the parent event and back to all siblings:

```bash
prediction-goat-pp-cli polymarket siblings will-ghana-win-the-2026-fifa-world-cup --agent
```

Returns the parent event metadata plus every sibling market with `yesPercent` populated. Bypasses the upstream `/public-search` endpoint which goes stale for celebrity and multi-outcome hub topics. Pair with `polymarket event-of <market-slug>` when you only want the parent event slug.

### Find a Kalshi series by substring

```bash
prediction-goat-pp-cli kalshi-series-search WEST --agent
```

Substring grep over locally synced `kalshi_series` rows. Useful when the FTS ranker buries the series ticker you want (e.g. KXNBAWEST below higher-term-frequency matches). Requires a prior `prediction-goat-pp-cli kalshi sync`.

### Apples-to-apples cross-venue probabilities

Every priced row in JSON output carries both `yesProbability` (0-1 float, canonical machine field) and `yesPercent` (rounded 0-100, for display). Always surface `yesPercent` to humans; use `yesProbability` for math. mispriced pairs additionally carry `deltaPercent` alongside the canonical `delta`. Kalshi markets without real trading (no last price, no volume, wide spread overshooting $1.00) carry `untraded: true` and the text-mode YES column shows `untraded` instead of a misleading platform-default percent.

## Auth Setup

No authentication required.

Run `prediction-goat-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- Pipeable - JSON on stdout, errors on stderr
- Filterable - `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  prediction-goat-pp-cli comments list --agent --select id,name,status
  ```
- Previewable - `--dry-run` shows the request without sending
- Offline-friendly - sync/search commands can use the local SQLite store when available
- Non-interactive - never prompts, every input is a flag
- Explicit retries - use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set - piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Automatic learning

Two-call protocol: `recall` before discovery, `teach &` before emitting. The CLI does entity-aware match validation AND surfaces stored playbooks for the query family; you read the envelope and follow the six-branch decision tree. Skipping either side costs you free recall hits in future sessions.

### Step 1: `recall` before any discovery

Before `topic`, `compare`, `polymarket siblings`, `kalshi events list`, or any other discovery command on a new user question, run:

```bash
prediction-goat-pp-cli recall "<user's question>" --agent
```

The response envelope:

```json
{
  "query": "...",
  "normalized": "world cup",
  "query_entities": ["England"],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|recipe", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "kalshi events get $EVENT --with-markets", "purpose": "..." }, ... ],
      "entity_slots": ["$TEAM", "$EVENT", "$SERIES"],
      "expected_tool_calls": 2
    },
    "slots_resolved": { "$TEAM": { "token": "portugal", "canonical": "Portugal" } },
    "notes": "kalshi events list pages by created_at desc; --series forces --data-source live"
  },
  "notes": "kalshi events list pages by created_at desc; --series forces --data-source live"
}
```

### Step 2: six-branch decision tree

Read `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> Playbook.expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via teach-playbook at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live prices for Results[*].ResourceID in parallel
       (kalshi markets get <ticker>, markets get-by-slug <pm-slug>, etc.)

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (e.g., a "Portugal" learning won't satisfy an "England" query — different canonical)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward AND record
       a playbook + notes via teach-playbook so the next session of the same
       family is faster.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a Results[] hit -- use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

The three playbooks prediction-goat ships out of the box cover the highest-volume query shapes: `odds_for_team` (single-team odds inside a multi-outcome event), `event_markets` (every child market under a specific Kalshi event or Polymarket parent event), and `series_summary` (Kalshi series header + active events + top markets). Each ships with notes capturing the dogfood-discovered gotchas for that shape (untraded-flag filtering, --series forces live data-source, parent-vs-child ticker disambiguation).

### Step 3: always read `warnings`

- `parent_event_when_child_exists`: do NOT fetch the parent. The warning carries the suggested child ticker; fetch that instead even when the parent ticker is in `Results`.
- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity` (top-level): a structurally matching row exists but its canonical entity differs from the live query's (e.g., a Portugal learning when the user asked about England). Treated as cold start; replaces the misleading `no_learnings_for_query_family` warning when this case applies.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity to the user before committing to a ticker.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response

After assembling your user-facing response containing tickers/slugs but BEFORE emitting it, fire the teach call in the background. Append a shell `&` so the call returns immediately and does not block the user-facing response:

```bash
prediction-goat-pp-cli teach --query "<user's question>" --resource <id1> --resource <id2>
```

Silent on success. Teach the **most specific** resource — if you fetched a parent event during discovery but the answer lives in a specific child ticker, teach the child. The CLI writes a `parent_event_when_child_exists` warning to `teach.log` when you don't, surfaced by `learnings list --warnings`. The user never sees it. The CLI uses seeded `entity_lookups` (ISO 3166 country codes plus NFL/NBA/MLB/MLS team rosters with aliases) for cross-alias resolution at recall time, so a teach under "Niners" satisfies future "49ers" queries automatically.

### Step 5: record a playbook when discovery took >5 calls

If you ran more than 5 tool calls to answer the question AND you discovered workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas, untraded-flag noise), record them so the next session of the same query family skips that discovery. Prefer the **integrated one-call form** — record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
prediction-goat-pp-cli teach \
  --query "<user's question>" \
  --resource <id1> \
  --playbook-file ~/playbooks/series-summary.json \
  --playbook-notes-file ~/playbooks/series-summary-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource ticker to record alongside).
prediction-goat-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/series-summary.json \
  --notes-file ~/playbooks/series-summary-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. On the integrated `teach` form, the playbook flags are optional — at least one of them must be set to upsert a playbook, but you can also omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of `--playbook-file` and `--notes`/`--notes-file` must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from "odds Portugal wins world cup" applies to England, Brazil, and every other country query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

The three families prediction-goat already ships with playbooks (`odds_for_team`, `event_markets`, `series_summary`) cover the common shapes; if your query family already has a playbook, prefer `playbook amend` (Step 6) over a full re-teach.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
prediction-goat-pp-cli playbook amend \
  --query "<exact recall query string>" \
  --add-note "<your concrete correction>"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:
- "kalshi events list --series forces --data-source live; local store doesn't index series_ticker"
- "polymarket /public-search goes stale for celebrity hub topics; fall back to polymarket siblings via the parent event"
- "untraded Kalshi markets show 17c platform-default ask with zero volume; filter via `untraded: true` before pairing"
- "compare returns structured unpaired diagnostic when only one venue has a market for the topic; don't read it as 'no data'"

What does NOT belong in notes:
- The year-specific answer ("Portugal is at 8% to win"). That's the response, not a learning.
- Per-country or per-team data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land). Use `playbook list --agent` to inspect what's stored.

### Worked examples

The three traces this protocol was built from:

1. **Cold: "odds USA wins world cup"** — `recall` returns `found=false` (no taught row yet, preseed may have created one). Run discovery (`kalshi-series-search WORLD`, walk children). Answer with `KXMENWORLDCUP-26-US`. Teach the child:

   ```bash
   prediction-goat-pp-cli recall "odds USA wins world cup" --agent
   # found=false -> discovery
   prediction-goat-pp-cli kalshi-series-search WORLD --agent
   prediction-goat-pp-cli kalshi events get KXMENWORLDCUP-26 --with-markets --agent
   # ...respond with KXMENWORLDCUP-26-US...
   prediction-goat-pp-cli teach --query "odds USA wins world cup" --resource KXMENWORLDCUP-26-US
   # (append shell `&` to background it)
   ```

2. **Warm: "odds portugal wins world cup"** — `recall` returns `found=true`, `results[0].entity_match="exact"`, `results[0].confidence>=2`. Skip discovery; fetch live prices in parallel:

   ```bash
   prediction-goat-pp-cli recall "odds portugal wins world cup" --agent
   # found=true, results=[{resource_id: "KXMENWORLDCUP-26-PT", entity_match: "exact"}, ...]
   prediction-goat-pp-cli kalshi markets get KXMENWORLDCUP-26-PT --agent
   ```

3. **Warm-but-mismatched: "odds england wins the world cup"** — `recall` returns `found=false` even though a Portugal learning shares the non-entity tokens. The entity validator filtered the Portugal row into `mismatches` (only visible with `--debug-mismatches`). Treat as cold; the recipe engine may resolve `KXMENWORLDCUP-26-GB` directly via the seeded `country_iso2` lookup; otherwise discover normally.

### `teach-recipe` for explicit template authorship

Recipe inference auto-extracts templates from two structurally-similar teaches (Portugal + USA + the `country_iso2` lookup yields the World Cup template). You can author a template up front:

```bash
prediction-goat-pp-cli teach-recipe \
  --query-template "odds {entity} wins world cup" \
  --resource-template "KXMENWORLDCUP-26-{entity:country_iso2}" \
  --resource-type kalshi_markets
```

Optional — only reach for it when you want a recipe to land before two teaches accumulate, or when the resource template needs a strategy (`*` suffix for prefix search).

### `teach-lookup` for adding entity mappings

ISO country codes and major-league sports rosters are pre-seeded. Use `teach-lookup` only for gaps:

```bash
prediction-goat-pp-cli teach-lookup --kind country_iso2 --canonical "Curaçao" --value CW
```

Computed kinds (`lowercase`, `uppercase`, `kebab-case`, `capitalize-first`, `slug`) are resolved by string transform — they need no rows.

### Disabling learning

- `--no-learn` on a single command: short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `PREDICTION_GOAT_NO_LEARN=true` in the environment: globally disables the pipeline.

### Auditing past teaches

```bash
prediction-goat-pp-cli learnings list --agent             # all rows
prediction-goat-pp-cli learnings list --warnings --agent  # rows whose teach raised a warning (parent-vs-child, no-entity-overlap)
prediction-goat-pp-cli forget "<query>" --resource <id>   # undo one teach
prediction-goat-pp-cli forget "<query>" --all             # undo every teach for that query
```

Use `learnings list --warnings` to find mis-teaches the CLI flagged but didn't block.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
prediction-goat-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
prediction-goat-pp-cli feedback --stdin < notes.txt
prediction-goat-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.prediction-goat-pp-cli/feedback.jsonl`. They are never POSTed unless `PREDICTION_GOAT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PREDICTION_GOAT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
prediction-goat-pp-cli profile save briefing --json
prediction-goat-pp-cli --profile briefing comments list
prediction-goat-pp-cli profile list --json
prediction-goat-pp-cli profile show briefing
prediction-goat-pp-cli profile delete briefing --yes
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

1. Empty, `help`, or `--help` → show `prediction-goat-pp-cli --help` output
2. Starts with `install` → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. Anything else → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add prediction-goat-pp-mcp -- prediction-goat-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which prediction-goat-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   prediction-goat-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `prediction-goat-pp-cli <command> --help`.
