# iClassPro Browser-Sniff Discovery Report

**Run:** `20260805-132802-de816ded` · **Captured:** 2026-08-05 · **Backend:** browser-use v0.12.9 (CLI mode, headless, anonymous)
**Target:** `https://portal.iclasspro.com/scottsdalegymnastics/`

Tenant chosen because it is the one verified-open account with **classes, camps AND parties** all enabled. The user's own tenant (`nadoclub`) gates its catalog behind customer sign-in, so it could not expose the public route surface.

## 1. User Goal Flow

**Goal:** *Find a class or camp with space at a gym, and know when registration opens.*

| # | Step | API operations triggered |
|---|---|---|
| 1 | Load portal landing | `bookings/{loc}`, `count-cart-detailed/{loc}/{device}`, `POST jwt/v1/organizations` |
| 2 | Click **Booking** | (client-side route; booking menu already cached) |
| 3 | Click **Find a Class** | `levels/active/{loc}`, `class-programs/{loc}`, `instructors/{loc}/classes`, `classes?locationId&limit&page`, `sessions` |
| 4 | Type `ninja` into the search box | `classes?locationId=1&q=ninja&limit=24&page=1` ← **revealed the `q` search param** |
| 5 | Open the Filters modal, toggle facets | modal rendered no visible inputs in the 800×600 headless viewport — **step incomplete** |
| 6 | Click **Book a Party** | `parties/create/{loc}` ← **the previously-unlocatable parties route** |
| 7 | Click camp category **OPEN GYM** | `camp-programs/{loc}`, `camps?locationId&typeId&limit&page&sortBy` |
| 8 | Click a camp card | `camps/{campId}` |

**Coverage:** 7 of 8 planned steps completed. Step 5 (filter facets) failed in-browser and was **recovered by direct empirical probing** against the live API — see §5, which produced a stronger result than the UI walk would have (exact honored-vs-ignored classification, not just the names the UI happens to emit).

**Secondary flows attempted:** JS-bundle extraction (`main.42718f6647a8fe3a.js`, 4.7 MB) — see §10.

## 2. Pages & Interactions

| URL | Interaction |
|---|---|
| `/scottsdalegymnastics` | initial load; installed fetch + XHR body interceptor |
| `/scottsdalegymnastics/booking` | clicked `Booking` nav link (SPA click, interceptor preserved) |
| `/scottsdalegymnastics/classes` | clicked `Find a Class` tile |
| `/scottsdalegymnastics/classes` | set `input[name=searchTerm]` to `ninja` via native setter + `input`/`change`/`keyup` events |
| `/scottsdalegymnastics/classes` | clicked `Filters`, attempted facet toggles (no visible inputs) |
| `/scottsdalegymnastics/party-booking-01-date` | clicked `Book a Party` |
| `/scottsdalegymnastics/camps/5?sortBy=time` | clicked `OPEN GYM` category tile |
| `/scottsdalegymnastics/camp-details/2035` | clicked first camp card heading |

All navigation after interceptor installation used SPA clicks; `browser-use open` was used only for the initial load, per the interceptor-preservation rule.

## 3. Browser-Sniff Configuration

- **Backend:** browser-use v0.12.9, CLI mode (no LLM key), headless, anonymous — no cookies, no profile, no session transfer.
- **Interceptor:** combined `fetch` + `XMLHttpRequest` wrapper writing to `window.__cap`. The XHR half was mandatory — the portal is an Angular SPA and `HttpClient` uses XHR, so a fetch-only interceptor would have captured nothing but analytics.
- **Pacing:** ~1 request/s effective; no 429s at any point.
- **Proxy pattern:** **not detected.** Plain REST-over-JSON, one path per resource.
- **Credential strip:** applied at write time (header allowlist emptied, token-shaped query values redacted). Capture was anonymous, so nothing sensitive was present to begin with.

## 4. Endpoints Discovered

Base: `https://app.iclasspro.com/api/open/v1/{account}` unless noted. All public — no auth header, no cookie, no UA requirement.

| Method | Path | Status | Content-Type | Source |
|---|---|---|---|---|
| GET | `/{account}/locations` | 200 | application/json | prior probe |
| GET | `/{account}/bookings/{locationId}` | 200 | application/json | sniff |
| GET | `/{account}/class-programs/{locationId}` | 200 | application/json | sniff |
| GET | `/{account}/camp-programs/{locationId}` | 200 | application/json | sniff |
| GET | `/{account}/appointment-programs/{locationId}` | 200 | application/json | prior probe |
| GET | `/{account}/levels/active/{locationId}` | 200 | application/json | **sniff-only** |
| GET | `/{account}/instructors/{locationId}/classes` | 200 | application/json | **sniff-only** |
| GET | `/{account}/classes` | 200 | application/json | sniff |
| GET | `/{account}/classes/{classId}` | 200 | application/json | prior probe |
| GET | `/{account}/camps` | 200 | application/json | sniff |
| GET | `/{account}/camps/{campId}` | 200 | application/json | sniff |
| GET | `/{account}/sessions` | 200 | application/json | sniff |
| GET | `/{account}/parties/create/{locationId}` | 200 | application/json | **sniff-only** |
| GET | `/{account}/products/{locationId}` | 200 | application/json | **probe-only** (ProShop) |
| GET | `/{account}/news/{articleId}` | 400¹ | application/json | **probe-only** |
| GET | `/{account}/count-cart-detailed/{locationId}/{deviceId}` | 200 | application/json | sniff |
| GET | `/{account}/appointments` | 400² | application/json | prior probe |
| POST | `/api/jwt/v1/organizations` | 200 | application/json | sniff |

¹ `{"message":"News not found."}` for a probe id — route exists, id space unknown.
² `{"message":"Appointment subscription plan expired. Please contact Admin."}` — per-tenant plan gate surfaced in the body, **not** the status code.

**Confirmed non-routes** (404, so no command should be generated): `organizations`, `parties`, `party-programs`, `students`, `enrollments`, `families`, `waitlist`, `events`, `schedules`, `search`, `levels`, `instructors`, `rooms`, `settings`, `news`, `products`, `gift-certificate(s)`, `policies`, `terms`, `class-sessions`, `class-filters`, `classes/{id}/sessions`.

## 5. Traffic Analysis

`traffic-analysis.json`: `version: 1`, protocol `rest_json` (confidence 0.75), auth candidate `none` (confidence 0.95), no warnings, 12 endpoint clusters with populated response shapes.

### Reachability — analyzer verdict CORRECTED

The analyzer initially emitted `reachability.mode: browser_required` (confidence 0.9, reason *"CAPTCHA challenge observed"*), which under Phase 1.9's decision matrix would mean **HOLD**. That was a **false positive** and was corrected in-place to `standard_http`. Evidence:

1. The flagged "CAPTCHA marker" is the string `recaptchaPublic` inside the **org-settings payload** of `POST /api/jwt/v1/organizations` — the reCAPTCHA *site key* the portal renders on its own registration form. It is configuration data, not a challenge response.
2. Zero challenge sentinels anywhere in the capture: no `Just a moment`, `cf_chl_opt`, `_abck`, `Access Denied`, `Pardon Our Interruption`, `verifying you are human`.
3. All 13 captured entries returned **HTTP 200** with real JSON bodies.
4. `cli-printing-press probe-reachability` is authoritative and returned `mode: standard_http`, confidence **0.95**: stdlib HTTP 200 (581 ms) and surf-chrome 200 (507 ms), `needs_browser_capture: false`, `needs_clearance_cookie: false`.
5. Every route in §4 was reached with bare `curl` — no User-Agent, no Referer, no cookie.

`generation_hints` was correspondingly cleared of `requires_page_context` and `requires_protected_client`.

**Runtime shape for the printed CLI: standard HTTP.** No Surf, no browser, no clearance cookie.

### Path-templating defects in the sniffed spec

The auto-generated `iclasspro-browser-sniff-spec.yaml` (12 endpoints / 2 resources) is **discovery evidence, not a generation input**. It mis-infers static segments as identifiers and hardcodes the tenant:

| Sniffed | Correct |
|---|---|
| `bookings/{booking_id}` | `bookings/{locationId}` |
| `camp-programs/{camp_program_id}` | `camp-programs/{locationId}` |
| `class-programs/{class_program_id}` | `class-programs/{locationId}` |
| `levels/active/{active_id}` | `levels/active/{locationId}` |
| `parties/create/{create_id}` | `parties/create/{locationId}` |
| `instructors/{instructor_id}/classes` | `instructors/{locationId}/classes` |
| `count-cart-detailed/{id}/{id_2}` | `count-cart-detailed/{locationId}/{deviceId}` |
| `/scottsdalegymnastics/...` baked into every path | `{account}` — multi-tenancy is the product |

Phase 2 therefore generates from a **hand-authored internal YAML spec** built from this capture plus live probing, with `{account}` as a first-class path parameter.

### Parameter-name evidence (`GET /{account}/classes`)

The search box is `input[name=searchTerm]`; the filter facets are labelled **Ages, Genders, Program, Day, Openings**. The UI walk yielded `q` directly; the rest were resolved by differential probing against `scottsdalegymnastics` (96 classes baseline — an honored filter moves the count, an ignored one does not).

| Param | Baseline 96 → | Verdict |
|---|---|---|
| `q=ninja` / `q=tumbling` | 30 / 13 | **honored** (free-text search) |
| `openings=1` | 62 | **honored** (has-openings) |
| `ages=5` | 31 | **honored** |
| `genders=2` | 45 | **honored** (ID-typed; `genders=1` → 0) |
| `levels=1` | 0 | **honored** (ID-typed) |
| `instructors=1` | 0 | **honored** (ID-typed) |
| `days=1` | 6 | **honored** |
| `programs=246` | 0 | **honored** (ID-typed) |
| `sessions=1380` | 5 (on `scaq`) | **honored** |
| `hasOpenings`, `openingsOnly`, `showOpenings` | 96 | ignored |
| `gender`, `age`, `ageYear`, `minAge` | 96 | ignored |
| `level`, `instructorIds`, `dayIds` | 96 | ignored |
| `search`, `name` | 96 | ignored |
| `sortBy` (on `/classes`) | 96 | ignored — but **honored on `/camps`** |

This is the single most valuable output of the sniff. The API returns **HTTP 200 with the full unfiltered set and an unchanged `totalRecords`** for every unrecognized parameter, so a wrapper that forwards user filters blindly produces wrong answers that look right.

## 6. Coverage Analysis

**Exercised:** locations, booking menu, class programs, camp programs, levels, instructors, classes (list + search + detail), camps (list + detail), sessions, party availability, products, cart count, org settings.

**Not exercised:**
- **Appointments** — `scottsdalegymnastics` and `scaq` both lack the subscription plan; the route returns a plan-gate message. Modeled from the route, untested against a tenant that has it.
- **News article IDs** — route confirmed, no live article id found on either tenant.
- **Party booking beyond step 1** — the flow continues past `party-booking-01-date` into date/package/checkout steps. Not walked: it trends toward a booking mutation, and the approved scope is read-only.
- **The sign-in-gated catalog path** — `nadoclub` requires customer login. Not captured (user chose to sniff an open tenant instead), so the read-only login path is modeled from the community driver's `POST /api/jwt/v1/login` → `access_token` rather than from first-hand capture.

Against the Phase 1 brief: every entity the brief predicted was found, plus four the brief did not (levels, instructors, products, news).

## 7. Response Samples

| Endpoint | Shape |
|---|---|
| `bookings/{loc}` | `{title, subTitle, image, target, targetParams:{typeId}, pluralTitle}` — `target` ∈ `classes`\|`camps`\|`parties` |
| `classes` | `{totalRecords, excludeTotal, forceStartDate, showFutureOpenings, data[]}`; item: `{id, name, minAgeYear/Month/Days, maxAge…, schedule[{dayNumber,startTime,endTime,dayName,timeStamp,tsId}], programId, levelId, instructors[], allowWebRegistration, openings, futureOpenings, futureOpeningDate, allowWaitlist, autoApprove, dates{start,end,regStart,regEnd,priorityStart,priorityEnd}, availableDates[], sessions[{id}], startDate, endDate}` |
| `classes/{id}` | adds `description` (HTML) |
| `camps` | `{totalRecords, campTypeName, campTypeNamePlural, data[]}`; item adds `openingsDisplay` (`"10 OPEN"`), `hasOpenings`, `allowToRequestCampThatIsFull`, `registrationStartDate/EndDate` |
| `camps/{id}` | adds `description` (HTML), `blocks[{bid,tsid,sqlDate}]`, `programName`, `roomName`, `instructors[]`, `programIsDeleted`, `campRegisterExpired`, `openingsDisplay` (`"10 Openings Available"`) |
| `levels/active/{loc}` | `{id, sortOrder, name, color, textColor, image}` |
| `instructors/{loc}/classes` | `{id, firstName, lastName}` |
| `sessions` | `{id, name, startDate}` |
| `parties/create/{loc}` | `{date, availableDays[], useInclusiveTax}` |
| `products/{loc}` | `{id, name, slug, price, salePrice, activePrice, sale, inventory{}, inventoryCount, variations[], images[], description, programName, taxRate, createdAt, updatedAt, active, hidden, showOnCustomerPortal}` — **the only resource carrying `updatedAt`** |
| `locations` | `{id, name, email, phone, address…, primaryColor, secondaryColor, logo, colorTheme, showAddressOnPortal, active}` |

Same-shape note: `openingsDisplay` differs between list (`"10 OPEN"`) and detail (`"10 Openings Available"`) for the same camp — presentation strings, not data; the CLI reads `openings`/`hasOpenings`.

## 8. Rate Limiting Events

None. Zero 429s across the browser session and roughly 90 direct probe requests at ~1 req/s. No rate-limit headers observed on any response.

## 9. Authentication Context

**No authenticated session used.** The capture was fully anonymous — no Chrome profile, no cookie transfer, no login. `auth.candidates` = `none` @ 0.95, consistent with every route answering bare `curl`.

Separately established (not from this capture): tenants may set a portal flag that gates `classes` and `camps` behind customer sign-in, returning **HTTP 200 with `data: []`** and a `"Please sign in to see classes."` message. `nadoclub` has this flag set; `scaq`, `scottsdalegymnastics`, `oasisgymnastics`, `tigar`, and `aerials` do not. The approved scope adds a **read-only** `auth login` (`POST /api/jwt/v1/login` → `access_token`, replayed on the same open endpoints) solely to lift that gate. No cart, enrollment, or checkout endpoints are in scope.

Session state was never written: the capture was anonymous, and `$SESSION_DIR` lives outside `$DISCOVERY_DIR` by construction, so manuscript archiving cannot pick it up.

## 10. Bundle Extraction

**Bundle:** `https://portal.iclasspro.com/main.42718f6647a8fe3a.js` (4,734,960 bytes, Angular, minified).

Route paths are assembled from template literals with interpolated variables, so static extraction yielded little. Recovered literals: `classes/search`, `enroll/new-cart-item`, `news/${articleId}`, `sessions/${id}/orders`, `sessions/${id}/payments`, `sessions/${id}/paymentDetails`, `settings/${id}`, `staff/${id}`, plus non-tenant routes `/api/open/v1/kiosk/info/`, `/api/open/v1/mobile-apps/by-bundle`, `/api/open/v1/mobile-apps/accounts-by-bundle`, `/api/open/v1/organizations/privacy-policy`, `/api/open/v1/organizations/account-deletion-info`, and `${base}/api/open/v1/${org}/gateway/family/get-guest-checkout-key`.

Query-parameter names are built as object literals rather than `HttpParams.set(...)` calls, so the filter contract was **not** recoverable from the bundle — differential probing (§5) is what settled it.

## 11. Crowd-Sniff Decision (Phase 1.8)

**Skipped — spec complete, channel exhausted.** Not a time-budget decision:

- **npm:** registry search for `iclasspro` returns exactly 1 package, `@iclasspro/icp-tinymce-variable`, a TinyMCE editor plugin with no API surface.
- **PyPI:** `iclasspro`, `iclass-pro`, `iclasspro-api` all 404.
- **GitHub code search:** 21 hits for `app.iclasspro.com/api/open`, all originating from 3 repositories, **all of which were fetched and read in full** during Phase 1.5a.5 (`johnmarcovici/iclasspro-driver`, `DevCabin/icp-widget`, `Jaymelynng/master-events-calendar*`).
- The route gaps that *did* remain after the sniff (`news`, `products`, `gift-certificate`) were closed by direct probing in seconds — crowd-sniff searches npm and GitHub and would not have found them.

Running `crowd-sniff` would re-execute the same two searches at lower fidelity than the source reads already performed.
