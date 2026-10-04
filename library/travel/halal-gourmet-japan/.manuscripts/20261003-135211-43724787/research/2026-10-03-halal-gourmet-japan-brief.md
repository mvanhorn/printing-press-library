# Halal Gourmet Japan CLI Brief

## API Identity
- Domain: https://halalgourmet.jp/; public English/Japanese restaurant and mosque/prayer-space directory.
- Users: Muslim travelers comparing food conditions and prayer facilities across Japan.
- Data profile: read-only Next.js pages with structured page data; website, not a published API. No account use is needed for scoped discovery.

## Reachability Risk
- Low for public pages; initial shell sandbox DNS failed with curl status 000, while native Chrome rendered the site and approved ordinary HTTP returned 200 text/html.
- Live browser search: `/search?q=ramen&prefecture=Tokyo&feature=certified`, seven results. No challenge, login or CAPTCHA observed.
- No known SDK/CLI or wrapper repository found in focused search; wrapper-issue access-risk check is not applicable. No invented official API.

## Users
- Muslim visitors planning food and prayer stops for an active Japan trip: use the directory while arranging each destination/day, retain a shortlist and recheck details before visiting.
- Diners requiring documented certification in addition to other explicit conditions: compare source certification labels and verification dates separately; missing certifier/validity evidence needs clarification with the venue.
- Travelers fitting prayer stops around sightseeing and transport: check facility labels, access instructions, hours and restrictions before including a stop.
These are workflow personas inferred from the user's Muslim-travel brief and the provider's own visitor/resident audience description, not interview participants.

## Top Workflows
1. Search restaurants by Japanese destination, keyword, cuisine and explicit source food/facility condition filters.
2. Inspect a stable restaurant ID and preserve certification label separately from halal meat, seasonings, no pork, no alcohol, tableware and prayer-space claims.
3. Find mosques or prayer spaces by prefecture and facility conditions, then inspect hours/access restrictions.
4. Compare selected places against explicit requirements with reported, not-reported and inapplicable states.
5. Keep bounded factual saved inspections locally for offline planning with observation timestamps.

## Table Stakes
- Source website already offers region, cuisine, ten distinct restaurant feature filters and prayer-type/facility filters.
- Other public directories such as https://www.halalfoodmaps.com/ and https://halaljp.com/ offer mixed food/prayer map discovery. Japan Halal Foundation's https://japanhalal.or.jp/shop offers certifier-specific records.
- Install advantage: structured evidence, source URLs and dates, transparent requirement matching, multi-place comparison and cached offline inspection without conflating certification with friendly labels.

## Data Layer
- Primary entities: factual restaurant and prayer-place snapshots, keyed by source kind and stable numeric ID, with observed timestamp and canonical page URL.
- No bulk mirror, personal contributor profiles, session state, raw signed media URLs or full editorial prose retained.
- Cache only successful scoped reads; bounded TTL and explicit offline provenance. Source search itself stays live and bounded.

## User Vision
- Broad nationwide Muslim-travel coverage beyond ordinary place discovery; compare certification, halal dishes/meat/seasonings, pork/alcohol policies, kitchen/tableware and prayer facts when explicitly supplied.
- Read-only source planning; publishing is already authorized after gates. No bookings, messages, posts or account changes.

## Product Thesis
- Name: halal-gourmet-japan-pp-cli.
- Preserve the difference between a venue's source-reported conditions and HGJ platform verification, and expose unknown evidence instead of blanket halal assurance.

## Build Priorities
1. Real restaurant and prayer search, source-supported parameters and bounded results.
2. Real detail inspection, canonical IDs/URLs, source names, verification date and condition evidence.
3. Compare explicit requirements; missing condition evidence never means explicit false or certification.
4. Local successful-snapshot cache with offline search/show, bounded network limits, meaningful parsing and domain tests.
5. Agent-native JSON and CLI/MCP surfaces, honest docs and real live checks before publication.

## Evidence and limits
- Source homepage/native UI observed 2026-10-03: ten food/facility condition filters and four prayer conditions Wudu, Wi-Fi, Hot Water and Qibla. Advertised counts are source claims, not an independently checked inventory.
- HGJ Verified month/year labels appear on search cards and remain separate from certification. Expiry/certifier evidence may be absent.
- Browser action uses no user location or authentication. No API credentials are requested.
- Next phase must resolve exact search/detail replay and prayer paths before approving implementation scope.

## Evidence refinement
- Search cards show only five icons followed by `+`; omitted card labels are not complete-detail omissions. Full detail reads are required for matching all requested conditions.
- Source repeated `feature` query keys are source-defined discovery semantics and are never assumed to require every condition. The CLI's strict matcher evaluates each requested label from full detail.
- Detail pages include JSON-LD Restaurant/PlaceOfWorship with canonical numeric URL, names, address, coordinates and explicit positive amenityFeature labels. HGJ badge month and weekly hours are separate rendered evidence.
- API/plugin/CLI/npm/PyPI ecosystem searches found no scoped wrapper; official plugin inventory returned no halal/gourmet match.
- Generation decisions: travel category; standard HTTP; no auth; stdio MCP with Cobra command mirror as the domain surface; three high-frequency prefecture aliases and numeric source IDs seed learn vocabulary. Generator automatic cache refresh is disabled because this is bounded manual snapshot state, not a bulk syncable catalog.

## Implementation references
- https://github.com/mvanhorn/cli-printing-press — generator used for scaffolding and validation.
- https://github.com/mvanhorn/printing-press-library — publication target and public attribution URL in the source client's User-Agent header; it is not an additional travel data source.
