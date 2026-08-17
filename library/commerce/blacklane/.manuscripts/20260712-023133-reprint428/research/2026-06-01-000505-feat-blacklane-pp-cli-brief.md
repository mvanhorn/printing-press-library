# Blacklane CLI Brief

## API Identity
- Domain: Premium global chauffeur / ground-transport booking. Airport transfers, hourly bookings (2–24h chauffeur on standby), and city-to-city long-distance rides across 500+ cities in 60+ countries. Upfront all-inclusive fixed pricing (tolls, tips, 1h airport wait, flight tracking included).
- Users: Business/corporate travelers, EAs booking for execs, travel agencies, frequent flyers, premium leisure travelers.
- Data profile: Quotes (origin/destination/time/vehicle-class → price), vehicle classes (Business/Exclusive/SUV/Electric/Sprinter/First), bookings/rides (status, chauffeur, vehicle, ETA), service cities/areas, airports.

## Reachability Risk
- [Medium] The consumer marketing site (blacklane.com) serves fine over plain HTTP (HTML reachable). The *booking* surface (quote/price + reservation) is a JS app that almost certainly calls a JSON/GraphQL backend, and premium booking sites commonly sit behind Cloudflare / bot-mitigation. No public OpenAPI spec exists; the internal `mobile-api-testing.blacklane.io` host is dead (ECONNREFUSED). The Partner/B2B "API Connectivity" program exists but is credentialed, sales-gated, and undocumented publicly (contact Blacklane's business team; GDS via Sabre/Amadeus/Travelport, OBT via Concur/Navan). Transport class must be settled by `probe-reachability` + browser capture.

## Top Workflows
1. **Get an upfront quote** for a ride: pickup + dropoff (or airport), date/time, passengers → list of vehicle classes with fixed prices. (Highest-value, likely public, no login.)
2. **Browse vehicle classes & what's available** for a given city/route.
3. **Book a ride** (create reservation) — requires a logged-in account + payment.
4. **Manage rides** — list upcoming/past bookings, get status, chauffeur/vehicle details, cancel/modify.
5. **Discover service areas** — which cities/airports Blacklane serves, city-to-city routes.

## Table Stakes (what any Blacklane tool must do)
- Quote a point-to-point transfer and an hourly booking.
- Resolve places/airports (autocomplete → place IDs the booking API needs).
- List vehicle classes with capacity (pax/luggage) and price.
- List a user's bookings and a single booking's live status.

## Data Layer
- Primary entities: `quotes`, `vehicle_classes`, `bookings`, `places`/`airports`, `cities`/`service_areas`.
- Sync cursor: booking `updated_at` for ride history; quotes are ephemeral (cache by route+time hash).
- FTS/search: cities, airports, vehicle-class names, saved bookings.

## Auth Profile
- Quote/price/place-autocomplete: expected public (anonymous), no key.
- Bookings (create/list/manage): browser session auth — Blacklane account login (email/password or SSO). No public API key for consumers. Will offer authenticated browser-sniff if the user is logged in.

## Product Thesis
- Name: blacklane-pp-cli
- Why it should exist: There is **no** existing Blacklane CLI, MCP server, or community wrapper anywhere (verified). An agent-native CLI that can quote a chauffeur, compare vehicle classes, and track/list rides from the terminal — with a local SQLite store of your ride history and offline search — is net-new. Pairs naturally with travel automation (it's the ground-transport leg between flights and hotels).

## Build Priorities
1. Quote engine: `quote` (point-to-point) + `quote --hourly` returning vehicle classes + prices. Plus place/airport resolution feeding it.
2. Local store + ride management: `bookings list/get`, `sync`, `search`, SQL.
3. Transcendence: ride-history analytics, trip-leg planning, fare watching, vehicle-class fit advisor — things only possible with everything in SQLite.

## Notes / Risk Flags
- Booking is a real-money, irreversible side effect. The `book` path (if reachable) ships dry-run-by-default and gated; never auto-books under verify/dogfood.
- If the booking backend is only reachable through live page-context JS (non-replayable), HOLD the write path and ship the read/quote surface via Surf/clearance-cookie HTTP.
