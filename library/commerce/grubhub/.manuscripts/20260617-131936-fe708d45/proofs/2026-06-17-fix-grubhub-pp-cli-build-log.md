Manifest transcendence rows: 4 planned, 4 built. Phase 3 will not pass until all 4 ship.

# Grubhub CLI Build Log

## Architecture
Reverse-engineered Grubhub web API (api-gtm.grubhub.com). Auth is a runtime-minted
anonymous bearer (no user key): a sibling package `internal/grubhub` scrapes a live
client_id from grubhub.com and exchanges it at /auth, caching the token in config.
The generated client auto-authenticates once the token is in config.

## Built — absorbed (friendly, hand-written, auto-mint + auto-geocode)
- `near <address>` — live restaurant search by address (the friendly "search"; named to avoid the framework `search` offline-FTS command). Flags: --cuisine, --pickup, --sort, --open-now, --limit.
- `menu <restaurant-id>` — full menu with prices (build-your-own price-variation fallback). Flags: --category, --popular, --limit.
- `item <restaurant-id> <item-id>` — menu item with modifier/choice categories.
- `geocode <address>` — address → lat/lng/POINT (replaces the generated `geo` command, whose array-response envelope extraction returned empty — see Generator limitations).
- `auth login` — credential-free anonymous token mint (for the raw endpoint surface).

## Built — transcendence (hand-written, all 4 shipping-scope rows)
- `compare <address>` — sortable fee/min/ETA/rating/distance board. Flags: --sort, --max-fee, --max-min, --eta-under, --cuisine, --pickup, --limit.
- `dish <address> <query>` — cross-restaurant menu-item search (FTS-style), scan-and-filter fan-out with --max-scan-restaurants cap, name-first relevance ranking, partial-failure accounting, local SQLite cache, and --data-source auto|local|live.
- `deals <address>` — cross-restaurant deal radar ranked by offer value. Flags: --sort (value|count), --pickup, --limit.
- `pick <address>` — best-value picker with transparent normalized score + breakdown. Flags: --weight-fee/-eta/-rating/-deal, --cuisine, --limit.

## Raw surface (generated, hidden, token-gated)
- `restaurants search|get|menu-item` — raw typed endpoint commands (work after `auth login` or with GRUBHUB_TOKEN).

## Deferred (documented gaps, not stubbed)
- Order history / re-order (`/diners/{ud_id}/search_listing`) and cart/checkout — require a credentialed account login; out of v1 scope (read-only marketplace browsing).

## Tests
- `internal/grubhub/parse_test.go` — ParseSearch/Geocode/Menu/Item, price-variation fallback, Dollars, FormatPoint.
- `internal/cli/{compare,deals,dish,pick}_test.go` — sortRows, filterComparison, dealRowsFromCards, matchDishes (name-first relevance, negative cases), scorePicks (normalization + weighting).

## Generator limitations found (retro candidates)
- The promoted single-endpoint command for an ARRAY response (`geo geocode`) wrapped the body in `{meta, results}` and extracted `results: []` (empty) even though the API returned a populated array. Object-response endpoint commands (`restaurants get/search`) wrap correctly. Worked around by hand-writing `geocode`.
- Novel-feature scaffolds were emitted as `fmt.Errorf("TODO: implement ...")` stubs (expected) — filled in.
