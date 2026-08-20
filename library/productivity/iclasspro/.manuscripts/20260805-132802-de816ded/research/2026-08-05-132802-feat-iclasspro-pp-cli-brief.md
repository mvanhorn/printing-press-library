# iClassPro CLI Brief

## API Identity

- **Domain:** Class/enrollment management SaaS for youth activity businesses — gymnastics, swim, dance, cheer, martial arts, tumbling. Thousands of tenants in the US/CA/AU.
- **Users:**
  - Parents/customers browsing and enrolling (public portal + customer login)
  - Gym owners / front-desk staff (staff dashboard — separate surface, staff login)
  - Web agencies and developers embedding class schedules on gym websites (iClassPro publicly states there is **no public API and none planned**, so this segment is entirely underserved)
  - Multi-location operators and franchise groups aggregating across gyms
- **Data profile:** Multi-tenant, keyed by **portal slug** (`scaq`, `scottsdalegymnastics`, `oasisgymnastics`, `tigar`, `aerials` all verified live). Per tenant: locations → booking menu → programs → classes/camps → sessions. Catalog is slow-changing; **openings, waitlist state, and registration windows change constantly and are never exposed historically**.

### Surfaces discovered (verified live, 2026-08-05)

| Surface | Base | Auth | Status |
|---|---|---|---|
| **Open API** | `https://app.iclasspro.com/api/open/v1/{slug}` | **none** | Live, plain `curl` 200s, no bot protection |
| **JWT API** (customer portal) | `https://app.iclasspro.com/api/jwt/v1` | email+password → `access_token` (query param `token`) | Live; cart/enroll/checkout |
| **Staff/business API** | `https://api.iclasspro.com` | AWS API Gateway → `{"message":"Missing Authentication Token"}` | Not publicly documented, no self-service keys |

### Open API routes confirmed against `scaq` and `scottsdalegymnastics`

| Route | Result |
|---|---|
| `GET /{slug}/locations` | 200 — id, name, email, phone, address, brand colors, logos |
| `GET /{slug}/bookings/{locationId}` | 200 — **authoritative typeId source** for camps |
| `GET /{slug}/class-programs/{locationId}` | 200 — program id/name |
| `GET /{slug}/classes` | 200 — `totalRecords` + paginated `data[]` |
| `GET /{slug}/classes/{classId}` | 200 — adds HTML `description` |
| `GET /{slug}/camps?locationId&typeId&limit&page&sortBy` | 200 — `campTypeName`, `totalRecords`, events |
| `GET /{slug}/camps/{campId}` | 200 (per community ref) — description, blocks, instructors, room |
| `GET /{slug}/camp-programs/{locationId}` | 200 — programIds (**not** typeIds) |
| `GET /{slug}/appointment-programs/{locationId}` | 200 (plan-gated) |
| `GET /{slug}/sessions` | 200 — session id/name/startDate |
| `GET /{slug}/appointments` | 400 when the tenant lacks the subscription plan |
| `/{slug}/organizations`, `/parties`, `/party-programs`, `/students`, `/enrollments`, `/waitlist`, `/search` | **404** — either non-existent or differently named; the booking menu advertises `target: "parties"`, so a parties route exists under an unknown path → browser-sniff target |

### JWT (customer) routes from community source

`POST /login` · `GET /count-cart` · `DELETE /clear-cart` · `GET /locations` · `GET /family-payment-method` · `GET /new-cart-item/{classId}/{sessionId}` · `POST /validate-cart-item` · `POST /add-cart-item` · `POST /add-promo-code` · `GET /validate-cart/{locationId}` · `POST /process-cart/{locationId}` (**real payment**)

## Reachability Risk

- **None / Low.** Direct `curl` returns 200 JSON with no UA/Referer requirement, no Cloudflare, no rate-limit headers observed. Verified across 5 independent tenants.
- **Stability risk: Medium-High (documented, not blocking).** iClassPro's own FAQ says there is no API for the website and none is planned; there is no public OAuth or API-key program. `/api/open/v1` is the portal's internal contract — versioned, but it can change without a changelog. The CLI must fail loudly on shape drift rather than silently returning empty.
- Tier/permission hints from 4xx body: `{"message":"Appointment subscription plan expired. Please contact Admin."}` (400 on `/appointments`) — per-tenant plan gating is surfaced in the body, not the status code.
- Probe-safe endpoint used: `GET /{slug}/locations`.

### Silent-filter footgun (verified, drives a differentiating feature)

`GET /{slug}/classes` **honors** `locationId`, `programs`, `days`, `sessions`, `limit`, `page`, `instructors` (ID-typed) and **silently ignores** everything else, returning the full unfiltered set with an unchanged `totalRecords`:

| Param | 27 total → result |
|---|---|
| `programs=57` | 25 ✅ honored |
| `days=5` | 11 ✅ honored |
| `sessions=1380` | 5 ✅ honored |
| `dayIds=5` | 27 ❌ ignored |
| `hideFullClasses=1` / `true` | 27 ❌ ignored |
| `search=Culver` / `name=Culver` | 27 ❌ ignored |
| `ageYear=8` / `minAge=8` | 27 ❌ ignored |
| `openings=1` | 27 ❌ ignored |
| `instructors=<name>` | 0 — expects an ID, not a name |

A naive wrapper that passes user filters straight through returns *wrong answers that look right*. Our CLI must split server-honored params from locally-applied predicates and label which is which.

## Reachability Gate (Phase 1.9)

- **Decision: PASS.**
- `GET /api/open/v1/scottsdalegymnastics/locations` → **200 application/json** via bare `curl` (no UA, no Referer, no cookie).
- `cli-printing-press probe-reachability` → `mode: standard_http`, confidence **0.95**; stdlib probe 200 in 581 ms, surf-chrome probe 200 in 507 ms; `needs_browser_capture: false`, `needs_clearance_cookie: false`.
- Browser capture: 13/13 entries HTTP 200, zero challenge sentinels.
- The analyzer's initial `browser_required` verdict was a false positive on the org-settings field `recaptchaPublic` and was corrected to `standard_http`; see `discovery/browser-sniff-report.md` §5.
- Probe-safe endpoint used: `GET /{account}/locations`. No mutation endpoint was probed — none is declared `x-pp-safe-probe`, and none is in scope.
- **Runtime shape: standard HTTP.**

## Top Workflows

1. **"Is there space?"** — watch `openings` / `allowWaitlist` / `futureOpenings` for a class or a whole program and know the moment a spot frees up. The API has no history and no notification.
2. **Registration-window watching** — `registrationStartDate` / `registrationEndDate` / `campRegisterExpired` decide whether a family can even try to book. Knowing "registration opens in 3 days at 9am" is the whole game for popular camps.
3. **Multi-tenant / multi-location catalog aggregation** — an agency or franchise pulls the full class + camp catalog for N gyms into one queryable place (the exact thing `Jaymelynng/master-events-calendar` built by hand for 10 gyms).
4. **Event/camp calendar extraction** — camps with full HTML descriptions, dates, ages, images → marketing calendars, website widgets, feeds.
5. **Parent-side enroll** — login → find class → cart → validate → checkout, scriptable and dry-runnable.

## Table Stakes

There is **no incumbent**: no official SDK, no CLI, no MCP server, no PyPI package, one unrelated npm package (`@iclasspro/icp-tinymce-variable`). Table stakes are therefore defined by what the three community projects do by hand:

- Fetch and paginate `classes` for an account, with `programs` filter (`DevCabin/icp-widget`)
- Discover locations → typeIds → camps → camp detail, paginating on `totalRecords` (`Jaymelynng/master-events-calendar`)
- Use `bookings/{locationId}` for typeIds, **never** `camp-programs` (documented trap: `camps?typeId=246` → "No camps found")
- Openings / waitlist / full status display and registration-URL construction (`portal.iclasspro.com/{slug}/class-details/{id}`)
- Strip HTML from descriptions; prefix relative images with `https://app.iclasspro.com/media/`
- JWT login + cart + promo code + validate + checkout (`johnmarcovici/iclasspro-driver`)
- Retry with backoff on transient failure

## Data Layer

- **Primary entities:** `locations`, `booking_menu` (typeId ↔ title), `class_programs`, `classes`, `class_details`, `sessions`, `camp_programs`, `camp_types`, `camps`, `camp_details`, `appointment_programs`. Tenant slug is part of every key — the store is multi-tenant by design.
- **Derived/time-series:** `openings_history` (slug, location, class/camp id, observed_at, openings, futureOpenings, waitlist flag) — the single most valuable table, because the upstream API is stateless-present-tense only.
- **Sync cursor:** none upstream (no `updatedAt`, no ETag). Sync is full-refresh per tenant/location with content hashing to detect change; `openings_history` appends on every sync.
- **FTS/search:** class/camp `name` + `description` + `programName` + `instructors` across every synced tenant. The upstream API has **no search parameter at all** (`search=` is silently ignored), so offline FTS is strictly more capable than the live portal.

## Codebase Intelligence

Read directly from source (higher fidelity than a DeepWiki summary; these are small single-file personal repos with no wiki index):

- `johnmarcovici/iclasspro-driver` — `iclasspro_jwt.py` (954 lines): JWT auth is `POST /api/jwt/v1/login` → `access_token`, then **token passed as a query param** (not a bearer header) on every call, with `Origin: https://portal.iclasspro.com` and `Referer: https://portal.iclasspro.com/{portal}/`. Cart flow is `new-cart-item/{classId}/{sessionId}` → `validate-cart-item` → `add-cart-item` → `add-promo-code` → `validate-cart/{loc}` → `process-cart/{loc}`.
- `DevCabin/icp-widget` — `icpClient.js`: Open API needs no auth; sends browser-ish `Accept`/`Referer`/`Origin` headers (not required in practice — bare `curl` works), 3× retry with 1s delay.
- `Jaymelynng/master-events-calendarMASTER` — `docs/TECHNICAL/ICLASS_DIRECT_API_REFERENCE.md`: the typeId-vs-programId trap, the four-step sync flow, per-gym typeId variance, and a table of fields the API exposes but nobody captures (`timezone`, `contactEmail`, `roomName`, `instructors`, `blocks`, `programIsDeleted`, `campRegisterExpired`, `allowToRequestCampThatIsFull`).

## Ecosystem Scan

| Channel | Result |
|---|---|
| Official SDK / docs | None. Vendor states no public API. |
| CLI | None. |
| MCP server | None (GitHub search: 0). |
| npm | 0 relevant (1 unrelated vendor package). |
| PyPI | 0 (`iclasspro`, `iclass-pro`, `iclasspro-api` all 404). |
| Claude plugin / skill | None. |
| Community repos | 3 (all 0★, all hand-rolled, all partial). |

## Product Thesis

- **Name:** `iclasspro-pp-cli`
- **Why it should exist:** iClassPro powers thousands of gyms and swim schools and ships **zero** developer surface — the vendor says so explicitly. Every agency, every multi-gym operator, and every parent chasing a full class re-derives the same undocumented portal API by hand, and three separate GitHub repos prove it. This is the first tool to treat that surface as an API: multi-tenant, offline-first, honest about which filters the server actually honors, and — because it keeps a local store — able to answer the questions the upstream API structurally cannot: *did openings change*, *when does registration open*, *how does my gym compare to the one across town*, *what got deleted*.

## Build Priorities

1. Data layer + sync for all Open API entities across multiple tenants, with `openings_history` append-on-sync.
2. Full Open API command surface (locations, booking menu, programs, classes, class detail, camps by type, camp detail, sessions, appointment programs) with honest server-vs-local filter split.
3. Offline FTS search across every synced tenant — the capability the upstream API has none of.
4. Transcendence: openings/registration watching, drift detection, multi-tenant comparison, calendar/feed export.
5. JWT customer surface (login, cart, enroll) — gated behind explicit opt-in because `process-cart` moves real money.
