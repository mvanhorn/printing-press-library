# tabiwa catalog CLI brief

## API Identity
Public consumer catalog on https://app.tabi-wester.westjr.co.jp. Tourists compare regional transport/attraction bundles and coupons. Public Japanese product summaries, price display, product category, regions/areas, usage-type hints and explicit points-only flags are available through anonymous GET requests. Accounts, purchased tickets, checkout, rewards balances and redemption actions are excluded.

## Access and source limits
Observed 2026-10-04: ordinary HTTP GET /ticketList/search and /ticketList/area return real JSON repeatedly. The website sets a non-authentication `regionId` preference cookie: 10 せとうち, 20 北陸, 30 山陰, 40 九州. This preference is the only cookie the CLI supplies. No browser cookies or account auth are imported. Four region reads returned real dated transportation rows. Full /eticketDetails pages redirect ordinary HTTP to Queue-it. Browser navigation shows details, but browser execution is discovery only and the runtime does not fetch full details or bypass the queue. Unverified full redemption terms, geographic coverage, passenger-specific fares and real availability remain unknown.

## Duplicate and competitor evidence
Shared complete public-library travel category has no tabiwa slug. NAVITIME `passes list` advertises passes and verification status and `routes --pass` uses advertised IDs; it does not inspect tabiwa overview restrictions, quote points-only products, or provide source-specific redemption evidence. Existing Asoview/activity-japan focus leisure/experience products and do not cover these JR-led bundles. Search for tabiwa SDK, MCP, CLI, npm/PyPI clients found no relevant wrapper; similarly named Tabi messaging projects are unrelated. There is no wrapper issue tracker to inspect. No additional provider integration is proposed.

## User pain points and priorities
1. Area labels can overstate inclusions: J0000900 is tagged 小豆島・直島・豊島 but the overview explicitly excludes unlisted 直島. Retain the contradiction and require the full coverage page.
2. J0001900 quotes 2,000P and `is_point_only: true`; it prohibits credit-card purchase and describes QR entry. Do not convert points into a cash price or confuse earned points with purchase currency.
3. Catalog inclusion on a requested date does not establish seats, operating status or purchasability. Preserve source date filtering as catalog membership only.
4. Restriction/redemption hints are spread across overviews: separate reservations, exchanges, per-vehicle quotes, excluded train tickets, taxes and venue closure periods can change the choice. Preserve bounded original Japanese evidence and unknown complete terms.

## Top workflows and table stakes
Discover a bounded regional/date shortlist; inspect a known product's catalog evidence; compare at most five IDs with units/conditions and optional requested-date membership; retrieve saved observations offline. Geography lists exact provider prefecture/area IDs. Cash-quoted, points-only, variable-price and unknown-price products remain distinct. Source URLs and original observation timestamps accompany facts. Errors never become empty availability.

## Data layer
Selected normalized catalog observations only, max 50 products keyed by region and ID; SQLite normal transactions and bounded payloads. Search saves only explicitly when `--save`; inspect/compare likewise. Offline saved command opens existing store read-only without migration or network. No auth, photos, tracking/profile data, account data or bulk source archiving. Historical observations are labelled saved with original retrieval time, not current inventory.

## Product thesis
Name: tabiwa catalog evidence. A compact tool for choosing a regional pass/coupon with precise payment units and overview-derived restrictions. It is useful alongside NAVITIME routing and canonical booking pages. It does not claim pass-holder savings, route coverage, ticket stock, complete redemption policies, or booking.

## Approved scope
Five decision behaviors inside catalog search, inspect, compare and saved, plus geography list. Direct task briefing and root's explicit catalog-scope acceptance authorize this trimmed plan and publication; no repeat approval menus or extra feature-agent are used. Exactly one fresh reviewer owns phases 14–17. No stubs ship.

## Sources
https://app.tabi-wester.westjr.co.jp/ticketList
https://app.tabi-wester.westjr.co.jp/javascripts/ticket/ticket-list.js
https://app.tabi-wester.westjr.co.jp/javascripts/switch-region.js
https://app.tabi-wester.westjr.co.jp/ticketList/search
https://app.tabi-wester.westjr.co.jp/ticketList/area
https://app.tabi-wester.westjr.co.jp/eticketDetails?ticket_id=J0001900
https://app.tabi-wester.westjr.co.jp/eticketDetails?ticket_id=J0000900

Local SQLite filename: `catalog-saved.db` is local storage under the resolved cache directory, never a network host.
