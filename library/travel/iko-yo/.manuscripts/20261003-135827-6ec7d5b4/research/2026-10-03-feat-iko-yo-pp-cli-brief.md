# Iko-yo Trip CLI brief

## API identity and user vision
Source: https://trip.iko-yo.net/, the same Actindi operator as Iko-yo. Its [FAQ](https://trip.iko-yo.net/faq) distinguishes local parent/child trips and municipal experiences from the core daily-outing catalog. The user wants nationwide family discovery beyond generic place lookup, explicit age/facility evidence, indoor alternatives, event dates, separate fees and booking conditions. Read-only source work and publication are already authorized.

## Reachability
High risk for core https://iko-yo.net/: new native Chrome homepage and web fetch both returned 403; prior builder also saw 403 on facilities/events/robots. Search-index descriptions do not establish runtime access. No documented public API emerged from a focused official API/app search. The official iOS listing describes ordinary/trial login, not a developer API; no mobile installation/authentication or private endpoint probing is planned.
Trip public FAQ, spot listing, event listing, spot 8220 and event 8412 returned ordinary HTTP 200 without credentials. Native Chrome verified server-rendered national, region and prefecture lists and pagination. The sandbox DNS failure was environmental: the same request succeeded outside the sandbox. No challenge solving or bypass is used.

## Product thesis
Name: Iko-yo Trip; binary: iko-yo-pp-cli. Family trips need published event schedules, fees, booking constraints and explicit family amenities, which generic POI search omits. This integration covers Trip’s selected municipal/event/experience catalog, not the full core Iko-yo catalog or its age-filter API. On 2026-10-03 the national lists had 44 spot pages and 241 event pages of 15 entries; archive entries are not all upcoming or operating.

## Users
- Parents of infants and toddlers checking the next weekend’s outing against explicitly published age, indoor, nursing and changing facts. This follows the user’s stated family scope and the official Mooovi facts; missing amenities are a key practical uncertainty.
- Parents taking children on a regional weekend trip and choosing between municipal events that overlap their limited trip window. The official Trip FAQ describes weekends/long weekends and local parent/child experiences.
- Parents organizing a multigenerational family outing who compare child/adult charges and application deadlines before shortlisting an event. The official Iko-yo app description lists weekend, travel and multigenerational planning use cases; Trip event 8412 supplies application/lottery evidence.

## Top Workflows
1. Discover selected spots/events by region and prefecture; filter keywords and event dates locally within a bounded, reported page window. Follow observed page links. Never describe an empty bounded window as source-wide absence.
2. Inspect stable kind/id references, normalize basic information tables and preserve Japanese names, location, official URL, published/updated/observed dates and source evidence. Date ranges are published schedule evidence, not guaranteed daily operation.
3. Compare up to eight selected entries against a family date and requested amenities, exposing matches, exclusions and unknowns. Numeric age suitability and facilities require explicit source statements; family branding does not establish them.
4. Save normalized facts and search cached records offline, with original observed time and coverage. Do not archive editorial article bodies, contributor profiles, cookies or session tokens.

## Table stakes and incumbents
Asoview and Activity Japan provide selected attraction/experience planning; generic map/Wanderlog discovery helps find places. Iko-yo Trip adds municipal local events and sparse published family facts with honest unknowns. Website-only research skips nonexistent wrapper/spec searches; plain observed HTML routes become a researched spec.

## Source contracts
GET /spots and /events; optional page query follows observed pagination. GET /{kind}/regions/{region} and /{kind}/regions/{region}/prefectures/{prefecture}, where kind is spots/events and region/prefecture choices are source links. GET /spots/{id} and /events/{id}. All observed surfaces are server HTML, not JSON APIs. Main listing entries use a.c-list__link within li.c-list__item, and details use .p-shared-basic_info table rows. Public editorial p.p-shared-paragraph is inspected only for tightly bounded factual evidence matching relevant labels. Exclude author profiles, ads and unrelated/sidebar content. Region ids 1–11 are source supplied; prefecture ids are validated against source links.

## Concrete source evidence
https://trip.iko-yo.net/spots/8220 publishes child ¥300 and adult ¥300 (adult fee includes ¥100 race admission on race days), cash only, access, hours and parking. A factual paragraph states age 6 months–12 years, indoor play, nursing room and diaper-changing space. Updated 2026-09-01; the source warns information can change.
https://trip.iko-yo.net/events/8412 publishes 2026-11-15, ¥2,000 payable on the day, an application interval 2026-09-01–2026-10-16 and a 30-person lottery if oversubscribed. It is a published application condition, not evidence that seats are currently available. Updated 2026-10-01.

## Bounds and data layer
One request timeout 15 seconds; command timeout 60 seconds; 2 MiB response cap; no unbounded retry; at most five listing pages or eight selected detail fetches per command. Pagination/coverage metadata reports fetched source window and remaining pages; keyword/date filtering is local. Store only normalized records and compact evidence. Offline search is limited to previously saved records and retains stale timestamps. Fees retain original qualifiers and units; unsupported totals/age conditions remain unknown. No bookings, reviews, contributor posts or account mutations.
