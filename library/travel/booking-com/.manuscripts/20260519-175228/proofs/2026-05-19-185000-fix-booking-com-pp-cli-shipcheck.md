# Shipcheck Report: booking-com-pp-cli

## Verdict: ship-with-gaps

All 6 shipcheck legs PASS. Scorecard 79/100 Grade B. Hotels search/get and trips list verified live against booking.com. Authenticated commands (trips, wishlist, rewards, account) routed to secure.booking.com via absolute URL support. 13 transcendence commands wired and respond to --help.

## Results

| Leg | Result | Notes |
|---|---|---|
| verify | PASS | Build green, vet clean |
| validate-narrative | PASS | 10 commands resolved, 0 failures |
| dogfood | PASS (WARN) | 13/13 novel features survived; 1 dead flag, 3 dead helpers (cosmetic) |
| workflow-verify | PASS | no manifest, skipped |
| verify-skill | PASS | SKILL claims align with shipped CLI |
| scorecard | PASS | 79/100 Grade B |

## Fixes applied in this loop

1. Renamed `search` resource to `hotels list` (framework `search` collision)
2. Renamed `profile` resource to `account` (framework `profile` collision)
3. Converted underscored aliases to kebab-case
4. Fixed narrative.quickstart `search --query` → `hotels list --query`
5. Replaced side-effectful `auth login --chrome` in narrative with `doctor --json`
6. Replaced `7d` duration with `168h` (Go time.Duration limitation)
7. Bypassed framework's `resolvePaginatedRead` for HTML hotels list (direct `c.Get`)
8. Added `domain := ".booking.com"` to auth login (generator emitted empty)
9. Routed cookie-shaped `Authorization: Bearer` to `Cookie:` header in client
10. Added absolute-URL support in client `doInternal` (`https://` prefix)
11. Patched sync.go path table to route trips/account/rewards to `secure.booking.com`

## Live smoke evidence (run 2026-05-19)

- `hotels list --query Paris --checkin 2026-06-20 --checkout 2026-06-23 --adults 2 --json` returns 25 real Paris hotels with prices, ratings, slugs, photo URLs, distance from center, and detail URLs.
- `hotels get fr auliviaopera --json` returns Hotel Aulivia Opéra detail (name, address, postal, score 8.1, review count 1032, hotel type, stars from JSON-LD).
- `attractions search fr paris --json` returns Paris activities (Disneyland Paris, Seine cruise, Musée d'Orsay, etc.).
- `trips list --json` reaches secure.booking.com, returns empty array (user has no Global trips).
- `compare auliviaopera comforthotelsaintmartin` returns paired struct.
- `doctor` reports auth configured (browser session, 5 cookies).

## Known gaps shipping in v0.1

1. **Trips/wishlist populated-state parsing is inferred.** User's account had empty trips and wishlist during browser-sniff, so PropertyCard/Trip/WishlistItem selectors for populated cards were derived from documentation and the empty-state markup. First user with real trips may surface selector misses.

2. **Hotel detail parser over-extracts amenities.** `hotels get` and `compare` pull breadcrumb/navigation items (Stays, Flights, Car rental, etc.) into the `amenities` array alongside real amenities. Parser needs a more selective DOM query than `[data-testid="property-most-popular-facilities-wrapper"] li` provides.

3. **Hotel detail lat/lon = 0.** JSON-LD `geo.latitude`/`geo.longitude` not always populated by booking.com. Parser falls back gracefully to zero.

4. **Attractions parser review_score = -20 sentinel.** Parser is catching the wrong numeric (likely a layout class id). Needs selector refinement.

5. **`/dml/graphql` POST integration deferred.** Map markers + paginated reviews use the internal GraphQL endpoint with CSRF token. The CSRF extraction is documented; the actual POST is not yet wired. `map markers` command exists but returns empty until the GraphQL POST path is built.

6. **price_history requires sustained CLI use.** The `prices cheapest`, `wishlist drops`, `destinations price-band`, and `watch run` commands depend on a populated `price_history` SQLite table that grows as the user runs searches over time. First-run experience may return empty until repeat use populates the table.

7. **`cars list` is read-only landing.** Booking.com cars (Rentalcars-powered) uses a self-posting search form, not deep-link URLs. The `cars list` command exposes the landing-page supplier list and city pickup pages; live car-rental search would need a token-exchange flow not built for v0.1.

8. **Genius/mobile-rate diff commands fetch twice.** `genius impact` and `deals mobile-rates` make two parallel search calls. Behavior depends on booking.com honoring the auth state difference (Genius rates may already be in the default response when authenticated).

## Verdict rationale

`ship-with-gaps`: shipcheck 6/6 PASS, scorecard 79/100 above the 65 threshold, headline command (hotels list) verified working end-to-end against real booking.com data. Gaps are documented inline above; none block the core "search Booking.com hotels" use case.
