# Rakuten Travel CLI brief

Research checked 2026-09-27. Official-document claims and observed HTTP behavior are distinguished below. No authenticated official API call has succeeded because the user has no credentials.

## Product thesis and user vision

A small read-only agent CLI for finding Japan accommodation, inspecting room/plan offers for explicit dates and party, comparing bounded alternatives, and handing off to Rakuten booking. Integrate Rakuten Travel only. Compact JSON, stable IDs, honest price/occupancy semantics and bounded network work are primary. Deliver locally, preserve shared configuration, use Printing Press staging/promotion and live acceptance. Astra orchestrates and accepts; Sol at max effort implements and tests.

## Official API identity

Official source: https://webservice.rakuten.co.jp/documentation . Raw endpoint documents were fetched successfully and read from local captures; extracted contracts are sibling documentation-*.txt files.

| Endpoint | Documented version | Role |
|---|---|---|
| SimpleHotelSearch | 20260731 | hotel IDs (up to 15), leaf area, or coordinates |
| KeywordHotelSearch | 20260731 | UTF-8 keyword, minimum two characters, space-separated AND; optional prefecture/name-only filter |
| HotelDetailSearch | 20260731 | one hotel ID, facilities/access/ratings/property policy |
| VacantHotelSearch | 20170426 | dates, rooms, adults, six child categories; hotel/area/coordinate selection |
| GetAreaClass | 20140210 | country/prefecture/city/detail hierarchy |

Base: https://openapi.rakuten.co.jp/engine/api/Travel/ . Each endpoint version is independently documented; do not use one global API version. No official OpenAPI document located. Docs are authoritative over old wrappers.

## Access and reachability

- Register a Rakuten member account and app. App registration currently requests name, URL, type, allowed websites, data usage purpose and expected QPS. An app ID and access key are both required; accessKey may be sent as a header. https://webservice.rakuten.co.jp/guide
- Official FAQ says APIs are currently free. https://webservice.faq.rakuten.net/hc/ja/articles/900001971983
- Rate limit: at most one request per second per application ID. Endpoint docs separately warn that frequent identical URLs may be temporarily blocked. No verified daily quota found. https://webservice.faq.rakuten.net/hc/ja/articles/900001974383
- Credentials are unavailable. API access/entitlements, allowed-origin enforcement and actual success/error shapes remain unverified. Public docs/explorer examples are not credentials to embed or borrow.
- Credit/branding required. Include official credit in README and help, retain attribution in JSON metadata. https://webservice.rakuten.co.jp/guide/credit
- Cache guidance permits price/availability caching up to 24 hours and other metadata up to three months; freshness labeling is required where appropriate. Design stricter defaults: metadata 24h; live inventory uncached by default, explicit brief cache only. Never serve stale inventory as current. FAQ raw fetch got HTTP 403; indexed official FAQ was readable. https://webservice.faq.rakuten.net/hc/ja/articles/900001974343
- Terms include restrictions on private-only environments absent permission. Registration/access conditions for a purely private CLI remain a documented uncertainty; do not assert account eligibility. https://webservice.rakuten.co.jp/guide/rule (Japanese original governs).
- Public travel homepage returned HTTP 200. Its public keyword form targets https://kw.travel.rakuten.co.jp/keyword/Search.do with charset=utf-8, f_query and f_max. One read-only Japanese search for 品川 with f_max=3 returned HTTP 200, Windows-31j HTML, 150577 bytes, three property IDs (191877, 51870, 72056) and canonical hotel URLs. No credentials or browser needed for that probe. This proves property lookup only, not availability.
- Homepage dated search form targets https://search.travel.rakuten.co.jp/ds/vacant/searchVacant/ . Public room/plan retrieval and replayability remain unverified and require discovery before being approved as build scope.

## Correctness contract

Preserve hotelNo + roomClass + nullable planId + salesformFlag, scoped to a query context. Names are labels, not IDs. Canonical source URLs should be retained, without affiliate rewriting by default.

Availability searchPattern=0 exposes at most three plans per property. Use searchPattern=1 for actual plan/room enumeration, bounded hits 1–30 and pages 1–100. Explicitly expose truncation and more-results metadata; neither mode proves coverage of every website offer.

Require check-in, check-out, rooms and adults for inventory. Child categories are upper/lower elementary, infant meal+bed, meal-only, bed-only, neither. Do not collapse to a child count or infer ages. Multiple-room allocation semantics are not explicit in the endpoint table and must be checked before supporting a distribution. Invalid/unsupported distributions are not zero inventory.

Documented dailyCharge contains the first night only. rakutenCharge is JPY per person or per room according to chargeFlag; dailyCharge.total is that night's total, not the stay. Preserve original values and units; whole-stay amount remains null unless a source explicitly supplies a matching full-stay quote. hotelMinCharge is an indicative per-room/per-night minimum including tax/service, not a date-specific quote. Never multiply first-night prices by nights or adults to invent a quote.

Meals are tri-state from source flags. Availability tax/fee inclusions and plan-specific cancellation may be unknown; hotel-detail cancelPolicy is property-level text, not a guarantee for each plan. Detail includes amenities, access, check-in/out, note and cancelPolicy.

Request datumType=1 (world/WGS degrees). Default datumType=2 is Tokyo Datum arcseconds. Keep datum/units explicit and reject unrecognized or missing coordinate contracts. Search radius is kilometers, 0.1–3.0 in 0.1 increments. Tests must cover any conversion path and prevent Tokyo arcseconds being relabeled as WGS degrees.

Structured API not_found with 404 is distinct from generic 404, invalid query, auth denial, throttling, upstream failures, parse failures and incomplete coverage. Empty inventory means no matches in the requested source/query, not global sold-out status.

Official gaps: no physical remaining-room count (reserveRecordCount is plan × room combinations), no documented full-stay price series, no documented English language switch or translated area names, no payment/booking execution scope. https://webservice.faq.rakuten.net/hc/ja/articles/37004462918041 . Exclude HotelRanking: official FAQ says data last updated in 2014 and retirement planned. https://webservice.faq.rakuten.net/hc/ja/articles/43312657146905 . Unknown English free-text matching must be tested; accepting ASCII input is not proof of English coverage.

## Top workflows and proposed command families

1. Resolve a destination or keyword into stable area/hotel IDs; distinguish Japanese labels from source codes and English query coverage.
2. Read one hotel's details, facilities, access, source ratings and property policies.
3. Fetch real room/plan offers for explicit dates and party; inspect one offer with the source identities, meal flags and booking URL intact.
4. Compare an explicit bounded set of areas/hotels/date pairs with identical occupancy; report each cell's status and comparable price basis.
5. Produce a source booking handoff for the selected offer, with no booking or payment action.

Candidate command families: areas, hotels search/show, offers search/show, compare. Do not finalize public website or dual-backend scope until live discovery proves fields and replayability.

## Ecosystem and table stakes

- mrslbt/rakuten-mcp: current broad Rakuten MCP, 28 tools across six families; Travel supports search, vacancy, details, area, keyword, chains, ranking. Source at https://github.com/mrslbt/rakuten-mcp/blob/main/src/tools/travel.ts . Primary code was fetched and inspected: uses old 20170426 metadata versions, flattens blocks, represents plan prices as pricePerNight/totalPrice without carrying chargeFlag. This CLI should preserve the basis explicitly and focus on Travel. Its README reports June 2026 live verification, which is not this run's proof.
- hirochachacha/rakuten_travel_mcp: Deno travel search/vacancy and area resource. README uses legacy app.rakuten.co.jp and app ID only. Public issue #2 reports LangGraph hotel-search failure; inspected report does not establish a broad upstream outage. https://github.com/hirochachacha/rakuten_travel_mcp/issues/2
- Wrapper issue searches for 403, blocked, broken, deprecated, rate limit, access/auth found no demonstrated recent systemic Travel outage in the two small issue sets. Authentication/migration risk remains high for stale wrappers.
- npm node-rakuten is old endpoint-level access (2013 versions); PyPI rakuten is an old basic wrapper. They establish parity candidates, not current contracts. https://github.com/tatsuya/node-rakuten ; https://pypi.org/project/rakuten/

Concrete pain points: misleading total-price labels; omitted child categories and ambiguous multiple-room party allocation; stale auth/version assumptions; large unbounded responses; unverified English destinations. Distinguish our verified findings from competitors' claims.

## Local data and efficiency

Highest-gravity entities: area hierarchy, hotel ID, room class, plan ID, query party/date context. Keep a bounded metadata cache, not a full travel database. No need for bulk sync, a resident browser, or a server. Live offers are fresh by default; caching is opt-in and explicitly timestamped. Store no cookies or credentials in cache/proofs.

Compact JSON stdout; diagnostics stderr. Null unknown scalars and empty arrays for collections; stable field selection, explicit page/limit/max-requests. Cap comparisons at a small matrix (candidate maximum 9 queries), serial outbound requests with >=1s spacing per app, bounded retries/timeout/response body and no nested hidden searches. Measure uncached/cached output bytes, requests, latency, peak RSS once a backend is viable.

## Build priorities and acceptance

P1: proven lookup + one-hotel detail + date/party-specific plan offers. P2: bounded same-basis comparisons and booking handoff. P3: concise help/README/agent skill, deterministic contract tests, efficiency report. Tests: price basis and first-night limitation, child/room occupancy, coordinate units, pagination and failure interpretation. Live E2E must cover property lookup and availability. If official credentials remain missing and public replay is not viable, HOLD rather than promote a mock-only product.

## Public-route discovery update

Anonymous browser interactions and cookie-free direct HTTP replay proved property lookup and real dated offers. Public pages expose full-stay totals with explicit per-room and tax labels, and preserve hotel/plan/room IDs. Recommend a focused credential-free website backend for this delivery; official API contracts remain documented as an optional future backend, not an unverified shipped path. Public route is more fragile than an official API, so parse contract mismatches must be errors. See ../discovery/browser-sniff-report.md. DeepWiki was unavailable; the direct source readings remain sufficient. Additional Rakuten skill inspection found useful absolute-date, ambiguity, paging and charge-basis conventions; no broader multi-service features are needed.
