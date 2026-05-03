# Craigslist CLI Brief

## API Identity

- **Domain:** Classifieds. ~700 city sites under `<city>.craigslist.org` plus `accounts.craigslist.org` for posting/account management.
- **Users:** Buyers/sellers of used goods, apartment hunters, job seekers/posters, gig workers, hobbyist communities. Power users run saved-search alerts to beat other buyers on deals.
- **Data profile:** Per-listing structured data (title, price, geo, address, images, body, attributes), per-category taxonomy, per-city geographic taxonomy with subareas, per-search-query historical snapshots.
- **No official public API.** But three undocumented (or barely-documented) JSON endpoints used by Craigslist's own mobile app are live, public, and need no auth: `sapi.craigslist.org`, `rapi.craigslist.org`, and `reference.craigslist.org`. The 3taps data partnership was killed in 2014 (`Craigslist Inc. v. 3Taps Inc.`); no data licensing exists.

## Reachability Risk

**Mode: standard_http (confirmed by `printing-press probe-reachability`, confidence 0.95).** Plain stdlib HTTP with a Mozilla-ish User-Agent returns 200 OK on every read endpoint we care about. No Cloudflare, no PerimeterX, no DataDome — Craigslist runs its own anti-bot system that gates only the public scrape paths it dislikes (RSS) and the user-action endpoints (`/reply`, `/flag`).

**Live JSON endpoints (probed 2026-05-02, all 200 OK with Mozilla UA):**

| Endpoint | Method | Returns |
|----------|--------|---------|
| `https://sapi.craigslist.org/web/v8/postings/search/full?batch=<page>-0-360-0-0&cc=US&lang=en&searchPath=<cat>&query=<q>&...` | GET | 116KB JSON. **360 items per batch** as positional arrays. Includes `data.items[]`, `data.areas{}`, `data.decode{locationDescriptions, locations, neighborhoods, ...}`, `data.filters{}`, `data.totalResultCount`, `data.canonicalUrl`. |
| `https://rapi.craigslist.org/web/v8/postings/<uuid>?lang=en` | GET | Per-listing JSON. Object with `attributes[]` (typed key/value with label/value/specialType), `body` (HTML body), `category`, `categoryAbbr`, `categoryId`, `hasContactInfo`, `images[]`, `name`, plus contact info when authorized. The UUID comes from the sapi item array (positional index `[6][1]`). The integer post ID in the HTML URL is a separate identifier. |
| `https://reference.craigslist.org/Categories` | GET | 15KB JSON. Array of 178 entries: `{Abbreviation, CategoryID, Description, Type}`. Cacheable for 30 days (`Cache-Control: max-age=2592000`). |
| `https://reference.craigslist.org/Areas` | GET | 165KB JSON (gzip). Array of **707 areas**: `{Abbreviation, AreaID, Country, Description, Hostname, Latitude, Longitude, Region, ShortDescription, SubAreas[], Timezone}`. Cacheable for 30 days. |

**Live HTML surfaces (also work but lower fidelity than JSON):**
- `https://<city>.craigslist.org/search/<cat>?query=<q>&...` — embeds two JSON-LD blocks (`ld_searchpage_results` schema.org Product list, `ld_breadcrumb_data`) plus listing URLs in `<a href>`.
- `https://<city>.craigslist.org/<sub>/<cat>/d/<slug>/<id>.html` — embeds `ld_posting_data` with full PostalAddress, GeoCoordinates, price, images.
- `https://<city>.craigslist.org/sitemap/index-by-date-cat.xml` and `/sitemap/date/<YYYY-MM-DD>/cat/<cat>/sitemap.xml` — fresh-listing discovery (RSS replacement). Each per-day-per-category sitemap returns one `<url><loc>` per listing posted that day.

**Dead surfaces:**
- `?format=rss` → **403 Forbidden** ("Your request has been blocked. blockID=..."). RSS used to be the primary scrape path; it is now hard-blocked. Every historical scraper that depended on RSS is broken (HN id=24840310, 3d-logic blog Oct 2022).
- `sapi.craigslist.org/search/full/<cat>...` (the *old* path without `/web/v8/postings/`) → 404. The path moved.
- `<city>.craigslist.org/api/search/...` → 404.
- 3taps bulk feed → defunct since 2015 settlement.

**Throttling/cookies:**
- Every request sets a `cl_b` cookie (e.g. `cl_b=4|<hex>|<epoch><junk>; expires=2038-01-01`). Likely a soft fingerprint/throttle token. Reusing it across requests is polite and probably required for sustained polling.
- hCaptcha is whitelisted in CSP — challenges fire when behavior triggers them. Stay below threshold (poll interval ≥ 5 min per saved search, jittered; bounded concurrency 3-5 parallel sites).
- robots.txt allows public listing pages; disallows `/reply`, `/fb/`, `/suggest`, `/flag`, `/mf`, `/mailflag`, `/eaf` (user-action endpoints).
- HN, multilogin, and old python-craigslist issues warn about volume-based IP blocks. Those reports cluster around scrapers doing thousands of requests/min, not single-user CLI use.

**Auth model:**
- Read path (browse/search/listing detail/reference): **no auth required**, no API key, no token.
- Write path (post/renew/delete/reply/account-management): requires a logged-in `accounts.craigslist.org` cookie session. Out of scope for v1 — those are the surfaces Craigslist's anti-bot watches hardest.

**Risk to ship:** Low for the read path. The endpoints used by Craigslist's own mobile app are stable, JSON-shaped, cacheable, and documented well enough by Alex Meub's writeup that a CLI that wraps them is robust. The risk is that Craigslist could change `apiVersion: 8` or move paths again — same risk every other unofficial CL tool runs.

## Top Workflows

1. **Watch a saved search for new listings.** Power users monitor "PS5 under $300 in sfbay" or "1BR apartments under $2500 in nob hill" and pounce on new posts within minutes. Native CL email alerts are slow and noisy (they fire on edits and reposts). CLI watch + diff is the killer use case.
2. **Cross-city hunting.** Looking for a rare item ("Leica M6", "Eames lounge chair", "specific 80s synth"). Visiting 20 city sites by hand is tedious; aggregating one query across N cities returns the universe of listings. The native site has zero cross-city search.
3. **Price-drift detection.** Sellers reduce prices on listings that aren't moving. Tracking the same listing's price over time signals motivated sellers — and listings that were just reposted with a lower price are negotiation gold.
4. **Repost / scam / dupe detection.** Same listing appears in multiple cities (cross-city scam). Same listing reposts every 7 days with same title/images (motivated seller, OR flooder). Brand-new account + below-market price + ships-only signals scam.
5. **Bulk export & local analysis.** Apartment hunters and resellers want a SQLite/CSV/JSONL dump of N days of a category in a city to do their own filtering, mapping, comparison.
6. **Geo filter.** Find listings within a bounding box or radius of a point — sapi items embed encoded location offsets, JSON-LD gives lat/lng on detail pages.

## Table Stakes

Existing tools and what they offer (the absorb manifest will refine this):

- **juliomalegria/python-craigslist** (Python, PyPI, ~1k stars, archived): per-category classes (CraigslistHousing, CraigslistJobs, etc.) with filters, `.show_filters()`. Currently broken since CL went JS-rendered + sapi path moved.
- **irahorecka/pycraigslist** (Python, MIT, more recently maintained): module-per-subcategory (`pycraigslist.forsale.cta`), `search()` and `search_detail()` with geo. HTML-parser; subject to same drift.
- **node-craigslist, imclab/craigslist, ecnepsnai/craigslist, mislam/craigslist-api, craigslist-automation** — assorted JS/Go/PHP HTML scrapers; mostly stale.
- **CPlus / CPlus Classifieds** (iOS/Android, ~218k downloads/mo): multi-city search, saved searches with push, photo grid/album/map view, in-app posting + renew/repost, multiple accounts, favorites + notes. Reportedly sells PII; gates >2 saved searches behind paywall.
- **SearchTempest, AdHunt'r, searchallcraigslist.org** (web aggregators): cross-city search, ad-supported.
- **Apify Craigslist scrapers** (8+ actors): hosted scraping-as-a-service; some expose MCP. Paid.
- **Visualping, PageCrawl.io**: visual diff watchers; price-drop / new-listing alerts. Paid.
- **MCP servers / Claude Code plugins:** **none open-source.** Apify owns the paid lane. **This is a clean greenfield.**

## Data Layer

- **Primary entities:**
  - `listing(pid_int, uuid, site, subarea, category_abbr, category_id, title, body_html, body_text, price_cents, currency, posted_at, updated_at, lat, lng, location_text, postal_code, image_count, attributes_json, source_url, first_seen_at, last_seen_at, status)` — `pid_int` is the integer ID from the HTML URL; `uuid` is the rapi UUID.
  - `listing_image(pid_int, idx, image_id)` — image IDs from sapi/rapi like `00h0h_4wHIC8zOZOa_0ew0ff`.
  - `listing_snapshot(pid_int, observed_at, price_cents, title, body_hash, status)` — powers price drift, repost detection, "what changed" diffs.
  - `saved_search(id, name, sites_csv, category, query, filters_json, min_price, max_price, postal, distance_mi, has_pic, negative_keywords_csv, last_run_at)` — local SQLite-only; no remote saved-search concept.
  - `seen_listing(saved_search_id, pid_int, first_alerted_at)` — distinguishes "newly posted to me" from "newly posted on CL" so reposts don't re-alert.
  - `area(area_id, abbreviation, hostname, country, region, lat, lng, timezone, parent_area_id)` — from `reference.craigslist.org/Areas`. SubAreas flattened.
  - `category(category_id, abbreviation, description, type)` — from `reference.craigslist.org/Categories`. 178 entries.
  - `duplicate_cluster(cluster_id, body_fingerprint, image_phash, member_pids_json)` — body shingling + image perceptual hash for cross-city dupe and repost detection.
- **Sync cursor:** Per saved search → list of post IDs seen on the previous run (high-water-mark not enough since CL doesn't return strict ordering). Per (city, category) full-sync → max(posted_at) seen.
- **FTS/search:** SQLite FTS5 over `listing(title, body_text, attributes_text)`. Powers offline search with regex/exclude-keyword/phrase/AND operators that CL itself doesn't support.

## Codebase Intelligence

- **Source:** No first-party SDK. Most-starred third-party scrapers (juliomalegria/python-craigslist, irahorecka/pycraigslist) are HTML-parser libraries written before sapi was public. The cleanest documentation of the JSON endpoints is **Alex Meub's writeup** (alexmeub.com — "A Craigslist Early Notification Exploit") which traced sapi/rapi from the mobile app traffic.
- **Auth:** No API key/token. `cl_b` cookie auto-set, harmless to reuse. Optional `accounts.craigslist.org` cookie for write paths (out of scope v1).
- **Data model:** sapi positional-array items (length 11): `[postingId_int, ?, categoryId_int, price_int, locationOffsets, thumbId_str, [13, uuid_str], [4, image_strs...], [6, slug_str], [10, price_str], title_str]`. The exact positional schema is determined by `data.detailsOrder` in the response, which we'll verify per-call.
- **Rate limiting:** Soft volume throttle on `cl_b` + IP. No documented limits. Single-user CLI polling at 5-min intervals across 1-3 cities is well under the threshold reports describe.
- **Architecture:** Mojolicious (Perl) backend (confirmed via `Server: Mojolicious (Perl)` header).

## Source Priority

Single source — `sapi.craigslist.org` + `rapi.craigslist.org` + `reference.craigslist.org` together form the read API. Optional fallback to `<city>.craigslist.org/sitemap/...` for fresh-listing discovery if sapi rate-limits. No combo CLI gate.

## Product Thesis

- **Name:** `craigslist-pp-cli` — *the Craigslist watcher and triage tool that knows what's a repost, what's a scam, and what just dropped in price.*
- **Why it should exist:** Every other Craigslist scraper broke when CL killed RSS and moved sapi paths. The PyPI/npm landscape is a graveyard of "last-published 2019, archived" repos that 404 on every call. Even recent forks just rename the broken caller. There is a real, durable opportunity to ship the only working CLI for Craigslist in 2026 — built on the JSON endpoints CL's own mobile app uses (sapi + rapi + reference), exposing them as a typed agent-native MCP surface that no open-source tool currently offers. Layered on top: a local SQLite snapshot history that every other tool throws away, enabling **price drift detection, repost detection, cross-city dupe detection, and saved-search watches that emit only true new listings** — the things the website can't do and the dead libraries never could. Free, scriptable, no PII selling, no monthly fee, no proxy required.

## Build Priorities

1. **HTTP transport + cookie jar.** `cl_b` reuse, Mozilla UA, conservative pacing, exponential backoff on 403, hCaptcha-detection-and-stop.
2. **Reference taxonomy bootstrap.** `reference catalog refresh` pulls Categories (178) and Areas (707, with subareas) into local SQLite. 30-day TTL aligned with CL's own `Cache-Control`.
3. **Search.** `search "<query>" --site <hostname> --category <abbr> --min-price --max-price --postal --distance --has-pic --bundle-dupes` calls sapi with the right params, decodes positional items via `detailsOrder`, returns table + JSON. Cross-city via `--sites <csv>` parallel calls.
4. **Listing detail.** `listing get <uuid>` calls rapi; `listing get-by-pid <int>` resolves PID → UUID via search lookup or HTML scrape.
5. **Sync to local store.** `sync --site sfbay --category sss --since <date>` reads sapi pages or sitemap-by-date; persists into `listing` + `listing_snapshot`.
6. **Saved searches.** `watch save <name> --query ... --sites ... --category ...`, `watch list`, `watch run <name>`, `watch tail <name>` — diff against `seen_listing`, emit `[NEW] / [PRICE-DROP $X→$Y] / [REPOST] / [DUP-OF #pid]`.
7. **Transcendence.** `drift <pid>` (price history), `dupe-cluster` (body shingles + image pHash), `cross-city-dupes` (same listing in N cities), `scam-score` (heuristics), `geo` (radius/bbox filter), `since <duration>` (one-shot diff for ad-hoc monitoring), and a few more from the Phase 1.5 manifest.
8. **MCP surface.** Auto-derived from spec; `search`, `listing get`, `watch run`, `watch tail`, `dupe-cluster`, `geo`, `drift`, `scam-score` are the agent-relevant typed tools.
9. **Posting / auth (deferred).** `auth login --chrome` cookie import + post/renew/delete/reply against `accounts.craigslist.org` — explicitly out of scope for v1; the read+watch surface is 80% of value and avoids the highest-monitored Craigslist surfaces.

---

*This brief is the build-driving doc. The full feature inventory and absorb-vs-transcend split lives in the absorb manifest (Phase 1.5).*
