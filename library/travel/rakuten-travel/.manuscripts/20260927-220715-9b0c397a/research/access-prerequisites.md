# Access prerequisites and preliminary findings

Checked 2026-09-27. Status: preliminary official-document inspection; authenticated live responses have not been verified. The active receipt gate is 03-resolve-and-reuse, awaiting credential location or an explicit decision to continue research before credentials.

## Run setup

Workspace began empty and was not a Git repository. Printing Press 4.32.5, skill 3.0.0, Go standard-library smoke passed. All run state is local under the workspace .printing-press directory. The user explicitly requires shared configuration to remain unchanged, so the global skill updater was omitted. Registry read succeeded (523 entries, no Rakuten match); optional blocked-apis.json returned HTTP 404. No Rakuten, API_KEY, or API_TOKEN credential environment variable was present. Values were not printed.

## Registration and access

- https://webservice.rakuten.co.jp/guide requires Rakuten account login and application registration. It lists application name, URL, type, allowed websites, data usage purpose and expected QPS as required registration information; up to five apps.
- The current endpoint documentation requires applicationId and accessKey together. Send accessKey as a header to reduce URL exposure. Do not embed credentials in generated code, logs, research, proofs or booking links.
- https://webservice.rakuten.co.jp/app/create redirects to Rakuten account login. No account or app was created.
- Official quota FAQ: at most one request per second per application ID. https://webservice.faq.rakuten.net/hc/ja/articles/900001974383
- Allowed-origin behavior, actual account entitlements and a complete quota schedule still need verification. Registration fields alone do not prove a Referer requirement or an approved origin.

## Preliminary contract observations

- https://webservice.rakuten.co.jp/documentation/simple-hotel-search advertises Travel/SimpleHotelSearch/20260731 at https://openapi.rakuten.co.jp/engine/api/.
- https://webservice.rakuten.co.jp/documentation/keyword-hotel-search advertises KeywordHotelSearch/20260731. UTF-8 keywords and AND-separated terms; searchField 0 includes property/plan/room names, 1 limits to hotel names. English matching/coverage remains unverified.
- https://webservice.rakuten.co.jp/documentation/hotel-detail-search advertises HotelDetailSearch/20260731, with detailed property facilities/access/source ratings via responseType=large.
- https://webservice.rakuten.co.jp/documentation/vacant-hotel-search still advertises VacantHotelSearch/20170426. It accepts dates, adultNum, roomNum and six meaningful child categories. Occupancy distribution for multiple rooms requires verification; avoid inventing per-room allocation semantics.
- Availability searchPattern=0 is property mode and exposes at most three plans per property. searchPattern=1 is room/plan mode, with up to 30 records per page and pages 1–100.
- roomClass, planId (conditional on salesformFlag), meals, payment and reserveUrl are documented. Missing plan identity must remain explicit; plan names are not identifiers.
- dailyCharge is explicitly first-night-only. rakutenCharge is per person or per room according to chargeFlag; total is that night's total. Whole-stay totals cannot be inferred by multiplying. hotelMinCharge is only an indicative per-room per-night minimum inclusive of tax/service charges.
- datumType=1 requests world/WGS coordinates in degrees, while the default datumType=2 uses Tokyo Datum in arcseconds. Prefer explicit datumType=1, preserve source datum/units, and avoid silent conversion.
- Availability 404 + structured not_found is documented separately from invalid parameters, throttling, system errors and maintenance. A generic HTTP 404 is not proof of no inventory.
- reserveRecordCount counts available plan × room combinations, not remaining physical rooms. https://webservice.faq.rakuten.net/hc/ja/articles/37004462918041
- Hotel ranking API is stale, last updated in 2014, and planned for retirement; exclude it from recommendations. https://webservice.faq.rakuten.net/hc/ja/articles/43312657146905

## Next work

Resolve access gate, enter official research phase through the receipt ledger, verify endpoint contracts/coverage and conditions, propose a focused scope at the absorb gate, then generate in staging. Implementation and tests must be delegated to gpt-6-sol with max effort. No implementation, live-test acceptance or promotion has occurred.
