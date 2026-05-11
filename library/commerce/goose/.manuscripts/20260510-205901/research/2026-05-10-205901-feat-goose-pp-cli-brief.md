# Goose CLI Brief

## API Identity
- **Domain:** Pet-care facility admin SaaS (dog boarding, daycare, grooming). Multi-tenant: each "location" is a facility; Goose runs the platform.
- **Vendor:** Goose (goose.pet) — private, no public API, no public docs. Used by independent pet-care businesses.
- **Users:** Facility owners/managers (e.g., this CLI's user = admin of the facility). Front-desk staff use the web app for daily check-in/out, booking, grooming schedules, customer lookups. Owners/managers use it for reporting, revenue management, marketing.
- **Data profile:** ~50 report types, ~16 CSV exports, 4 microservice hosts, deep relational model centered on **bookings (invoices) ↔ orders ↔ customers (location-user-profile) ↔ pets (location-pet-profile)** with first-class concepts for activities (check-in/out events on resource units), feeding/medication instructions, vaccinations, tags, agreements/contracts, vouchers (offer + cash credits), memberships, payments, and notes.

## Reachability Risk
- **Low.** API is reachable: `api.goose.pet/api/v1/admin/<facility>/<resource>` returns 401 (auth required) on unauthenticated requests — clean auth boundary, no bot mitigation observed. CORS pinned to `https://app.goose.pet`. CloudFront → API Gateway → Lambda back-end.
- **Bottleneck:** access tokens are short-lived (~1 hour). Refresh-token flow via Cognito InitiateAuth handles this.
- **Not a public API.** No published terms; user is acting under their own admin credentials for their own facility (the facility). The CLI ships read-only commands for v1 and never auto-mutates without explicit `--write` opt-in.

## Top Workflows (user-confirmed scope: all four)
1. **Today's roster** — who's checking in, who's checking out, who's already here, with pet tags + room assignments + feeding instructions. Daily morning operational view.
2. **Customer / pet lookup** — search by name/phone/email → load full profile (vaccinations, tags, alerts, recent bookings, balance, payment methods, notes).
3. **Bookings (reservations) management** — list past/upcoming bookings, filter by service type (boarding/daycare/grooming), date range, customer; get full booking detail.
4. **Operational reports & exports** — pull the daily Feeding & Medication report, Expiring Vaccinations report, customer/pet exports, sales/cash reports.

## Table Stakes
- Cognito JWT-bearer auth with browser-derived refresh-token flow (`Authorization: Bearer <accessToken>`).
- Multi-facility awareness (single user can belong to multiple locations; JWT `cognito:groups` lists them). Default to first location, allow `--facility` override.
- Includes-aware list/get commands — Goose's API uses `?includes[]=path.to.relation` heavily; the CLI must let agents request deep shapes.
- Date-range queries with `gte_/gt_/lte_/lt_/eq_` prefix on date params.
- Pagination via `limit` + `sortOrder` (server-side) with client-side aggregation for `--all`.

## Data Layer (local SQLite)
- **Primary entities:** `location_user_profiles` (customers), `location_pet_profiles` (pets), `invoices` (bookings), `orders`, `location_service_types`, `location_species` + `breeds`, `resources` (staff/rooms), `vouchers`, `notes`, `contracts`.
- **Sync cursor:** `period.startDate=gte_<last-sync>` for invoices; `updatedAt`-style cursor TBD per entity (need to inspect `?orderBy=updatedAt` support). MVP: full sync of users + pets, recent-window sync of invoices (last 90d + next 90d).
- **FTS5 search:** index `displayName`, `email`, `phone`, `lastName`, `firstName`, breed names, tag display names, pet alert text.

## Codebase Intelligence
- No public source (private SaaS). Inferred from network observation:
  - **Architecture:** React SPA (`app.goose.pet`) + REST API behind CloudFront/API Gateway/Lambda. Four backend microservices: `api`, `search-api` (Elasticsearch), `pawgress-report` (report cards), `soar-api` (Explo token broker).
  - **Auth:** AWS Cognito (User Pool `us-east-2_IqPUw1L4C`, ClientId `4qv4b8pvtsqigsontd3vfmf6kf`). Public client (no secret). Standard Amplify storage layout in `localStorage`.
  - **Data model:** strong "location" multi-tenancy — every customer/pet/service-type is a `locationX` join row over a canonical `X` (e.g., `locationSpecies → species`, `locationServiceType → serviceType`). Facility slug threaded through every path.
  - **Reports:** rendered via embedded Explo dashboards (third-party). The CLI can list reports and mint embed URLs but cannot run the SQL. CSV exports (~16 of them) hit `reports/<slug>` directly and return data the CLI CAN read.
  - **Error handling:** 401 on expired tokens, 304 with ETag for cache-fresh, 200 for happy path. No global rate-limit headers observed.

## User Vision
- "They have an admin backend that has a private API… `https://api.goose.pet/api/v1/admin/<facility-slug>` (the slug is the property id or something along those lines)."
- Confirmed: user is the admin of the facility and wants to automate their own admin workflows. Scope priorities: reservations, dogs, owners, reporting (all four).

## Product Thesis
- **Name:** `goose-pp-cli` (binary), `goose` if installed user-side.
- **Tagline:** "Operate your goose.pet facility from the terminal — today's roster, customer lookup, exports, and report cards without leaving your shell."
- **Why it should exist:** Front-desk and ops folks at goose.pet facilities live in the web app; the admin API is rich and useful but undocumented. A CLI lets the operator scriptify the morning routine, plug data into agents/dashboards, and grep through the customer base offline. **No competing CLI/MCP/wrapper exists** (private SaaS). This is novel ground.

## Build Priorities
1. **Auth foundation** — Cognito refresh-token flow + browser-cookie capture from Chrome (novel hand-written `auth login --chrome`). Bearer-token paste fallback (`GOOSE_ACCESS_TOKEN` env var).
2. **Core resource commands** — `bookings list/get`, `customers search/get`, `pets list/get`, `services list`, `reports list/run`, `report-cards`, `messages list` (all read-only).
3. **Local sync** — `goose sync` populates SQLite with users + pets + invoices (recent window). `goose search` does FTS over the local store.
4. **Novel transcendence** — `today` (composite arrivals/departures/here), `feeding today` (replay of feeding-medication-export), `vaccines expiring --within 30d`, `customer <name>` (search→detail one-shot), `pet <name>` (same).
