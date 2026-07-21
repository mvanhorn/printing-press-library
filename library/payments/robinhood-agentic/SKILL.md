---
name: pp-robinhood-agentic
description: "Every tool on Robinhood's official Agentic Trading MCP as a typed, review-first CLI — sanctioned OAuth instead of scraped tokens, plus an offline portfolio journal no Robinhood tool ships. Trigger phrases: `check my robinhood portfolio`, `quote AAPL on robinhood`, `review a robinhood order before placing it`, `what's in my robinhood watchlist`, `run my robinhood scanner`, `use robinhood-agentic`, `run robinhood-agentic`."
author: "Kevin Magnan"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - robinhood-agentic-pp-cli
    install:
      - kind: go
        bins: [robinhood-agentic-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/cmd/robinhood-agentic-pp-cli
---

# Robinhood Agentic Trading — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `robinhood-agentic-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install robinhood-agentic --cli-only
   ```
2. Verify: `robinhood-agentic-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/cmd/robinhood-agentic-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every other Robinhood tool rides the reverse-engineered private API with browser-lifted tokens that Robinhood periodically breaks. This CLI speaks the official agentic MCP surface: OAuth login with automatic refresh, server-side order simulation as the default dry-run, the undocumented tools (Level-2 books, financials, tax lots, scanner specs) surfaced as typed commands, and a local SQLite store that turns one-shot MCP calls into portfolio history, order audit trails, and offline search.

## When to Use This CLI

Reach for this CLI whenever a task touches a real Robinhood account through the official agentic surface: checking portfolio state, quoting and screening symbols, reviewing or auditing orders, managing watchlists, or running server-side scanners. It is the safe default for agent-driven Robinhood work — reads span all accounts, writes are triple-gated (env write gate + guard policy + review-first), and everything returns agent-shaped JSON. Do not use it for crypto trading, ACH/money transfers, multi-leg options, a dividends feed, or Robinhood banking/credit-card data: the official MCP does not expose those surfaces.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`portfolio history`** — Answer 'what was my portfolio worth on any given day' from a local time series Robinhood doesn't keep.

  _Reach for this when a task needs portfolio value over time — no MCP tool can answer it._

  ```bash
  robinhood-agentic-pp-cli portfolio history --since 30d --sparkline
  ```
- **`portfolio winrate`** — Round-trip win rate, average win/loss, and per-symbol stats computed from your synced trade history.

  _Journaling and strategy review without exporting to a spreadsheet._

  ```bash
  robinhood-agentic-pp-cli portfolio winrate --account 5XX12345 --by-symbol
  ```
- **`wheel status`** — Per-symbol wheel-strategy stage (cash-secured put → assigned → covered call → called away) inferred automatically.

  _The Friday post-expiration answer to 'what got assigned and what stage is each position in'._

  ```bash
  robinhood-agentic-pp-cli wheel status --account RH123456 AAPL
  ```

### Agent safety rails
- **`guard`** — Set per-trade caps, daily caps, symbol allow/denylists, and a kill switch that the CLI enforces before any order leaves the machine.

  _Use this before letting any agent loop place orders — it is the only enforceable budget/kill-switch layer for the agentic account._

  ```bash
  robinhood-agentic-pp-cli guard set --max-order 500 --daily-cap 2000
  ```
- **`equities settle`** — Resolve an order to verified terminal truth — actual fill price and state — instead of trusting the placement echo.

  _Run after every place or cancel to get the real outcome, not the optimistic echo._

  ```bash
  robinhood-agentic-pp-cli equities settle 1a2b3c4d-5678-90ab-cdef-1234567890ab --account RH123456 --wait
  ```
- **`audit`** — See everything the CLI (or an agent driving it) reviewed, placed, canceled, or was denied — with idempotency keys and outcomes.

  _The weekly agent-accountability review: reconstruct exactly what an automation did._

  ```bash
  robinhood-agentic-pp-cli audit --since 7d --denied
  ```

### Agent-native rituals
- **`brief`** — The whole pre-open check — portfolio value, day-over-day delta, open orders, positions, top movers among your holdings, and upcoming earnings for held symbols — in one command.

  _The one-command replacement for the four-round-trip morning ritual._

  ```bash
  robinhood-agentic-pp-cli brief --account RH123456 --agent
  ```
- **`surface diff`** — Know when Robinhood adds, removes, or reshapes MCP tools — with dates — instead of discovering breakage in production.

  _Check after any unexplained failure: the tool surface is beta and moves without notice._

  ```bash
  robinhood-agentic-pp-cli surface diff
  ```

## Command Reference

**accounts** — Brokerage accounts and the agentic-account boundary

- `robinhood-agentic-pp-cli accounts` — List all brokerage accounts; agentic_allowed marks the only account that can place agentic orders

**equities** — Equity positions, orders, tax lots, and the review-first order lifecycle

- `robinhood-agentic-pp-cli equities cancel` — Request cancellation of an open equity order.
- `robinhood-agentic-pp-cli equities orders` — Equity order history and single-order lookup, newest first; executions[] carries fills
- `robinhood-agentic-pp-cli equities place` — Place a REAL equity order in the agentic account. Same parameters as review plus a ref_id idempotency key.
- `robinhood-agentic-pp-cli equities positions` — Open equity positions with quantity, average cost, and hold breakdowns (pair with quotes for market value
- `robinhood-agentic-pp-cli equities review` — Server-side order simulation: returns Robinhood's pre-trade warnings WITHOUT placing anything.
- `robinhood-agentic-pp-cli equities taxlots` — Open acquisition lots for ONE symbol: lot id, quantity, cost basis, acquisition date, holding period.

**market** — Search, quotes, fundamentals, historicals, indicators, earnings, indexes

- `robinhood-agentic-pp-cli market book` — Level-2 bid/ask depth (max 4 symbols per call)
- `robinhood-agentic-pp-cli market earnings-calendar` — Market-wide earnings events in a window of up to 31 days
- `robinhood-agentic-pp-cli market earnings-results` — Up to 8 quarters of EPS history (estimate vs actual) plus the next report date for one symbol
- `robinhood-agentic-pp-cli market financials` — Reported quarterly or annual revenue, gross profit, net income, and net margin (max 20 symbols, 40 periods)
- `robinhood-agentic-pp-cli market fundamentals` — Valuation, market cap, session OHLCV, 52-week range, dividends, and company profile (max 10 symbols)
- `robinhood-agentic-pp-cli market historicals` — OHLCV bars for up to 10 symbols; intervals from 15second to 50year; bounds cover regular, extended, and 24/7 sessions
- `robinhood-agentic-pp-cli market index-quotes` — Real-time index levels by instrument UUID (from market indexes; match responses by instrument_id, not symbol)
- `robinhood-agentic-pp-cli market indexes` — Look up market indexes by symbol (comma-separated string; unmatched symbols are silently dropped)
- `robinhood-agentic-pp-cli market indicators` — Server-computed technical indicators (18 types) for one symbol
- `robinhood-agentic-pp-cli market quotes` — Real-time equity quotes with official prior close (max 20 symbols per call; beyond 20 the close blocks are omitted)
- `robinhood-agentic-pp-cli market search` — Search instruments, currency pairs, or market indexes by name or ticker
- `robinhood-agentic-pp-cli market tradability` — Per-account tradability: fractional, extended-hours, all-day, and short-selling flags (max 10 symbols)

**options** — Option chains, contracts, quotes, positions, orders, and the option watchlist

- `robinhood-agentic-pp-cli options cancel` — Request cancellation of an open option order; accepted=true can race a fill — re-read for terminal state
- `robinhood-agentic-pp-cli options chains` — Option chains for an underlying: chain id, expiration dates, multiplier, underlying instruments
- `robinhood-agentic-pp-cli options instruments` — Option contract discovery with expiry/strike/type filters; returns contract UUIDs used by quotes and orders
- `robinhood-agentic-pp-cli options orders` — Option order history with state and chain filters
- `robinhood-agentic-pp-cli options place` — Place a REAL single-leg option order in the agentic account (multi-leg unsupported by the MCP).
- `robinhood-agentic-pp-cli options positions` — Option positions with type/expiry/chain filters
- `robinhood-agentic-pp-cli options quotes` — Live option quotes and prior-session closes by contract UUID
- `robinhood-agentic-pp-cli options review` — Server-side option order simulation (single-leg): pre-trade warnings without placing
- `robinhood-agentic-pp-cli options upgrade-info` — Options-access level for the account and the URL to apply for an upgrade
- `robinhood-agentic-pp-cli options watchlist` — The separate options watchlist (errors if options trading is not enabled on the account)
- `robinhood-agentic-pp-cli options watchlist-add` — Add option contracts to the options watchlist by contract UUID
- `robinhood-agentic-pp-cli options watchlist-remove` — Remove option contracts from the options watchlist (position type must match how they were added)

**portfolio** — Portfolio value, buying power, and P&L

- `robinhood-agentic-pp-cli portfolio pnl-trades` — Trade-by-trade realized P&L history, cursor-paginated, newest first (RHS account number)
- `robinhood-agentic-pp-cli portfolio realized-pnl` — Realized P&L buckets by span or date range.
- `robinhood-agentic-pp-cli portfolio show` — Per-account portfolio value breakdown by asset class plus authoritative buying power (get_accounts buying power is

**scans** — Server-side market scanners and the runtime-discoverable filter DSL

- `robinhood-agentic-pp-cli scans create` — Create a saved scan (validate filters against scans specs first)
- `robinhood-agentic-pp-cli scans list` — Saved scans with their filters, columns, and sort configuration
- `robinhood-agentic-pp-cli scans run` — Run a saved scan and return matching instruments
- `robinhood-agentic-pp-cli scans set-config` — Update a saved scan's name, columns, or sort configuration
- `robinhood-agentic-pp-cli scans set-filters` — Replace a saved scan's filter set
- `robinhood-agentic-pp-cli scans specs` — The valid scanner filter types, predicates, and parameter shapes — the scan-filter DSL is discoverable at runtime

**watchlists** — Custom and curated watchlists

- `robinhood-agentic-pp-cli watchlists add` — Add members to a custom watchlist — exactly one of symbols, currency pair ids, or index ids
- `robinhood-agentic-pp-cli watchlists create` — Create a custom watchlist (display name must be unique)
- `robinhood-agentic-pp-cli watchlists follow` — Follow a curated watchlist (cannot follow your own custom lists; a follow-limit error means unfollow another first)
- `robinhood-agentic-pp-cli watchlists items` — Members of one watchlist (instruments, currency pairs, indexes
- `robinhood-agentic-pp-cli watchlists list` — All watchlists: custom (user-writable) and robinhood-curated (manage via follow/unfollow)
- `robinhood-agentic-pp-cli watchlists popular` — Robinhood-curated popular watchlists available to follow
- `robinhood-agentic-pp-cli watchlists remove` — Remove members from a custom watchlist — exactly one of symbols, currency pair ids, or index ids
- `robinhood-agentic-pp-cli watchlists unfollow` — Unfollow a curated watchlist
- `robinhood-agentic-pp-cli watchlists update` — Update a CUSTOM watchlist's name, description, or emoji (curated lists are read-only)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
robinhood-agentic-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Agent morning brief, trimmed

```bash
robinhood-agentic-pp-cli brief --account RH123456 --agent --select portfolio.total_value,delta,open_orders,movers
```

The whole pre-open check as compact JSON, narrowed with dotted --select paths so an agent doesn't parse kilobytes it won't use.

### Review an order without placing it

```bash
robinhood-agentic-pp-cli equities review --account RH123456 --symbol AAPL --side buy --type limit --quantity 1 --limit-price 180
```

Server-side simulation returns Robinhood's own pre-trade warnings; nothing is placed.

### Set guardrails before an agent session

```bash
robinhood-agentic-pp-cli guard set --max-order 500 --daily-cap 2000
```

Client-side caps the CLI enforces on every subsequent place command — the platform has no native equivalent.

### Watchlist quote board

```bash
robinhood-agentic-pp-cli watchlists quotes 11111111-2222-3333-4444-555555555555 --csv
```

Joins a watchlist's members to live quotes in one command (get the list id from `watchlists list` first) and emits CSV for spreadsheets.

### Portfolio sparkline

```bash
robinhood-agentic-pp-cli portfolio history --since 30d --sparkline
```

30 days of locally-snapshotted portfolio value — a series Robinhood's API cannot return.

## Auth Setup

First run: `auth login` self-registers a public OAuth client against Robinhood's dynamic-registration endpoint (no shipped secrets), opens your browser for the PKCE authorization on robinhood.com, catches the localhost callback, and stores access + refresh tokens in your config with automatic refresh (~4-day access tokens). `auth status` shows expiry; `ROBINHOOD_AGENTIC_TOKEN` overrides for CI. Reads and `review` (server-side order simulation) are always allowed. Order placement and cancellation, watchlist writes, and scan writes are blocked at the transport unless `ROBINHOOD_AGENTIC_PP_ALLOW_WRITES=1` is set — the hard floor that keeps read-only testing safe by construction. On top of that gate, the `guard` policy adds per-order and daily notional caps, a symbol allow/denylist, and a kill switch, all enforced locally before any order leaves the machine, and every mutation is recorded to the local write journal (`audit`). Recommended flow: `equities review` first, then set the write gate to place for real.

Run `robinhood-agentic-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  robinhood-agentic-pp-cli accounts --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `ROBINHOOD_AGENTIC_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `ROBINHOOD_AGENTIC_CONFIG_DIR`, `ROBINHOOD_AGENTIC_DATA_DIR`, `ROBINHOOD_AGENTIC_STATE_DIR`, `ROBINHOOD_AGENTIC_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `ROBINHOOD_AGENTIC_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `robinhood-agentic-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "robinhood-agentic": {
        "command": "robinhood-agentic-pp-mcp",
        "env": {
          "ROBINHOOD_AGENTIC_HOME": "/srv/robinhood-agentic"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `ROBINHOOD_AGENTIC_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `ROBINHOOD_AGENTIC_HOME`, or `doctor` will not find credentials left under the former root.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
robinhood-agentic-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "robinhood-agentic-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `robinhood-agentic-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `robinhood-agentic-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities — direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "USA" teach satisfying a "United States" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Cards" → Arizona Cardinals + St. Louis Cardinals). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `robinhood-agentic-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
robinhood-agentic-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
robinhood-agentic-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
robinhood-agentic-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
robinhood-agentic-pp-cli playbook amend \
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

`robinhood-agentic-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `ROBINHOOD_AGENTIC_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
robinhood-agentic-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
robinhood-agentic-pp-cli feedback --stdin < notes.txt
robinhood-agentic-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `ROBINHOOD_AGENTIC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ROBINHOOD_AGENTIC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
robinhood-agentic-pp-cli profile save briefing --json
robinhood-agentic-pp-cli --profile briefing accounts
robinhood-agentic-pp-cli profile list --json
robinhood-agentic-pp-cli profile show briefing
robinhood-agentic-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `robinhood-agentic-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/cmd/robinhood-agentic-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add robinhood-agentic-pp-mcp -- robinhood-agentic-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which robinhood-agentic-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   robinhood-agentic-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `robinhood-agentic-pp-cli <command> --help`.
