# BuildAlert Browser-Sniff Discovery Report

## Capture environment
- Site: https://www.buildalert.uk
- Backend: Next.js (148.x) on AWS CloudFront
- Capture tool: browser-use v0.12.7 (CLI mode)
- Capture method: Authenticated session, Chrome Profile 1 (Syed Shahbaz), fresh login via headed window
- Reachability mode (probe-reachability): `standard_http` (confidence 0.95)

## Auth profile
- **Type:** `cookie`
- Session cookies set by the `/auth/login` form post to `www.buildalert.uk`
- No `Authorization` header observed — replay is via cookie credentials only
- Logged-out probes return 200 HTML (Next.js shell) but the dashboard renders an unauthenticated state; `/dapi/*` returns 401-like behavior when cookies are absent (verified during the initial Default-profile attempt that redirected to `/auth/login`)
- Cookies persist on the Chrome profile after the user logs in via the headed window

## API surface (`/dapi/*`)

| Method | Path | Auth | Description |
|--------|------|:----:|-------------|
| GET | `/dapi/user/user` | cookie | User profile + filter preferences |
| GET | `/dapi/user/details` | cookie | Alias of `/dapi/user/user` |
| GET | `/dapi/user/dashboard` | cookie | Dashboard overview: counts, credits, recent userLeads |
| GET | `/dapi/leads/live-leads` | cookie | Paginated planning-application leads |
| GET | `/dapi/letter-templates` | cookie | User's letter templates + baseLogoUrl |
| GET | `/dapi/transactions` | cookie | Letter-send purchase history |
| GET | `/dapi/tracking` | cookie | ROI tracking + chart data |
| GET | `/dapi/healthcheck/keep-warm` | none | Liveness probe |
| GET | `/dapi/reviews/should-show-modal` | cookie | Review modal trigger (skipped — low CLI value) |

## Filter parameters on `/dapi/leads/live-leads`

| Param | Type | Notes |
|-------|------|-------|
| `states` | string | Lead state filter. `-1` = all. Other values observed (0/1/2) did NOT change result count in the test account |
| `page` | int | 1-indexed |
| `itemsPerPage` | int | Default 50 in the dashboard |
| `orderBy` | string | `createdDate`, `value` observed |
| `force` | string | Cache-bust marker; passed empty |
| `minValue` | int | GBP threshold |
| `projectTypes` | csv | `Extension`, `Loft_Conversion`, `Garage_Conversion`, `Outbuilding`, `Porch` (from response's `quickFilters[].id`) |

## Sample lead schema (from one captured response)

Lead envelope (`data[i]`):
- `id` (string, nullable — only set if user has interacted)
- `aiMatchScore` (int, nullable)
- `application` (object, see below)
- `date` (unix ms)
- `state` (int)
- `read` (bool)
- `isNew` (bool)
- `userId` (string)
- `templateId` (string, nullable)

`application`:
- Identifiers: `id`, `internalUniqueReference` (`{council}__{reference}`), `reference`
- Council: `councilIdentifier`, `countyIdentifier`, `url` (council planning portal link)
- Description: `fullDescription`, `summary`
- Address: `address`, `postCode`, `applicant: {name, address, postCode}`
- `addressLookup` (Google geocoding): `{type, formattedAddress, country, county, ...}`
- Coords: `longitude`, `latitude`, `distanceAway` (miles from user)
- Status: `rawStatus`, `status`, `appealStatus`, `appealDecision`, `decision`
- Dates (all unix ms): `dateReceived`, `dateValidated`, `createdDate`, `updatedDate`, `decisionDate`
- Classification: `category`, `categoryDescription`, `tags[]`, `subTags[]`, `developmentClass`
- Value estimation (AI): `estimationValueBand` (e.g., `50K_100K`), `estimationValueBandDescription`, `estimationReasoning`
- Letter state: `canSendLetter`, `letterBeenSent`, `letterSentLeadState`, `badge`
- `documents[]`

## Pagination shape (universal across list endpoints)
```
{
  totalItems: int,
  itemsPerPage: int,
  pageCount: int,
  currentPage: int,
  data: [...]    // (or `items` for tracking endpoint)
}
```

## Replayability
- All endpoints replay over plain HTTP (stdlib `net/http` is sufficient — no Surf needed)
- Cookies-only auth; the printed CLI ships `auth login --chrome` to import cookies from the user's Chrome profile, then sends them on each request
- No CSRF token observed on read paths
- No GraphQL, no persisted queries — clean REST

## Out-of-scope for v1 (not browser-sniffed in this pass)
- Mutation endpoints (filter update PATCH, letter send POST) — these are write paths the user said should default to dry-run; would need a follow-up capture with deliberate interactions
- `/dashboard/lead-pack?id=<slug>` — lead-pack detail page is client-rendered with no separate XHR fetch observed
- Individual lead detail — no `/dapi/leads/<id>` endpoint exists; lead data is fully embedded in the list response

## Replayability evidence
- 9 GET endpoints captured at HTTP 200 with stable JSON content-types
- Cookies-only — fully replayable without browser presence (standard_http transport)
- Sample bodies captured at `discovery/sample-lead-full.json` and `discovery/probe-results-{1,2,3}.json`
