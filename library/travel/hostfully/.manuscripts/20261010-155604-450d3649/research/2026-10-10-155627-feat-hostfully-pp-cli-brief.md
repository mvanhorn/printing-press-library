# Hostfully CLI Brief

Reprint of run 20260525-111012 (press v4.9.0). Prior research reused; updated with live-operation lessons recorded as post-print patches.

## API Identity
- Domain: Vacation rental / small-hotel property management (Hostfully PMS), REST v3 (v3.3 for messaging and quotes) plus a GraphQL read surface. Bookings arrive from channels such as Airbnb, Booking.com, VRBO and direct.
- Users: Property managers and small operators running roughly 4-50 listings, and the agents that answer guest calls and messages on their behalf.
- Data profile: leads (reservations), properties, guests, threads/messages, quotes, jobs (cleaning/maintenance services), tags.

## Reachability Risk
- Low. API-key auth (`X-HOSTFULLY-APIKEY`), agency-scoped via `agencyUid`.
- Known quirk: `GET /leads` without a `propertyUid` filter is unreliable for full history and intermittently returns HTTP 500 on serialization of individual leads. Complete history needs per-property fetches; failures are recovered through a selective GraphQL scan.

## Top Workflows
1. Caller ID / guest lookup: phone number or name -> every stay, payment status and reservation access PIN.
2. Daily operations: today's arrivals, departures, in-house guests; live availability across properties.
3. Guest messaging: read recent inbox messages and threads; reply safely (dry-run first).
4. Quoting: live complete quote with itemized rent, taxes and fees for a property and date range.
5. Planning and analytics: vacancy windows across several properties, turnover/cleaning windows, maintenance slots, occupancy, channel mix, revenue estimates; jobs list for service invoice review.

## Table Stakes
- List/get leads, properties with filters; guest info lookup; today summary; availability grid; channel breakdown.

## Data Layer
- Primary entities: leads, properties. Cursor pagination (`_cursor`, `_limit`).
- Property list is discovered from the agency's `/properties` endpoint (optionally narrowed by config or `--property`), never hardcoded.
- Local SQLite archive for historical/compound analysis; operational reads (availability, whois, today) go live and never silently fall back to the archive.

## User Vision
- Publish a generic, operator-agnostic Hostfully CLI to the public library. No hardcoded property UIDs or operator names; no real guest data in examples, tests or mocks.
- Keep the live operational commands (whois, inbox, availability, quote, jobs, today/guests, --mock) as novel features alongside the offline analytics.

## Product Thesis
- Name: hostfully-pp-cli
- Why it should exist: No Hostfully CLI exists. Operators and their agents need instant caller-ID lookups, safe messaging, quotes, and multi-property vacancy analysis that the web app doesn't provide.

## Build Priorities
1. Live leads/properties with per-property fetch + GraphQL recovery; property discovery.
2. whois, today/guests, availability (live only).
3. inbox (recent/thread/reply with --dry-run), quote, jobs.
4. Vacancy/turnover/maintenance/occupancy/channels/revenue/patterns over the synced archive.
5. --mock fixtures for agent testing.
