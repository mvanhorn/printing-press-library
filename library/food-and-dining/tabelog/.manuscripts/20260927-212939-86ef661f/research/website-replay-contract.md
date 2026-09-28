# Tabelog English website replay contract

Captured on 2026-09-27. This is an observed public source contract, not an official API. Runtime replay uses Go `net/http`, without cookies, browser TLS emulation, accounts, credentials, or a browser process. Native probes are bounded, paced one second apart, with a 20-second request timeout and 4 MiB body limit. Fixture manifests retain actual status, effective URL, content type, bytes, time, and body path.

## Shipping source surface

| Method | Path | Response | Evidence |
|---|---|---|---|
| GET | `/en/` | English HTML: prefecture/cuisine anchors | `discovery/native-confirm/english-home.html` |
| GET | `/en/suggest/keyword_suggest?keyword=...` | Mixed-type JSON array | `discovery/native-ajax/suggest-{ginza,shinjuku,bar}.json` |
| GET | `/en/rstLst/` | English listing HTML, source filters, native pagination links | `discovery/native/ginza-{bars-dinner5000,lunch1000to2000,keyword-sushi}.html` |
| GET | `/en/{prefecture}/{area1}/{area2}/{id}/` | HTML detail tables plus Restaurant JSON-LD | `discovery/native-confirm/restaurant-detail.html` |

`research/tabelog-browser-sniff-spec.yaml` models these four raw endpoints in the internal YAML-compatible format. HTML extraction is honestly `response_format: html` / `html_extract.mode: page`. It does not pretend the source sends normalized restaurant JSON. Generated page/links helpers cannot extract all typed restaurant fields: implement the domain parser in the source adapter during phase11. The auto sniff spec remains separate evidence, not the final source model.

## Location and cuisine resolution

Suggestions require ordinary public AJAX headers:

```
Accept: application/json, text/javascript, */*; q=0.01
X-Requested-With: XMLHttpRequest
Referer: https://tabelog.com/en/
```

Without these, the same Go URL returned400 HTML; with them it returned200 JSON. Add these only to suggestion requests in the source adapter. The minimal spec describes the requirement; its generic generated request needs that small adapter change. No auth or session replay is required.

Each suggestion has `name`, `datatype`, `id_in_datatype`; optional fields depend on type: `exact_match`, `pal`, `LstPrf`, `LstAre`, `station_id`, `site_name`, `url`, `sub_name`. Preserve optionality; the auto inferencer incorrectly treats fields from one item as universally required.

Schema audit: every captured `id_in_datatype` is a JSON integer, including English RailroadStation, AreaRestaurant, AddressMaster, Genre0, Genre1 and Genre2 variants. Bar IDs2/13/125 are numeric; the strings `bar`, `BC01`, `RC2103` belong to `site_name`. Shinjuku City ID36189 is numeric; `C13104` belongs to `LstPrf`. Keep raw IDs numeric and route codes separate; there is no captured heterogeneous ID union to model. Required detail identity fields have no sample defaults; realistic `happy_args` remain verification fixtures only.

Observed examples:

- `Ginza`: `RailroadStation`, station3368, `pal=tokyo`, `LstPrf=A1301`, `LstAre=A130101`; plus restaurants with Ginza in their names in several prefectures.
- `Shinjuku`: station5172 with `A1304/A130401`, and exact `AddressMaster` Shinjuku City with `LstPrf=C13104`. An exact name is still ambiguous across entity types.
- `Bar`: exact `Genre0` with `site_name=bar` and exact `Genre1` with `site_name=BC01`. These are distinct taxonomy levels. The broad `bar` route also includes wine bars and restaurants classified with a bar category. Legacy Japanese genre constants are unsafe.

Use homepage anchors for verified prefecture/cuisine slugs and native listing sidebar links for area routes. The dynamic filter panel's controls are retained in `discovery/browser-more-filters-ui.json`: e.g. Ginza area `A1301/A130101`, and stations Ginza3368 / Ginza Itchome3371 / Higashi Ginza8188. This limited capture is not a complete nationwide embedded catalog.

Human-name resolution must return typed choices or honor a specified kind. Never silently pick a restaurant or station where the caller requested an area. A canonical source route/ID can bypass fuzzy name resolution.

## Search, geography and pagination

Browser selecting the homepage popular **Ginza area** submitted `/en/rstLst/` with `pal=tokyo`, `LstPrf=A1301`, `LstAre=A130101`, `area_datatype=Area2`, `area_id=Ginza`. Its result title was `Best Restaurants in Ginza`. Clicking Bar produced `/en/tokyo/A1301/A130101/rstLst/bar/`; clicking Highest rated produced that route with `SrtT=rt`. Native replay returned200 and ranked cards.

Native GET `/en/rstLst/?SrtT=rt&pal=tokyo&LstPrf=A1304&LstAre=A130401&station_id=5172` returned `Best Restaurants near Shinjuku Sta.`. This is a source station vicinity, not a measured radius from the user.

Free text uses observed `sw`. `/en/rstLst/?SrtT=rt&pal=tokyo&LstPrf=A1301&LstAre=A130101&area_datatype=Area2&area_id=Ginza&sw=sushi` returned Ginza heading and separate sushi chip; its Next link retained the Ginza native route and `sw=sushi`. Validate effective area/station/genre and active constraints against the response heading, selected controls, breadcrumb/native pagination route. Reject mismatches rather than returning apparently constrained data. Other locations need live E2E confirmation in the later dogfood matrix.

Follow the actual **Next 20** anchor, HTML-unescape it, and accept only HTTPS `tabelog.com/en/` listing paths. Do not guess `PG`, discard query filters, or synthesize infinite page requests. Observed dinner next page: `/en/tokyo/A1301/A130101/rstLst/bar/2/?LstCosT=5&RdoCosTp=2&SrtT=rt`; actual native page2 returned200 and a page2 title. Source returns20 cards per observed page. Stop at caller limit or page cap; report returned count, scanned count, pages, next URL/has_more and coverage. A partial scan is not the entire source result set.

Only `SrtT=rt` is approved by this minimal spec. Preserve its source order; do not claim a custom quality score or silently sort ties. Ordinary unfiltered source pages default to most reserved by travelers, so default omission differs from highest rated.

## Genuine source meal budget filters

Static English pages hide these controls. Clicking **More filters** loads `/en/search_form?...` and exposes `RdoCosTp=2` Dinner / `RdoCosTp=1` Lunch, `LstCos` minimum, `LstCosT` maximum. Do not mistake absence in static markup for lack of source filtering. Full observed panel evidence: `discovery/browser-ginza-bars-filters.html` and `browser-more-filters-ui.json`.

Both min/max have the same indexed thresholds:

| Code | JPY threshold |
|---:|---:|
| 0 | No minimum / no maximum |
| 1 | 1000 |
| 2 | 2000 |
| 3 | 3000 |
| 4 | 4000 |
| 5 | 5000 |
| 6 | 6000 |
| 7 | 8000 |
| 8 | 10000 |
| 9 | 15000 |
| 10 | 20000 |
| 11 | 30000 |
| 12 | 40000 |
| 13 | 50000 |
| 14 | 60000 |
| 15 | 80000 |
| 16 | 100000 |

Expose yen thresholds to users and map exactly; reject unsupported arbitrary values instead of silently rounding. Native `/en/rstLst/?SrtT=rt&pcd=13&LstPrf=A1301&LstAre=A130101&Cat=BC&RdoCosTp=2&LstCos=0&LstCosT=5` matched the browser submit and displayed **Dinner - JPY 5,000 / Ginza / Bar**. The first cards had dinner brackets3000–3999,4000–4999,3000–3999,2000–2999. Lunch min1000/max2000 displayed **Lunch JPY1,000 - JPY2,000 / Ginza**, with observed brackets1000–1999. These are source average-price bracket filters: a5000 threshold is not a guaranteed maximum bill, and unseen boundary cases are not proven inclusive. Preserve displayed ranges rather than turning threshold labels into precise cost predictions. Ensure requested meal and active chips survived replay.

## Typed parser contract

Listing parsing is per `.list-rst` card, never global first-number extraction:

- Name/link: `.list-rst__rst-name-target`; source ID from its final numeric segment or matching source attributes. Keep canonical English URL and complete name.
- Rating: `.list-rst__rating-val`; absent or `-` is unknown, not0. Parse review count from that card's review anchor, independently from rank/award years.
- Station/category text: `.list-rst__area-genre`; parse station name and meter distance separately from cuisine labels. Name this `nearest_station` and `nearest_station_distance_m`; it is not distance from the traveler.
- Lunch/dinner: identify each `i[aria-label="Average lunch price"]` / `i[aria-label="Average dinner price"]` and its associated `.c-rating-v3__val`. Retain currencyJPY, raw string, lower/upper bounds when parseable, and unknown status.
- Closures and facilities: the specific card info item/icon and facility tags. Retain source text, do not infer opening now, dietary suitability or availability.
- Awards: award tooltip text within the card; award year is not a review count.

Detail parsing: select Restaurant JSON-LD objects by `@type`, including array or `@graph` shapes. Observed `aggregateRating.ratingCount` is the review count; do not rely on `reviewCount` alone. JSON-LD address streetAddress was empty, so use the labeled Address table row for full address. Match normalized complete table labels: Restaurant name, Categories, Reservation availability, Address, Transportation, Business hours, **Average price**, **Average price (Based on reviews)**, Payment methods, Service charge & fee, etc. Preserve the two budget sources separately. Nearest Shiodome250m and transportation prose Shimbashi270m are distinct facts. Optional user-visible detail sections can be drawn from this one page without fetching every result.

Normalize to typed restaurant records with source URL and fetched_at. Unknown facts remain null/explicit unknown across all renderers. Keep user notes and list membership outside source attributes. A failed refresh retains the last valid snapshot and returns a visible error.

## Failures, limits and optional scope

Non2xx, challenge/interstitial/login-only pages, oversized body, missing card identity/structure, or unexpected effective area must produce explicit errors. Do not call a200 challenge or parser failure a successful empty list. Legitimate zero results require recognizable source result structure and constraints. A source-page parse failure should not leave partially persisted restaurant state.

No authenticated session was used. No cookies, credentials, request auth headers or Set-Cookie values were captured. DOM evidence is scrubbed for CSRF values and injected third-party telemetry scripts; native fixtures do not include response headers containing cookies.

Optional read-only links from detail were also replayed once: `/party/` course/menu HTML and `/dtlrvwlst/` review-list HTML, both200. Bodies are in `discovery/native-optional/`; these are extension evidence, not approved shipping commands. Review-text harvesting, image download, booking/account actions, map/radius claims and catalog crawling are outside the minimal product contract. Existing community research identifies no missing replayable endpoint needed by the selected compact find/details/location/budget/trip-list workflows, so additional crowd mining has no material gap to resolve.
