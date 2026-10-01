# ecbo cloak CLI research brief

Product: focused Japan luggage-storage discovery and read-only offer inspection. Source: https://cloak.ecbo.io/en and first-party api.ecbo.io/search.ecbo.io language variants. No account, paid key, bookings, payments or account actions.

## Access and economics
2026-10-01: GET search.ecbo.io/api/v1/spaces returns HTTP 200 without auth. It returns at most 50 nearest hits despite page/page_size parameters; total describes more hits, not a nationwide inventory. Dates and small/large bag counts reduce matching hits. Legacy IDs redirect from /en/space/ID to /en/spaces/UUID. Public GET api.ecbo.io/api/web/spaces/UUID returns full facility JSON without auth. POST /api/web/reservations/price and /validate are anonymous read-only operations (first-party page calls) returning live price/currency and valid/errors. They create no reservation. Legacy app capacity endpoint returns 401 Not API Authorized and is excluded; no credentials copied from browser scripts. No documented public API, SLA or rate quota found; bounded conservative HTTP calls, optional local cache only. Public website costs nothing for reads. Booking requires account/payment and is out of scope.

## Coverage and domain findings
Facilities have UUID/encrypted ID, localized address, coordinates, weekly/holiday hours, daily category prices, maximum small/large quantities, can_reserve_multiple_days, same-day and holiday rules, extra_info and introduction. JR station rules can require two suitcase slots for long items. Raw availability ratio can exceed 100 and has no documented remaining-count meaning; preserve as uninterpreted source observation with source fully_status_updated_at. Maximum capacity is not remaining capacity. Price quote is not validation and validation is an observation, not reserved/guaranteed capacity. Separate acceptance/pickup cutoff fields are absent: return null rather than infer from closing hours. Overnight closing hours cross midnight. Holiday exceptions and overnight flags preserved; rely on source validation rather than guessed hours logic.

## Policy evidence
First-party home/translation and help articles disagree at exactly 45cm and about calendar vs business-day charging. Expose boundary uncertainty and source links; source quote is authoritative for requested interval. General rates vary by facility. Prohibited items and 20kg maximum summarized with official help links; facility restrictions preserved verbatim, not replaced by generic policy.

## Workflows and data layer
1. Find nearby facilities using coordinates, Japanese/English names, compact results and local pagination over source window.
2. Fetch full detail lazily using either source ID or canonical UUID.
3. Inspect exact from/to and small/large counts with source price and source validation; both remain distinct.
4. Explicitly refresh a bounded local inventory window and search it offline with freshness.
5. Project selected fields and inspect diagnostics/request counts without polluting JSON stdout.
Cache: response files in an explicitly selected project cache during build, default user cache at runtime; detail TTL 1h, discovery TTL 5m; offer calls always live. No auto nationwide sync.

## Ecosystem
Targeted search found no established ecbo-specific CLI/MCP/SDK on GitHub/npm/PyPI. Incumbent is first-party map/app. Adjacent luggage services exist but are excluded. Wrapper-issue/deepwiki audit not applicable without a matching wrapper repo. Provider's map supports time/count discovery and reservations; only read scope absorbed. No extra providers.

## Build priorities
Cobra Go direct HTTP; focused facilities near/get, offer inspect, inventory refresh/list; source IDs and canonical URLs; bounded retries/timeouts/body/cache; structured predictable errors; deterministic tests for parser/input/cache/projection and no inferred availability. All research/implementation sole-builder; novel-feature brainstorming delegation overridden by user's exactly-one-reviewer constraint.

First-party language URL probe after build: zh-TW and ja facility routes return 200 at the requested canonical route; zh-CN redirects to /ja/spaces/UUID. Booking URL normalization therefore uses ja for zh-CN API-language requests.
