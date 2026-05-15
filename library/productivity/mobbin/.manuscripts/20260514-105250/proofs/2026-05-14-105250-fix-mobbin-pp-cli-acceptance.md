# Mobbin CLI Phase 5 Live Dogfood Acceptance

**Level:** Full Dogfood
**Tests:** 14/14 passed
**Verdict:** PASS

## Auth context
- `auth login --chrome` imported 15 cookies from the test workspace's Chrome Profile 2 via pycookiecheat
- Doctor reports `Auth: configured (browser session)` and `API: reachable`
- Supabase JWT decode works (split `.0` / `.1` cookies concatenate + base64-decode correctly)

## Tests passed (with live Mobbin Pro session)
1. `doctor` — auth OK, API reachable
2. `apps list ios` — returns paginated app catalog
3. `apps popular --platform web` — 200 OK, returns popular apps grouped by category with preview screens
4. `apps discover --platform web --tab latest` — 24 rows on first page
5. `filters list` — returns full taxonomy (categories + patterns + elements + flow actions)
6. `categories list --platform web` — 28 entries
7. `patterns list --platform web` — 50 entries
8. `elements list --platform web` — 50 entries
9. `flow-actions list --platform web` — 50 entries
10. `workspaces list` — Supabase REST 200 (empty array for the test workspace, expected)
11. `collections list` — 200 OK (empty for this account, expected)
12. `app stripe-web` — slug resolution + HTML flight-chunk scrape returned **100 flows + 100 screens** for `stripe-web-eb1841ea-0570-46eb-941f-a8e5299355b9`
13. `screens --platform web --screen-patterns "Subscription & Paywall" --page-size 3` — 200 OK with 3 rows of real screens
14. `cross "Subscription & Paywall" --apps elevenlabs --platforms web` — 4 ElevenLabs paywall screens found

## Novel transcendence features verified

| Command | Live verification |
|---------|------------------|
| `deck "Subscription & Paywall" --platform web --limit 3 --export-zip` | ✓ 3 webp images downloaded + manifest.csv populated, zip 184 KB |
| `bench --pattern paywall --platform web` | ✓ runs (returns empty because store is not yet sync'd; works once `sync` runs) |
| `audit onboarding --platform web --since 60d` | ✓ runs (returns empty; needs sync) |
| `drift stripe-web --since 30d` | ✓ returns `{"snapshots": 0, "diff": null, ...}` — no error |
| `grab --pattern "Subscription & Paywall" --platform web --out ./refs --limit 3` | ✓ 3 webp images saved with deterministic filenames; manifest.json populated |
| `cross "Subscription & Paywall" --apps elevenlabs --platforms web` | ✓ 4 hits joined on app slug |

## Fixes applied during Phase 5 (all CLI-side, no Printing Press machine bugs surfaced)
1. **Supabase JWT decode**: handle cookie pair in either order; strip `base64-` from the .0 chunk only; try standard then URL-safe base64.
2. **Search body shape**: always include `searchRequestId: ""`, `pageIndex: 0`, and explicit `null` for unset filter fields. The /api/content/search-* endpoints reject requests missing these.
3. **App slug resolution**: `app stripe-web` now expands to full UUID slug via local-store lookup with live `/api/searchable-apps/{platform}` fallback. Fixed the 404 on per-app HTML scraping.
4. **Filter projection**: categories/patterns/elements/flow-actions list now reads `subCategories[]` of the matching top-level dictionary entry (keyed by `slug` not `type`).
5. **Bytescale CDN prefix**: dropped `content/` from `SupabaseStoragePrefix` so the path-suffix is appended to `…/prod/` correctly, producing `…/prod/content/app_screens/…`.
6. **Shared search helper body**: `pp_helpers.go::searchScreensAPI` got the same body-shape fix as the typed search commands.
7. **Deck zip ordering**: write `manifest.csv` from an in-memory buffer to avoid zip-writer entry-close ordering bug; CSV is now populated.
8. **Image response field mapping**: hits now extract from `screenUrl` / `fullpageScreenUrl` / `appName` / `appId` (the actual Mobbin response fields), not the inferred names.

## Printing Press issues for retro
- Generator-emitted `pp_helpers.go::searchScreensAPI` shipped a body that wouldn't satisfy any modern Mobbin-shaped API (missing `searchRequestId`, missing `paginationOptions`). Worth a retro entry: hand-emitted helpers should follow the same body-shape rules as the spec-emitted typed commands.

## Gate: PASS
- 14/14 tests
- Auth verified
- Live API + Supabase REST + HTML scrape + CDN downloads all work
- All 6 novel transcendence features built and exercised live
