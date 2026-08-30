# Browser-Sniff Discovery Report: Best Food Trucks (bestfoodtrucks)

## 1. User Goal Flow

- **Goal:** Look up which food truck is scheduled at a given lot (today/tomorrow/full schedule), matching the user's literal target URL (`/lots/playa-district`).
- **Steps completed:**
  1. Navigated to `https://www.bestfoodtrucks.com/lots/playa-district` — confirmed Chrome-fingerprint transport clears the Vercel checkpoint (matches earlier `probe-reachability` result).
  2. Extracted the page's `__NEXT_DATA__` script tag — this is a Next.js app (the Astro-branded HTML seen in early curl probes was the Vercel Security Checkpoint interstitial, not the real site).
  3. Parsed the embedded Apollo Client cache (`props.pageProps.apolloState`) — fully normalized entity graph for the lot's schedule (Lot, LocationSchedule, Location/shift, Truck, Menu, Item, LocationItem, FoodType, ItemTag, Organization).
  4. Clicked "Full Schedule" and attempted a date-filter button — confirmed no new network calls fire on these clicks; the schedule window is client-rendered from data already present in `__NEXT_DATA__` (SSR'd upfront, not lazily fetched).
  5. Navigated to a secondary flow: `https://www.bestfoodtrucks.com/shifts/179609-playa-district-on-2026-08-26` (a specific truck's shift/detail page). Extracted its `__NEXT_DATA__` — revealed additional entities: Restaurant, PublicRating (review count), Market (city taxonomy), full LocationItems (9, vs. the 6-item preview on the lot page), UpsellType.
  6. **Critical test:** replayed the exact query shape as a direct `fetch()` from within the page context against `https://api.bestfoodtrucks.com/graphql` — got clean structured JSON back, matching the SSR data exactly.
  7. **Critical test:** replayed the same GraphQL POST via plain `curl` with zero special headers (no cookies, no TLS fingerprint spoofing, no User-Agent) — **200 OK, correct JSON**. This settles the runtime transport decision: `api.bestfoodtrucks.com/graphql` needs no browser-compatible HTTP at all. Only the `www.bestfoodtrucks.com` HTML marketing pages are Vercel-checkpoint-protected; the actual data API is open to standard HTTP.
- **Steps skipped:** Did not attempt any authenticated flow (login, subscribe, cart, order-ahead) — user explicitly declined (`AUTH_SESSION_AVAILABLE=false`) since the core workflows are fully public.
- **Secondary flows attempted:** Shift/truck-detail page (step 5 above).
- **Coverage:** 7 of 7 planned discovery steps completed (goal fully achieved, plus a bonus: discovered the API is directly callable without any browser at all).

## 2. Pages & Interactions

1. `https://www.bestfoodtrucks.com/lots/playa-district` — lot schedule page. No interaction beyond load; extracted embedded data.
2. Clicked "Full Schedule" button (ref e15) — no new network activity (client-side render from existing data).
3. Attempted click on "Sep 8, Tuesday" date button — ref became stale after the "Full Schedule" click (page state changed); not retried since the goal (confirm no new fetch fires) was already established.
4. `https://www.bestfoodtrucks.com/shifts/179609-playa-district-on-2026-08-26` — shift/truck-detail page (secondary flow). No interaction beyond load; extracted embedded data.
5. In-page `fetch()` eval against `https://api.bestfoodtrucks.com/graphql` with a hand-reconstructed query — direct API replay test.
6. External `curl` POST against the same GraphQL endpoint — transport-requirement test (outside any browser context).

## 3. Browser-Sniff Configuration

- **Backend used:** agent-browser v0.32.1 (the machine's installed `browser-use` binary, v0.1.4, turned out to be an incompatible different tool — different CLI shape, not the CLI-2.0 `open`/`eval`/`scroll`/`close` interface this skill expects. User approved switching to agent-browser.)
- **Session hygiene note:** the first `agent-browser open` reused an existing daemon session that had stale, unrelated traffic from a prior automation task (Concur) logged in its request buffer (`"reused":true`). Closed the daemon fully and relaunched under an isolated named session (`--session bestfoodtrucks-sniff`) with an explicit `network requests --clear` before navigating. All capture in this report is from the clean, isolated session only.
- **Pacing:** two page loads plus a handful of interactions; no rate limiting encountered (0 HTTP 429s). No adaptive backoff needed.
- **Proxy pattern detection:** NOT a proxy-envelope pattern. This is standard GraphQL — distinct, named operations (`CurrentCartQuery`, `CurrentCustomerQuery`, `LotActionSubscription`, `LotPageQuery` (reconstructed)) each with their own query document, not a single routing envelope. Apollo Client **Automatic Persisted Queries (APQ)** is in use: the client first sends a query-hash-only POST; on `PersistedQueryNotFound`, it retries with the full query text attached. The generated CLI should always send full query text (skip the hash-first optimization) to avoid depending on the server's persisted-query cache.

## 4. Endpoints Discovered

| Method | Path | Status | Content-Type | Auth |
|--------|------|--------|--------------|------|
| POST | `https://api.bestfoodtrucks.com/graphql` (operation: query by field, e.g. `lot(seoName)`, `location(id)`, `currentCart`, `currentCustomer`, `upsellTypes`) | 200 | application/json | public (anonymous `currentCustomer: null` confirmed) |
| POST | `https://api.bestfoodtrucks.com/track-referrer.json` | 200 | application/json | public (analytics beacon, not a data endpoint) |
| GET | `https://www.bestfoodtrucks.com/lots/<seoName>` (SSR HTML w/ embedded `__NEXT_DATA__`) | 200 (via Chrome-fingerprint transport; 429 via plain stdlib HTTP) | text/html | public |
| GET | `https://www.bestfoodtrucks.com/shifts/<id>-<lotSeoName>-on-<date>` (SSR HTML w/ embedded `__NEXT_DATA__`) | 200 (Chrome-fingerprint transport) | text/html | public |
| GET | `https://www.bestfoodtrucks.com/_next/data/<buildId>/lots/<seoName>/schedule.json` | 200 | application/json | public — **not stable long-term**: path is keyed by a Next.js build ID that changes on every site deploy. Do not use as a printed-CLI runtime target; the GraphQL endpoint above is the stable equivalent. |
| GET | `https://www.bestfoodtrucks.com/_next/data/<buildId>/shifts/<id>-....json` | 200 | application/json | public — same build-ID-fragility caveat as above |

The GraphQL endpoint is the only stable, generation-worthy target. The `_next/data/*.json` routes are documented here for completeness/evidence but should NOT be used by the printed CLI.

## 5. Traffic Analysis

- **Protocols observed:** `graphql` (primary, confirmed via direct query/response round-trip), `ssr_embedded_data` (Next.js `__NEXT_DATA__` — the mechanism the automated `browser-sniff` HAR analyzer actually classified, since its HAR lacked response bodies for the AJAX calls).
- **Auth signals:** none required for the targeted surface. `currentCustomer: null` when anonymous. Login-gated actions (subscribe, cart mutations, order placement) exist but were not explored (`AUTH_SESSION_AVAILABLE=false`).
- **Parameter-name evidence:** `lot(seoName: String!)` — the lot's URL slug (e.g., `"playa-district"`). `location(id: Int!)` — numeric shift ID (e.g., `179609`). `Lot.locationSchedule(days: Int!)` — schedule window size in days (observed default `5`). These are clean, self-describing GraphQL argument names — no cryptic single-letter params requiring `flag_name` remapping.
- **Protection signals:** Vercel "Security Checkpoint" (JS challenge, `x-vercel-mitigated: challenge`, `x-vercel-challenge-token`) guards `www.bestfoodtrucks.com` HTML pages only. The GraphQL API host (`api.bestfoodtrucks.com`, Cloudflare-fronted) has **no protection signal observed** — plain curl succeeded with zero special headers.
- **Generation hints:** none of `requires_browser_auth`, `requires_js_rendering`, `requires_protected_client`, or `browser_required` apply to the GraphQL endpoint. The automated HAR-based `traffic-analysis.json` reachability classification (`mode: browser_http`, confidence 0.78) reflects the `www` HTML host only and should NOT be applied to the API host — confirmed by direct testing (see Section 1, steps 6-7). **The printed CLI should use standard `net/http` transport against `api.bestfoodtrucks.com/graphql`, no Surf/Chrome-TLS-fingerprint transport needed.**
- **Candidate commands worth considering:** lot lookup + schedule (by seoName), shift/truck detail lookup (by location id), city/market directory browse, reverse truck-schedule lookup (would require a different root query — not yet confirmed to exist; may need a `trucks`/`truck(id)` root field probe in Phase 3).
- **Warnings:** the automated `browser-sniff` tool's HAR-based analysis reported `size_class: "empty"` for the GraphQL cluster because agent-browser's HAR export omits response bodies for XHR/fetch entries. This was fully remediated by direct evidence gathering: `direct-graphql-test.txt` (in-page fetch), `next-data-parsed.json` / `shift-next-data-parsed.json` (full Apollo cache dumps), and a successful external curl replay — all richer ground truth than the HAR alone would have produced.

## 6. Coverage Analysis

Exercised: lot schedule view (5-day window), shift/truck detail view (menu, items, tags, food types), city/market taxonomy reference (`Market:1` = Los Angeles), anonymous cart/customer state. Likely missed (not explored, in scope for Phase 3 hand-authored commands if valuable): a `trucks`/`truck(id)` root query for reverse lookup ("where is Truck X scheduled"), `lotGroup(seoName)` (returned null for this lot — likely used for lots with multiple sub-locations), city/state directory root query (the `/food-trucks/<city>` pages were not re-visited with capture on this run — Phase 1 research confirmed their existence via `webfetch` only). These gaps are candidates for a short, targeted Phase 3 exploration pass (one more GraphQL introspection-style probe) rather than a full re-run of browser-sniff.

## 7. Response Samples

**`Lot` entity (from `next-data-parsed.json`):**
```json
{
  "__typename": "Lot", "id": 4702, "lotPath": "/lots/playa-district", "name": "Playa District",
  "active": true, "address": "[REDACTED STREET ADDRESS]", "fullAddress": "[REDACTED STREET ADDRESS], Los Angeles, CA, 90045",
  "facebook": "bestfoodtrucksla", "instagram": "bestfoodtrucksla", "website": "http://foodtruckcatering",
  "referralEnabled": true, "subscribed": false
}
```

**`Location` (shift) entity:**
```json
{
  "__typename": "Location", "id": 179609, "startTime": "2026-08-26T11:00:00-07:00", "endTime": "2026-08-26T14:00:00-07:00",
  "workStatus": "work_started", "workStatusHuman": "Live", "allowOrders": true,
  "customerUrl": "https://www.bestfoodtrucks.com/shifts/179609-playa-district-on-2026-08-26",
  "truck": {"__ref": "Truck:11869"}, "menu": {"__ref": "Menu:39243"}
}
```

**`Item` (menu item) entity:**
```json
{
  "__typename": "Item", "id": 365712, "name": "Cluckin' Classic", "active": true,
  "description": "Pickles and Chick's Special Sauce. ", "tags": [],
  "price": {"__typename": "Money", "cents": 1300, "formatted": "$13"}, "hasSpecialInstructions": false
}
```

**Direct GraphQL POST response (`direct-graphql-test.txt`, truncated):**
```json
{"data":{"lot":{"id":4702,"name":"Playa District","fullAddress":"[REDACTED STREET ADDRESS], Los Angeles, CA, 90045",
"locationSchedule":[{"id":"4702_2026-08-26","dateAlias":"Today","locations":[{"id":179609,"startTime":"2026-08-26T11:00:00-07:00",
"endTime":"2026-08-26T14:00:00-07:00","workStatusHuman":"Live","allowOrders":true,
"customerUrl":"https://www.bestfoodtrucks.com/shifts/179609-playa-district-on-2026-08-26",
"truck":{"id":11869,"name":"The Chick Truck"}}]}, ...]}}
```

**PersistedQuery miss/retry pattern:**
```json
// First attempt (hash only): {"errors":[{"message":"PersistedQueryNotFound"}]}
// Retry (full query attached): {"data":{"lot":{"id":4702,"subscribed":false,"__typename":"Lot"}}}
```

## 8. Rate Limiting Events

None. Zero HTTP 429 responses across the entire session (two page loads, one in-page fetch, one external curl). No backoff was needed.

## 9. Authentication Context

No authenticated session used. `AUTH_SESSION_AVAILABLE=false` per Phase 1.6 (user declined — core workflows are fully public). `currentCustomer: null` confirms anonymous state throughout. Authenticated-only surfaces (subscribe-to-schedule, order-ahead/checkout, customer account) were not explored and are out of scope for this run's absorbed feature set.
