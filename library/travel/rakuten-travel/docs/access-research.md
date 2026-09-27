# Access research and shipped scope

Verified 2026-09-27 against Rakuten documentation and anonymous public requests. Raw captures, research and Printing Press receipts remain in the workspace's `.printing-press` run archive.

## Official APIs investigated first

The [Rakuten Web Service guide](https://webservice.rakuten.co.jp/guide) requires a Rakuten account and registered application. Current Travel documentation requires an application ID and access key. Registration collects application purpose, allowed websites and expected request volume. The published guidance is no more than one request per second per application; errors and endpoint limits still apply. The traveler supplied no credentials, so authenticated API behavior was not live-tested and the official API is not a shipped backend.

| Official interface | Documented capability | Material boundary |
|---|---|---|
| [Area class](https://webservice.rakuten.co.jp/documentation/get-area-class) | Area hierarchy | API area codes differ from public website path IDs |
| [Simple hotel search](https://webservice.rakuten.co.jp/documentation/simple-hotel-search) | Geographic/area property lookup | Datum and units depend on request mode |
| [Keyword hotel search](https://webservice.rakuten.co.jp/documentation/keyword-hotel-search) | Keyword property lookup | English literal input is not guaranteed translation or equivalent coverage |
| [Hotel detail](https://webservice.rakuten.co.jp/documentation/hotel-detail-search) | Property, amenities, access and ratings | Availability requires a dated query |
| [Vacant hotel search](https://webservice.rakuten.co.jp/documentation/vacant-hotel-search) | Dated party-specific plans and rooms | Documented daily charge is for the first night; charge basis can be per person or per room |

Official coordinate modes distinguish WGS84 degrees from Tokyo Datum arcseconds. No conversion is used in the shipped website backend because its coordinate datum and units have not been established. Official ranking data was documented as no longer updated; rankings are outside scope.

## Approved credential-free route

The user approved public website discovery and this narrower delivery. The runtime uses only bounded HTTPS GET requests and requires neither cookies nor a browser. Discovery confirmed:

- National/prefecture directories and area result pages, including source page two.
- Japanese and English literal keyword searches with different result subsets.
- Property overview and facilities pages with access, amenities, rating and property policy/fee notes.
- Dated plan pages with exact hotel/plan/room IDs, query echoes, labelled whole-stay prices and actionable reservation controls.
- Uniform multi-room parties and the six source child categories, including a live infant-with-neither query.

Public website access is an observed capability, not an official API stability or quota guarantee. The client paces serial requests at a minimum one-second interval, uses bounded retries and request budgets, and stops on access/challenge or page-contract errors.

## Meaningful gaps

The website can expose more dated whole-stay price evidence than the documented first-night API charge fields, but HTML is less stable than a versioned API. A page cannot prove exhaustive inventory. Cancellation data without a reliable plan association remains unknown; property rules remain separately labelled. Consumption-tax-inclusive prices can exclude accommodation tax and optional fees.

No authenticated/member/coupon rates, automated booking, automatic translation, radius search, unequal room allocations, or other Rakuten services are included. Booking handoff uses the observed dated plan page and room anchor. The CLI never submits the website's reservation POST.

See [data contract](data-contract.md) and [verification](verification.md) for exact output semantics and measured evidence.
