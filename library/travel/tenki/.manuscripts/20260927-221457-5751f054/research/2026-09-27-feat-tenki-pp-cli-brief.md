# tenki.jp sightseeing CLI brief

Researched 2026-09-27 Asia/Tokyo. Target: tenki.jp only; lightweight read-only local Go CLI, no resident browser. The user supplied detailed scope and authorized research; implementation requires the absorb gate.

## Identity and access
Public website HTML is the verified source; no official OpenAPI or stable supported API was found. Unauthenticated direct GET returns HTTP 200 for municipal hourly/2-week pages, location search, mountain index/detail, sakura index, and foliage index/detail. Raw captures are in discovery/http. Shell DNS is restricted inside the execution sandbox; escalated network requests work. No API key was needed. Current usage terms https://tenki.jp/docs/rule/ article 8(6) restrict acquisition/use outside browsers/RSS; this material access constraint must be disclosed at scope, independently of technical reachability. No authentication or anti-bot bypass is proposed.

## Verified product semantics
- Municipality source: https://tenki.jp/forecast/3/16/4410/13101/1hour.html. Full HTML has dates 2026-09-27, 28, 29; issue comment `announce_datetime:2026-09-27 23:00:00`. Hourly weather/rain apply to the preceding hour, instantaneous temperature/wind to the labelled hour. Municipality/attraction forecasts refer to town-hall vicinity; locations must retain their forecast reference.
- Two-week source: same prefix /10days.html. Current markup has daily weather/high/low/rain probability/rain amount, nested six-hour detail, then last three daily periods with forecast confidence. Expose actual returned dates as the supported horizon, never synthesize missing dates or hourly rows beyond source coverage. Source issue time differs from advertising analytics metadata (22:00 versus product 23:00 in captured HTML); use product context.
- Official help https://tenki.jp/docs/help/ redirects to official FAQ at https://tenkijp.tayori.com/q/tenkijp-faq/. Current cadence FAQ https://tenkijp.tayori.com/q/tenkijp-faq/detail/1083867/ says municipal weather updates hourly. FAQ 1138228/1138224 describes hourly today+2, six-hour through today+10, final days 12–14 daily with A–E confidence. FAQ 1137776 documents town-hall reference; 1137777 documents foothills.
- Location search: GET /search/?keyword=<Japanese address or postal code>. Mountain index /mountain/ lists named Japan destinations including Fuji /mountain/famous100/5/25/150.html and Takao /mountain/normal/3/16/1043.html. Preserve ambiguity, require stable URL/ID for detail, limit result counts and pages.
- Mountain detail Fuji: summit elevation 3776 m is destination metadata. Forecast is explicitly foothill Fujinomiya municipality. Separate altitude table contains nearby numerical model calculations, explicitly NOT a weather forecast, at labelled levels. Retain initialization time and each altitude; do not interpolate summit conditions.
- Foliage: https://tenki.jp/kouyou/ says 2026 and about 700 sites; current condition and historical usual window are separate. Murododaira https://tenki.jp/kouyou/4/19/30314.html has current seasonal issue time 2026-09-27T15:00+09:00, usual window mid-September–early October, and municipality weather issued 23:00. A sidebar still says 2025 seasonal prediction: global year regex would be unsafe. Seasonal section parsing and provenance must be scoped.
- Sakura: https://tenki.jp/sakura/ explicitly says 2026 updates ended and 2027 may differ. Current seasonal result must say season-ended; archived 2026 facts only with explicit historical selection, never a 2027 prediction.

## Top workflows
1. Resolve a municipality, attraction, or mountain; inspect forecast reference and available products.
2. Shortlist dates using daily forecasts, then inspect available hourly windows for rain and wind.
3. Compare 2–5 destinations over up to 14 actual forecast days using user thresholds; return pass/fail/unknown criteria with source values and missing evidence.
4. Discover current foliage sites and pair their seasonal condition with the correct municipality/foothill weather; show historical normals separately.
5. Inspect mountain foothill forecast and source-labelled altitude model guidance side by side without a summit or safety guarantee.

## Competitor context and table stakes
https://github.com/algon-320/tenki offers URL-selected 3-hour forecasts, 1–3 day limit and Conky output. Archived 2020; issue list has no reported open issues. https://github.com/rrencanno/tenki_scraper offers selected-area weather, high/low, weekly forecast and cached Flask UI; no open issues. Neither provides the requested auditable sightseeing comparisons or seasonal-year contract. Search results for tenki.cloud and weatherapi-backed Tenki are different services and excluded. Browser-only presentation/Conky are outside the user's focused JSON CLI scope.

## Data layer and architecture
Cache by canonical URL/product with fetched_at, source issue/init time, expiry, stale flag, content hash, source URL and schema/parser version. Municipality forecast TTL at most one hour with refresh; unknown seasonal cadence use cautious documented TTL until verified. Reuse cached location results; no nationwide crawler or unbounded sync. Missing numbers are null, collections [] and statuses explicit. All diagnostics stderr. Retain JST RFC3339 times and timezone name; units explicit. Summary omits verbose source text; detail carries observations and assumptions. A bounded local cache suffices; avoid unrelated SQL/query machinery.

## Product thesis and build priorities
`tenki-pp-cli`: evidence-first Japan outdoor planning from tenki.jp. First validate extraction contracts; implement resolution, hourly/daily forecast and seasonal status; implement bounded transparent comparison; add differentiated mountain model detail; verify multiple live locations plus labelled dated fixtures for ended seasons, mixed years, missing rows and month/year boundaries. Measure output bytes, request count, elapsed time and peak RSS for cold/warm/refresh commands. Write concise help, README and agent skill using writing-for-agents. Deliver with Printing Press receipt gates, staging and promotion, buildable in the requested workspace; shared configuration stays unchanged.

## Users
User-provided audience: a travel-planning agent selecting Japan outdoor sightseeing days/destinations. Two concrete roles implied by the requested workflows are (1) itinerary agent checking alternate day trips against rain/temperature/wind before choosing an outing date and (2) seasonal-outing agent checking blossom/foliage status and mountain forecast level before recommending a destination. These are role models from the user brief, not interview claims. Both repeatedly inspect multiple source pages and need concise comparable evidence instead of unqualified scores.

## User Vision
Focused tenki.jp-only local delivery. Bound comparisons, preserve evidence and uncertainty, verify live products and fixtures separately, measure efficiency, and keep shared configuration unchanged. Scope breadth takes precedence over the generic Press instruction to absorb unrelated ecosystem features.

## Ecosystem search completion
Searched tenki.jp plugins, MCP, skills, CLI, npm wrappers, PyPI clients and automation. No relevant npm/PyPI client or MCP identified; tenki.cloud results excluded. Additional tools: gpsoft/odekake aggregates multiple providers (other integrations excluded by user); cafelatte00/weather-scraper exports current weather/high-low/period rain probability CSV; whtsky/skills describes japan-ski-weather (ski/snow outside sightseeing scope; guessed source path 404). Official Claude plugin directory contains no tenki/weather/Japan match. DeepWiki unavailable for algon-320/tenki; no code intelligence claimed. No community code imported.

## Acquisition efficiency
Raw Chiyoda hourly HTML was 241,977 bytes; /lite/ equivalent 236,559 bytes. Both return three hourly tables. This small saving does not justify a second markup contract initially. Browser DOM was 552 KB due to injected content; runtime uses direct response HTML. Request and output measurements at acceptance remain required.

## Semantic corrections during direct review
Official web forecast FAQ https://tenkijp.tayori.com/q/tenkijp-faq/detail/1138228/ explicitly says current-day min/max become estimated actuals after the morning/daytime periods elapse. Daily current-day rows therefore need field-specific temperature-kind uncertainty, separate from the post-issue weather/probability outlook; never imply all daily values refer only to the remainder of the day. Day-only seasonal reports retain report_date; publication issue_at is separate. Mountain initialization is model_initial_at and freshness reference, never a fabricated issue_at. Municipality address search must preserve city+ward and raw address, leaving exact reference name to selected place resolution.
