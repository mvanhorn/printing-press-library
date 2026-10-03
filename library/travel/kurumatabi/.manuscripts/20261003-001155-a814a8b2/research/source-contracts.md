# Kurumatabi public source contracts

This is one ecosystem: RV Park, Kurumatabi Park and related provider park categories share official discovery/detail contracts. It is a read-only planning integration. No command reserves, pays, logs in, contacts a host or claims an exact dated vacancy.

## Verified primary pages

- Official discovery: https://www.kurumatabi.com/park/search.php
- Modern RV Park detail: https://www.kurumatabi.com/park/rvpark/1086.html
- Legacy RV Park detail: https://www.kurumatabi.com/park/rvpark/712.html
- Contrasting membership detail: https://www.kurumatabi.com/park/yypark/213.html

Discovery began with ordinary native browser navigation and DOM inspection. Public form/assets were then replayed through ordinary HTTP; no CDP, login or clearance bypass was required. The CLI uses standard HTTP rather than a resident browser.

## Search contract and bounded coverage

POST /park/search.php preserves the native form vocabulary: from_side_search=on, area_pref, category[], okonomi_search[], availability_period[], vehicle_size_search[] and src_word. The CLI exposes verified labels and wire values through parks filters. Park kinds include rvpark, yypark, camp3000, campjrva, gourmet, minpark, train and kurumatabipark; types are not separate competing CLIs.

The initial POST creates an ephemeral public search-cookie jar. Continuation pages use GET /park/search.php?start_num=20 rather than repeating the POST, which resets the native filtered search. Cookies are kept in memory and never exported. Native pages ordinarily contain20cards. --max-scan-pages bounds provider work(1..5); --limit bounds output(1..100). Provider total, scanned rows/pages, returned rows, output truncation and provider-page exhaustion have separate fields.

Observed Nagano RV Park fixtures represent28records across20+8cards; an older broad-search fixture represents1010provider records with20returned cards. Those are observation-specific counts, not a current nationwide-size guarantee. Minimal fixtures preserve count evidence, native card/detail nodes, enabled/disabled icon attributes and exact source labels; unrelated site chrome/scripts/widget keys and business email values are excluded or replaced with placeholders.

## Details and conservative interpretation

Canonical kind/numeric-id and source URLs identify records; Japanese names remain intact. Detail headings/definition lists, modern and legacy vehicle/facility icons, tariff-audience lists and published-update notes provide the evidence. Dimensions have metre units, nullable missing bounds and a distinct explicit-unrestricted-height flag. Their scope is the published parking space, not road clearance or guaranteed host acceptance.

Facility states are yes/no/unknown; fees are free/paid/included/unknown. Disabled legacy filenames override positive alt text. Explicit sections override icons while preserving conflicts. A generic dump station never proves black/grey acceptance; explicit prohibitions and qualified statements are retained. Toilet hour ranges and negations override broad24h icons. Bath/shower evidence does not invent access or availability.

Per-record membership requirements/waivers override broad park-type policy. RV Park and Kurumatabi Park generally permit nonmembers; YY/other categories and individual restrictions can require membership. Discounts do not imply a membership requirement. Qualified tier exemptions stay uncertain rather than granting universal nonmember access.

Tariffs preserve audience and basis in JPY, not dated inclusive stay totals. Same-day reservation acceptance is separate from vacancy, which remains unknown. Opening-period labels do not cancel seasonal notes. Permitted overnight vehicle stay does not establish general outdoor-camping permission.

## Native architecture and cache decisions

Foundation commands are parks search/detail/filters/handoff. Derived workflows are fit/compare/near/audit/match. Live reads use caller context, HTTP/body bounds and polite source pacing; a429is a typed failure, not an automatic cached success. Auto fallback after ordinary failures explicitly reports local evidence; live mode requires real source responses.

SQLite stores observed Park records and preserves unrelated tables. Lower-detail search cards do not overwrite richer details. URI escaping handles unusual paths; context-bounded busy waits handle concurrent writers without unbounded lock hangs. --no-cache bypasses observation writes/fallback. Default paths are resolved after runtime home flags.

Compare aligns22evidence rows across two to eight distinct IDs and separates requested/compared/fetch failures. Agent output preserves meta/results. Near uses Haversine straight-line distance inside the observed cache, with explicit scan/output caps and coordinate gaps. Match sorts proven before uncertain before ruled-out candidates; unknowns and card-only observations remain uncertain, and returned ID partitions cover only displayed rows.

## Accepted limitations and durable customization

There is no dated vacancy/quote API in this contract. Near/match require a primed observation cache. Source HTML can change; host confirmation remains necessary for conditional pitches, activities, dates and extras. Generated raw endpoint tools are hidden; the useful native MCP tools mirror Cobra domain workflows. Some static metadata still counts two hidden endpoints rather than the complete runtime catalog. Native parks_compare is clearer than the generated recipe alias; generic context advice must not replace actual help.

The native evidence/cache/planning code, CLI extension hooks and minimal fixtures are indexed in the per-patch customization record. Original installed artifacts and historical generation evidence remain private and unchanged; this public manuscript is a curated research record, not a raw response dump.
