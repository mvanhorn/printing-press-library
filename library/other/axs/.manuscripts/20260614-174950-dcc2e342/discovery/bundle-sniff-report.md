# SRAM AXS — Client-Bundle Discovery Report

**Method:** Static analysis of the official web client (`https://axs.sram.com/assets/index-Dj88THjE.js`, ~5 MB Vite/React bundle). No live browser-sniff backend and no credentials available in this environment, so the API surface was reverse-engineered from the app's own call sites. This is high-confidence evidence — these are the exact endpoints the shipping web app calls.

## Hosts & base URLs (from bundle constants)
- `cz = https://api.axs.sram.com` → `/ble-service/api/v1/`, `/firmware-service/`
- `uz = https://nexus.quarqnet.com/`, `_4 = ${uz}api/v2/` → **primary account/device API**
- `lt1 = api.quarqnet.com/api/v2/`, `Va = https://${lt1}` → **activity/telemetry API**
- `Fx = https://brodo.quarqnet.com/` → `manual/fit`, `thirdparty/poll`, `v2/backfill` (FIT-file ingest)
- `api.quarqrace.com/api/v1/` → Quarq race activities/component summaries

## Auth (Auth0 password-realm)
- Domain: `sramid-auth.sram.com`
- Token endpoint: `https://sramid-auth.sram.com/oauth/token`
- clientID: `zIvfleoh46jy4behzZdkFoUIiW70KX23` (public SPA client, baked in bundle)
- realm/connection: `sramid-db`
- grant_type: `http://auth0.com/oauth/grant-type/password-realm`
- audience: `https://api.quarqnet.com`
- scope: `openid profile email offline_access`
- Runtime: `Authorization: Bearer <access_token>` on every API call.

## nexus.quarqnet.com/api/v2/  (account & devices)
| Path | Verbs (observed) | Notes |
|---|---|---|
| `users/self/` | GET | current user profile |
| `users/self/flags/` | GET | feature flags |
| `users/self/public_accessgroups/` | GET | access groups |
| `users/self/export/` | POST | GDPR data export request |
| `users/{id}/` | GET | user by id |
| `users/changeemail/` | POST | change email |
| `bikes/` | GET, POST(multipart) | registered bikes |
| `components/` | GET, POST | AXS components |
| `componentregistrations/` | POST `{email, serial}` | register a component by serial |
| `devicetypes/` | GET | device-type catalog |
| `models/` | GET | model/firmware catalog |
| `productdetails/{id}/` | GET | product detail |
| `advancedunits/` | GET (paginated `page`/`page_size`) | measurement units |
| `notifications/` | GET | notifications inbox |
| `communicationprefs/` | GET | comms preferences |
| `messages/` | POST `{users:[id]}` | messaging |
| `linkedids/` | GET, GET `?sync=true`, POST `{access_token}` | linked 3rd-party accounts |
| `linkedids/{id}/` | DELETE | unlink |

## api.quarqnet.com/api/v2/  (activity & telemetry)
| Path | Verbs | Notes |
|---|---|---|
| `activities/` | GET | ride activities |
| `activitysummaries/` | GET | per-activity summaries |
| `activitytypes/` | GET | activity-type catalog |
| `componentsummaries/` | GET | component usage stats (wear, shift counts) |
| `users/self/stats/` | GET | aggregate user stats |

## api.axs.sram.com  (BLE & firmware)
- `/ble-service/api/v1/bleservices/3_1/{id}/` — BLE service metadata
- `/firmware-service/` — firmware service (model/version lookups)

## Pagination
Django REST Framework convention: `?page=N&page_size=M`, total pages surfaced via response headers/`count`.

## Reachability
- `api.axs.sram.com` → 200/404 (Go backend, reachable).
- nexus/api quarqnet endpoints require a valid Bearer token; unauthenticated calls return 401 (expected).
- No public OpenAPI spec exists. Bundle is the authoritative source.
