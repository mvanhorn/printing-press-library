# Jalan browser discovery

## User Goal Flow
Goal: search a dated Hakone stay, page results, open a room/plan and property. Opened dated results and inspected the real form controls. Attempted next-page, plan, and property clicks; this browser-use session remained on the results URL (plan navigation likely opened another tab). These attempted transitions are not claimed as completed navigation. Browser capture supplies one useful SSR HTML endpoint; separate direct HTTP probes successfully captured property, offer-list and exact-plan pages.

## Configuration
Anonymous browser-use CLI session jalan-discovery; closed after capture. No cookie export, login, installs or shared configuration changes. No proxy-envelope. Conservative bounded reads, no 429 observed. Browser interaction requests only; no booking or coupon-claim action.

## Endpoints and replayability
GET /uw/uwp1400/uww1400.do: HTTP 200 HTML, full dated inventory. Direct HTTP returned the same schema without browser state.
GET /yad385995/: HTTP 200 public property facts.
GET /yad385995/plan/: HTTP 200 dated offer list.
GET /uw/uwp3200/uww3201init.do: HTTP 200 exact plan/room details, dated prices, tax notes and cancellation bands.
Browser capture intentionally removes scripts/styles/hidden inputs and carries no headers. Direct HTTP captures supply raw contract evidence. The printed runtime will use standard HTTP with Shift_JIS decoding and structured HTML extraction.

## Parameter evidence
Actual rendered form labels establish mealType 0 none, 1 breakfast, 2 dinner, 3 both; yadRk ryokan; careNsmr non-smoking room; carePribateBath room with outdoor bath (spelling as upstream); careOnsen property hot spring; careBathRent reservable/private bath; careOpenbath outdoor bath. These are distinct filters. Date fields and roomCrack occupancy observed in plan links. Native per-page search shows 30 properties; page index contract remains to test independently.

## Traffic analysis and coverage
Analyzer reports standard_http, no warnings. SSR results and form controls are useful; broad request logging consists mostly of telemetry/assets. No need for browser runtime. API-shaped endpoint count is thin because the core flow is SSR HTML. Legacy-key API and new Korean-widget MCP remain separately characterized; neither is silently substituted for this flow.

## Samples
Saved sanitized browser-sniff-capture.json and direct raw/decoded HTML samples under this directory. Example sampled property/plan/room IDs: 385995 / 03912759 / 0576806. Price evidence: quoted 74,800 JPY, conditional coupon 68,800 JPY, 1,496 earned points, extra bathing tax text. These are observation-time facts, not future availability promises.

## Authentication
No authenticated session used. Legacy credentials unavailable. No live credential content was persisted.
