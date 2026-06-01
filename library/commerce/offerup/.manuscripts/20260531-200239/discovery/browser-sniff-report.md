# OfferUp Browser-Sniff Discovery Report

**Run:** 20260531-200239 · **Target:** https://offerup.com/ · **Backend:** browser-use 0.12.5 (CLI mode, anonymous)
**Primary goal:** Search OfferUp for an item near a location and view results, item detail, and seller.

## Reachability
- `probe-reachability https://offerup.com/` → **`standard_http`** (confidence 0.95). Both stdlib HTTP and Surf-Chrome returned HTTP 200. No bot challenge observed on homepage or SSR pages.
- **Runtime: plain stdlib HTTP.** No browser-compatible transport, no clearance cookie. The printed CLI ships the default HTTP client.
- Self-imposed rate limit from reverse-engineered projects: ≥1s between calls. CLI must use `cliutil.AdaptiveLimiter` with a conservative default.

## Architecture: Next.js SSR + embedded `__NEXT_DATA__`
OfferUp's web app is **Next.js (pages router)**. Data pages are **server-rendered** with the page state embedded in a `<script id="__NEXT_DATA__">` JSON blob. The homepage fires `POST https://offerup.com/api/graphql` (XHR) for some content, but the **search results, item detail, and category pages embed their data in `__NEXT_DATA__`** — no client GraphQL needed for the headline reads. `buildId` observed: `2026.22.0-...`.

**This maps directly to the generator's `response_format: html` + `html_extract: {mode: embedded-json}` primitive** (Next.js is the documented use case): `script_selector: "script#__NEXT_DATA__"`, `json_path: "props.pageProps.<route-data>"`.

## Replayable surfaces (all unauthenticated)

### 1. Search / browse listings — `GET https://offerup.com/search?q=<query>&cid=<categoryId>`
- Query params: `q` (keyword), `cid` (category id; hierarchical e.g. `1`, `1.1`, `1.2`), `skip_forced_category`.
- Extract: `props.pageProps.searchFeedResponse.looseTiles[]` — array of 100 tiles.
  - `tileType` ∈ `LISTING`, `AD_3P_GOOGLE_DISPLAY`, `AD_1P`. **Filter to `tileType === "LISTING"`** to drop ads.
  - Each listing tile: `listing { listingId, title, price, locationName, conditionText, isFirmPrice, vehicleMiles, flags[] (e.g. LOCAL_PICKUP), image{ url, height, width } }`.
- Also in `searchFeedResponse`: `nextPageCursor` (pagination), `filters[]` (facets: price/condition/etc. with `targetName`,`queryParam`,`options`), `feedOptions[]` (sort options with `queryParam`), `searchData{requestId,searchSessionId}`.
- Category browse uses the same page via `cid`; `/explore/k/<n>` and `/explore/sc/<state>/<city>` are SPA routes that resolve to the same search feed.

### 2. Item detail — `GET https://offerup.com/item/detail/<listingId>`
- Page: `/item/detail/[id]`. Extract from `props.pageProps.initialApolloState`.
- Listing detail lives under `ROOT_QUERY` keyed `listing({"listingId":"<id>"})` (dynamic Apollo cache key — needs cache-walk, not plain dot-path).
- Fields: `listingId, title, originalTitle, description, descriptionHast, condition, price, originalPrice, isFirmOnPrice, distance, isLocal, isMerchantItem, lastEdited, listingCategory, locationDetails, owner, ownerId, photos[], badges, extractedAttributes, fulfillmentDetails, shippingOptions, shippingRate, sku, state, isRemoved, vehicleAttributes{...}, merchantProfile, discussionCount`.
- Seller: `User` entity → `{id, profile{name, avatars{squareImage}, avatarBadges{primaryBadge (e.g. BUSINESS), secondaryBadge}, dateJoined, isBusinessAccount, isAutosDealer, isPremium, isSubPrimeDealer, isTruyouVerified, lastActive, openingHours, phoneNumber, c2cPhoneNumber, clickToCallEnabled, profileFeatures{...}}}`.
- Other ROOT_QUERY entries seen: `getTaxonomy({input:{zipcode}})` (category taxonomy by zip), `relatedTopics({data:{categoryId}})`, `userContext({})`.

### 3. Categories / taxonomy
- Apollo `CategoryNode` entities present on every page (~150). `getTaxonomy({input:{zipcode:"98155"}})` returns the category tree. Categories addressable by `cid`.

## Location mechanism (NOT auth)
- Location is a **cookie**, not a query param: `ou.location` / `ou.location.business.posting` = `{city, state, zipCode, longitude, latitude, source}`. Default is IP-geo (here Seattle / 98155 / 47.7495,-122.2976).
- To search a different area, the CLI sets the `ou.location` cookie before the GET. Exact minimal field set (zip-only vs lat/lon required) to be confirmed during build; lat/lon present in the observed cookie.
- This is a per-request location preference the CLI controls from `--zip`/`--city`/`--lat`/`--lon` flags — it is **not** an authentication credential.

## Auth classification
- **All headline surfaces are anonymous** (search, item, seller profile, categories). `no_auth: true`.
- Account-bound actions (saved items, messages, my-listings, posting, offers) require a logged-in **session cookie** (`auth.type: cookie`). These are **secondary/opt-in** per the user directive ("prefer unauthenticated; only require auth when required") and are NOT part of the default first-run experience.

## Generation implications
- `category: commerce`, `base_url: https://offerup.com`, runtime = plain HTTP.
- Search/category: clean `response_format: html` embedded-json (json_path `props.pageProps.searchFeedResponse.looseTiles`), but ad-filtering + `.listing` flattening + the location cookie favor a hand-built `// pp:client-call` command for clean agent-native output and SQLite storage.
- Item detail + seller: Apollo ROOT_QUERY dynamic-key navigation → hand-built extraction.
- Transcendence (price-check, watch/new-since, deals/underpriced, price-drops, seller-scan) all build on the local SQLite store populated by search/sync.
