# AllTrails Route Map

Checked: 2026-05-26

## Evidence Labels

- `official_export`: documented user-facing export/import surface.
- `browser_observed`: seen in a live browser capture.
- `community_inferred`: found in community code/research.
- `subscription_gated`: likely Pro/Plus-gated.
- `mutation_unverified`: live-capable write route, but method/body/schema still need current authenticated capture.
- `unverified`: expected route family still needing authenticated capture.

## Seed Routes

| Method | Route | Mutation | Labels | Notes |
|---|---|---|---|---|
| GET | `/api/alltrails/v3/trails/{trail_id}?detail=offline` | no | community_inferred, browser_observed | Useful trail geometry/detail payload; confirmed by community projects. |
| GET | `/api/alltrails/v3/trails/{trail_id}/static_map` | no | browser_observed | Seen nested inside AllTrails image proxy requests. |
| GET | `/api/alltrails/v3/maps/{map_id}/static_map` | no | browser_observed, subscription_gated | Seen nested inside AllTrails image proxy requests. |
| GET | `/api/alltrails/v3/trails/search` | no | unverified | Search by query/location/filter; needs authenticated capture. |
| GET | `/api/alltrails/v3/trails/{trail_id}/reviews` | no | unverified | Trail review list. |
| GET | `/api/alltrails/v3/trails/{trail_id}/photos` | no | unverified | Trail photo list. |
| GET | `/api/alltrails/v3/trails/{trail_id}/recordings` | no | unverified | Community GPS recordings. |
| GET | `/api/alltrails/v3/trails/{trail_id}/gpx` | no | subscription_gated, unverified | GPX export/download surface. |
| GET | `/api/alltrails/v3/trails/{trail_id}/elevation_profile` | no | unverified | Elevation surface. |
| GET | `/api/alltrails/v3/user/me` | no | unverified | Current profile/account. |
| GET | `/api/alltrails/v3/user/{user_id}/activities` | no | unverified | Activity list. |
| GET | `/api/alltrails/v3/user/{user_id}/lists` | no | unverified | Saved/custom lists. |
| GET | `/api/alltrails/v3/user/{user_id}/favorites` | no | unverified | Favorited trails. |
| GET | `/api/alltrails/v3/user/{user_id}/completed` | no | unverified | Completed trails. |
| GET | `/api/alltrails/v3/activities/{activity_id}` | no | unverified | Activity detail. |
| POST | `/api/alltrails/v3/activities/upload` | yes | mutation_unverified, unverified | Upload GPX/FIT activity; personal package executes live unless `--dry-run` is used. |
| GET | `/api/alltrails/v3/maps/offline/{region}` | no | subscription_gated, unverified | Offline map metadata. |
| GET | `/api/alltrails/v3/maps/3d/{trail_id}` | no | subscription_gated, unverified | 3D map tile URLs. |
| GET | `/api/alltrails/v3/weather/{lat}/{lng}` | no | subscription_gated, unverified | Weather overlay. |
| GET | `/api/alltrails/v3/lists/{list_id}` | no | unverified | Curated/custom list detail. |
| GET | `/api/alltrails/v3/parks/{park_id}` | no | unverified | Park detail. |
| GET | `/api/alltrails/v3/regions/{slug}` | no | unverified | Region page detail. |
| GET | `/api/alltrails/v3/friends` | no | unverified | Social graph. |
| GET | `/api/alltrails/v3/feed` | no | unverified | Account feed. |
| POST | `/api/alltrails/v3/follow/{user_id}` | yes | mutation_unverified, unverified | Follow user; personal package executes live unless `--dry-run` is used. |
| DELETE | `/api/alltrails/v3/follow/{user_id}` | yes | mutation_unverified, unverified | Unfollow user; personal package executes live unless `--dry-run` is used. |

## Browser Route Families

Public trail pages expose route families for `/trail/...`, `/parks/...`, `/poi/...`, `/members/...`, `/explore/recording/...`, `/plans`, `/plus`, `/explore/custom-routes/new`, directories, and support/legal pages.

## 2026-05-26 Authenticated CDP Capture

Sanitized Chrome CDP proof files:

- `alltrails/proofs/cdp-alltrails-california-sanitized-2026-05-26.json`
- `alltrails/proofs/cdp-alltrails-account-sanitized-2026-05-26.json`

| Method | Route | Mutation | Evidence | Notes |
|---|---|---|---|---|
| POST | `/api/alltrails/explore/v1/search` | no | browser_observed | Explore and custom-route search endpoint observed during logged-in California/explore capture. POST is read-only search in this context. |
| GET | `/api/alltrails/me` | no | browser_observed | Current authenticated user route observed from the logged-in browser session. |
| GET | `/api/alltrails/geo-service/routing/info` | no | browser_observed | Routing service metadata used by explore/map surfaces. |
| GET | `/api/alltrails/trails/{trail_id}/surface_types` | no | browser_observed | Trail surface-type metadata observed from trail detail pages. |
| GET | `/api/alltrails/v2/trails/{trail_id}/photos` | no | browser_observed | Trail photo list v2 endpoint observed from trail detail pages. |
| GET | `/api/alltrails/trails/{trail_id}/photos/{photo_id}` | no | browser_observed | Trail photo detail/list endpoint observed with landscape and limit query keys. |
| GET | `/api/alltrails/weather-service/v2/trails/{trail_id}/overview` | no | browser_observed | Trail weather overview endpoint observed from detail pages. |
| GET | `/api/alltrails/community/blazes/v0/users/{user_id}/feeds` | no | browser_observed | Authenticated user community feed endpoint observed from /members/me. |
| GET | `/api/alltrails/community/blazes/v0/users/{user_id}/feeds/public` | no | browser_observed | Public feed route for a user; observed with requested feed-item version query keys. |
| GET | `/api/alltrails/community/blazes//v0/users/{user_id}/feeds/public` | no | browser_observed | Paged public feed variant observed exactly with a double slash before v0. |
| GET | `/api/alltrails/connect/users/{user_id}/connections` | no | browser_observed | Connection list route observed from member/account pages. |
| GET | `/api/alltrails/connect/users/{user_id}/connections/{connection_id}` | no | browser_observed | Single connection detail route observed from member/account pages. |
| GET | `/api/alltrails/connect/v0/my/mutuals/{user_id}` | no | browser_observed | Authenticated mutual-connection route observed from member/account pages. |
| GET | `/api/alltrails/users/{user_id}/lists` | no | browser_observed | Observed current/user lists endpoint from logged-in member pages. |
| GET | `/api/alltrails/users/{user_id}/maps` | no | browser_observed | Observed current/user maps endpoint from logged-in member pages. |
| GET | `/api/alltrails/users/{user_id}/trails/completed` | no | browser_observed | Observed completed trails endpoint from /members/me/completed. |
