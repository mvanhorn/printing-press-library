---
name: pp-psx
description: "Every PSX data-portal surface in one Go binary, plus the local price history the exchange itself throws away. Trigger phrases: `KSE-100 level today`, `quote OGDC`, `what changed in my PSX watchlist`, `PSX dividend announcements this week`, `which PSX sector is leading`, `Pakistan Stock Exchange historical data`, `use psx`, `run psx`, `PSX listing rules PDF`."
author: "qazmataz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - psx-pp-cli
    install:
      - kind: go
        bins: [psx-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/psx/cmd/psx-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/payments/psx/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Pakistan Stock Exchange — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `psx-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install psx --cli-only
   ```
2. Verify: `psx-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/psx/cmd/psx-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The PSX data portal renders only what is true right now and keeps nothing for you. This CLI mirrors the instrument master, screener metrics, sector aggregates and market snapshots into local SQLite, so longitudinal questions the portal structurally cannot answer become one command: what changed since last week (diff), which names are abnormal against their own history (unusual), and when a valuation multiple compressed (drift). Announcements, payouts and OHLCV history are read live rather than mirrored. No API key, no account, no browser.

## When to Use This CLI

Use this CLI for any question about Pakistan Stock Exchange instruments, prices, indices, sectors, valuation metrics, corporate actions or debt instruments. It is strongest when the question is longitudinal or cross-entity, because it keeps a local mirror the exchange portal does not. Note that longitudinal commands (diff, drift, unusual, rotation) read snapshots captured by 'snapshot take', so run that on a schedule before expecting trend answers. It is strictly read-only: PSX exposes no write surface and this CLI never mutates anything upstream.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to place, modify or cancel trades. PSX exposes no order-entry surface and this CLI is strictly read-only.
- Do not use it for non-PSX markets. It covers Pakistan Stock Exchange only, not NSE India, NYSE, or crypto venues.
- Do not use it for real-time streaming or tick-by-tick execution data. The portal is poll-based and the CLI paces requests conservatively.
- Do not use it as a source of investment advice or Shariah-compliance verdicts. It reports exchange data; interpretation is the caller's responsibility.
- Do not use it for portfolio profit-and-loss against purchase prices. PSX exposes no trade or cost-basis data.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-surface joins
- **`actions`** — See every announcement, payout, AGM date and circuit-breaker event for the symbols you track, in one place.

  _Reach for this when the question is 'did anything happen to my names', instead of running four searches and merging them by hand._

  ```bash
  psx-pp-cli actions --watchlist --since 7d --agent
  ```
- **`basis`** — Compare futures-board prices against the regular spot board to see premium or discount per symbol.

  _Use this for carry and roll questions that require crossing PSX's separate market boards._

  ```bash
  psx-pp-cli basis --market DFC --top 20 --agent
  ```
- **`docs search`** — Search PSX regulatory documents, listing guides and notices by keyword, with last-updated dates.

  _Use this to locate the governing PSX rule, guide or notice for a question instead of browsing the corporate site by hand._

  ```bash
  psx-pp-cli docs search "rule book" --agent
  ```

### Local state that compounds
- **`diff`** — Show what changed in price, volume and valuation metrics between two synced snapshots.

  _Use this when the user asks what moved since last week rather than what the price is right now._

  ```bash
  psx-pp-cli diff --since 7d --agent
  ```
- **`watchlist`** — Keep a saved symbol set and price it on demand, with change measured from the day you added each name.

  _Use this to give an otherwise stateless market portal a memory of which names the user actually cares about._

  ```bash
  psx-pp-cli watchlist prices --agent
  ```
- **`drift`** — Trace one symbol's PE, dividend yield, market cap or free float across time.

  _Use this when the question is about the trajectory of a multiple, not its current value._

  ```bash
  psx-pp-cli drift OGDC --metric pe --since 90d --agent
  ```

### Baselines the portal never keeps
- **`unusual`** — Find names trading abnormally against their own trailing history, not just the day's biggest movers.

  _Use this to separate a genuine volume spike from a name that always trades that way._

  ```bash
  psx-pp-cli unusual --baseline 30d --top 20 --agent
  ```
- **`rotation`** — Rank sectors by movement over a window and name the constituents that drove each move.

  _Use this for 'what is leading and lagging, and why' instead of a bare current-volume ranking._

  ```bash
  psx-pp-cli rotation --window 30d --top 5 --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 12 API entries from 235 total network entries
- Protocols: rest_json (75% confidence)
- Generation hints: weak_schema_confidence
- Candidate command ideas: create_calendar — Derived from observed POST /calendar traffic.; list_KSE100 — Derived from observed GET /timeseries/int/KSE100 traffic.; list_OGDC — Derived from observed GET /timeseries/int/OGDC traffic.; list_symbols — Derived from observed GET /symbols traffic.; list_top_10_sectors — Derived from observed GET /data/top-10-sectors traffic.
- Caveats: empty_response_shapes: All endpoint clusters have empty response shapes; capture may have omitted response bodies. Re-fetch discovered endpoints with curl or another direct HTTP path before generating types.; weak_schema_evidence: Binary or protobuf response cannot provide reliable JSON schema evidence.

## Command Reference

**announcements** — Corporate and regulatory announcement streams

- `psx-pp-cli announcements` — Search announcements across company, PSX, SECP, NCCPL and CDC streams

**board** — Live trading boards with bid/offer depth across markets

- `psx-pp-cli board <market> <board>` — Trading board for one market and board combination

**calendar** — AGM and EOGM meeting calendar

- `psx-pp-cli calendar` — AGM and EOGM events in a date window

**circuit-breakers** — Securities that hit price circuit breakers

- `psx-pp-cli circuit-breakers` — Instruments halted by a price circuit breaker, with the limit band and halt state that triggered the suspension

**company** — Per-company profile and financial report index

- `psx-pp-cli company profile` — Company profile and quote page for an instrument
- `psx-pp-cli company reports` — Financial report index for an instrument

**debt** — Debt market instruments

- `psx-pp-cli debt` — TFCs, Sukuks and other debt instruments with coupon, maturity and yields

**eligible-scrips** — Margin-trading eligible securities

- `psx-pp-cli eligible-scrips` — Securities eligible for margin trading and margin financing

**history** — Full historical OHLCV series

- `psx-pp-cli history` — Full OHLCV history for a symbol. PSX ignores date parameters and always returns the complete series

**indices** — Index listing and constituents

- `psx-pp-cli indices` — All PSX indices with current level and change

**listings** — Listing status by board

- `psx-pp-cli listings <board> <status>` — Listed companies filtered by board and listing status

**market** — Whole-market snapshots and movers

- `psx-pp-cli market debt-performers` — Most active debt instruments by turnover, covering TFCs, Sukuks and government paper on the bonds board
- `psx-pp-cli market performers` — Top active, gainer and loser tables for the equity market
- `psx-pp-cli market watch` — Whole-market snapshot table: LDCP, open, high, low, current, change, volume

**payouts** — Dividend, bonus and right-issue payout history

- `psx-pp-cli payouts company` — Full payout history for one instrument
- `psx-pp-cli payouts list` — Payout history across all instruments

**screener** — Fundamental screening metrics for the listed universe

- `psx-pp-cli screener` — Market cap, price, PE (TTM), dividend yield, free float, 30-day average volume and 1-year change

**sectors** — Sector aggregates and rankings

- `psx-pp-cli sectors summary` — Per-sector aggregates: advances, declines, turnover and market cap
- `psx-pp-cli sectors top` — Top sectors ranked by traded volume

**symbols** — Listed instrument master: equities, ETFs and debt securities

- `psx-pp-cli symbols` — List every listed instrument with sector, ETF and debt flags

**timeseries** — Intraday tick and end-of-day price series

- `psx-pp-cli timeseries eod` — End-of-day series as [epoch, close, volume, open] tuples
- `psx-pp-cli timeseries intraday` — Intraday tick series as [epoch, price, volume] tuples (last active session)


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `PSX_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `psx-pp-cli circuit-breakers`
- `psx-pp-cli circuit-breakers get`
- `psx-pp-cli circuit-breakers list`
- `psx-pp-cli circuit-breakers search`
- `psx-pp-cli debt`
- `psx-pp-cli debt get`
- `psx-pp-cli debt list`
- `psx-pp-cli debt search`
- `psx-pp-cli eligible-scrips`
- `psx-pp-cli eligible-scrips get`
- `psx-pp-cli eligible-scrips list`
- `psx-pp-cli eligible-scrips search`
- `psx-pp-cli indices`
- `psx-pp-cli indices get`
- `psx-pp-cli indices list`
- `psx-pp-cli indices search`
- `psx-pp-cli screener`
- `psx-pp-cli screener get`
- `psx-pp-cli screener list`
- `psx-pp-cli screener search`
- `psx-pp-cli sectors`
- `psx-pp-cli sectors get`
- `psx-pp-cli sectors list`
- `psx-pp-cli sectors search`
- `psx-pp-cli symbols`
- `psx-pp-cli symbols get`
- `psx-pp-cli symbols list`
- `psx-pp-cli symbols search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
psx-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Weekly corporate-action sweep for your holdings

```bash
psx-pp-cli actions --watchlist --since 7d --agent --select symbol,kind,date,headline
```

Joins announcements, payouts, AGM dates and breaker events for saved symbols, narrowed with --select so an agent reads four fields instead of a large nested payload.

### Find genuinely abnormal volume, not just big movers

```bash
psx-pp-cli unusual --baseline 30d --top 20 --agent
```

Ranks each name against its own trailing median and dispersion, so a thin scrip doubling on tiny size does not crowd out a real institutional print.

### Trace a valuation multiple over a quarter

```bash
psx-pp-cli drift OGDC --metric pe --since 90d --agent
```

Reads accumulated screener snapshots to show when the multiple moved, which the portal cannot answer at all.

### See which constituents drove a sector

```bash
psx-pp-cli rotation --window 30d --top 5 --agent
```

Ranks sectors by change over the window and attributes each move to its largest contributors.

### Check futures premium to spot

```bash
psx-pp-cli basis --market DFC --top 20 --agent
```

Fetches the futures and regular boards and computes the per-symbol spread and percent premium.

### Find the governing PSX rulebook or guide

```bash
psx-pp-cli docs search "shariah compliant" --agent --select title,url,updated
```

Full-text search across the corporate site's document map, returning just the fields an agent needs to cite a source.

## Auth Setup

PSX publishes no API and requires no credentials. Every endpoint this CLI uses is unauthenticated plain HTTP, so there is nothing to configure before your first command. The CLI paces itself to stay within the portal's observed politeness ceiling.

Run `psx-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  psx-pp-cli announcements --agent
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

- Use `--home <dir>` for one invocation, or set `PSX_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PSX_CONFIG_DIR`, `PSX_DATA_DIR`, `PSX_STATE_DIR`, `PSX_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PSX_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `psx-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "psx": {
        "command": "psx-pp-mcp",
        "env": {
          "PSX_HOME": "/srv/psx"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PSX_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PSX_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
psx-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "psx-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `psx-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `psx-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `psx-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
psx-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
psx-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
psx-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
psx-pp-cli playbook amend \
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

`psx-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `PSX_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
psx-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
psx-pp-cli feedback --stdin < notes.txt
psx-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PSX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PSX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
psx-pp-cli profile save briefing --json
psx-pp-cli --profile briefing announcements
psx-pp-cli profile list --json
psx-pp-cli profile show briefing
psx-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `psx-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/psx/cmd/psx-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add psx-pp-mcp -- psx-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which psx-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   psx-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `psx-pp-cli <command> --help`.
