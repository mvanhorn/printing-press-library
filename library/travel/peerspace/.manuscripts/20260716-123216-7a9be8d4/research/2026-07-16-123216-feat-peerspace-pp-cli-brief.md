# Peerspace CLI Brief

## API Identity
- Domain: Peer-to-peer venue marketplace — unique event spaces, meeting rooms, film/photo locations, workshops, parties
- Users: Event planners, producers, creators, hosts listing spaces; guests searching/booking; power users who shortlist venues across cities
- Data profile: Search-heavy Elasticsearch listings (title, city, neighborhood, capacity, pricing, amenities, instant-book, host signals); authenticated favorites/projects, profile, message counts; public filter/env dictionaries
- Spec source: User-provided HAR (`www.peerspace.com.har`, 702 entries → cleaned to 11 useful `/v1/*` endpoints on `www.peerspace.com`)

## Reachability Risk
- **Medium (Cloudflare, mitigated by Surf):** stdlib HTTP gets `403` + `cf-mitigated: challenge` on JSON APIs. `probe-reachability` on `/v1/search/filters/list` returns `mode: browser_http` — Surf with Chrome TLS fingerprint returns `200 application/json`. Runtime ships browser-compatible HTTP; no resident browser.
- CAPTCHA/Forter markers appear on HTML/search-shell traffic; they do **not** block the JSON search API under Surf.
- Probe-safe endpoint used: `GET /v1/search/filters/list?activity_type=meetup`
- No official public OpenAPI; no community SDK/CLI found. Reachability depends on Cloudflare policy stability.

## Top Workflows
1. **Find a venue for a use-case** — search by city/bbox + activity (`meetup`, party, film) + space type + guest count + price + date window
2. **Filter vocabulary discovery** — list amenities / space types / styles for an activity before searching
3. **Shortlist management** — pull favorites board + project details for the logged-in guest
4. **Inbox pulse** — unread message thread count while coordinating bookings
5. **Offline re-query** — sync listings into SQLite and re-filter locally (price bands, capacity, neighborhood, instant-book)

## Table Stakes
- Search listings with location + activity + space type + guests + price + availability
- List search filters (amenities, space types, styles)
- Authenticated profile (`profiles/me`)
- Favorites / saved boards
- Message thread counts
- Agent-native JSON + offline search over synced listings

## Data Layer
- Primary entities: `listings` (search hits), `filters` (amenities/space_types/styles), `favorites` (fav_board attachments), `projects`, `profile`
- Sync cursor: full refresh of search snapshots by (location, activity) query key; favorites by collaborator_id
- FTS/search: listing title, description, city, neighborhood, space_type_tag

## User Vision
- HAR provided; user is logged in to Peerspace in browser — enable cookie auth via `auth login --chrome` for favorites/profile/messages

## Product Thesis
- Name: **peerspace-pp-cli**
- Why it should exist: Peerspace has no official API, no CLI, and no MCP. Venue scouting is a multi-tab browser chore. A Surf-backed, offline-first CLI lets agents and power users search, shortlist, and re-query spaces with SQL/FTS — including session-gated favorites without reverse-engineering auth from scratch each time.

## Build Priorities
1. Public search + filters (flagship, no_auth, Surf transport)
2. Cookie auth + profile / favorites / messages
3. Local store + FTS over listings + novel offline commands (budget scout, capacity bands, shortlist deltas)
4. Agent-native output (`--json`, `--select`, `--compact`) end-to-end

## Reachability Gate
- Decision: PASS (browser_http)
- Evidence: Surf probe 200 on filters list; HAR contains real JSON search/profile/favorites payloads

## User Vision (amended)
- Primary use: **tech events** (meetups, workshops, community events)
- Need: venue fit ranking (headcount, date, budget, vibe), amenity gap analysis, shortlist→Luma/Eventbrite/Slack export, price drift watch, multi-city campaign scout
- Agent requirements: all novel commands `--json --agent --select`; expose listing store for ad-hoc SQL; single-listing `venues get` for finalists
