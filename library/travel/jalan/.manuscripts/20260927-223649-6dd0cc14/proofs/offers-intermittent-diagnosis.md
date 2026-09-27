# Intermittent Jalan offers parse failure diagnosis

The final efficiency benchmark reported `parse_failure` for the second date
alternative, 2026-11-11, in the two-date comparison for property 385995. The
benchmark did not persist the original response body; its generic diagnostic
cannot identify the precise parser branch or distinguish an upstream empty/access
page from an unrecognized markup variant.

One fresh, anonymous read-only request was made to the exact reported URL:

`https://www.jalan.net/yad385995/plan/?adultNum=2&distCd=01&roomCount=1&roomCrack=200000&stayCount=1&stayDay=11&stayMonth=11&stayYear=2026`

Request bound: **1 fresh request**, no retry, no redirect following, 20-second
request timeout, 4 MiB response limit. No additional live requests were made.
Capture filesystem observation time: `2026-09-27T15:40:09.349630+00:00`.

The response was HTTP 200 with `Content-Type: text/html;charset=Windows-31J`,
393,641 raw bytes, and 1.449 seconds reported transfer time. CP932 decoding
produced UTF-8 without replacement characters. The native page identifies itself
as `page-planIndex` and names 箱根湯本温泉　ホテル南風荘. Dated controls echo
2026-11-11, one night, one room, two adults, and `roomCrack=200000`.

A temporary deterministic replay through the current `ParseOffers` succeeded:
11 native plan cards, 52 distinct property/plan/room tuples, no next page, and
`NoResults=false`. The replay harness was removed after the check. The later
successful umbrella verification reported by root is consistent with this
capture, but does not explain the absent original failed body.

| Capture | Bytes | SHA-256 |
| --- | ---: | --- |
| `offers-385995-20261111.raw` | 393641 | `b8822e72230202400790076a14f40ad2793a7b0a0c388ca31aa96a5723a198f8` |
| `offers-385995-20261111.html` | 406349 | `2090c134f477893cf0f9c2a8e26efd6f976d2a2febec4e9d1c7a329feef94a33` |

Captures are under `discovery/`; the HTTP header capture is
`offers-385995-20261111.headers`. No parser defect was established, no parser
behavior was loosened, and no regression test or implementation change was added
for an unreproduced failure. The original-body absence remains the material
limitation; the upstream failure mode cannot be classified from this successful
later capture.
