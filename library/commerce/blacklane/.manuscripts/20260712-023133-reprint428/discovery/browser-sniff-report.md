# Blacklane Browser-Sniff Discovery Report

Primary user goal: **get an upfront chauffeur quote** (pickup → dropoff, date/time → vehicle classes + fixed prices).

## Transport / Auth
- Runtime: `standard_http` (plain HTTPS, no browser/clearance needed for the public surface).
- The public pricing surface needs **no authentication**.
- Account/booking surface uses **Auth0** (tenant `login.blacklane.com`, aud `https://blacklane.com`) — intentionally **out of scope** (real-money writes).

## Endpoints (public, replayable — verified via curl)
| Method | URL | Auth | Purpose |
|---|---|---|---|
| POST | `https://pricing-bff-api.blacklane.com/prices` | none | **Quote.** Body: serviceCategory, serviceType (transfer/hourly), departAt, duration (hourly), pickup{address,airportIata,latitude,longitude,placeId}, dropoff{...}, featureFlags[], voucherParameter{autoApplyPromotion}. Returns packages[]{packageSlug,title,subtitle,models,settings,price.totals.grossAmount,currency} + meta{estimatedDuration,estimatedDistance}. |
| GET | `https://pricing-bff-api.blacklane.com/packages/{slug}` | none | **Catalog.** slug ∈ {business, first, van}. Full vehicle-class metadata (models, capacity, features, imagery). |

## Address resolution
- Blacklane's own `locationsAutocomplete` / `locationsGeocode` GraphQL ops are **SSR-only** (`apollographql-client-name: web-ssr`) and return UNAUTHENTICATED to external callers.
- CLI resolves addresses → coordinates via **OpenStreetMap Nominatim** (`https://nominatim.openstreetmap.org/search`, free, no key). `/prices` accepts `placeId: null` and prices off coordinates.

## Verified sample (an example hotel → airport, transfer)
- business $141.63, suv_us $167.96, first $199.33 (USD) — anonymous curl.

## Out of scope (Auth0-gated)
- serviceClasses (authed quote variant), bookings list, ride status, wallet, payment methods, contacts, create booking.

## Hourly quote (corrected, verified)
- `serviceType: "hourly"`, **no `dropoff`**, `duration` in **SECONDS** (hours × 3600; min 7200 = 2h).
- Response `meta.includedDistance` (meters) included in the hourly package.
- Verified: 3h Union Square SF → Business $284.85, SUV $341.79, First $448.83 (USD), included 120 km.
