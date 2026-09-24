# Immovlan browser-sniff report (2026-09-22)

Backend: chrome-MCP (user's Chrome, fresh tab, closed after capture) after browser-use and agent-browser were both served HTTP 403 by Immovlan's bot management. Direct HTTP with a Chrome User-Agent returns 200 from a Mac and from a Linux VM; `probe-reachability` = `standard_http` (stdlib 200, surf-chrome 200). Replayability: the printed CLI ships plain HTTP with a Chrome UA and structured HTML extraction — the exact transport that produced the bodies in `browser-sniff-capture.json`.

## Surfaces (all GET, no auth, server-rendered HTML)

| Surface | URL | Extraction |
|---|---|---|
| Search | `/fr/immobilier?transactiontypes=a-vendre|a-louer|en-vente-publique|en-colocation&propertytypes=maison,appartement,terrain,garage,kot&towns=<postcode>-<slug>[,…]&epcratings=unknown|bad|poor|good|excellent&minprice&maxprice&minbedrooms&maxbedrooms&sortby=price|nrOfBedrooms|totalSurface|zipCode&sortdirection=ascending|descending&page=N` | 20 `article.v3-search-card` per page, schema.org microdata: `data-url`, reference (`data-value-id` VBE…/RBU…), `.v3-search-card-price`, `itemprop=postalCode/addressLocality/description`, pills (chambres, m², salles de bain), `.v3-epc-watermark.Brussels<Letter>` (PEB letter), ribbon "Nouveau", `data-value-prouser-types` (estateAgents / private), phone in `data-value-phone`. Pagination links `page=N`; result count is filled client-side (empty in HTML) — count = last page × 20 bound. |
| Detail | `/fr/detail/<type>/<transaction>/<postcode>/<slug>/<ref>` | JSON-LD `RealEstateListing` (`datePosted`, `mainEntity` House/Apartment: `address.streetAddress`, `geo`, `floorSize`, `numberOfBedrooms`, `numberOfBathroomsTotal`, `yearBuilt`, `offers.price`, `offers.offeredBy` RealEstateAgent name/url/address), `dataLayer.push` (`seller_type`, `seller_id`, `livable_surface`, `vlan_code`, `software`), meta `cXenseParse:rbf-immovlan-peb` = `Brussels<Letter>`, feature table (état du bien, revenu cadastral, année, surfaces jardin/terrasse/terrain, chauffage, ascenseur, façade…), photos on `api-image.immovlan.be/v1/property/<REF>/images/…`. |
| Town slug | `towns=<postcode>-<slug>`; `municipals=<postcode>` accepted | slug = lowercase locality; see reachability test |

Internal endpoints seen in JS but **not used** (robots.txt disallows `*/api*`, `*SearchPartial*`): `/api/terms` (facet counts).

## Filters confirmed live (result set changes)
`epcratings=bad` 5 pages → 2 pages with `minprice=500000`; `epcratings=bad,poor` 9 pages; price band 5 pages. EPC bands, not letters — letters come from the card watermark / detail meta.

## Failure detection
No challenge markers in real Chrome (title, cards=80, result count "50 résultats (1 - 20)"). Headless: 403 on document request (both backends). Rate: 5 direct fetches at 1 req/s, all 200.
