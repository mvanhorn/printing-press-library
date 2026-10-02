# SmartEX CLI brief

## API identity
Official source `https://smart-ex.jp/en/` and its linked JR Central/West/Kyushu public planning sources. Website target, with no official public OpenAPI or developer SDK. Runtime is bounded plain HTTP and structured public HTML extraction. Authenticated reservations, exact train inventory, payments, accounts and reservation mutation are outside this build.

## Reachability
Native Chrome public workflow and cookie-free HTTP POST to JR-supported `https://unchin-navi.jp/cgi-bin/plusex/tokai_exic.cgi` both work. Source GETs all HTTP200. Reservation login displayed scheduled maintenance; member login is required for exact inventory. This limitation does not apply to public dated fares. SDK/registry and wrapper-issue discovery is inapplicable to this website target under phase04's BROWSER_SNIFF_TARGET_URL branch.

## Top workflows
1. Resolve English/Japanese station names across Tokaido/Sanyo/Kyushu; plan corridor and direction with explicit train categories.
2. Compare dated reserved, unreserved and Green adult fares for regular ticket, smartEX and paid-member EX Reservation. Retain date, season, class, currency, source, retrieval time and upstream assumptions; do not synthesize child fares.
3. Compare current basic and five Hayatoku products by train, corridor, class, sale window, party minimum, change/refund policy and unresolved route/seat/exclusion-date conditions.
4. Compute precise JST reservation windows and luggage suitability, including one-year request limits, one-month confirmation, after-hours party limits and oversized baggage seat requirements.
5. Locate official basic timetable publications and provide any reliably extracted services with explicit basic-timetable and date-validity limits, then hand off to canonical booking for actual seats.

## Table stakes and pain points
Incumbents: smartEX website/app and JR official timetable PDFs. They provide authoritative pricing and real reservation inventory; CLI adds compact machine-readable comparisons, Japanese-name resolution, correct calendar boundaries, and one-query/one-source auditability. Common pain points: browser frame/encoding complexity, discount deadlines/limited route eligibility, baggage seat constraints, differences between static schedule, dated fare and inventory.

## Data layer
Versioned station catalog, product/policy metadata and validated published basic timetable snapshot. Embed only public facts with source URLs and as-of date. No user account, credentials or personal travel store. Fetch dated quotes fresh; no broad crawls, background sync or unbounded pagination. HTML responses capped; HTTP timeouts. Bounded JSON default, optional fields/detail, stable IDs, explicit unknowns.

## Product thesis
Name: `smartex-pp-cli`. Read-only Shinkansen planning grounded in smartEX and linked JR sources, with a clear booking handoff.

## Build priorities and acceptance
- `stations`, `route`, `fare`, `products`, `window`, `baggage`, `policy`, `timetable`, `handoff`, `sources`.
- Fare uses EUC-JP form replay, validates normalized station/segment/date/class, parses active one-way rows and fails on source drift. Class values reserved0, unreserved1, Green2. Navigator displays one adult; multiple-adult totals are arithmetic planning totals, never inventory. Children remain unquoted.
- Current month + next two months only for public fare navigator. Actual basic product reservations may start one year ahead; the two horizons are distinct.
- Basic service covers six train types and three classes. EX Hayatoku1: six to one days, 00:00 opening, 23:30 deadline, Tokaido Hikari/Kodama unreserved. Hayatoku3/7/21 and Family7: one-month 10:00 opening, 3/7/21/7 day 23:30 deadline; partial section and exclusion dates must remain unresolved unless verified. Family7 requires 2–6; others 1–6. Hayatoku21 changes require refund/rebook (seat-position exception). Round-trip product ended March31,2026.
- Boarding methods QR/registered individual IC/paper tickets. Conventional connections and city-zone inclusion differ from station tickets. Refund320 JPY per person only while applicable pre-departure/pre-pickup/pre-gate; post-departure amounts product-specific and unknown.
- Baggage >160cm up to250cm needs oversized area reserved seat; >250cm, length>200cm, weight>30kg or >2 pieces fails normal baggage limits. Current JR summary controls160cm boundary. Special equipment exceptions explicitly require separate checking.
- Consequential deterministic tests: calendar/leap-year/month ends, exact JST deadline and overnight limits, baggage thresholds, name ambiguity, parser drift, class mapping, HTML limits/non200/timeouts, unknown child prices and unsupported parameters.
- Live proof: Tokyo/Shin-Osaka, Sanyo, Kyushu and reverse quotes; three classes; invalid/empty cases; upstream source and timetable status; no mutation. Record latency, output bytes, upstream calls and memory.
- Exactly one fresh-context gpt-6.1-sol **max** reviewer. User supersedes Press extra-agent recommendations and stale xhigh instructions. Fix findings and reverify. Complete actual Press phases/checks and promote locally only; no GitHub publication.

## Discovery evidence
`../discovery/`, `../proofs/source-fetches.json`, `../proofs/extra-fetches.json`, project `evidence/native-browser.md`, and the explicit requirements in `../.japan-cli-builds/batch2-brief.md`. Browser discovery was user-authorized in that briefing.


## MAX review source reconciliation

Current Japanese reception/advance guidance and English guidance differ on oversized one-year and after-hours requests. Do not infer a universally earliest oversized opening or overnight booking permission. Return null actual eligibility/opening and retain both rules; ordinary one-month 10:00 sales remains the documented fallback. Oversized-area party limits are five ordinary/four Green passengers per operation, no cross-car grouping; train-specific exceptions and actual availability need official confirmation. Seat maps are unavailable during one-year requests, the 07:30–09:59 processing gap, overnight and unreserved-class planning.
