# Workiz CLI Brief

## API Identity
- Domain: Field Service Management (FSM) — scheduling, invoicing, CRM, and job/lead pipeline management for home-service businesses (HVAC, plumbing, electrical, cleaning, etc.)
- Users: FSM business owners, dispatchers/schedulers, and developers wiring Workiz into billing/marketing/CRM stacks
- Data profile: jobs (scheduled service calls), leads (pre-job estimates), clients (customers), team members (techs/crew), time-off records. Base URL `https://api.workiz.com/api/v1/{api_token}/`. Response envelope: `{flag, has_more, found, data: [...]}` (or `{Flag, Data, Code, Details}` on error). Auth: token embedded in URL path (all calls); `auth_secret` embedded in POST body for every write call (create/update/assign/unassign). No official OpenAPI/Swagger spec published — `developer.workiz.com` is a client-rendered React SPA with no accessible raw content via direct HTTP.

## Reachability Gate
- Decision: PASS
- Evidence: `GET https://api.workiz.com/api/v1/` (no token, no key) returned `403` with a clean, well-formed JSON body: `{"success": false, "error": "Forbidden", "message": "Invalid API path or malformed API key."}` — server-side, Cloudflare-fronted, JSON API error (not an HTML bot-protection/challenge page). Equivalent to the standard "401 no key provided" pass case: the API is alive and behaves exactly as the community SDKs document.

## Reachability Risk
- Low. Zero open issues on either community wrapper repo (`forward-force/workiz` PHP SDK, `BeelineRoutes/workiz` Go SDK) mentioning 403/blocked/deprecated/rate-limit problems. Both wrappers are actively maintained and describe a stable, working v1 contract.
- Auth requires opt-in "Developer API" add-on enablement in the Workiz app (Feature Center > Developer API), then token+secret issued under Settings > Integrations > Developer. This CLI's user will need to do this manually before any live call succeeds — expect early `401 Unauthorized` during onboarding, not a systemic reachability problem.
- Rate limit: undocumented exact number, but a `429 Too Many Requests` / quota-exceeded response is a known, handled case in the Go SDK (`ErrQuota`). Treat as real and implement backoff-aware messaging.

## Top Workflows
1. **List/search jobs** with status, date-range, and crew filters — the daily dispatcher view (`job/all/`, params: `records` (max 100), `offset`, `start_date`, `only_open`, `status[]`).
2. **Create and schedule a job** from a client inquiry (`job/create/`), then **assign/unassign crew** (`job/assign/`, `job/unassign/`).
3. **Manage leads** through the pipeline: create a lead (`lead/create/`), list/filter leads, convert language ("Lead" becomes a "Job" once accepted — no direct convert endpoint, so this is app-side).
4. **Client lookups** — create and fetch customer records tied to jobs/leads (`Client/create/`, `Client/get/{id}/`).
5. **Crew/roster visibility** — list team members and their time-off (`team/all/`, `TimeOff/get/`, `TimeOff/get/{username}/`) to know who's available to dispatch.

## Table Stakes
- List + get-by-id for jobs, leads, clients, team members, time-off (every wrapper implements this baseline)
- Create job / create lead / create client (Pipedream actions, Go/PHP/Python SDKs all cover this)
- Update job/lead schedule (date/time/timezone) — Go SDK's `UpdateJobSchedule`/`UpdateLeadSchedule`
- Assign/unassign crew members to a job or lead (all wrappers implement this, notably fiddly: Workiz identifies crew by *name* on assign/unassign, not id, so the Go SDK does an id→name lookup dance)
- Polling-based "new job created" / "new lead created" triggers (Pipedream sources) — no true webhook registration endpoint found in any source; polling `CreatedDate` is the established pattern
- Offset/records pagination with a documented 100-record page cap and a `has_more` continuation flag

## Data Layer
- Primary entities: `jobs`, `leads`, `clients`, `team_members`, `time_off`
- Sync cursor: `CreatedDate` (matches the polling pattern every existing integration uses); jobs/leads also carry `LastStatusUpdate` useful for incremental refresh of status changes
- FTS/search: job/lead notes (`JobNotes`/`LeadNotes`), `Comments` (irregularly-shaped array of `{Comment}` objects — needs custom unmarshal, confirmed by the Go SDK's custom `workizComment` type), client name/address/phone/email — none of this is searchable via the live API at all

## Codebase Intelligence
- Source: `BeelineRoutes/workiz` (Go SDK) — most complete ground-truth source found; `forward-force/workiz` (PHP, jobs/leads/timeoff only); `OkoyaUsman/workiz-python-wrapper` (Python, jobs/leads/team/timeoff); `PipedreamHQ/pipedream` `components/workiz/` (JS actions+sources, official Pipedream integration)
- Auth: token as a URL path segment (`.../api/v1/{token}/...`), `auth_secret` as a JSON body field named `auth_secret` on every POST. No header-based auth observed anywhere.
- Data model: `Job`/`Lead` share nearly identical shape (both have `UUID`, `*DateTime`, `*EndDateTime`, `CreatedDate`, `PaymentDueDate`, `LastStatusUpdate`, `*TotalPrice`, `*AmountDue`, `SubTotal`, `Status`, `Team[]`, address fields, contact fields). `Team[]` sub-objects carry `{id, name}` (types differ: Job's `Team[].Id` is numeric, Lead's is string — API is inconsistent about this). Custom parsing needed for: `workizTime` (format `2006-01-02 15:04:05`, or the literal string `"null"`), `Unit` (API returns either a string or an int for the same field depending on user input), `Comments` (empty string `""` when there are none, else an array of `{Comment}` objects).
- Rate limiting: HTTP 429 = quota exceeded (`ErrQuota` in the Go SDK); HTTP 401 = auth expired/invalid (`ErrAuthExpired`). Error bodies come back in one of two shapes — `{Details: {Error: "..."}}` or `{Details: [{Error: "..."}]}` — the Go SDK implements a custom unmarshaler that tries both.
- Architecture: no real relational query surface — every "list" endpoint pages through all records with `records`/`offset`/`has_more`, and every cross-entity view (crew utilization, lead conversion funnel, revenue by source) has to be computed client-side. This is exactly the gap a local SQLite mirror closes.

## Product Thesis
- Name: Workiz (canonical display name, single word)
- Why it should exist: No CLI, MCP server, or Claude skill exists for Workiz today — every integration path is either a thin polling-based Zapier/Pipedream automation or a hand-rolled SDK wrapper with no cross-entity intelligence. Dispatchers and FSM operators currently have zero terminal/agent-native way to ask "who's overbooked this week," "which lead sources actually convert," or "what changed since I last checked" — all answerable only by joining jobs+leads+team locally, which no existing tool does.

## Build Priorities
1. Data layer for jobs, leads, clients, team_members, time_off with sync via `CreatedDate`/`LastStatusUpdate` cursor + FTS across notes/comments/contact fields
2. Absorb every list/get/create/update/assign/unassign command across all 5 resources, matching-and-beating every SDK found
3. Transcendence: crew utilization/bottleneck view, lead-to-job conversion funnel, revenue pipeline by source/status, missing-data audit, "since" digest — all only possible via the local join no live endpoint offers
