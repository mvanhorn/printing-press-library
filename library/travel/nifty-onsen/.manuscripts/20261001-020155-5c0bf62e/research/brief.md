# Nifty Onsen CLI research brief

## API Identity and Product Thesis
Single-source, unauthenticated discovery for Japan day-use onsen, ordinary baths/sento, spas and hotel day-use facilities. Nifty Onsen publishes regional/keyword search, detailed admission and schedules, map discovery, ratings, and public coupon descriptions. This is an unofficial public website integration, not a documented public API.

## Verified Source Surface (2026-10-01 JST)
- https://onsen.nifty.com/ — 200, UTF-8 HTML.
- https://onsen.nifty.com/tokyo/search/ — 200, real list cards, source IDs, coordinates, source filters; page-2 link. Sponsored cards exist separately from li.shop organic listings and must be excluded.
- https://onsen.nifty.com/saitamashi-onsen/onsen012278/ — 200, server rendered detailed semantic h4/value fields, Japanese source text, source ratings.
- Legacy /cs/catalog/onsen_onsen-detail/catalog_onsen012278_1.htm redirects to canonical facility URL; supports ID lookup.
- /saitamashi-onsen/onsen012278/coupon/ — 200, public coupon descriptions, app restrictions, subscription conditions, validity and terms. No issuance or redemption.
- /map/ and /map/js/pc/common.js — 200. Public map uses POST /android_app/api/get_detail_search_data.jsp as a read-only query. Replayed same-origin AJAX headers plus ephemeral page key succeeds, result=1, 20 facility summaries near Shinjuku. Missing AJAX context returns HTTP 200 result=2 (__illegal_request), so semantic errors must be checked. Key and session state remain memory-only.

## Top Workflows and User Research
1. Traveler picks a prefecture or Japanese keyword, narrows to day-use and meaningful bath features, then lazily checks admission/access for a shortlist.
2. Traveler near a station uses explicit coordinates to find day-use baths sorted by distance; wants bounded source coverage, not a claim of exhaustive nearby inventory.
3. Family checks family/private bath evidence and separate fee text without mistaking a private room, a generic private-bath tag, or a hotel listing for rentable inventory.
4. Budget traveler compares weekday/weekend/holiday/add-on and coupon terms; source minimum price is not a payable quote.
5. Agent needs concise JSON and reusable IDs/URLs, unknown tattoo/child/accessibility policies, and timestamps rather than guesswork.
Pain points evidenced by source structure: extensive ads/review text surrounds key facts; mixed facility types; prices/terms vary; combined private bath/private room category is ambiguous.

## Table Stakes and Ecosystem
Primary incumbent is Nifty's official website and mobile app (https://play.google.com/store/apps/details?id=com.nifty.onsen.ofulog.android): area/nearby lookup, coupons, ratings and filters. Searches for Nifty Onsen / ニフティ温泉 CLI, MCP, SDK, npm, PyPI and GitHub yielded no relevant public SDK/CLI/MCP. Unrelated Nifty stock index tools excluded. No wrapper repository exists to audit for access issues. Alternative discovery websites are outside this explicitly single-source scope.

## Data Layer
Parsed response cache only, bounded 128 entries / 32 MiB, 6h freshness; offline mode surfaces stale status. No bulk mirror, SQLite, review corpus, account cookies, API keys, or hidden hydration data persisted. Source IDs and canonical URLs are primary identity. Explicit nulls encode unavailable facts. All times are JST.

## Domain Rules
Source labels preserved. Natural hot spring only from explicit natural-hot-spring field/tag, ordinary bath only from explicit source category; never infer from a name. Private rentable bath vs room remain separate nullable states with exact evidence. Display raw schedule/admission lines, label price hints as JPY source minima with basis unknown. Preserve weekday/holiday/session/per-facility text and add-ons instead of inventing normalized quotes. Tattoo, child admission and accessibility require explicit facility evidence. Listings and coupons do not establish reservable inventory or guaranteed acceptance.

## Build Priorities
bath search, bath nearby, bath show, bath coupons, regions, filters; compact JSON default, --select/--limit/--page, no resident browser. Bounded HTTP retries/deadlines/response sizes, semantic errors, caching and offline mode. Deterministic tests for identity, source-only claims, hours/prices/coupon preservation. Live region/type/filter/detail/coupon cross-checks and metrics.

## User Authorization and Workflow Overrides
User requests autonomous build, no routine questions, no publishing/PRs/purchases/global config changes, sole builder plus exactly one fresh-context reviewer. This supersedes Press prompt gates and novel-feature/review delegation defaults. Approved scope is the manifest below; one reviewer reserved for the final independent code/evidence audit. Global skill updater omitted to preserve global state. Browser capture is unnecessary because full public HTML and map query replay are verified; browser gate records direct discovery under preauthorization.
