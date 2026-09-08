---
name: pp-mufap
description: "Pakistan's mutual fund industry as a dated local panel: two decades of daily NAVs, monthly PKR asset allocation, and a market-implied short rate no other tool derives. Trigger phrases: `pakistan mutual fund nav`, `mufap fund returns`, `money market fund yields pakistan`, `mutual fund asset allocation pakistan`, `pakistan fund industry aum`, `use mufap`, `run mufap`."
author: "qazmataz"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - mufap-pp-cli
    install:
      - kind: go
        bins: [mufap-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/payments/mufap/cmd/mufap-pp-cli
---

# MUFAP — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `mufap-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install mufap --cli-only
   ```
2. Verify: `mufap-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/payments/mufap/cmd/mufap-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

MUFAP publishes one HTML page per date and nothing else. This CLI walks those dates into a local SQLite panel with an observed_at stamp on every row, then derives series the site never shows: a daily short-rate proxy from money-market fund yields, industry equity exposure in rupees, and cross-sectional return dispersion. It records which dates returned zero rows, so a gap is never mistaken for a zero.

## Prerequisites: Populate the local mirror

Six of the ten headline capabilities — `rates`, `panel`, `dispersion`, `universe`, `coverage`
and `dump` — read **only** the local SQLite mirror. They never call MUFAP. Against an empty or
thin mirror they return an empty result **at exit code 0**:

```bash
mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --agent
# {"meta":{"source":"local"},"results":[]}       exit 0
```

The `run: mufap-pp-cli backfill daily --from <date> --to <date>` hint is written to **stderr
only**, which `--agent` JSON consumers routinely discard. So: an empty `results` from one of
those six commands means the mirror is thin for that range, not that the industry was quiet.
Backfill the range first, then check `coverage`, then derive.

Canonical first run:

```bash
mufap-pp-cli doctor                                                        # config + reachability
mufap-pp-cli amcs --json                                              # 27 AMC GUIDs
mufap-pp-cli backfill daily --from 2026-09-01 --to 2026-09-04              # one request per date
mufap-pp-cli coverage --resource daily-returns --from 2026-09-01 --to 2026-09-04 --agent
mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --agent               # now non-empty
```

`backfill` has three subcommands mirroring three separate resources, and running one does not
populate the others: `backfill daily` (a daily tab), `backfill monthly` (the monthly net-assets
panel — where **industry AUM** lives), `backfill allocation` (per-fund monthly asset
allocation). It is resumable: each date commits on its own and dates already in the coverage
ledger are skipped unless `--force` is passed.

The two commands that do **not** read the mirror, `exposure` and `verify allocation`, fetch live
from MUFAP on every run and fan out over the whole industry. Read their entries below before
running either.

## When to Use This CLI

Reach for this CLI when you need Pakistani mutual fund data as a time series rather than a single lookup: building a dated panel of fund NAVs, deriving a short-rate proxy, measuring industry asset allocation in rupees, or checking how many funds actually reported on a given date. It is built for research pipelines that care about publication timing and about telling a real zero apart from a missing observation.

Routing the common asks: daily NAVs and returns -> `backfill daily` then `panel`; money-market yields -> `rates`; **industry AUM / net assets** -> `backfill monthly` then `dump monthly` (AUM is the monthly net-assets panel, not the daily NAV panel and not `exposure`, which covers listed equities only); PKR asset allocation -> `backfill allocation` then `dump allocation`, or `exposure` for the industry equity total.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for live PSX equity prices or index levels; it holds fund NAVs, not stock quotes.
- Do not use it to buy, sell, or redeem fund units; MUFAP is an industry association site and exposes no transaction surface.
- Do not use it for daily fund AUM; net assets and asset allocation are published monthly, and only NAV and returns are daily.
- Do not use it as an official policy-rate source; the rates command is a market-implied proxy derived from fund yields, not a State Bank publication.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Series only a local panel can produce
- **`rates`** — Derive a daily short-term interest rate series from the cross-section of money-market fund yields.

  _This is the only daily, backfillable PKR short-rate proxy obtainable without a blocked government source._

  ```bash
  mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --agent
  ```
- **`exposure`** — Total PKR the mutual fund industry holds in listed equities, by month, from per-fund asset allocation.

  _It is the asset-side counterpart to a mutual-fund net-flow series, enabling a cross-source join no single publisher offers._

  ```bash
  mufap-pp-cli exposure --from 2026-07 --to 2026-07 --max-amcs 1 --agent
  ```
- **`dispersion`** — Cross-sectional spread of fund returns within a category on each date.

  _Dispersion is a daily breadth measure that a single-fund page cannot express._

  ```bash
  mufap-pp-cli dispersion --category Equity --from 2026-09-01 --to 2026-09-04 --agent
  ```

### Local panel construction
- **`backfill`** — Fetch the daily fund panel or monthly allocation across a date range into the local store.

  _Publication timing is what makes a variable admissible as ex-ante, so every row records when it was actually observed._

  ```bash
  mufap-pp-cli backfill daily --from 2026-09-03 --to 2026-09-03
  ```
- **`panel`** — Query the stored daily NAV and return panel by date, fund, category or sector.

  _Turns 21 years of single-date HTML pages into one queryable table._

  ```bash
  mufap-pp-cli panel --category Equity --from 2026-09-01 --to 2026-09-04 --agent --select date,fund,nav
  ```
- **`coverage`** — Show which dates were fetched, which returned zero rows, and which were never attempted.

  _Distinguishes fetched-and-empty from never-fetched, so a gap is never mistaken for a zero._

  ```bash
  mufap-pp-cli coverage --resource daily-returns --from 2026-09-01 --to 2026-09-04 --agent
  ```
- **`dump`** — Dump the stored panel, allocation or coverage tables as JSONL for piping, a single JSON array for agents, or CSV.

  _Lets an analysis pipeline consume the panel directly instead of scraping._

  ```bash
  mufap-pp-cli dump daily-returns --from 2026-09-01 --to 2026-09-04 --format json --agent
  ```

### Trust the numbers
- **`verify allocation`** — Check that fund asset-class percentages net to 100 and flag months whose percent columns are unpopulated.

  _A month that fails the invariant is corrupt input, not a weak signal, and must be excluded before modelling._

  ```bash
  mufap-pp-cli verify allocation --month 2026-07 --max-amcs 1 --agent
  ```
- **`universe`** — Report how many funds reported on each date, by sector and category.

  _A silently narrowing universe fakes verdicts, so width is printed alongside every cross-sectional result._

  ```bash
  mufap-pp-cli universe --from 2026-09-01 --to 2026-09-04 --agent
  ```
- **`freshness`** — Detect funds whose published NAV validity date lags the requested date.

  _Differencing the live view without this check manufactures returns that never happened._

  ```bash
  mufap-pp-cli freshness --agent
  ```

## Command Reference

**allocation** — Per-fund monthly asset allocation in PKR millions and percent

- `mufap-pp-cli allocation --fund-code <int> --month <M-YYYY>` — Asset allocation for one fund in one month.

  `--fund-code` takes the **integer** `fund` field from `funds by-amc`, never the FundID GUID (a GUID returns HTTP 500). `--month` is required and is M-YYYY: not zero-padded, not ISO.

  ```bash
  mufap-pp-cli allocation --fund-code 12766 --month 7-2026 --json
  ```

**amcs** — Asset management companies (AMCs) registered with MUFAP

- `mufap-pp-cli amcs` — List the 27 asset management companies MUFAP tracks.

**dates** — Reporting periods MUFAP has published

- `mufap-pp-cli dates` — List the Year/Month periods MUFAP has published industry statistics for.

**funds** — Funds managed by an AMC, with category and pricing mechanism

- `mufap-pp-cli funds` — List funds for one AMC (AMCId is the GUID from `amcs list`)

**payouts** — Announced fund payouts and distributions

- `mufap-pp-cli payouts` — List announced fund payouts and distributions, with the per-unit amount and the ex-NAV the payout is struck against.

**unitholders** — Unit-holder pattern by investor type and sector

- `mufap-pp-cli unitholders --year <YYYY>` — Unit-holder pattern for one calendar year

**netsales** — Monthly industry flow, headline and by investor class

- `mufap-pp-cli netsales monthly --month <1-12> --year <YYYY>` — Sales, redemptions and net sales by sector and category in PKR millions, with the sheet's Total row returned separately as an invariant and reconciled under a rounding-aware bound.
- `mufap-pp-cli netsales investor --month <1-12> --year <YYYY>` — The same month across the nine investor classes. **Partial slice: no net column, and it failed to reconcile against the headline total in 19 of 25 measured months. Never apportion the industry total with it.**

**vps** — Voluntary Pension Scheme breakdowns

- `mufap-pp-cli vps age-wise` — VPS allocation broken down by contributor age band
- `mufap-pp-cli vps retired-cash` — Break down Voluntary Pension Scheme assets held as retired cash.
- `mufap-pp-cli vps withdrawals` — Report cash withdrawn from Voluntary Pension Scheme funds.

**Do not use `import`.** It appears in `mufap-pp-cli --help` as "Import data from JSONL file via API create/upsert calls", but MUFAP publishes no write surface: it is inert generator scaffolding whose own examples are placeholders (`import <resource> --input data.jsonl`). There is no endpoint behind it.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
mufap-pp-cli which "<capability in your own words>"
```

`which` scores a natural-language capability query against this CLI's curated feature index by term overlap. Exit code `0` means at least one entry shared a term with the query; exit code `2` means nothing matched — fall back to `--help` or use a narrower query. **There is no confidence floor**, so exit 0 is not proof the CLI serves the question: `which "book a flight to paris"` exits 0 with `exposure` at score 2. Read the returned `score` and the entry's description, and confirm the command actually answers the ask before acting on it. `--json` (and other machine formats) keep the exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Build the short-rate series

```bash
mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --agent --select date,median_yield,fund_count
```

Returns one row per date with the cross-sectional median money-market yield and the number of funds behind it, so a thin date is visible rather than silently averaged.

### Industry equity exposure by month

```bash
mufap-pp-cli exposure --from 2026-07 --to 2026-07 --max-amcs 1 --agent
```

Sums the PKR stocks-and-equities column across every fund, giving the asset-side series to pair against mutual-fund net flows.

### Audit a month before trusting it

```bash
mufap-pp-cli verify allocation --month 2026-07 --max-amcs 1 --agent
```

Flags funds whose asset-class percentages do not net to 100 after subtracting liabilities, which the site's own 100% label hides.

### Check universe width before any cross-section

```bash
mufap-pp-cli universe --from 2026-09-01 --to 2026-09-04 --agent --select date,fund_count
```

Prints how many funds reported per date so a narrowing universe cannot fake a result.

### Export the panel for external analysis

```bash
mufap-pp-cli dump daily-returns --from 2026-09-01 --to 2026-09-04 --format jsonl
```

Streams the stored panel as newline-delimited JSON for loading into a research database without re-fetching.

## Data model gotchas

Properties of MUFAP's published tables, each one measured during discovery rather than assumed.
Do not simplify them away.

- **Negatives are written in accounting notation, never with a minus sign.** `"(4.97)"` is -4.97. On 2026-09-04, tab=returns, 96 of 388 rows (24.7%) carried a parenthesised YTD and **zero** rows carried a leading minus. `rates`, `dispersion`, `panel`, `exposure` and `dump` decode it; `--raw-values` on `panel`/`dump` keeps MUFAP's text verbatim. A parser that skips unparseable cells removes exactly the left tail: before this was decoded, equity YTD dispersion on 2026-09-03 read median **+3.15** over 17 funds instead of **-3.70** over 91 — the sign of the market was inverted.
- **The row key is `Sector | Category | Fund Name`, not the fund name.** Fund names are not unique within a date: 49 of 388 rows on 2026-09-04 (12.6%) collide on name alone, mostly because VPS pension funds legitimately repeat one name across their Money Market, Debt and Equity sub-fund series. `dump` emits the composite as `row_key`; keying your own table on fund name collapses the pension universe with no error raised anywhere.
- **Columns differ per tab.** The fund-name column is `Fund Name` on `--tab returns` and `Fund` on all four other tabs. `--tab payout` has **no** `Validity Date` column at all — its date column is `Payout Date`, so `--from`/`--to` select on a different field there. `--tab pricing` and `--tab ter` are current reference data rather than a dated panel: they return 551 rows regardless of the date requested, so their row counts are not a universe width.
- **Percent columns are 0.0 for every month before roughly 2024** while the PKR amount columns stay correct. Derive percentages as amount/Total — which is what `exposure` does — and read `verify allocation`'s UNPOPULATED verdict as "no percentages published", not as a failure.
- **`TotalPercentage` is the literal string `"100%"`**, not a computed check; MUFAP displays a passing invariant it never performs. `verify allocation` computes it.
- **`message: "No data found"` appears even when `data` is fully populated.** Never gate on it; gate on the parsed row count.
- **The net-sales Total row is an invariant, not data.** `netsales monthly` returns SectorId 100 / Sector `Total` separately as `total` and excludes it from `rows`; summing it alongside the others double-counts the whole month.
- **The same VPS figures appear under two pension sector labels.** MUFAP renders identical voluntary-pension rows under both `Pension Funds (Open-End Funds)` and `Employer Pension Funds`. Summing both overstates pension flow by about 13% — 4,680 rows before de-duplication versus 4,072 after. `netsales monthly` drops the duplicates and counts them in `vps_duplicates_dropped`.
- **On the net-sales pages a dash means MISSING, not zero — the opposite of the daily tables' convention and of CDC's.** A category showing `-` did not report; it is not a category that reported zero. Those figures come back as `null`, never `0`. For 2026-05, 33 of 40 rows are non-reporting while the month's total flow is 9,458m.
- **Reconciling net sales needs a rounding-aware tolerance, not a fixed one.** MUFAP renders whole PKR millions, so summing n rows against a rounded total carries up to ±0.5·(n+1) of rounding error. Across the 25 months that carry a Total row every residual is ≤ 2.0 while the bound ranges 3.0–7.5, so **all 25 reconcile**; a fixed ±1.5 tolerance falsely fails three of them (2025-12, 2026-01, 2026-04). Both the sales and the redemption residual must be checked — 2026-04 is exact on sales and off by 2.0 on redemptions.
- **The investor-class feed is a partial slice.** It carries no net column (derive net as sales − redemptions, only where both are present) and its class totals failed to reconcile against the headline month total in 19 of 25 measured months. Use it as a coverage-matched comparison; never to apportion the industry total.
- **Most net-sales months are legitimately empty, and a challenge is not an empty month.** MUFAP serves a rendered page for every (Month, Year) whether or not it published data — 79 of the 104 months from 2018-01 to 2026-08 are empty this way, with a Total row whose cells are all dashes. Cloudflare also challenges these two paths intermittently (3 of 8 sequential requests when measured); that is retried with backoff and then reported as a challenge, never as an empty month.
- **Four date encodings.** `YYYY-MM-DD` for the daily and monthly range flags, `M-YYYY` (not zero-padded, not ISO) for `allocation --month`, `YYYY` for `unitholders --year`, and `Mon DD, YYYY` as displayed in the table. A wrong encoding returns HTTP 200 with an empty table, or HTTP 500 — never an informative error.

## Auth Setup

No credentials. MUFAP sits behind Cloudflare, so the CLI ships a Chrome-fingerprint HTTP transport that clears the challenge without a browser, a clearance cookie, or any login.

Run `mufap-pp-cli doctor` to verify setup.

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
  mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --agent --select date,median_yield,fund_count
  mufap-pp-cli panel --from 2026-09-01 --to 2026-09-04 --agent --select date,fund,NAV
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — MUFAP exposes no write surface, so do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests. `mufap-pp-cli --help` still lists a generated `import` command ("Import data from JSONL file via API create/upsert calls"): it is inert scaffolding with placeholder examples and no endpoint behind it. Do not use it.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `MUFAP_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `MUFAP_CONFIG_DIR`, `MUFAP_DATA_DIR`, `MUFAP_STATE_DIR`, `MUFAP_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `MUFAP_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and saved profiles. `data` contains `data.db` — the local mirror every derived command reads — plus `feedback.jsonl` and the learning-loop tables. `state` contains persisted queries and `teach.log`. `cache` contains regenerable HTTP/cache files.
- This CLI stores **no secrets**: there is no `credentials.toml`, no cookie jar on disk, no auth sidecar and no `auth` command, so relocation never has to move a credential.
- Relocating the data dir relocates the mirror. A fresh root starts with an empty `data.db`, so the mirror-only commands return an empty result at exit 0 until `backfill` runs against it.
- Run `mufap-pp-cli doctor --fail-on warn` to surface path warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "mufap": {
        "command": "mufap-pp-mcp",
        "env": {
          "MUFAP_HOME": "/srv/mufap"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `MUFAP_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `MUFAP_HOME`, or `doctor` will report the new root while your mirrored `data.db` sits under the old one.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, run:

```bash
mufap-pp-cli recall "<user's question>" --agent
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
      "next_action": ["<trial command>", "mufap-pp-cli learnings confirm 12"] }
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
       materially more, record the divergence via `mufap-pp-cli playbook amend`
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

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `mufap-pp-cli learnings candidates` lists the full open set.

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

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately:

```bash
mufap-pp-cli teach --query "<user's question>" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "Niners") satisfies future queries under another alias (e.g., "49ers", "San Francisco") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
mufap-pp-cli teach \
  --query "<user's question>" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
mufap-pp-cli teach-playbook \
  --query "<user's question>" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know — a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback — fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`.

```bash
mufap-pp-cli playbook amend \
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

`mufap-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `MUFAP_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
mufap-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
mufap-pp-cli feedback --stdin < notes.txt
mufap-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `MUFAP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `MUFAP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
mufap-pp-cli profile save briefing --json
mufap-pp-cli --profile briefing allocation get --fund-code 12766 --month 7-2026
mufap-pp-cli profile list --json
mufap-pp-cli profile show briefing
mufap-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `mufap-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/payments/mufap/cmd/mufap-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add mufap-pp-mcp — mufap-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which mufap-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   mufap-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `mufap-pp-cli <command> --help`.
