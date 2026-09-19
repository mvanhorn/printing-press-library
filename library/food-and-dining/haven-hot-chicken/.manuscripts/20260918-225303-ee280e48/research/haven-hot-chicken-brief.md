# Haven Hot Chicken CLI Brief

## API Identity
- Connecticut restaurant brand, official site https://www.havenhotchicken.com/.
- Consumer ordering uses https://order.thanx.com/havenhotchicken; official site also links Toast ordering for gift-card use.
- No Haven-specific OpenAPI, GitHub wrapper, npm package, or PyPI package was found in targeted searches on 2026-09-18.
- Public-library registry fetched and parsed: no Haven or Toast named entry. Blocked journal returned 404.

## Reachability Risk
- High pending browser discovery: public New Haven location page returns HTTP 200. /menu redirects to Thanx; direct and Chrome-compatible HTTP return 403 Cloudflare challenge.
- Actual Chrome loads the ordering interface, with location picker, menu flow, cart and login controls. Replayability remains unproven.

## Top Workflows
1. Find a Haven location and inspect its current menu, prices, options and ordering availability.
2. Search menu items and compare locations without opening multiple menus.
3. Prepare a local meal shortlist with estimated subtotal and official checkout links.
4. Refresh local location/menu data for offline search and identify menu changes.
5. Rewards and order history only if the user requests account access and supplies a session.

## Table Stakes
- Official Haven website: locations, hours, addresses, menu/order links.
- Thanx ordering: location choice, pickup/delivery, item customization, cart, account rewards.
- Toast ordering: menu prices and customization; official gift-card ordering route.
- No public Haven SDK repository identified, so issue-health review is not applicable.

## Data Layer
- Primary entities: locations, menu categories, items, modifier groups; exact contracts pending capture.
- Full refresh with observation timestamps rather than invented cursors.
- Offline text search; provenance and freshness in output.

## User Vision
- User requested printing Haven Hot Chicken with current Printing Press and Astra Instructions.
- Repair stale Claude skill conflict autonomously; avoid requiring another conversation restart.

## Product Thesis
- Name: haven-hot-chicken-pp-cli.
- Find a meal and compare Haven locations using live, traceable data and a local searchable cache.
- Account mutations, real orders and payment are not authorized live tests.

## Build Priorities
1. Establish replayable public menu and location contracts.
2. Generate only observed operations with useful structured output.
3. Add offline search, comparison and a local meal plan if approved at the feature gate.
4. Validate all supported commands against real public targets before promotion.

## Evidence
- https://www.havenhotchicken.com/locations/new-haven (raw HTTP 200; exact order links inspected).
- https://www.havenhotchicken.com/locations (location directory).
- https://www.havenhotchicken.com/app (ordering and rewards description).
- https://order.thanx.com/havenhotchicken (HTTP 403, live Chrome ordering interface).

## Users
- A Connecticut regular choosing pickup from one of the nearby Haven shops: repeatedly checks item availability, modifiers, prices and ordering hours.
- A household meal organizer: assembles several known menu items and wants a transparent base-price subtotal before opening checkout.
- A customer who visits multiple Haven locations: compares the same dish and spots menu or price changes between saved observations.
These are workflow-based design hypotheses grounded in the official pickup/delivery, customization, multi-location and Feed the Flock interfaces, not interview findings.

## Verified public contract
2026-09-18: six GET routes return HTTP 200 with Accept-Version v3.5 and Thanx-Merchant havenhotchicken. Locations returns 10 Haven shops; North Haven menu returns current categories/items and modifier constraints. No credential or cookie was used. The storefront challenge does not apply to these API routes. Public bundle source plus direct HTTP replay replaces Claude-specific network-capture steps under the user's explicit Codex adaptation instruction. Raw captures and inferred OpenAPI live in this run. No HAR is claimed.
