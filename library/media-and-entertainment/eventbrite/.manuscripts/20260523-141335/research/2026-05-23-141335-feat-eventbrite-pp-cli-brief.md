# Eventbrite API v3 CLI Brief

## API Identity
- **Domain:** Event management / ticketing. Organizer-side platform: create & manage events, sell tickets, track orders & attendees, run reports, configure discounts/inventory/webhooks.
- **Users:** Event organizers, ops teams, ticketing managers, agencies running events on behalf of clients. NOT consumers browsing events (that surface was removed — see Reachability).
- **Data profile:** Hierarchical and organizer-scoped. Organizations → Events → (Ticket Classes, Orders, Attendees, Discounts, Inventory Tiers, Questions, Teams, Reports). Money fields are integer-minor-unit + currency. Pagination via **continuation tokens** (not page numbers).
- **Base:** `https://www.eventbriteapi.com/v3`
- **Spec source:** Apiary API Blueprint (`eventbriteapiv3public.source`, FORMAT 1A) → converted apib2swagger → swagger2openapi (--patch --warnOnly) → OpenAPI 3.0.0. 72 paths / ~100 operations / 33 resource groups. 12 polluted MSON-derived property keys sanitized.

## Reachability Risk
- **Medium (one dead endpoint, rest healthy).** `GET /events/search/` (public Event Search) was **removed Dec 2019 / fully denied Feb 20 2020** — it still appears in the Blueprint as "Search Events - deprecated." Must be excluded or clearly marked; it is NOT a headline feature.
  - Evidence: Automattic/eventbrite-api issue #83; Eventbrite migration notice. Recommended replacements: events by organization, by venue, by series, by ID.
- Everything else (events, orders, attendees, ticket classes, reports, discounts, organizations, venues, webhooks) is live and OAuth-gated. Reachable with a private token.
- No GitHub-issue evidence of broad 403/blocking on the organizer endpoints.

## Auth
- **Type:** OAuth2 "private token" → sent as `Authorization: Bearer <token>`. For a CLI the user pastes a private token from eventbrite.com/platform/api-keys; no interactive OAuth dance needed for own-account use.
- **Canonical env var:** `EVENTBRITE_API_KEY` (used by joshuachestang MCP, Composio toolkit, most community tools). Slug-derived `EVENTBRITE_OAUTH2` would be wrong → enrich spec with bearer scheme + `x-auth-env-vars: [EVENTBRITE_API_KEY]` before generate.
- Get a key at: https://www.eventbrite.com/platform/api-keys

## Top Workflows
1. **Manage your events end-to-end** — create → update → publish → cancel/copy; list events by organization.
2. **Track orders & attendees** — list orders/attendees for an event, get attendee detail, check-in status, by-organization rollups.
3. **Sell & price** — ticket classes (create/update), discounts, inventory tiers, pricing.
4. **Report on sales** — Reports group (sales, attendees) for revenue/attendance.
5. **Offline search & analytics over synced data** — because public search is dead, sync your org's events/orders/attendees into local SQLite and query offline (the differentiator).

## Table Stakes (from MCP servers + SDKs to absorb)
- create_event, list_events (by org), get_event, update_event, publish_event, cancel_event, copy_event, delete_event (joshuachestang MCP, all 5 MCP servers)
- list_categories, list_formats, create_venue / get venue, list venues by org
- attendees: list by event / by org, get attendee
- orders: list by event / by org, get order
- ticket classes: list/create/update/delete
- discounts: list/create/update/delete
- inventory tiers, questions, webhooks, reports
- Official SDKs (Python/JS/PHP) are thin GET/POST helpers — every public method maps to one of the above endpoints.

## Data Layer
- **Primary entities (sync targets):** events, orders, attendees, ticket_classes, discounts, organizations, venues, reports.
- **Sync cursor:** continuation token per list endpoint (`?continuation=`); response carries `pagination.continuation` until exhausted.
- **FTS/search:** offline FTS5 over synced events + attendees + orders. This is the headline value — it restores searchability Eventbrite removed, over the organizer's own data.

## Codebase Intelligence
- Official SDKs (eventbrite-sdk-python, -javascript, -php): thin clients; auth = private token Bearer; pagination = continuation token; responses JSON with `pagination` envelope on list endpoints.
- MCP servers converge on the same ~8-12 organizer operations + EVENTBRITE_API_KEY; none offer offline store, FTS, or cross-event analytics.

## Product Thesis
- **Name:** Eventbrite (display) / `eventbrite-pp-cli`.
- **Headline:** "Every Eventbrite organizer endpoint, plus a local SQLite mirror of your events, orders, and attendees you can search offline — restoring the search Eventbrite shut off, over your own data."
- **Why it should exist:** Existing MCP servers wrap a handful of event CRUD calls. None give organizers offline search, sales-velocity, repeat-attendee, or discount-performance analytics that require the whole order/attendee history in one place. Public event search is dead; the only way to "search Eventbrite" today is to own the data locally. This CLI does that, agent-native, with `--json`/`--select`/`--dry-run`/typed exit codes.

## Build Priorities
1. P0: data layer + sync (continuation-token pagination) for events/orders/attendees/ticket_classes/discounts/organizations/venues; FTS5 search.
2. P1: full endpoint mirror of all 33 resource groups (generator-emitted), with `/events/search/` excluded/marked dead.
3. P2 (transcendence, hand-built via Codex): offline analytics over synced data — sales velocity, repeat attendees, discount performance, check-in/capacity rollup, sales-since drift. (Final set from Step 1.5c.5 subagent.)
