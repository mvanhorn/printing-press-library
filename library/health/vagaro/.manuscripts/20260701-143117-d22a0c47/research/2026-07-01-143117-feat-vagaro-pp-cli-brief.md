# Vagaro CLI Brief

## API Identity
- **Domain:** Consumer marketplace for salon / spa / fitness / wellness discovery + booking (vagaro.com). This is the *consumer* side, NOT Vagaro Pro (business management).
- **Users:** People searching for and booking beauty/wellness appointments; power users who juggle multiple providers; AI agents doing cross-business availability lookups.
- **Data profile:** High-gravity, time-sensitive: businesses (merchants), services, staff/providers, availability slots, reviews, prices, geo, deals, livestream classes. Location-scoped by `city--state` slugs and a `proximitystate_v3` geo cookie.

## Reachability Risk
- **None / Low.** Public pages return **HTTP 200 with full SSR content to every UA tested** — Chrome UA, `curl/8.1`, and `python-requests` all got the complete 653 KB listings HTML.
- `probe-reachability` reported `browser_clearance_http` (conf 0.6) but that is a **false positive**: the only body evidence is an embedded reCAPTCHA form widget, not a clearance wall. Both stdlib and surf-chrome probes returned 200 + real business data.
- Site sits behind **Imperva Incapsula** (cookies `visid_incap_*`, `incap_ses_*`) but it did not challenge headless Chrome or raw curl during capture.
- Mitigation baked into the spec: ship a Chrome `User-Agent` header (research noted a server-side bot-UA blocklist even though it did not fire in testing — cheap insurance). `http_transport: standard`.
- Probe-safe endpoint used: `GET /listings/massage/san-francisco--ca` (public search page).

## Discovered API Surface (browser-sniff, 2026-07-01)
Two request styles observed on `www.vagaro.com`, plus the SSR HTML itself:

1. **SSR HTML + schema.org JSON-LD (primary, most robust — no auth, no signing):**
   - `GET /listings/{service}/{city--state}` → JSON-LD `ItemList` of `LocalBusiness` (name, image, business-slug URL, telephone, priceRange, full address, aggregateRating {value, count}). This is the search-results surface.
   - `GET /{business-slug}` → SSR business profile (embeds `BusinessID`, breadcrumb JSON-LD, reviews, description, rating). e.g. `/kellybsmassagetherapy`.
   - `GET /{business-slug}/services` → SSR service menu (ServiceTitle, durations, prices, descriptions; `ServiceID` embedded).
   - `GET /{business-slug}/book-now` → SSR availability summary ("next available date"; booking uses Stripe/Affirm/Google-Pay iframes).
   - `GET /deals/{city--state}`, `GET /professionals/{city--state}`, `GET /photos/{city--state}` → SSR category pages.
2. **`/websiteapi/{area}/{method}` plain-JSON POST (works with browser UA, NO signing headers):**
   - `POST /websiteapi/homepage/getupcominglivestreamclasses` → JSON list of livestream classes (BusinessID, ServiceID, ServiceTitle, provider name, price, capacity, times). **Confirmed replayable via raw curl.** Dates are OData `/Date(ms)/` format → use `cliutil.ParseODataDate`.
   - `POST /websiteapi/homepage/getallpetdetails`, `POST /websiteapi/homepage/getupcominglivestreamclasses` seen on homepage.
3. **`/WebServices/MySampleService.asmx/PageMethodsProxyJson?token={Method}` (AVOID):**
   - Envelope body `{"Data":"...","Token":"{Method}"}`. Methods seen: `GetMasterCategoryValues`, `GetAdminServices`, `getcitybylatlong`, `searchrelatedservice`.
   - **Carries per-request signing headers `h`, `val`, `k`, `i` (client-computed HMAC/signature).** Not replayable without reversing the JS crypto. **Do not depend on these** — the SSR HTML + `/websiteapi/` surface covers the same data.

## Auth
- Public discovery surface needs **no auth**.
- Authenticated surface (user's own bookings/profile) is **cookie-based** — the user is logged into Chrome. Implement via `auth login --chrome` cookie import (composed/cookie auth). Authenticated endpoints NOT yet sniffed (requires user session) — pending scope decision at absorb gate.
- **Booking** is a real mutation gated behind Stripe/Affirm payment — a payment side-effect, not a clean v1 endpoint. Ship as a safe deep-link ("prepare booking" → prints book-now URL), gated per side-effect rules.

## Top Workflows
1. **Cross-business availability search** — find open slots matching service + geo + date/price/rating across all nearby businesses (the killer feature; website has no marketplace-wide slot query).
2. Search businesses by service + location, ranked by rating/price/distance.
3. Browse a business's full service menu with prices, durations, providers.
4. Read reviews and compare businesses by rating.
5. Track deals / on-sale offers across nearby businesses.
6. Browse upcoming livestream classes.
7. (Auth) Manage own bookings / profile across the whole marketplace.

## Table Stakes
- Location-scoped search by service/category.
- Business profile: services, prices, staff, reviews, hours.
- Availability lookup.
- Deals, livestream classes, professionals directory.

## Data Layer
- **Primary entities:** Business/Merchant, Service, Provider/Staff, AvailabilitySlot, Review, Category, Location/Geo, Deal, LivestreamClass, (auth) Appointment.
- **Sync cursor:** location + service scoped snapshots (search results); business detail by slug.
- **FTS/search:** businesses (name, category, description, address), services (title, description), reviews.

## Competitors / Landscape
- Closest: **Fresha, Booksy, StyleSeat** (same consumer beauty/wellness discovery+booking). Mindbody has an official API but merchant/partner-side (ClassPass is its consumer arm).
- **None of them expose a consumer public API or ship a CLI.** A Vagaro consumer CLI is **first-of-kind** in the entire category.
- Apify scrapers actively return Vagaro business data (BusinessID, Name, Description, URL, Category, Address, Phone, Rating, Reviews) → confirms the search surface is scrapable at scale.

## Product Thesis
- **Name:** Vagaro CLI (`vagaro-pp-cli`)
- **Thesis:** The marketplace-wide availability engine Vagaro's website refuses to be. The site forces a per-business click-through funnel; this CLI fans out search + availability across every nearby business and answers *"find an available deep-tissue massage under $120 this Saturday afternoon near me, ranked by rating"* in one shot — with offline SQLite storage, `--json`/`--select` agent output, and cross-business price/rating comparison no Vagaro tool offers.

## Build Priorities
1. **P0 data layer:** businesses, services, providers, reviews, availability, deals, livestream classes in SQLite; sync from SSR/JSON-LD + `/websiteapi/`.
2. **P1 absorbed (table stakes):** search, business detail, services, reviews, deals, professionals, livestream classes — all replayable, no auth.
3. **P2 transcendence:** cross-business availability search, price/rating comparison, deal radar, slot watch, service-menu diff — the compound features that need everything in SQLite.
4. **Auth (scope decision):** cookie `auth login --chrome` for own-bookings/profile read; booking as safe deep-link.
