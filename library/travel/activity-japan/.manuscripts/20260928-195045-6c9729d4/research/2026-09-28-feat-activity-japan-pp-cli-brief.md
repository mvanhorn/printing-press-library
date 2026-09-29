# Activity Japan CLI brief

## API identity and access

Activity Japan sells Japan activities and tours through its Japanese and localized sites. The desired CLI is an anonymous, read-only itinerary discovery and canonical booking handoff tool. Its primary entities are operator, plan, option, session and a date/party-specific quote or availability observation; these must remain distinct.

The official information API and reservation API are **contract-gated**, not public documentation. The [official API page](https://pr.activityjapan.co.jp/api) says documentation and connection details follow a nondisclosure agreement, then an affiliate or sales-agent contract. The affiliate route hands booking to Activity Japan. The page gives no public fixed API price; commissions depend on traffic and scale. It describes 17,000+ plans, with possible request limits if other partners are affected. Reservation API requires a fixed IP and has no payment function. No credentials or specification were supplied for this run.

The public English and Japanese homepages, robots.txt, and sitemap indexes returned HTTP 200 on 2026-09-28. The declared sitemap plan indexes are accessible. Direct unauthenticated HTTP reads of representative `/search/...` and `/publish/plan/...` pages in **both** languages returned CloudFront 403. A browser-like header probe returned `x-amzn-waf-action: challenge` (HTTP 202). Browser access was rejected by automatic approval review as an access-control bypass; no further browser attempt was made.

The [English robots file](https://en.activityjapan.com/robots.txt) and [Japanese robots file](https://activityjapan.com/robots.txt) allow browse paths but disallow filtered search query parameters (date, party, budget, page and other parameters). No public OpenAPI, partner schema, or maintained Activity Japan wrapper was found. A public registry snapshot contained no Activity Japan CLI entry; the raw registry endpoint returned 404 from the terminal, while the web reader found no matching text in its indexed copy.

## Surface coverage

The [English sitemap](https://en.activityjapan.com/sitemap/en/sitemap.xml) and [Japanese sitemap](https://activityjapan.com/sitemap/ja/sitemap.xml) point to separate plan and search indexes. Directly counted on 2026-09-28: English plan URLs 11,331; Japanese plan URLs 17,942; plan ID overlap 11,072; English-only 259; Japanese-only 6,870. English search URLs 37,969; Japanese search URLs 48,144. These are **sitemap listings**, not proof of current bookability or exact live inventory. The two language surfaces must be queried and reported separately; an English listing must not imply an English-speaking instructor.

Search results visible through the indexed [Kyoto English page](https://en.activityjapan.com/search/kansai/kyoto/) expose plan ID, operator name, headline `JPY～` price, age range, broad duration, and some displayed start times. They do not establish a selected-date quote or seat availability. The [FAQ](https://en.activityjapan.com/faq) says a request booking requires the operator to respond, and cancellation policy varies by plan. Those conditions must not be promoted to an instant confirmed reservation.

## Reachability risk

**High.** The website’s important search/detail routes are WAF challenged from this environment; the documented partner API is unavailable without a contract. The directly reachable sitemaps contain URLs and timestamps, not enough plan facts to satisfy detailed search, compare, or date availability. A CLI built from them alone could provide an honest language-aware browse and handoff index, but not the requested full experience discovery.

## Top workflows

1. Find plans by itinerary stop and interest, returning compact plan IDs, operators, broad cost/duration/eligibility and canonical URLs.
2. Open a selected plan lazily for meeting point, venue, guide language, inclusions, restrictions, cancellation and options.
3. Check a future date and party for session status and price, retaining observation time and unknowns.
4. Compare a bounded shortlist against the traveler’s hard constraints, with no false compatibility claims.
5. Hand off a selected plan/session through its canonical Activity Japan URL.

These are product requirements. At present only language-aware sitemap inventory discovery and canonical URL handoff have a verified unauthenticated runtime source.

## Table stakes and user pain

The website itself provides category/area browse, date and party filters, plan details and booking flow. Agents need concise structured results, explicit uncertainty, cross-language coverage and identity continuity. Common failure modes are treating `from` as selected-party total, treating general schedule text as live seats, and inferring instructor language from the site locale. The CLI should make all three impossible by schema and labels.

## Data layer

If a source becomes accessible, cache bounded plan summaries/details keyed by source language and plan ID, with TTL, observation time, source URL, and separate ephemeral availability observations. Never cache a reservation claim. Sitemap IDs and URLs alone may be held in a bounded on-disk inventory for coverage checks, not described as detailed plans. A global sync of all 17,000+ plans is outside this focused product.

## Product thesis

**Name:** Activity Japan CLI (`activity-japan-pp-cli`). A narrow Activity Japan itinerary discovery tool that returns decision-useful source facts and explicit unknowns. Its value over the website is compact JSON, stable identity and language-aware comparison. Its value depends on reliable detail and availability access; a sitemap-only catalog is a limited fallback.

## Build priorities and scope gate

- **Verified now:** homepage taxonomy, separate JA/EN sitemap plan and search URL indexes, canonical plan IDs/URLs, no-auth access to these sources.
- **Credential-gated:** official information API and its plan data, and any API-side availability/price capability whose schema the partner contract may expose.
- **Unavailable from this environment now:** public search/detail route reads, filtered search, selected-date/party availability, plan-level conditions, and live price interpretation.

Grounded workflows with current verified data: (a) itinerary-stop to canonical browse URL; (b) determine whether a known plan ID is indexed in EN, JA or both; (c) compare language-surface coverage for regions/categories based on search URL paths. These are insufficient for the requested full product, so the build must not claim broader capability until source access changes.

## Browser discovery addendum (2026-09-28)

With the user's explicit approval, an installed headless browser still received CloudFront 403 on EN search, JA search and an EN plan. Codex's in-app browser was unavailable. Codex computer-use showed the user's existing Firefox could read an EN Kyoto search result and plan 62375, while a fresh private Firefox window reached a Human Verification challenge. No challenge was solved. A full Firefox HAR export was rejected by automatic approval review because it could persist session credentials; no browser cookies or authorization headers were exported.

The Firefox Network Monitor exposed six useful GET routes on `https://gd.activityjapan.com`: `/get_plan_price_info`, `/plan/get_plan_data`, `/plan/get_calendar`, `/plan/get_plan_price`, `/select_plan_course`, and `/plan/check_calendar_data`. All six replayed anonymously with direct HTTP and real response bodies. A credential-free enriched capture and Printing Press sniffed spec now exist in `discovery/` and `research/` respectively. The sniffed spec needs repair before generation: it selected `plan_price` as the `/get_plan_price_info` response path, losing sibling `plan_data` and `price_items`, and inferred numeric IDs rather than source string IDs.

For plan 62375, `/get_plan_price_info?lang_flag=en` and `?lang_flag=ja` returned the same `plan_id=62375` and `partner_id=9326` with distinct English/Japanese names. `plan_price_id` identifies a price option separately from the plan; the premium option is not the base option. `/plan/get_calendar` for 2026-10-07 returned JPY 2,000, while 2026-10-08 returned JPY 1,800; `/plan/get_plan_price` confirmed the selected-date amounts for the named price option. The calendar and `/select_plan_course` return source status integers and distinct course IDs. The public calendar JS labels status 1 with `p_bookingOk`, status 3 with `p_request`, 4 with `p_closed`, and renders 5 as unselectable. The English page legend spells out immediate booking, request booking, reception closed, and not accepted. Treat status 2 as unknown until separately verified. `/plan/check_calendar_data` is a read-only GET stock check; for 2 people on a future course it returned `result=1, stock=30`. A count above the plan's listed maximum returned `result=3` (switch to request), so the CLI must enforce the plan maximum before making an availability claim.

The detail endpoint returned `support_language=true` in both EN and JA for plan 62375, and the EN page displayed "Supported language: English". Another plan (28670) returned `support_language=true` for JA and `false` for EN. This flag's full semantics are not yet established, so expose it as source evidence instead of asserting instructor language from website locale alone.

**Updated capability classification:** Verified anonymous direct reads: known-ID detail, operator/plan/option identities, source conditions, date-specific price lists, calendar dates, course/session IDs and read-only stock checks. Credential/clearance-gated: server-rendered destination/category search listings. Unavailable/unknown: robust broad search without a cleared session or contracted API, exact participant totals for ambiguous per-pair/group pricing, any reservation guarantee, and source states not yet mapped.
