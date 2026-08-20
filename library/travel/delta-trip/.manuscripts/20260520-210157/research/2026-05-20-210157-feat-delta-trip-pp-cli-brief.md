# Delta Trip CLI Brief

## API Identity
- Domain: delta.com — Delta Air Lines trip management (My Trips surface)
- Users: Travelers who have Delta booking confirmation numbers and need to view/modify trips programmatically or via CLI without using the web UI
- Data profile: PNR-based trip data — confirmation number + passenger name unlocks flight details, seat assignments, baggage info, upgrade status, boarding passes, gate info, check-in status across 1..N flights per confirmation

## Reachability Risk
- **Medium** — Delta.com uses WAF / anti-bot measures (Cloudflare or similar). However:
  - The `findPnr.action` form endpoint is publicly documented in URLs and has been accessed via direct HTTP
  - Precedent: `Scrape-Delta-Flights` (GitHub) successfully scraped Delta data
  - My Trips page is JS-heavy; plain curl to `/mytrips/findPnr.action` may return a redirect or HTML wrapper, requiring Surf or browser-compatible HTTP
  - Risk is medium, not hard-blocked; browser-sniff needed to confirm replayable surface

## Top Workflows
1. **Trip lookup** — Enter confirmation number + passenger name → get complete trip summary with all flights, seats, status
2. **Show flight details** — Expand each flight: aircraft, gate, baggage, boarding group, upgrade eligibility, meal, check-in status
3. **Seat management** — View current seat; see what's available; understand upgrade options (miles or cash)
4. **Check-in** — Determine if check-in is open (24h window); get boarding pass details
5. **Baggage tracking** — Track checked bags by tag number or confirmation; get carousel info post-arrival

## Table Stakes
- Look up trip by confirmation + first + last name (no login)
- Display all flights under a multi-leg itinerary
- Show flight details: departure/arrival times, flight number, aircraft, route
- Show seat assignment, gate (if available), boarding group
- Show baggage count and status
- Show upgrade eligibility and cost
- Check-in status and 24-hour window awareness
- JSON output for agent-native usage
- Offline caching of looked-up trips in SQLite

## Data Layer
- Primary entities: Trip, Flight, Passenger, Seat, Bag, BoardingPass
- Sync cursor: confirmation number (user-provided key); cache with TTL for subsequent reads
- FTS/search: search cached trips by passenger name, confirmation, route, date

## User Vision
- Narrow scope: only "My Trips" functionality that doesn't require login
- Read AND modify (seat, baggage add, check-in initiation) using confirmation + first + last name
- Multiple flights per confirmation number (full itinerary support)
- "Show flight details" as a first-class operation

## Source Priority
- Primary: delta.com My Trips web surface (`/mytrips/findPnr.action` and related trip management pages)
  - Spec state: no official spec — browser-sniff required
  - Auth: confirmation number + passenger name (no login required)
  - Reachability: medium risk

## Product Thesis
- Name: `delta-trip-pp-cli` → binary name `delta-trip`
- Why it should exist: delta.com's My Trips is buried in a JS-heavy web UI. Travelers who manage multiple trips, developers who integrate travel data, and power users who want instant terminal access to their booking details have no CLI alternative. A SQLite-backed offline cache plus agent-native `--json` output turns a web-only workflow into a scriptable, composable tool.

## Build Priorities
1. Trip lookup by confirmation + first + last name with full flight list
2. Flight detail expansion (all fields from "show flight details" section)
3. Seat info and upgrade eligibility display
4. Baggage status and tracking
5. Check-in status check
6. Offline trip caching with TTL (so repeated lookups don't hit the site)
7. Multi-passenger / multi-flight itinerary handling
