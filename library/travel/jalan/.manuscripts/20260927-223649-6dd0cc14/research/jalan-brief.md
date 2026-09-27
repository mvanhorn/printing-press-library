# Jalan CLI research brief

## Intent and product thesis
A focused read-only Go CLI for Japan accommodation discovery and exact room/plan understanding. Jalan is the sole source. Preserve Japanese text, independent property/room/plan IDs, canonical booking handoff links, explicit unknowns, Tokyo dates, and observed availability timestamps. Root owns research, architecture and acceptance; gpt-6-sol/max workers own bounded implementation/tests. No global tool configuration changes.

## Access evidence (2026-09-27)
- Official portal https://www.jalan.net/jw/jwp0000/jww0001.do and FAQ https://www.jalan.net/jw/jwp0300/jww0303.do return HTTP 200. New API registrations closed 2020-02-25. Existing API keys are required. No key available so far.
- Stock API docs https://www.jalan.net/jw/jwp0100/jww0105.do return HTTP 200; documented http://jws.jalan.net/APIAdvance/StockSearch/V1/ returns HTTP 400 without a key. HTTPS endpoint times out. This establishes endpoint reachability, not successful authenticated use. Preserve this credential-gated distinction.
- Public homepage, Hakone area listing, dated search (2026-11-10, one night, two adults, one room), and properties 385995 / 371898 all return full HTML with HTTP 200 without credentials. Ordinary HTTP is sufficient for these observations. Charset Windows-31J/Shift_JIS needs decoding.
- Public native search form: /uw/uwp1400/uww1400.do with lrgCd, stayYear/Month/Day, stayCount, roomCount, adultNum; dateUndecided explicitly separate. Inventory rows carry separate yadNo, planCd, roomTypeCd.
- Keyword search form: /uw/uwp2011/uww2011init.do; destination alias coverage and date/filter behavior still require verification.

## Users and pain points
Agents helping couples choose ryokan need to distinguish private in-room baths from shared reservable baths and hot-spring water. Families need child meal/bedding categories and per-room occupancy, not an undifferentiated headcount. Flexible travelers need comparable payable totals for a few dates without confusing conditional coupons or earned points with cash prices. Regional stays often need Japanese names and access instructions preserved.

## Top workflows
1. Resolve destination and search a dated party, inspect a small shortlist.
2. Open property facts, bath/amenity/access and source review categories lazily.
3. Inspect room and plan identity, meals, restrictions, price basis, fees and cancellation evidence.
4. Compare bounded alternative dates or plans using equal occupancy and price units.
5. Hand off to the canonical Jalan offer page for booking.

## Ecosystem evidence
ngs/yadosearch-api is an open Go proxy offering hotel and plan/vacancy discovery through legacy keys; inspect its source/docs as corroboration only. Its single issue/PR concerns plan/vacancy support, not access failure. Apify piquno/jalan-japan-hotel-scraper advertises generic hotel/ryokan extraction from $80/1000 results; it is a paid alternative, not a CLI dependency. No relevant npm/PyPI wrapper found in targeted searches (the npm jalan package is unrelated Redux routing).

## Architecture direction and table stakes
Prefer one lightweight HTTP fetch per page; separate search from expensive detail. Compact response-level metadata, bounded pagination, field selection, fresh inventory by default, optional explicit cache reuse, request/latency metrics. Preserve incomplete results and access errors as distinct states. No reservation/account mutation or alternate OTA integration. No offline inventory claims.

## Data layer
Property, room, plan, and dated offer remain separate entities. Cache public observations with exact observation time and freshness policy. Static destination aliases may be local with source links; unknown queries must fail clearly instead of inventing location matches.

## Verification priorities
Live regional ryokan, onsen property and urban hotel searches; dated party/filter fidelity; room/plan source cross-check; empty/unsupported/access-failure distinctions. Deterministic fixtures for extraction, price units, children, paging and partial failure. Measure stdout bytes, requests, latency and peak RSS separately cold/warm.

## Remaining discovery
Exact room/plan details, cancellation and tax evidence, keyword and English support, filter contracts, page sizing and limits. Final scope will distinguish verified, credential-gated, unavailable and unverified fields.

## Final implementation decision
User approved focused anonymous HTML scope. Standard HTTP runtime, local Go CLI, compact JSON. Browser capture confirmed rendered controls; raw/direct fixtures establish property, offers and exact-plan contracts. Legacy API and Korean-widget MCP excluded. MCP initialize/tools/search were live-tested: hotel-search with ko_KR returned10 of126 hits, no room/child/page schema. No credentials or paid service used by runtime.

## Verified source semantics
- Child categories: elementary, infant meals+bedding, infant meals, infant bedding, infant neither. Native dynamic_pctop.js confirmed per-category0..5, comma-separated six-digit per-room encoding. Adults9 is a9-or-more bucket; exact CLI accepts1..8. Rooms1..10,nights1..9 from native controls.
- Native search idx5 rounded to first30; logical CLI pages align upstream30 and slice locally. Sponsored cards excluded from pageable organic rows. Fresh calls are non-atomic; explicit cache reuse stabilizes slices within an observed sourcepage. A firstlive paging comparison differed; subsequent two identicalfresh queries matched. No claim of deterministic live ordering.
- Offers source plan count is not room/plan tuplecount. One sample has12plans/54tuples. Preserve both rather than misuse the count as a pagination total.
- Property371898 has onsen facilities, while room outdoor baths explicitly are not hot springs. Private shared bath use can be available without reservations; private and reservable are distinct.
- Plan385995/03912759/0576806 observation:1-night base74800JPY,2-night149600JPY, conditionalcoupons and points separate, bathing tax additional. Family sample385995/03806855/0546600 has2rooms with2adults+1elementary each, base124740JPY and per-room breakdown. Values are captured observations, never future promises.
- Requested2026-12-31 exactplan returnsHTTP200 with undated referenceprice and native rejection; availability unavailable, dated amountnull. Property offers2027-09-20 native message means no matchingplans OR stopped reservations, not globallysoldout.
- Filter echo must preserve source dates, party and checked filters before publishing matchedresults. Exactplan/property-inapplicable filters are rejected.

## Root design decisions overriding generic workflow breadth
No speculative features, alternate providers, account actions, broadsync or offlineinventory. Root directly performs synthesis/review as instructed; no planner subagent. Three implementation/test workers ran gpt-6-sol max. Shared config updater and publishing steps excluded under explicit user localdelivery constraint. Full live dogfood is already authorized by the user's verification requirement; no redundant depth prompt.
