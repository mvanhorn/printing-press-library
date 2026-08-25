# Forkable Browser-Sniff Discovery Report

## Source
- Authenticated manual HAR capture of `forkable.com/mc/` (Vue SPA), user-provided.
- 319 total entries; 150 to `forkable.com`; 54 GraphQL POSTs.

## API shape
- **GraphQL over HTTP.** All data flows through `POST https://forkable.com/api/v2/graphql`.
- CSRF handshake: `GET https://forkable.com/api/v2/csrf_token` → `{"token": "..."}`, echoed back as `x-csrf-token` header on GraphQL POSTs.
- Base URL: `https://forkable.com`

## Auth (session_handshake / cookie)
- Session cookie (HttpOnly) required. Replay test: unauthenticated `me` query → `HTTP 401`.
- `x-csrf-token` header required (fetched from `/api/v2/csrf_token`).
- Runtime: Chrome cookie import (`auth login --chrome`) + per-run CSRF fetch + session replay.
- Cookie-replay validation (Step 2d): **401 without cookies** → auth required → ship browser auth.

## Read operations captured (9) — all returned real data, no errors
| Operation | Args | Returns |
|---|---|---|
| `me` | none | current user: id, name, email, roles, settings, likes, dislikes, restrictions, addressInfo, companies, allManagedClubs, availableDelegations, permissions |
| `menus` | `ids: Int, clubId: Int` | menu with venue, optionSets, sections{items{modifiers,ratings,dietLevel}} |
| `myDeliveries` | `from: "YYYY-MM-DD"` | deliveries: state, forDeliveryAt, deliveryWindow, address, orders[]{total,state,menu,mealGroups,tally}, club, userReceipt |
| `myInProgressDeliveryIds` | none | list of delivery IDs in progress |
| `myBuffetAddresses` | none | buffet delivery addresses: street, city, postalCode, lat/lng, mealClubId |
| `mealClubsAs` | `roles: [String]` | meal clubs: id, name, market, deliveryAddress, deliveryDays, userRoles, allowance settings |
| `mealGenerationScores` | `deliveryId: Int, userId: Int, menuIds: [Int]` | auto-selection scores: menuId, itemId, score |
| `venueUsage` | `ids: [Int], from, to` | per-venue usage keyed by venue id |
| `myNotifications` | none | notifications list |

## Excluded (read-only CLI decision)
- `markNotificationAsViewed` [mutation]
- `addTopRatedMealsImpression` [mutation]

## Replayability
- Fully replayable via direct HTTPS (`standard_http`, no bot protection) once the session cookie + CSRF token are supplied. No browser sidecar needed at runtime.

## Auto-converter note
- `cli-printing-press browser-sniff` collapsed all GraphQL POSTs into 4 generic endpoints (expected for GraphQL — one URL, many operations). Spec is hand-authored from the operations above instead.
