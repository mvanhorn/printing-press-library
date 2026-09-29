# Activity Japan absorb manifest — verified core approved

## Capability gate

| Capability | Classification | Evidence / disposition |
|---|---|---|
| Known-plan detail, operator and option identity, original/localized names | Verified anonymous | `gd.activityjapan.com/get_plan_price_info` replayed for EN/JA and multiple plan types |
| Future-month date status and headline price | Verified anonymous | `plan/get_calendar` replayed; blanks and unknown codes stay explicit |
| Selected-date option prices | Verified anonymous | `plan/get_plan_price` replayed; source price integers retained |
| Selected-date session IDs and instant/request status | Verified anonymous | `select_plan_course` replayed; page calendar JS/legend mapped 1/3/4/5 |
| Selected course and count stock recheck | Verified anonymous | `plan/check_calendar_data` replayed; read-only GET, plan bounds enforced first |
| English/Japanese indexed plan-ID coverage and canonical links | Verified anonymous | Separate public sitemaps; 11,331 EN / 17,942 JA IDs observed |
| Destination/category search result listings | Clearance-gated | Existing Firefox session reads pages; fresh private Firefox sees Human Verification; terminal gets WAF |
| Official information API | Contract-gated | Vendor requires NDA and affiliate/sales-agent agreement |
| Confirmed reservation, payment, account-specific discounts | Unavailable/out of scope | Booking stays on canonical Activity Japan site |
| Fully interpreted guide language, missing fee basis, every source status code | Unverified | Preserve source fields and mark interpretation unknown |

## Source and existing-tool parity

No dedicated Activity Japan CLI, MCP server or maintained SDK wrapper was found. The website itself offers area/category search, plan details, calendar, date prices, options and booking handoff. The following verified site capabilities are the shipping baseline; search listing extraction is held pending clearance replay.

| # | Feature | Best source | Our implementation | Added value |
|---|---|---|---|---|
| 1 | Inspect a known plan and operator | Activity Japan plan page/data GET | activity-japan-pp-cli experience detail | Stable IDs, original Japanese, explicit missing facts |
| 2 | Inspect future date and session statuses | Activity Japan calendar/session GETs | activity-japan-pp-cli experience sessions | Asia/Tokyo date, source status and observation time |
| 3 | Read selected-date price options | Activity Japan dated price GET | activity-japan-pp-cli experience price | Headline/date/option basis separated |
| 4 | Recheck party stock for one course | Activity Japan stock-check GET | activity-japan-pp-cli experience check | Count bounds and result transitions explicit |
| 5 | Handoff to canonical plan URL | Activity Japan plan link | activity-japan-pp-cli experience handoff | Locale-specific URL and unresolved conditions alongside link |
| 6 | Check indexed language coverage | EN/JA public sitemaps | activity-japan-pp-cli inventory languages | Language presence is separated from instructor speech |

## Agent workflows (novel)

| # | Feature | Command | Buildability | Value and source basis | Long Description |
|---|---|---|---|---|---|
| 1 | Date-fit scan | activity-japan-pp-cli experience dates | hand-code | Compare at most seven explicit dates using live calendar and option prices; preserve unknown availability and observation times | Use for one known plan across possible itinerary dates. |
| 2 | Constraint-aware shortlist | activity-japan-pp-cli experience compare | hand-code | Compare at most five known plan IDs against age, party, duration, budget and language constraints; unknown never counts as match | Use after collecting plan IDs; do not treat the website language as guide language. |
| 3 | Price-basis report | activity-japan-pp-cli experience price | hand-code | Explain source option ID, date price, participant basis and excluded extras before price comparison | Use before saying one plan is cheaper for a party. |
| 4 | Language-surface check | activity-japan-pp-cli inventory languages | hand-code | Check whether a plan ID is indexed on JA, EN or both, with distinct canonical URLs | Use before handing a traveler an English URL. |
| 5 | Handoff readiness | activity-japan-pp-cli experience brief | hand-code | Produce a compact evidence packet with constraints, cancellation, meeting point, date/session status and unresolved questions | Use at booking handoff; the observation is not a reservation. |

No shipping stub is approved by this draft. Destination search remains a source-access decision. A possible bounded `experience search` would require replayable site search HTML or the contracted information API; sitemap URLs alone do not contain plan listings. The user explicitly requested exactly one sub-agent for later independent code review, so no separate brainstorm sub-agent is used in this gate.

## Approval

The user approved building the verified core now and separately authorized temporary site-scoped search capture. Destination search remains an unresolved extension until a replayable source is proven; it is not silently counted as a shipping feature.
