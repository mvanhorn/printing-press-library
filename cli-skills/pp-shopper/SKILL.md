---
name: pp-shopper
description: "Every Shopper storefront in one CLI — catalog, cart, delivery schedule, charge calendar, and spend analytics no web UI surfaces. Trigger phrases: `check my Shopper basket`, `when is my next Shopper delivery`, `when will Shopper charge me`, `show my Shopper spend`, `use shopper-pp-cli`, `search Shopper catalog`, `add item to Shopper cart`, `Shopper charge calendar`, `Shopper cashback tier`."
author: "educrvz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - shopper-pp-cli
    install:
      - kind: go
        bins: [shopper-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/commerce/shopper/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Shopper — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `shopper-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install shopper --cli-only
   ```
2. Verify: `shopper-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

shopper-pp-cli covers all six Shopper storefronts (Compra Programada, Fresh, Pet, Compra Única, Now, Now Bebidas) with correct store/cluster scoping, full siteapi REST surface, browser-deep-link helpers for subscription mutations, and a local SQLite layer for offline product search, basket diffs, price tracking, and cross-store spend rollup.

## When to Use This CLI

Use shopper-pp-cli when automating Shopper basket management, monitoring charge schedules, analyzing spend patterns across storefronts, or building agent workflows around the Shopper subscription cycle. Ideal for recurring-basket optimization, pre-cycle audit, and delivery-schedule awareness.

## Anti-triggers

Do not use this CLI for:
- Do not use for placing orders or confirming checkout — use checkout open to hand off to the browser
- Do not use for adding or removing saved credit cards — card management requires browser session
- Do not use for ultra-fast delivery slot selection on now/now-bebidas — delivery slots for these stores require the browser checkout flow
- Do not use for cancelling orders — cancellation is not available via siteapi REST

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Subscription intelligence
- **`charge-calendar`** — Every upcoming cycle's charge date, edit-lock deadline, and delivery date in one timeline so you never miss an edit window or get surprised by a charge.

  _Use when an agent needs to know whether the edit window is still open before modifying a recurring basket._

  ```bash
  shopper-pp-cli charge-calendar --store programada --agent
  ```
- **`basket diff`** — Compares your current recurring basket against a previous cycle's snapshot to show exactly what was added, dropped, or re-quantified before the template locks.

  _Use when an agent needs to verify what changed in the basket since the last confirmed delivery cycle._

  ```bash
  shopper-pp-cli basket diff --store programada --agent
  ```
- **`cashback optimize`** — Computes the cheapest set of items to add (or whether to wait) to cross the next cashback tier, favoring things you'll need anyway.

  _Use when an agent is finalising a basket and wants to maximise cashback return before the edit window closes._

  ```bash
  shopper-pp-cli cashback optimize --tier 2399 --store programada --agent
  ```

### Local state that compounds
- **`price-watch`** — Tracks the price history of the SKUs you actually buy and alerts when one rises or drops meaningfully versus your own purchase baseline.

  _Use when an agent needs to know if a subscribed product has changed price significantly since the last order._

  ```bash
  shopper-pp-cli price-watch --store programada --agent
  ```
- **`restock predict`** — Predicts when you'll run out of each staple from your historical buying cadence and suggests what to add to the upcoming basket.

  _Use when an agent needs to pre-populate a recurring basket with items likely running low before the next cycle._

  ```bash
  shopper-pp-cli restock predict --store programada --agent
  ```
- **`catalog drift`** — Flags products you buy that were discontinued, silently swapped, or kept their price while shrinking the pack, surfacing the real R$/kg or R$/L change.

  _Use when an agent needs to audit whether the recurring basket still contains the same products as originally added._

  ```bash
  shopper-pp-cli catalog drift --store programada --agent
  ```

### Customer journey plumbing
- **`checkout preview`** — Aggregates cart totals, next delivery date, charge date, minimum-order status, and accepted payment types into one pre-checkout view before you open the browser.

  _Use when an agent needs to confirm basket readiness and charge schedule before directing the user to open the browser for final payment._

  ```bash
  shopper-pp-cli checkout preview --store programada --agent
  ```

## Command Reference

**address** — Saved delivery addresses with per-address available-store information

- `shopper-pp-cli address` — List delivery addresses and which stores are available at each address

**cart** — Cart: view summary, add products, remove products

- `shopper-pp-cli cart add` — Add a product to the cart or increase its quantity
- `shopper-pp-cli cart list-summary` — Show current basket: items, quantities, totals, cashback, and minimum-order status
- `shopper-pp-cli cart remove` — Remove a product from the cart or decrease its quantity

**catalog** — Product catalog: search, departments, banners, suggestions

- `shopper-pp-cli catalog create-count` — Count products matching a search query and optional filters
- `shopper-pp-cli catalog create-filters` — Get available filter options for a search query
- `shopper-pp-cli catalog create-search` — Search the product catalog by query with optional brand/type/metadata filters
- `shopper-pp-cli catalog get-view` — Get details for a specific catalog banner
- `shopper-pp-cli catalog list-banners` — List promotional banners for the current store
- `shopper-pp-cli catalog list-departments` — List product departments/categories for the current store
- `shopper-pp-cli catalog list-news` — List new product arrivals for the current store
- `shopper-pp-cli catalog list-suggest` — Get search suggestions for a query prefix

**delivery** — Delivery schedule: upcoming delivery date, edit-lock window, and reschedule calendar

- `shopper-pp-cli delivery list-calendar` — Get delivery reschedule calendar — allowed date range and disabled days
- `shopper-pp-cli delivery list-summary` — Show scheduled delivery date, current delivery status, and store message

**features** — Storefront configuration, feature toggles, and timer state

- `shopper-pp-cli features create-select` — Select active store (sets session context; no-op for header-scoped reads)
- `shopper-pp-cli features create-start` — Start a named feature timer
- `shopper-pp-cli features create-view` — Mark a feature toggle as viewed
- `shopper-pp-cli features list-stores` — List all available storefronts with store IDs, cluster IDs, payment parameters, and feature flags
- `shopper-pp-cli features list-tick` — Get current timer state
- `shopper-pp-cli features list-toggle` — Get active feature toggles for the current store

**orders** — Purchase history and spend — reads from GET /orders/orders (web 'Histórico de compras')

- `shopper-pp-cli orders` — List past orders for the active store (newest-first, paginated by size)

**session** — Session and social-login validation

- `shopper-pp-cli session` — Validate social-login session status


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `SHOPPER_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `shopper-pp-cli address`
- `shopper-pp-cli address get`
- `shopper-pp-cli address list`
- `shopper-pp-cli address search`
- `shopper-pp-cli cart`
- `shopper-pp-cli cart get`
- `shopper-pp-cli cart list`
- `shopper-pp-cli cart search`
- `shopper-pp-cli catalog`
- `shopper-pp-cli catalog get`
- `shopper-pp-cli catalog list`
- `shopper-pp-cli catalog search`
- `shopper-pp-cli catalog-departments`
- `shopper-pp-cli catalog-departments get`
- `shopper-pp-cli catalog-departments list`
- `shopper-pp-cli catalog-departments search`
- `shopper-pp-cli catalog-products-news`
- `shopper-pp-cli catalog-products-news get`
- `shopper-pp-cli catalog-products-news list`
- `shopper-pp-cli catalog-products-news search`
- `shopper-pp-cli catalog-search-suggest`
- `shopper-pp-cli catalog-search-suggest get`
- `shopper-pp-cli catalog-search-suggest list`
- `shopper-pp-cli catalog-search-suggest search`
- `shopper-pp-cli delivery`
- `shopper-pp-cli delivery get`
- `shopper-pp-cli delivery list`
- `shopper-pp-cli delivery search`
- `shopper-pp-cli delivery-v2-calendar`
- `shopper-pp-cli delivery-v2-calendar get`
- `shopper-pp-cli delivery-v2-calendar list`
- `shopper-pp-cli delivery-v2-calendar search`
- `shopper-pp-cli features`
- `shopper-pp-cli features get`
- `shopper-pp-cli features list`
- `shopper-pp-cli features search`
- `shopper-pp-cli features-timer-tick`
- `shopper-pp-cli features-timer-tick get`
- `shopper-pp-cli features-timer-tick list`
- `shopper-pp-cli features-timer-tick search`
- `shopper-pp-cli features-toggle`
- `shopper-pp-cli features-toggle get`
- `shopper-pp-cli features-toggle list`
- `shopper-pp-cli features-toggle search`
- `shopper-pp-cli orders`
- `shopper-pp-cli orders get`
- `shopper-pp-cli orders list`
- `shopper-pp-cli orders search`
- `shopper-pp-cli session`
- `shopper-pp-cli session get`
- `shopper-pp-cli session list`
- `shopper-pp-cli session search`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
shopper-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Hand-written Extensions

These commands are declared by the spec author and require separate hand-written wiring; the generator does not emit Cobra registration for them. They are listed here for discoverability and are intentionally outside `## Command Reference` so the verify-skill unknown-command check does not treat them as generator-owned paths.

- `shopper-pp-cli stores` — List all available Shopper storefronts with IDs, cluster IDs, payment parameters, and capability flags
- `shopper-pp-cli checkout preview` — Pre-checkout summary: basket totals, delivery date, charge date, min-order status, and accepted payment types (read-only
- `shopper-pp-cli checkout open` — Open the Shopper checkout page in the system browser (subscription-based payment via session login)
- `shopper-pp-cli delivery reschedule [--store <store>]` — Open the delivery reschedule calendar in the browser (POST /shop/minha-conta/alterar-data requires session cookie)
- `shopper-pp-cli delivery skip` — Open the skip-delivery page in the browser — subscription stores only (POST /shop/minha-conta/pular-entrega/ requires
- `shopper-pp-cli delivery suspend` — Open the suspend-subscription page in the browser (POST /shop/minha-conta/suspender-entrega requires session cookie)
- `shopper-pp-cli delivery boleto` — Open the boleto-retrieval page in the browser to get the bank slip for the current order
- `shopper-pp-cli subscription pause` — Open the subscription-pause page in the browser (POST /shop/carrinho/pause/ requires session cookie)
- `shopper-pp-cli subscription resume` — Open the subscription-resume page in the browser (POST /shop/carrinho/play/ requires session cookie)
- `shopper-pp-cli payment cards` — Show saved payment card count and open card-management in the browser (card add/delete is browser-required

## Recipes

### Check if the edit window is still open

```bash
shopper-pp-cli charge-calendar --store programada --agent --select next_delivery_date,edit_lock_date,charge_date
```

Returns the key dates for the next cycle; compare edit_lock_date to today to know if the basket can still be changed.

### Find items to add for next cashback tier

```bash
shopper-pp-cli cashback optimize --store programada --agent
```

Computes the cheapest catalog additions to cross the next cashback threshold using your live cart and synced product data.

### Cross-store spend rollup for last 12 months

```bash
shopper-pp-cli orders spend --agent --select store,month,total
```

Queries all 6 storefronts and returns a month-by-store spend matrix for budgeting analysis.

### Detect if a subscribed product changed price

```bash
shopper-pp-cli price-watch --store programada --agent --select product_id,name,price_now,price_baseline,change_pct
```

Scans synced price history for products in your basket and flags meaningful price movements.

### Pre-checkout summary with charge date

```bash
shopper-pp-cli checkout preview --store programada --agent --select total_amount,delivery_date,charge_date,min_order_met,payment_methods
```

Aggregates cart/delivery/payment data so an agent can confirm basket readiness before directing the user to open the checkout browser.

## Auth Setup

Set SHOPPER_TOKEN to your Shopper JWT (from the siteapi Authorization header in browser DevTools). Run 'shopper-pp-cli doctor' to verify. The token is long-lived for the subscription cycle. Never store raw card numbers or CPF in config; card management always opens the browser.

Run `shopper-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  shopper-pp-cli address --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SHOPPER_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SHOPPER_CONFIG_DIR`, `SHOPPER_DATA_DIR`, `SHOPPER_STATE_DIR`, `SHOPPER_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SHOPPER_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `shopper-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "shopper": {
        "command": "shopper-pp-mcp",
        "env": {
          "SHOPPER_HOME": "/srv/shopper"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SHOPPER_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SHOPPER_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
shopper-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "shopper-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `shopper-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `shopper-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `shopper-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
shopper-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
shopper-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
shopper-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
shopper-pp-cli playbook amend \
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

`shopper-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `SHOPPER_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
shopper-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
shopper-pp-cli feedback --stdin < notes.txt
shopper-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SHOPPER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SHOPPER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
shopper-pp-cli profile save briefing --json
shopper-pp-cli --profile briefing address
shopper-pp-cli profile list --json
shopper-pp-cli profile show briefing
shopper-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `shopper-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/commerce/shopper/cmd/shopper-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add shopper-pp-mcp -- shopper-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which shopper-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   shopper-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `shopper-pp-cli <command> --help`.
