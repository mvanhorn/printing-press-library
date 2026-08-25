# Concur CLI Brief

## API Identity

- Domain: SAP Concur — enterprise expense management, business travel
  booking, and invoice/AP automation. Two faces: (1) a documented OAuth2
  partner REST API (v3/v4) at `developer.concur.com`, gated behind a
  Partner Enablement Manager relationship; (2) the actual employee-facing
  web app at `{tenant}.concursolutions.com`, which runs on its own
  undocumented, cookie-authenticated internal GraphQL BFF
  (`www-{region}.api.concursolutions.com/cds/graphql`).
- Users: individual employees filing expense reports and booking/tracking
  business travel (this CLI's target); secondarily finance/AP teams and
  approvers; the partner REST API's actual audience is enterprise
  integrators (accounting systems, T&E platforms), not individuals.
- Data profile: expense reports (header + line-item entries, allocations,
  attendee associations, workflow/approval state, receipts), available
  expenses queue (imported corporate-card charges and e-receipts not yet
  on a report), trips/itineraries (flights, hotels, cars, trains), travel
  requests/pre-trip authorization, travel allowance (per diem).

## Reachability Risk

- [Low] for the CLI's actual runtime target (cookie-authenticated internal
  GraphQL BFF) — confirmed reachable and functioning via a live
  authenticated session during this run (see discovery report). Auth
  setup requires a one-time interactive login (Okta SSO + MFA, user-driven
  — not automatable without the user's credentials), same as any
  cookie/browser-session CLI.
- [High, by design] for the *documented partner REST API* as a path for
  individual users — confirmed via official docs that `clientId`/
  `clientSecret` issuance requires a Partner Enablement Manager
  relationship and Business Development vetting; there is no self-serve
  signup. This is not a bug to work around; it's why this CLI defaults to
  cookie/browser-session auth instead of OAuth2.
- Tier/permission hints from 4xx body: none observed — no auth failures
  were hit during the live session (session was valid throughout).
- Probe-safe endpoint used: `GET messagenexus/v1/messages/newMessageCount`
  on `www-us2.api.concursolutions.com` — cheap, side-effect-free, cookie
  session's presence and health can be checked against it. Good
  `health_check_path` candidate (region-relative).

## Top Workflows

1. **File an expense report** (priority 1, per user). Create report →
   pull "Available Expenses" (corporate card charges / e-receipts) → apply
   per-expense-type Business Purpose + optional reimbursement-cap
   itemization → submit for approval. Confirmed end-to-end via working
   local prior art (see Codebase Intelligence).
2. **Check report/approval status.** "Where is my report?" is a named
   pain point across public reviews — surfacing `approvalStatus`,
   `paymentStatus`, and exception/audit flags per report directly in the
   CLI (`concur reports list`, `concur reports status <id>`) is high value
   and something the incumbent web UI buries in clicks.
3. **List/manage available (unfiled) expenses.** The queue of
   not-yet-reported corporate-card charges and e-receipts is the raw
   material for #1 and a workflow in its own right (e.g., "what's sitting
   unfiled right now").
4. **Travel: view upcoming trips/itinerary** (priority 2, per user).
   Read-heavy — trip list, itinerary detail, booking confirmations. No
   evidence of individual users needing to *book* travel via CLI (booking
   flows are complex, multi-step, and best left to the web/agent UI); read
   access to itinerary/trip data is the realistic CLI-shaped slice.
5. **Travel requests / pre-trip authorization** (secondary, connects
   travel and expense). `Request v4` in the official API models this;
   worth a read-only `concur requests list/get` if the GraphQL BFF exposes
   equivalent data — unverified, flagged as a build-time discovery task.

## Table Stakes

- Report CRUD (create, list, get, delete-while-unsubmitted), line-item
  listing, submit-for-approval — table stakes per the official Reports
  v4/Expenses v4 API surface and confirmed as the core loop by prior art.
- Receipt attachment (Receipts / Spend Documents v4 in the official API) —
  competitors (Expensify, in our own library) lead with receipt
  capture/OCR as a headline feature; Concur's own users cite receipt
  matching as a pain point (see below), so at minimum a `receipts upload`
  command tied to an expense entry is table stakes, not a nice-to-have.
- Per diem / travel allowance visibility (`Travel Allowance v4`).
- Trip/itinerary listing (`Travel: Itinerary v1/v4, Trip v1.1`).
- Multi-tenant/multi-region base URL — Concur is single-tenant-per-company
  with data-center-specific hosts (`us2`, `eu2`, `apj1`, `usg`/Gov, etc.);
  the CLI must NOT hardcode `us2` (that's this account's tenant, not a
  Concur universal).

## Data Layer

- Primary entities: `ExpenseReport` (header + status + total), `Expense`
  (line item: type, amount, currency, date, vendor, business purpose,
  receipt ref), `AvailableExpense` (unfiled queue item, same shape as
  `Expense` pre-report-assignment), `Trip`/`Itinerary` (travel), `Receipt`.
- Sync cursor: report list and available-expenses queue are the
  highest-value syncable resources (both change frequently as new
  card transactions/e-receipts land and reports move through approval).
- FTS/search: expense vendor/description free-text search and report-name
  search are the two searches a real user actually wants ("find that
  Uber charge from last month").

## Codebase Intelligence

- Source: local prior art at
  `~/Documents/code/work-projects/expense-report-filer` (private, not on
  GitHub — Playwright-based Python CLI automating this exact company's
  Concur tenant) plus its dependency
  `~/Documents/code/work-projects/magnite-playwright-okta-auth`.
- Auth: Concur signin → HRD → **Okta SSO** (password + TOTP from macOS
  Keychain) → SAML redirect → session persisted as cookies. Confirms
  cookie/browser-session is not just *available* but the *only* practical
  auth path for an individual employee at this company (and likely most
  Concur customers using enterprise SSO).
- Data model: `ExpenseReport{name, start_date, end_date, business_purpose,
  expenses[], report_id, report_url}`, `ExpenseData{vendor, amount,
  currency, expense_date, category, description, receipt_path}`,
  `ExpenseTypeRule{expense_type, business_purpose, reimbursement_cap}`
  (config-driven per-Concur-Expense-Type default business purpose, with
  optional reimbursement cap that triggers an itemized personal/business
  split when exceeded — e.g. a $50/mo cell phone stipend). This
  cap-and-split behavior is a genuinely useful, non-obvious business rule
  worth carrying into this CLI's design (see Novel Features in the absorb
  manifest).
- Architecture: pure UI automation (Playwright Page Objects, accessible
  role/name selectors — Concur's UI has no stable `data-automation`
  attributes). Confirms real, working URLs:
  `https://{tenant}.concursolutions.com/home`,
  `https://{tenant}.concursolutions.com/nui/expense?confNum=new`
  (direct new-report deep link). This CLI will NOT replicate the
  UI-automation approach (printing-press ships replayable HTTP, not a
  resident browser) — it calls the internal GraphQL BFF directly, using
  cookie auth captured the same way (a one-time interactive login), but
  the *workflow logic* (available-expenses queue → move → per-type rules
  → submit) is carried forward verbatim as the CLI's command design.

## User Vision

- Logged-in browser session should be the default auth mode, not an API
  key — confirmed correct: SAP Concur's OAuth2 partner API requires
  Partner Enablement Manager-issued credentials with no self-serve path;
  two independent open-source projects and this user's own private prior
  art all converged on browser/cookie automation for the same reason.
- Two priorities, in order: (1) expense reports, (2) travel.

## Product Thesis

- Name: `concur` (binary: `concur-pp-cli`)
- Why it should exist: no CLI exists for Concur today — the ecosystem scan
  found zero PyPI packages, one low-traffic Python REST client
  (`Tevasoft/sap-concur-connector`, requires the gated OAuth2 API), one
  fresh MCP server (`bharath2020/concur-mcp`, also OAuth2-gated), and two
  independent browser-automation projects (npm
  `@browser-automation-hub/sap-concur-browser-automation`, GitHub
  `Nilesunknowing346/sap-concur-browser-automation`) that exist
  specifically *because* the official API is unreachable for individuals.
  This CLI fills the same gap those two projects target, but as a proper
  CLI (fast, scriptable, agent-native JSON output, local SQLite for
  status/search) calling the same replayable cookie-authenticated surface
  the real web app itself uses — not a resident-browser automation
  wrapper.

## Build Priorities

1. Expense reports: list/get/create/submit, available-expenses queue
   list + move-to-report, per-expense-type business-purpose/reimbursement
   rule engine (config-driven, ported from proven prior art), receipt
   attachment.
2. Travel: trip/itinerary list + detail (read-heavy), travel
   allowance/per-diem visibility.
3. Auth: cookie/browser-session as default (`auth login --chrome` /
   press-auth, `JWT` cookie as the carrier candidate), OAuth2
   client-credentials as an optional secondary path documented for users
   whose company IT already has partner API access.
4. Cross-cutting: multi-region base URL (data-center-specific, not
   hardcoded), local SQLite sync for reports + available-expenses queue
   (both change frequently and are the two things a user actually wants
   to search/filter offline).

## Crowd-Sniff Note

Formal `cli-printing-press crowd-sniff` was not re-run as a separate step:
equivalent npm/PyPI/GitHub community-signal research was already
performed manually during Phase 1 (see Product Thesis above for
findings). The primary spec is hand-authored from official docs + live
authenticated browser-sniff + private prior art, not from an
auto-discovered community spec, so crowd-sniff's marginal value here is
low. Documented for auditability per Phase 1.8's intent, not skipped via
a banned rationale.
