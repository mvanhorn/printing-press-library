# Asoview CLI research brief

Asoview public website, Japanese first-party leisure tickets and experiences. Public search and product HTML return 200 without credentials or cost. No published discovery API, public SDK, CLI or MCP identified in targeted web searches; official tech blog describes private protobuf/TypeScript contracts. Account, purchase and reservation actions are outside scope.

## Users and workflows
Trip-planning agents need relevant nearby attractions, age-compatible options, inclusion/cancellation terms, date eligibility, and canonical handoff links. Pain points: lowest advertised price may be a child band; date validity is not timed admission; marketing stock is not confirmed party inventory. Discover by keyword/region/category, inspect a few products lazily, compare terms, check public dates without transacting.

## Source and access
https://www.asoview.com/search/ and /item/ticket/ticket0000049223/, /item/activity/pln3000044589/ expose ASOVIEW_DATASOURCE JSON in script elements; product JSON-LD also available. 200 observed 2026-09-30 UTC / 2026-10-01 JST. Homepage is a client-rendered shell. Ticket datasource has price bands, cancellationPolicy, overview, validityPeriod, unablePeriods. Activity has ageLimit, priceIncluded, cancelPolicies, sellingPrice, reservableParticipant. Search HTML has initial query state and cards. Browser capture will establish query wire keys and public inventory coverage before generation.

Robots disallows /api/, /stocks/, /purchase/, /book/, /reserve/ and search URLs containing experienceDate. Prefer replayable public HTML; do not crawl excluded routes. Terms: https://www.asoview.com/info/terms ; keep usage bounded and personal/read-only. No account or paid dependency for selected scope. No published rate-limit contract found; conservative bounded GET requests, retry/backoff, caching. Japanese terms are canonical; localized variants supported only if established first-party replayable contracts exist.

## Competitor context
Activity Japan, Klook and KKday provide comparable region/date/category discovery and per-band ticket terms. They are research context only, never runtime sources. Incumbent Asoview website offers richer checkout-specific choices; CLI reduces browsing/output work and explicitly labels missing stock/quote values.

## Product thesis
asoview-pp-cli: compact agent-first source-grounded discovery/comparison/handoff. Single Go binary, no resident browser, bounded public HTML cache. Source IDs, names, exact price units and timestamps survive projection.

## Data layer and priorities
Bounded URL-keyed public-response cache (summary/detail independent); taxonomy inventory refresh is explicit. No full-country crawl, embedded heavy database or checkout session. Ship search, product, options, date eligibility, compare, taxonomy, doctor, cache controls; expose dated slots only where genuinely public and distinguish unknown from sold out. Deterministic domain parsing tests plus live correctness and request/latency/memory measurements.

## Authorization
User preapproved focused scope and routine gates. Sole builder, exactly one fresh-context code-review subagent at finish; this overrides Press novel-feature/planner delegation. Global/shared config stays unchanged, task-isolated Go/cache/version advisory artifacts. No publish/PR.

## Dated access findings
# Public source discovery
Temporary isolated browser-use session verified /search/?destinationType=prefecture&destinationId=prf130000&leisureType=category&leisureId=192 -> Tokyo aquariums, 7 venues. Search q parameter is ignored upstream; free text must be an honest bounded local filter. Observed public HTML datasource + exact query state + first-party stock routes from public client code. Browser activity calendar interaction was observed in loading state before close; stock endpoint bodies were independently fetched with direct anonymous GET, not claimed as intercepted traffic. Capture combines replayed genuine source responses; no cookies or credential headers. 2026-09-30 UTC. Replayable stdlib HTTP runtime; browser is closed.

Ticket calendar yearMonth is YYYY-MM (YYYYMM returned 400); ticket courses date YYYY-MM-DD. Dated band endpoint time must be normalized startTimeLabel (09:30), not course ID (01 -> error). Public GETs require no auth. /stocks/calendars and /stocks/courses are activity surfaces; /stocks/ticket/... is ticket. Public activity fee endpoint GET /reservations/plans/{id}/dates/{date}/basicfees returns price bands and units, no reservation is created. Robots disallows stock/API routes for crawlers; runtime restricts queries to explicit bounded requests and never crawls these paths. Source has Japanese content; language variants unverified and outside agreed shipped scope.
