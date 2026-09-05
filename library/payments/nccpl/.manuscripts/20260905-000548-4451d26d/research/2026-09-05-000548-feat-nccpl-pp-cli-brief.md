# NCCPL CLI Brief

## API Identity
- **Domain:** National Clearing Company of Pakistan Ltd — the clearing/settlement layer beneath
  the Pakistan Stock Exchange. Publishes daily post-settlement datasets no exchange feed carries:
  investor-class capital flows (FIPI/LIPI), leverage-market open positions (MTS/MFS/MSF/SLB),
  risk parameters (VAR margins, haircuts, free float), and trade-vs-settlement reconciliation.
- **Users:** PSX quant/factor researchers, buy-side flow analysts, financial media
  (Portfolio360/FinHisaab/BullsView all republish NCCPL numbers), broker research desks.
- **Data profile:** Daily, market-wide AND per-symbol. Published ~6:00–7:00 pm PKT after each
  session. FIPI/LIPI history runs back to **9 Dec 2015** (~2,500 sessions).
- **Base URL:** https://www.nccpl.com.pk

## Reachability Risk
- **High (mitigated).** `probe-reachability` on `/` and `/api/fipi/latest-date` both return
  `mode: browser_clearance_http`, confidence 0.6 — stdlib AND surf-chrome each get HTTP 403 with
  `cf-mitigated: challenge`. Runtime must be Surf browser-compatible HTTP **plus** a `cf_clearance`
  cookie imported from Chrome.
- HAR-derived traffic analysis reports `browser_http` only because Chrome redacted cookies from
  the export. **Trust the live probe, not the HAR.**
- Tier/permission hints from 4xx body: none — the 403 body is a Cloudflare interstitial, not an
  API tier message. No paid tier exists; this is bot protection, not entitlement.
- Probe-safe endpoint used: `GET /api/fipi/latest-date` (read-only, no params, no side effects).
- No GitHub issues reporting breakage — because essentially no tooling exists (see Table Stakes).

## Auth Model (session_handshake)
Three layers, all required for `POST /api/*/data`:
1. **Cloudflare clearance** — `cf_clearance` cookie, obtainable only from a real browser.
   Imported via `auth login --chrome`.
2. **Laravel session** — `nccpl-session` cookie, Max-Age 7200 (2 h). Minted by
   `GET /market-information`.
3. **CSRF token** — two interchangeable routes:
   - `X-CSRF-TOKEN` from `<meta name="csrf-token">` on the page (HTML parse), or
   - `X-XSRF-TOKEN` from the `XSRF-TOKEN` cookie, URL-decoded (**no HTML parse — preferred**).
   Confirmed independently by `hmehmood56-debug/PSX-Trader` probe scripts.

`GET /api/*/latest-date` and `GET /api/graph-data/latest-data` need clearance only — no CSRF,
no session. These are the natural `no_auth: true` endpoints and the health-check surface.

## Top Workflows
1. **Backfill a multi-year daily panel into local SQLite**, then hand it to the PSX research
   engine. This is the primary workflow — everything else supports it.
2. **Pull today's investor-class flow matrix** (sector × investor type, net USD) after the
   ~6–7 pm PKT publication, as an ex-ante input to the next morning's forecast.
3. **Track per-symbol leverage and short interest** (MTS/MFS/MSF open positions, SLB net open
   position as a short-interest proxy) as a cross-sectional factor panel.
4. **Audit coverage** — which dates are missing per resource, so a research run never silently
   treats a gap as a zero or a stale pull as current.
5. **Validate the data against its own invariants** before it reaches a regression.

## Table Stakes
Every incumbent is a **dashboard**, not a feed. None is a CLI. None is agent-native. None
backfills. None exposes symbol-level MTS/SLB/VAR for cross-sectional work.
- **Portfolio360** — sector × investor-type net-flow matrix in USD mn, 30-session foreign-flow
  bar chart, 20-session and YTD cumulative totals, outlier capping for display.
- **FinHisaab** — interactive FIPI/LIPI dashboard, daily/weekly/monthly aggregation.
- **Youngs Capital** — FIPI/LIPI tracker with sector-wise flows.
- **BullsView / StockIntel / PakFinance Hub** — retail-facing flow views bundled with prices.
- **NCCPL's own site** — one date or range at a time, per tab, behind Cloudflare, no export.

Features to absorb from them: the sector × investor matrix, window aggregation (week / 20-session
/ YTD), USD and PKR views, foreign-flow time series, sector ranking by absolute net.

## Codebase Intelligence
- Source: `hmehmood56-debug/PSX-Trader` (`scripts/probe-nccpl-live-api*.mjs`, Playwright).
  The only public code touching this API.
- Auth: reads `XSRF-TOKEN` cookie, `decodeURIComponent`s it, sends it as a header alongside a
  full cookie jar. Warms cookies with a real browser first.
- Data model: sector-wise rows carry `SECTOR_NAME`, `BUY_VALUE`, `SELL_VALUE`, `NET_VALUE`
  (uppercase keys), aggregated by sector client-side.
- Rate limiting: none observed; Cloudflare challenge is the only gate.
- Architecture insight: their test matrix probes `same-day`, `two-day`, `seven-day` and
  **`one-year`** ranges against `fipi-sector-wise/data`, implying the `{fromDate,toDate}`
  endpoints accept wide windows. Backfill should request wide and narrow on failure.

## Data Layer
- **Primary entities:** `flows` (date × investor_type × sector × segment), `positions`
  (date × symbol, MTS/MFS/MSF/SLB), `margins` (date × symbol VAR/haircut/free-float),
  `settlement` (date × symbol UIN- and CM-wise), `market_totals` (date, value/volume),
  `refinancing`, `force_release`, `financier_pairs`, `tfc_trades`.
- **Sync cursor:** per-resource `latest_date` from `GET /api/<r>/latest-date`, plus a
  per-resource coverage ledger of which dates have been fetched. Resources are NOT in lockstep —
  observed on 2026-09-04: fipi/lipi `2026-09-04`, slb `2026-08-27`, msf `2026-08-21`,
  un-listed-tfc `2026-05-22`. Freshness must be per-resource and surfaced, never assumed.
- **FTS/search:** symbol and sector names across positions/margins/settlement.
- **Backfill shape:** range endpoints (`fipi`, `lipi`, `fipi-sector-wise`, `lipi-sector-wise`)
  take `{fromDate,toDate}` and pull wide windows. All other `/data` endpoints take a single
  `{date}` and must be iterated per session date.

## User Vision
Verbatim from the user, captured before research:
> "major purpose is to use any info available on NCCPL that can feed into the PSX research
> analysis project. incorporate any use case possible or required for that into this cli."

This CLI is a **research data feed first, a browsing tool second.** Its primary consumer is
`~/psx-research`, not a human reading tables. Design consequences:

- **It exists to break a time-starvation problem.** Every signal that project tested died for
  lack of observations, not lack of breadth: SUE had 11 quarterly cohorts, the nowcast graded on
  22 names. The one surviving result is macro and daily (Brent → next-session KSE-100, t=-7.86
  over 1,204 sessions). NCCPL's FIPI/LIPI archive back to Dec 2015 is ~2,500 daily observations
  of a *flow* variable that has never been tested — the same shape as the one thing that worked,
  with twice the history.
- **Publication timing makes it ex-ante and therefore clean.** NCCPL publishes session T's data
  at ~6–7 pm PKT on day T; the project's pre-registered forecast fires at 09:15 the next morning.
  Foreign net flow is knowable before the open it would predict. It slots into `macro_v1`
  (ex-ante Brent+SPX) without touching the guards.
- **It must respect the project's contamination rules.** "A missed observation is the correct
  outcome; a late or back-dated one is a contaminated one." The CLI must never fabricate,
  interpolate, or forward-fill a missing date, and must make gaps loudly visible.
- **It must ship controls, not just data.** "An agent finding is evidence about ITS filer. A
  control that cannot move for the same reason the signal does is what turns it into a claim."
  NCCPL hands us two exact arithmetic invariants for free — FIPI net ≡ −LIPI net, and every
  sector row nets to zero across investor types. Both are checkable per date, and a date that
  fails them is corrupt input that must not reach a regression.
- **Always print universe width.** A prior control factor in this project was silently empty for
  7 of 25 quarters and faked a "no factor story" verdict. Any filtered or ranked output here
  must state how many symbols/sectors/dates it actually spanned.

## Product Thesis
- **Name:** `nccpl-pp-cli`
- **Why it should exist:** Six websites will show you today's foreign-flow number. None will give
  you ten years of it as a local table with a coverage audit and an arithmetic self-check. NCCPL
  publishes the only per-symbol leverage, short-interest and free-float data in the Pakistani
  market, once, behind Cloudflare, one date at a time, with no export — and every tool built on
  it so far throws the history away after rendering a chart. This CLI keeps it.

## Build Priorities
1. Session-handshake auth (clearance + Laravel session + XSRF) and the `latest-date` health
   surface — nothing else works without it.
2. Local store + resumable backfill with a per-resource coverage ledger.
3. FIPI/LIPI flow surface: daily, sector-wise, investor-class matrix, USD and PKR.
4. Per-symbol panels: leverage (MTS/MFS/MSF), short interest (SLB), risk (VAR/haircut/float),
   settlement (UIN/CM).
5. Research-grade exports: panel emission and direct write into the PSX research database.
6. Integrity controls: the two arithmetic invariants, freshness/staleness reporting, gap audit.

## Research Consumer Schema (~/psx-research/data/research.db, 338 MB)
Verified 5 Sep 2026. Any export must land in these conventions, not invent new ones.

| Table | Rows | Span | Key |
|---|---|---|---|
| `daily_bars` | 968,899 | 2016-08-22 .. 2026-09-04 | `(symbol, date)` |
| `macro_bars` | 17,692 | 2016-09-01 .. 2026-09-04 | date-keyed |
| `index_bars` | 3,750 | 2021-08-20 .. 2026-09-03 | date-keyed |
| `forecasts` | 1 | — | the daily loop closed for the first time on 4 Sep |

`daily_bars` is `PRIMARY KEY (symbol, date)` with `idx_daily_date` and
`idx_daily_symbol(symbol, date)`.

**Consequences for this CLI:**
- Per-symbol NCCPL tables (MTS/MFS/MSF/SLB positions, VAR margins, settlement) must be
  `(symbol, date)` keyed so they join to `daily_bars` with no adapter.
- Market-level tables (FIPI/LIPI totals, sector flows) must be date-keyed so they join to
  `index_bars` and `macro_bars` — the same shape that produced the Brent -> next-session
  result over 1,204 sessions.
- NCCPL's FIPI/LIPI archive begins 2015-12-09, which is **earlier** than `daily_bars` begins
  (2016-08-22). Flow history fully spans the existing price panel; no coverage gap on the
  NCCPL side limits a join.
- Date storage must be `TEXT` `YYYY-MM-DD` to match. The `DD/MM/YYYY` wire format on the
  fipi/lipi range endpoints is a transport detail and must never reach the store.

## Reachability Gate (Phase 1.9)
- **Decision: PASS** (browser-clearance exception).
- Live probe on `/` and `/api/fipi/latest-date`: `mode: browser_clearance_http`, HTTP 403 with
  `cf-mitigated: challenge` on both stdlib and surf-chrome. A plain curl 403 is expected
  evidence here, not a stop.
- Browser capture exists and contains useful non-challenge traffic: 204 HAR entries including
  20 successful `latest-date` JSON responses and the full `/market-information` document whose
  inline JS defines every `/data` request. `traffic-analysis.json` reports
  `reachability.mode: browser_http`.
- Matrix row satisfied: 403/429 with bot detection + successful useful capture +
  `browser_http` -> PASS.
- Runtime committed in the spec and verified in generated output: Surf browser-compatible HTTP
  (`http_transport: browser-chrome`) plus Chrome clearance-cookie import
  (`auth.type: composed` over `cf_clearance`, `nccpl-session`, `XSRF-TOKEN`).
- No resident-browser transport is shipped; all runtime calls replay over HTTP.
