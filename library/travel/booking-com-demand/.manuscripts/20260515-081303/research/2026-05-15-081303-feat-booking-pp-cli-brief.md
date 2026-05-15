# Booking.com CLI Brief

## API Identity
- Domain: Travel & hospitality — accommodation search, car rentals, flights, booking management
- Users: Travel agents, affiliate partners, travel app developers, travel automation builders
- Data profile: Rich structured data — properties with photos/reviews/pricing, car rentals, flight itineraries, order management, messaging

## Reachability Risk
- Low — Official OpenAPI 3.1 spec with 14970 lines, sandbox environment available at demandapi-sandbox.booking.com
- No GitHub issues reporting API blocks or 403 errors on wrapper repos
- Rate limiting exists but documented

## Top Workflows
1. Search accommodations by destination, dates, guests — compare prices across properties
2. Check availability and detailed pricing for specific properties
3. Create and manage booking orders (preview → create → modify → cancel)
4. Browse property reviews and scores for decision-making
5. Search car rentals and flight options for complete trip planning
6. Message properties about reservations (send/receive/attachments)

## Table Stakes
- Search accommodations with filters (location, dates, guests, price range, facilities)
- Property details with photos, amenities, descriptions
- Availability and pricing checks (single and bulk)
- Order lifecycle (preview, create, details, modify, cancel)
- Reviews and review scores
- Car rental search and details
- Location reference data (airports, cities, countries, districts, landmarks, regions)
- Constants/reference data (facilities, room types, bed types)

## Data Layer
- Primary entities: Accommodations (properties), Orders (bookings), Car rentals, Locations, Reviews
- Sync cursor: accommodations/details/changes endpoint provides change tracking
- FTS/search: Property names, descriptions, reviews, location names
- Secondary: Payment methods, currencies, languages, chains/brands, suppliers

## Product Thesis
- Name: booking-pp-cli
- Why it should exist: No CLI tool exists for the Booking.com Demand API. Travel agents and affiliate partners currently interact through web dashboards or custom code. A CLI with offline search, local SQLite store, and agent-native output would enable automated travel workflows, price monitoring, bulk availability checks, and programmatic booking management that no existing tool provides.

## Build Priorities
1. Full accommodation search/availability/details/reviews surface
2. Order lifecycle (preview → create → details → modify → cancel)
3. Car rental search and details
4. Location and reference data commands
5. Messaging (send/receive with properties)
6. Transcendence: price monitoring, trip planning, review analytics, bulk comparison
