# Access research

Checked 2026-09-27. The CLI uses the public [Japan Travel route website](https://japantravel.navitime.com/en/area/jp/route/), not the contracted NAVITIME API. Live cookie-free Firefox HTTP replay verified bilingual lookup, station/POI routes, depart/arrive/first/last modes and one JR Pass constraint. The route-result page also advertises a pass catalogue. Its entry page was less reliable, so it is not a runtime dependency.

| Surface | Access and coverage |
|---|---|
| Shipped website adapter | No key/subscription/browser required for the verified surface; undocumented HTML may change. |
| Official API via marketplace | Registration and a key required, including the free plan. Basic train routing uses average travel times. |
| Direct NAVITIME contract | Quote-based access; scheduled rail/bus times and multilingual output are additional options. |
| Premium website views | Full stop lists and standalone timetables were marked premium; not included. |
| Live operations / seats / bookings | Not verified or implemented by this CLI. |

The official [route specification](https://api-sdk.navitime.co.jp/api/specs/api_guide/route_transit.html) distinguishes `train_data=average` from the optional timetable mode. Marketplace availability must not be treated as scheduled routing. The [station-search specification](https://api-sdk.navitime.co.jp/api/specs/api_guide/transport_node.html) does not establish English lookup; the website's English/Japanese behaviour was tested independently.

Published monthly API prices at research time, per subscribed API product:

| Plan | Price | Included requests | Requests/minute |
|---|---:|---:|---:|
| RapidAPI BASIC / API Hub Trial | Free | 500 | 50 |
| RapidAPI PRO | USD 200 | 5,000 | 100 |
| API Hub Basic | JPY 26,000 | 5,000 | 100 |
| RapidAPI ULTRA | USD 300 | 10,000 | 150 |
| API Hub Standard | JPY 39,000 | 10,000 | 150 |

The upper plans list overage at USD 0.05 / JPY 7 per request. Direct contracts require a quote and may include setup/option charges; a trial is available. These are published comparison-page figures, not independently verified checkout prices. Station lookup and routing are separate marketplace products. Source: [official API pricing and coverage](https://api-sdk.navitime.co.jp/api/specs/description/about_navitime_api.html).

Direct API signing details are supplied during onboarding; this project does not invent them. See [client IDs](https://api-sdk.navitime.co.jp/api/specs/description/about_cid.html) and [authentication](https://api-sdk.navitime.co.jp/api/specs/description/about_signature.html). No API credentials, cookies or personal browser state are included in the checkout.
