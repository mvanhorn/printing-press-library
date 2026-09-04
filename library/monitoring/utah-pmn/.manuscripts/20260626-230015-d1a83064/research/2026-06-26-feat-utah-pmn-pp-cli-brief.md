# Utah Public Notice Website (PMN) CLI Brief

## API Identity
- Domain: Statewide registry of Utah open-meeting notices (UCA §52-4-202). Every public
  body — cities, counties, school boards, special/service districts — must post meeting
  time, place, and agenda here, plus approved minutes and distributed materials.
- Users: residents, journalists, and — for this build — a real-estate principal tracking
  land-use / development approvals across Delta City and Millard County.
- Data profile: notices with inline agenda text, body + entity names, government type,
  meeting datetime, address, and a notice ID linking to a full HTML detail page.

## Reachability Risk
- None. `getUpcomingNotices.json` returns HTTP 200 JSON with no auth, no CSRF, no cookie.
- Probe-safe endpoint used: `GET /getUpcomingNotices.json?zipOrCity=84624` → 200, rich payload.

## Discovered Surfaces (from /pmn/js/app.js)
1. **`GET /getUpcomingNotices.json`** — PRIMARY. Params: `zipOrCity` (ZIP or city name),
   `startDate`, `endDate`, `listSize`, `returnFormattedDateValues`. Returns
   `{noticeDtoList:[{entityName, publicBodyName, noticeId, meetingTitle, meetingAddress1/2,
   meetingState, meetingZip, meetingCity, meetingStartTime, meetingAgenda, governmentType,
   valid}]}`. Works for PAST and FUTURE windows. No auth. This is the workhorse.
2. **`GET /sitemap/notice/{noticeId}.html`** — notice detail page: full agenda, minutes,
   attached material links. HTML, no auth.
3. `POST /searchresult.html` (by body/entity/title/agenda/date) — exists but server-side
   flaky ("technical difficulties" even with correct CSRF+cookie+JSON). KNOWN GAP, not core.
4. `publicBodiesByName.html` / `entitiesByName.html` autocomplete — 302 without full browser
   session. Not needed; the JSON endpoint already returns body names.
5. RSS/email subscription per body — human-facing, superseded by the JSON poll.

## Top Workflows
1. "What land-use meetings are coming up in Millard County?" — sweep the county's towns,
   keep planning/council/commission/board bodies, surface agendas mentioning rezones,
   subdivisions, CUPs, variances, annexations, plats.
2. "Anything new since I last checked?" — diff against a local store, show only new notices.
3. "Pull the full agenda + minutes for this notice" — fetch the detail page.
4. "Watch these specific bodies" — Delta City Council, Millard County Planning Commission, etc.

## Data Layer
- Primary entity: `notices` keyed by `noticeId`.
- Sync cursor: `meetingStartTime` + a `first_seen` timestamp for new-since diffing.
- FTS/search: agenda text + meeting title (land-use keyword search runs offline).

## Land-Use Relevance (the core differentiator)
- Body filter: Planning Commission, City Council, Town Council, County Commission,
  Board of Adjustment, Board of Supervisors, Redevelopment/Community Reinvestment Agency,
  Zoning, Design Review.
- Agenda keyword filter: rezone, zoning, conditional use, CUP, subdivision, plat, variance,
  annexation, site plan, ordinance, development agreement, general plan, easement, setback.

## Millard County location registry (curated; query union, dedup by noticeId)
Delta 84624, Fillmore 84631, Hinckley 84635, Oak City 84649, Holden 84636, Scipio 84656,
Kanosh 84637, Meadow 84644, Lynndyl 84640, Leamington 84638. County bodies surface under
the nearest town (e.g. Millard County Commission appears under Fillmore).

## Competing Tools
- None found. No CLI, MCP server, or SDK wraps utah.gov/pmn. RSS/email subscriptions are
  the only existing automation, and they are per-body and human-facing. All value here is
  in the geographic + land-use aggregation and the local-store diffing we build.

## User Vision
Paul (HighBridge): "cover all planning and commission or board related meetings related to
approvals for land use." Land-use relevance is the headline, not a filter bolted on later.

## Product Thesis
- Name: Utah PMN CLI (utah-pmn)
- Why it should exist: turns a ZIP-by-ZIP, click-through state website into one command
  that sweeps a whole county, keeps only land-use approval bodies, scans agendas for the
  actions that matter, and tells you what's new since last run — scriptable and schedulable.

## Build Priorities
1. Data layer + sync over `getUpcomingNotices.json` across a location set, dedup by noticeId.
2. `notices upcoming` (raw endpoint) + notice detail fetch.
3. Transcendence: `millard` county sweep, `landuse` relevance filter, `since` new-notice
   diff, `watch` named bodies, `agenda` keyword scan, location registry command.
