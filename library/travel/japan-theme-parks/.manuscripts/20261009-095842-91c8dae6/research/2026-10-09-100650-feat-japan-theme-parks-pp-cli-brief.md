# Japan Theme Parks CLI Brief

## API Identity
- Domain: Japan theme-park trip planning. Live and historical ride waits (queue-times.com), dated Tokyo Disney Resort (TDR) ticket sale status, price tier and hours (tokyodisneyresort.jp), Universal Studios Japan (USJ) park hours and dated Studio Pass price (usj.co.jp).
- Users: trip planners (often foreign visitors) who must pick a park date before tickets sell out, and pick ride order on the day.
- Data profile: small, public, read-only. 5 parks; ~30-50 rides per park; ticket calendars ~60-90 dates per park.

## Reachability Risk
- queue-times.com: None. Plain HTTP 200 for /parks.json and /parks/{id}/queue_times.json (verified 2026-10-09). Data updates every 5 minutes. Terms: free Real Time API, must show "Powered by Queue-Times.com" linking https://queue-times.com/ prominently. No historical API; crowd calendar and stats exist only as HTML pages (about page says owner prefers links, not embedding).
- tokyodisneyresort.jp: Low-Medium. stdlib HTTP times out (Akamai TLS fingerprint); Surf Chrome impersonation returns 200 in ~0.4 s (probe-reachability mode browser_http, confidence 0.85). robots.txt disallows only /iframe/ and /app/.
- usj.co.jp: Medium. Plain HTTP 200 but pages are an Angular SPA shell (9 KB) with Akamai bmak script. Data comes from configured endpoints (mobile-service.usj.co.jp/api with an embedded web client token; upr.calendar.getMonthParkHours). Needs browser capture to find a replayable, keyless surface.

## Top Workflows
1. Pick a date: for a date range, show TDL/TDS 1-Day Passport sale status (on sale / few left / sold out / not yet on sale / no online sale), adult/junior/child price for that date, and park hours; USJ hours and Studio Pass price for the same dates.
2. Check one date across parks: "Is 2026-11-14 still on sale at DisneySea, what does it cost, when does it open?"
3. Live waits now: per park, open rides sorted by wait, with the source update time.
4. Typical waits from local history: after `snapshot` runs, summarize per-ride waits by weekday/hour with sample size and date span. No forecasts.
5. Ride order hint on the day: top-N longest typical waits (from local history, labeled) vs live now.

## Table Stakes
- Live wait list per park (queue-times site, official apps, themeparks.wiki API).
- TDR ticket calendar with price and status (official site only).
- Park hours per date (official sites).
- Attribution "Powered by Queue-Times.com".

## Data Layer
- Primary entities: park, ride (queue-times id, land), wait snapshot (ride id, wait, is_open, source last_updated, fetched_at), TDR ticket day (park, date, ticket id T1/T2/T8, status class, prices, open/close, fetched_at).
- Sync cursor: fetched_at per park; queue-times last_updated per ride (dedupe on ride id + last_updated).
- FTS/search: not needed. Ride name lookup is a simple substring over ~200 names.

## User Vision
- Decide "which date (on sale, price tier, typical crowd for weekday/season) and which rides first".
- TDR dated availability check is core. Local SQLite history of queue-times snapshots, honest sample sizes and dates, no invented crowd forecasts.
- Less is more: only commands that change a planning decision. Read-only, never buy, never add to cart.
- Style of the user's other Japan CLIs (e.g. asoview-pp-cli): source-backed facts, meta.sources with fetch time, explicit unknowns, bilingual JA/EN names where the source has them (TDR has JA + EN pages with the same ticket ids).

## Source Priority
- Primary: queue-times.com — community JSON API, no spec — free, attribution required.
- Secondary: tokyodisneyresort.jp — no spec; server-rendered HTML embeds `var ticketPopup = {...}` JSON per month page (/ticket/index/YYYYMM/ and /en/ticket/index/YYYYMM/) with per-date per-park ticket lists, `classPopup` status (`conversion`, `conversion is-few`, `conversion is-none`, `conversion is-notSales`), `ageText` prices, `openTime`. Calendar table also carries the adult price per cell. Free; browser_http transport (Chrome TLS).
- Tertiary: usj.co.jp — no spec, SPA; endpoints to be found by browser capture. Free.
- Economics: all sources free; no keys. Any USJ endpoint that needs a login or a private key is out of scope (stop rule).
- Inversion risk: TDR data is richer than queue-times for the date decision. Keep queue-times as primary for live/history commands, but the date-picking command is the headline workflow per user vision; both lead the README.

## Product Thesis
- Name: japan-theme-parks-pp-cli
- Why it should exist: no single tool answers "which date + which rides" for Japan parks. The TDR calendar is a modal-heavy web page, USJ hours sit in an SPA, and queue-times has live waits but no local history you control. One read-only CLI with source dates and explicit unknowns removes three browser tabs.

## Build Priorities
1. `dates` (TDR status/price/hours by date range, JA/EN names) — headline.
2. `waits` (live queue-times per park, attribution) and `snapshot` (store into SQLite).
3. `typical` (local history summary with sample size/date span).
4. USJ hours/price for dates, if a keyless replayable surface is found.
5. `parks` (ids, names, timezone, which sources cover which park; Fuji-Q and Legoland: queue-times only, hours/tickets unknown).

## Reachability Gate (Phase 1.9, 2026-10-09)
- queue-times.com /parks.json: 200 (plain HTTP).
- tokyodisneyresort.jp /ticket/index/202611/: stdlib 000 (timeout), Surf Chrome-TLS 200 with ticketPopup. PASS via browser_http.
- mobile-service.usj.co.jp /api/Venues/10251/Hours: 401 Unauthorized (RFC 9110 problem JSON) on 4 repeated plain calls. The 200 seen during discovery was most likely an edge-cache hit created by the browser session. Community code (romualdoag/parksapi-mcp) gets a Bearer token via OAuth2 client_credentials at api.usj.co.jp/oidc/connect/token using client id/secret embedded in the USJ web/app bundles. => USJ official hours need a key. Per user rule, STOP and ask.
- Possible keyless substitute (not in the user's source list): api.themeparks.wiki/v1/entity/{park}/schedule returns USJ hours (31 days) and Fuji-Q hours (31 days) with plain HTTP.

### Gate decision (Phase 1.9), user answer relayed by the orchestrator 2026-10-09
- Option 1: use api.themeparks.wiki (keyless, plain HTTP) for USJ and Fuji-Q hours. Label it as a third-party aggregator (not official), with fetched_at on every hours value from it.
- Horizon: ThemeParks.wiki returns about 31 days, while TDR sale data covers about 2 months. 31 days does NOT cover the 2-month TDR window. For USJ/Fuji-Q dates beyond the returned schedule, `dates` shows hours = null with reason "beyond third-party source horizon". No extrapolation.
- Never use option 3 (USJ web/app client credentials). The USJ mobile-service Hours endpoint is removed from all shipped code paths; the 401 finding stays here for the record.
- ThemeParks.wiki terms (themeparks.wiki/api and /terms, read 2026-10-09): free tier allowed for commercial and personal use; products that show the data, including AI assistants and agents, must show a visible "Powered by ThemeParks.wiki" credit linking to https://themeparks.wiki; do not redistribute/mirror/re-API/bulk-export the feed as data; rate limited per IP when keyless, 429 with Retry-After must be honoured; live data cached 60 s (schedule changes slowly). Implementation: on-demand fetch of 2 park schedules only, short response cache, credit in README and in `dates` output metadata, no bulk export command.
- Reachability: api.themeparks.wiki /v1/entity/47f61fac-7586-41ac-ae80-61c9257cf33e/schedule -> 200.
