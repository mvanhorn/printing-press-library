# iClassPro CLI — Absorb Manifest

## Ecosystem Scan Result

| Channel | Searched | Result |
|---|---|---|
| Official SDK / developer docs | vendor site, support portal, changelog | **None.** Vendor states there is no API for the website and none planned; no public OAuth or API keys. Independently confirmed by CRM Coach: iClassPro "does not provide an open API, a standard automation connector, or native embeddable forms." |
| Competing CLI | GitHub repo search | **None.** |
| MCP server | GitHub search `iclasspro mcp` | **0 results.** |
| Claude plugin / skill | official plugin index, web search | **None.** |
| npm | registry search `iclasspro` | 1 package — `@iclasspro/icp-tinymce-variable`, a TinyMCE editor plugin. No API surface. |
| PyPI | `iclasspro`, `iclass-pro`, `iclasspro-api` | All 404. |
| Community projects | GitHub code search (21 hits, 3 repos) + repo search (4 repos) | 3 partial hand-rolled tools, all 0★, all read in full |

There is no incumbent to beat on features. The bar is set by three community projects that each solved one slice by hand, plus the portal UI's own capabilities.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Fetch classes for an account | `DevCabin/icp-widget` `icpClient.js` | `(generated endpoint) classes list` | Multi-tenant via `--account`, typed flags, `--json`/`--csv`/`--select`, SQLite persistence |
| 2 | Paginate classes past page 1 | `iclasspro-driver` `_fetch_open` | `(behavior in iclasspro-pp-cli classes list) auto-paginate on totalRecords until exhausted or --limit` | Community tools hardcode `limit=50`; ours follows `totalRecords` and reports partial fetches |
| 3 | Filter classes by program | `icpClient.js` `programs` param | `(behavior in iclasspro-pp-cli classes list) --programs` | Server-honored, verified |
| 4 | Filter classes by day | `master-events-calendar` | `(behavior in iclasspro-pp-cli classes list) --days` | Server-honored, verified |
| 5 | Filter classes by session | `iclasspro-driver` | `(behavior in iclasspro-pp-cli classes list) --sessions` | Server-honored, verified |
| 6 | Free-text class search | **portal UI only** (`input[name=searchTerm]`) | `(behavior in iclasspro-pp-cli classes list) --q` | **No community tool has this.** Discovered by browser-sniff: the param is `q`, honored server-side (96→30) |
| 7 | Filter to classes with openings | **portal UI only** | `(behavior in iclasspro-pp-cli classes list) --openings` | Discovered by probing: `openings=1` honored (96→62); every guessable spelling is silently ignored |
| 8 | Filter by age / gender / level / instructor | **portal UI only** | `(behavior in iclasspro-pp-cli classes list) --ages --genders --levels --instructors` | All four verified server-honored and id-typed |
| 9 | Reject silently-ignored filters | *nobody* | `(behavior in iclasspro-pp-cli classes list) unknown filters are applied locally with a stderr warning naming which predicate ran client-side` | The API returns HTTP 200 + the full unfiltered set + an unchanged `totalRecords` for unrecognized params. Blind pass-through returns wrong answers that look right. |
| 10 | Class detail with description | `master-events-calendar` ref doc | `(generated endpoint) classes get` | HTML description, plus `cliutil.CleanText` entity handling |
| 11 | Resolve camp typeIds correctly | `master-events-calendar` ref doc | `(behavior in iclasspro-pp-cli camps list) typeId resolved from bookings/{loc}, never camp-programs` | The documented trap (`camps?typeId=246` → "No camps found") is encoded in sync, not left to the caller |
| 12 | List camps by type | `collectAllGyms.js` | `(generated endpoint) camps list` | Server-honored `sortBy` (works on camps, ignored on classes) |
| 13 | Camp detail | `master-events-calendar` ref doc | `(generated endpoint) camps get` | Includes `blocks`, `roomName`, `instructors`, `programIsDeleted`, `campRegisterExpired` — all fields the source doc lists as captured by nobody |
| 14 | List locations | `collectAllGyms.js` | `(generated endpoint) locations list` | Brand colors, logos, contact, `active` |
| 15 | Booking menu | browser-sniff | `(generated endpoint) bookings list` | The authoritative typeId source |
| 16 | Class programs | browser-sniff | `(generated endpoint) programs classes` | |
| 17 | Camp programs | `collectAllGyms.js` | `(generated endpoint) programs camps` | |
| 18 | Appointment programs | probe | `(generated endpoint) programs appointments` | Plan-gate message surfaced as a typed error, not an empty list |
| 19 | Levels | **browser-sniff only** | `(generated endpoint) levels list` | Undocumented anywhere; supplies the ids `--levels` needs |
| 20 | Instructors | **browser-sniff only** | `(generated endpoint) instructors list` | Undocumented anywhere |
| 21 | Sessions | browser-sniff | `(generated endpoint) sessions list` | |
| 22 | Party availability | **browser-sniff only** | `(generated endpoint) parties availability` | Route (`parties/create/{loc}`) was unguessable — every plausible path 404s |
| 23 | ProShop products | **probe only** | `(generated endpoint) products list` | Only resource carrying `updatedAt` |
| 24 | News article | **probe only** | `(generated endpoint) news get` | |
| 25 | HTML description → plain text | `master-events-calendar` ("strip tags") | `(behavior in iclasspro-pp-cli classes get) cliutil.CleanText on description fields` | Handles `&#39;`-class entity bugs the naive strip misses |
| 26 | Absolute image URLs | `master-events-calendar` ("prepend media prefix") | `(behavior in iclasspro-pp-cli classes list) image paths resolved to https://app.iclasspro.com/media/…` | Applied uniformly across classes, camps, levels, products, locations |
| 27 | Portal deep link per class/camp | `icpClient.js` `_getRegistrationUrl` | `(behavior in iclasspro-pp-cli classes list) portal_url emitted on every row` | Extended to camps and to `camp-details`, not just classes |
| 28 | Retry with backoff | `icpClient.js` (3× / 1s) | `(behavior in iclasspro-pp-cli classes list) adaptive limiter with typed RateLimitError` | Throttling is distinguishable from "no results" |
| 29 | Openings / waitlist / full status | `icpClient.js` `_getStatusDisplay` | `(behavior in iclasspro-pp-cli classes list) status derived from openings + allowWaitlist + futureOpenings` | Includes `futureOpenings`/`futureOpeningDate`, which the community version drops |
| 30 | Multi-location handling | `master-events-calendar` (`hasMultipleActiveLocations`) | `(behavior in iclasspro-pp-cli sync) sync walks every active location per account` | |
| 31 | Read-only customer login for gated tenants | `iclasspro_jwt.py` `login()` | `iclasspro-pp-cli auth login` | Read-only by design: token is replayed **only** on catalog GETs. No cart, enroll, promo, or checkout command exists in this CLI. |

**Explicitly out of scope** (present in `iclasspro-driver`, deliberately not absorbed): `new-cart-item`, `validate-cart-item`, `add-cart-item`, `add-promo-code`, `validate-cart`, `process-cart`, `clear-cart`. `process-cart` charges a real payment method. The user scoped this CLI to read-only.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Openings watcher | `watch --class 16010` | 10/10 | hand-code | Polls `/classes` + `/camps` on an interval and diffs `openings`/`hasOpenings`/`allowWaitlist` against the previous row in local `openings_history` | A 954-line community repo exists solely to win the race for a spot; the portal has no notification of any kind | Use this command to be alerted when a spot frees up in a class or camp that is currently full. Do NOT use it to find registration that has not opened yet; use 'opens-soon' instead. |
| 2 | Catalog drift | `drift --since 7d` | 10/10 | hand-code | Compares the two most recent synced snapshots to report added / removed / retimed / repriced classes and camps, including `programIsDeleted` transitions | `master-events-calendar` flags `programIsDeleted` as "instant deletion detection!" in its unused-fields table | Use this command for what CHANGED between syncs. Do NOT use it to judge whether the catalog is well-formed right now; use 'lint' instead. |
| 3 | Registration windows | `opens-soon --days 14` | 10/10 | hand-code | Scans synced classes and camps for `registrationStartDate` in the future or `registrationEndDate` closing inside the window; flags `campRegisterExpired` | All three fields verified live; the API offers no filter on any of them and the portal surfaces none | Use this command to see registration that has not opened yet or is about to close. Do NOT use it to watch for a spot in something already open; use 'watch' instead. |
| 4 | Calendar export | `calendar --format ics` | 10/10 | hand-code | Renders synced camps and classes to RFC 5545 from `schedule[]` / `blocks[]` / `availableDates[]`, one VEVENT per session, portal deep link as URL | An entire 10-gym community project exists to build exactly this by hand; iClassPro ships no feed | none |
| 5 | Tenant capability probe | `tenant --account nadoclub` | 9/10 | hand-code | Calls `locations`, `bookings/{loc}`, `classes`, `camps`, `appointments` and classifies each as open / sign-in-gated / plan-gated by matching message strings returned alongside HTTP 200 | Verified live: `nadoclub` → `200 {"data":[],"message":"Please sign in to see classes."}`; `scaq` → `400 "Appointment subscription plan expired"` | none |
| 6 | Fill-rate trend | `fill-rate --program 57` | 8/10 | hand-code | Aggregates `openings` over time from `openings_history` per class and program to report fill direction and velocity | Dana's Monday capacity review; the API is present-tense only so no external tool can compute this | Use this command for how fast classes are filling over time. Do NOT use it for a point-in-time list of what changed; use 'drift' instead. |
| 7 | Cross-tenant compare | `compare --accounts a,b` | 8/10 | hand-code | Joins the same program/level across multiple synced tenants in local SQLite to compare schedule, age bands, and openings | `collectAllGyms.js` implements this ritual by hand; program names and typeIds differ per gym so no upstream call can do it | none |
| 8 | Catalog lint | `lint` | 8/10 | hand-code | Flags synced rows with missing descriptions or images, expired registration windows, deleted-but-listed programs, and zero-opening classes with waitlist disabled | `master-events-calendar`'s "Data Available But Not Currently Captured" table is a list of exactly these unused quality fields | Use this command to audit catalog quality as it stands now. Do NOT use it to see what changed since the last sync; use 'drift' instead. |

**Hand-code count: 8 of 8.** No transcendence row is auto-emitted by the generator.

## Stubs

**None.** Every row above ships fully implemented.

## Data Layer

Multi-tenant by construction — `account` is part of every key.

| Table | Source | Notes |
|---|---|---|
| `accounts` | derived | slug, display name, gate status |
| `locations` | `locations` | |
| `booking_menu` | `bookings/{loc}` | typeId ↔ title ↔ target |
| `class_programs`, `camp_programs`, `appointment_programs` | `*-programs/{loc}` | |
| `levels` | `levels/active/{loc}` | supplies `--levels` ids |
| `instructors` | `instructors/{loc}/classes` | |
| `sessions` | `sessions` | |
| `classes`, `class_details` | `classes`, `classes/{id}` | |
| `camps`, `camp_details` | `camps`, `camps/{id}` | |
| `products` | `products/{loc}` | only resource with `updatedAt` |
| **`openings_history`** | append-on-sync | `(account, location, kind, entity_id, observed_at, openings, future_openings, has_openings, allow_waitlist)` — powers rows 1, 2, 6 |

**Sync cursor:** none upstream. No `updatedAt` (except products), no ETag, no `If-Modified-Since`. Sync is full-refresh per account/location with content hashing for change detection; `openings_history` appends every run.

**FTS:** class/camp `name` + `description` + `programName` + `instructors` across all synced tenants. Upstream `q` is single-tenant and single-resource; offline FTS is strictly more capable.
