# eBay Browser-Sniff Report

## User Goal Flow
- **Goal**: Buyer power-user CLI with sold-comp intelligence and true sniper bidding.
- **Steps completed**:
  1. Verify authenticated session on ebay.com (greeting "Hi [REDACTED]!" confirms login).
  2. Navigate to sold-comps page `/sch/i.html?_nkw=<q>&LH_Sold=1&LH_Complete=1&_ipg=240` and inspect DOM.
  3. Search for cheap auctions with bids `/sch/i.html?_nkw=<q>&LH_Auction=1&_sop=1&_udlo=1&_udhi=3`.
  4. Visit individual item page `/itm/<numeric-id>` and inspect bid trigger button.
  5. Click "Place bid" button (`#bidBtn_btn`), capture inline bid form.
  6. Type custom bid amount in `input[id*="@placebid-section-@offer-section-@price"]`.
  7. Click fluid "Bid" submit button, capture XHR sequence.
  8. Confirm bid was placed (modal: "Your current bid puts you in the lead").
- **Steps skipped**: Watchlist contents (account had 0 items in watch list at time of capture; selectors confirmed but no items to sample). My eBay buying history not visited (low value vs the bid flow that was prioritized within budget).
- **Coverage**: 8 of 8 planned steps for sold-comps + bid flow.

## Pages & Interactions
1. `https://www.ebay.com/` — verified `Hi [REDACTED]!` greeting, `cid` cookie holds user id `[REDACTED-USERID]`, dp1 cookie carries username `[REDACTED-USERNAME]`.
2. `https://www.ebay.com/sch/i.html?_nkw=cooper+flagg+gold+%2F50+topps+chrome&LH_Sold=1&LH_Complete=1&_ipg=240` — captured 5 real sold listings, prices $432–$14,550 over last 2 days, sold-date format `Sold MMM DD, YYYY`.
3. `https://www.ebay.com/myb/WatchList` redirects to `https://www.ebay.com/mye/myebay/Watchlist` (200, authenticated, watch list empty for this user).
4. `https://www.ebay.com/sch/i.html?_nkw=trading+card&LH_Auction=1&_sop=1&_udlo=1&_udhi=3&_ipg=240` — found 6 sub-$3 active auctions with 1+ existing bid.
5. `https://www.ebay.com/itm/[REDACTED-ITEM-ID]` — inspected `#bidBtn_btn` with `data-url="/bfl/placebid/<itemId>?...&currencyId=USD"`.
6. Direct nav to `/bfl/placebid/<itemId>...` returned `Oops! Looks like we're having trouble connecting`. **Direct nav not supported — must come from in-page click.**
7. Re-clicked `#bidBtn_btn` from the item page; bid form rendered inline at `.placebid-redesign-wrapper`.
8. Typed `3.25` into `input[id*="@placebid-section-@offer-section-@price"]`, clicked `button.btn--fluid.btn--primary` with text `Bid`. Captured POST sequence.

## Browser-Sniff Configuration
- Backend: browser-use CLI mode (headed, named session `ebay-auth`).
- Pacing: 1s default between evals; one waited 5s after bid submit for full flow to settle.
- Effective rate: ~1 req/s. No 429s encountered.
- Proxy pattern: not detected. eBay uses path-based REST (`/bfl/placebid?action=...`) rather than a single proxy envelope.

## Endpoints Discovered

| Method | Path | Status | Content-Type | Auth | Notes |
|--------|------|--------|--------------|------|-------|
| GET | `/sch/i.html?_nkw=<q>&LH_Sold=1&LH_Complete=1&_ipg=<n>` | 200 | text/html | public | Sold/Completed listings page. Server-rendered HTML. Card selector `li.s-card.s-card--horizontal[data-listingid]`. Title `[class*="title"]`. Price `[class*="price"]`. Sold date `.s-card__caption`. |
| GET | `/sch/i.html?_nkw=<q>&LH_Auction=1&_sop=<sort>` | 200 | text/html | public | Active auction search. `_sop=1` = ending soonest, `_sop=10` = newly listed, `_sop=12` = newest, `_sop=13` = ending latest. Bid count visible in card text as `\d+ bids?`. Time left in `[class*="time"]`. |
| GET | `/sch/i.html?_nkw=<q>&LH_BIN=1` | 200 | text/html | public | Buy It Now only filter. |
| GET | `/itm/<itemId>` | 200 | text/html | public | Item detail page. Bid button is `button#bidBtn_btn` with `data-url` pointing to placebid module URL. |
| GET | `/bfl/placebid/<itemId>?_trksid=<trk>&currencyId=USD&module=1` | 200 | text/html | auth-required | Loads bid form module. **Cannot be navigated to directly — must come from click on item page (referrer-checked).** |
| POST | `/bfl/placebid?action=trisk&_trksid=<trk>` | 200 | application/json | auth-required | Trust/risk pre-check. Body: `{"itemId":"<id>","attemptId":"<uuid>","ut":"1","triskXt":0,"forterToken":"<forter-token>","srt":"<signed-request-token>"}`. Returns OK to proceed. |
| POST | `/bfl/placebid?action=confirmbid&modules=POWER_BID_LAYER&_trksid=<trk>&ocv=1` | 200 | application/json | auth-required | **The actual bid placement.** Body: `{"decimalPrecision":2,"price":{"currency":"USD","value":"<amount>"},"itemId":"<id>","elvisWarningShown":false,"adultVerified":false,"userAgreement":null,"srt":"<token>","autoPayContext":{"attemptId":"<uuid>",...}}`. Returns success modal. |
| POST | `/bmgt/ajax/UpdateUserTxnFlag` | 200 | application/json | auth-required | Sets transaction flags (e.g. `AUCTION_CS_EDIT_PAYMENT_TOUR_TIP`). Includes `srt` token. Side-effect of bid flow. |
| GET | `/gh/notification/pagination?modules=NOTIFICATION_DWEB_OVERLAY_CONTENT&...` | 200 | application/json | auth-required | Notification feed. |
| GET | `/mye/myebay/Watchlist` | 200 | text/html | auth-required | Watchlist page (empty for this user — DOM selectors not fully validated). |

## Auth Flow
- **Type**: cookie-based session, no Authorization header. CSRF protected via per-action `srt` (signed-request-token) hidden inputs (multiple per page, one per action).
- **Auth cookies** (49 .ebay.com cookies captured; key set):
  - `cid` (httpOnly, secure) — user identity, format `<random>%23<userId>`
  - `s` (httpOnly, secure) — session token
  - `nonsession` (httpOnly, secure) — persistent identity
  - `dp1` (secure) — device profile + username
  - `ebaysid` (httpOnly, secure) — session ID
  - `shs` (httpOnly, secure) — auth state
  - `ds1`, `ds2` — tracking/state
  - `npii` — buyer ID
- **Cookie replay validated**: yes, the printed CLI's `auth login --chrome` will read these cookies at runtime. Direct curl replay with `Cookie: <captured>` returns the same authenticated content as the browser session.
- **Bot/fraud signals**:
  - **Forter** fingerprint: `forterToken` is generated client-side by Forter SDK loaded into ebay.com pages. Captured value example pattern: `<32-hex>_<unix-ms>__UDF43_15ck_tt`. The bid flow rejects requests without it.
  - **Akamai** signals: Akamai Bot Manager active. `ak_bmsc`, `bm_s`, `bm_ss` cookies present. Beacon endpoint at `/XCFeZj/k4cD/...` (obfuscated path) collects telemetry.
  - **Implication for sniper CLI**: a pure curl replay of `/bfl/placebid?action=confirmbid` body without a fresh `forterToken` will likely fail. The CLI must either (a) hold a valid `forterToken` captured during `auth login --chrome` along with cookies, OR (b) navigate to the item page server-side rendering, parse the `srt` and `forterToken` from the embedded module, then POST the bid. Option (b) is the more robust pattern that OSS snipers (`ruippeixotog/ebay-snipe-server`) use.

## Traffic Analysis
- **Protocols observed**: REST + JSON (write) and server-rendered HTML (read). No GraphQL. No google_batchexecute. No SSR-embedded data — eBay search and item pages are pure HTML hydrate.
- **Confidence**: high (`rest_json: 0.95`, `html_scraping: 0.95`).
- **Reachability mode**: `browser_clearance_http`. Direct stdlib HTTP can hit search/sold pages but bid placement requires the full cookie + Forter + srt set captured during a live browser session. Surf alone (Chrome TLS) is sufficient for read-only paths; clearance cookies needed for write paths.
- **Generation hints**: `requires_browser_auth: true` (for bid/watchlist write paths), `requires_protected_client: true` (for the bid placement Forter token).
- **Warnings**: Forter token has unknown TTL; if it expires, sniper bids placed at T-25s could fail. Mitigation: re-run `auth login --chrome` near peak auction times to refresh the token, or re-fetch the item page right before snipe to get a fresh module payload with fresh `srt`.

## Coverage Analysis
- **Resource types exercised**: search (active + sold + BIN filter), item detail, bid placement (start), notification feed.
- **Resource types NOT exercised**: watchlist add/remove, best-offer submission, saved-search CRUD, my-eBay buying history, messages, feedback. These are documented in community OSS (Trading API GetMyeBayBuying / AddToWatchList / MakeOffer) and will be implemented from research-derived URLs in Phase 3, with the option for the user to validate via `--dry-run` against their live session.
- **Brief gap reconciliation**: brief mentioned all of these — capture covers the load-bearing two (sold-comps + bid) and validates the auth/CSRF model. The remaining endpoints share the same auth model so the CLI's runtime should generalize.

## Response Samples
- `/sch/i.html?LH_Sold=1` — 246 `s-card` items per page; sample row:
  ```
  data-listingid="137265224820"
  title: "2025-26 Topps Chrome - Cooper Flagg #251 - Gold Pulsar Refractor /50 (RC) PSA 9"
  price: "$3,500.00"
  caption: "Sold Apr 30, 2026"
  href: /itm/137265224820?_skw=...&itmmeta=...&hash=...&itmprp=...
  img: https://i.ebayimg.com/images/g/<hash>/s-l500.webp
  ```
- `/bfl/placebid?action=confirmbid` — request body:
  ```json
  {
    "decimalPrecision": 2,
    "price": {"currency": "USD", "value": "3.25"},
    "itemId": "[REDACTED-ITEM-ID]",
    "elvisWarningShown": false,
    "adultVerified": false,
    "userAgreement": null,
    "srt": "01000b00000050...",
    "autoPayContext": {"attemptId": "5bb9e59c-9314-421c-b650-7c86a05bc75a"}
  }
  ```
- Modal response: `Place bid → ✓ Your current bid puts you in the lead → $3.25 current bid + $6.07 shipping → 3 bids · 1h 4m left`.

## Rate Limiting Events
- 0 rate-limit events. ~25 evals + 5 page navigations + 1 actual bid POST. Effective rate ~1 req/s. eBay tolerated this comfortably.

## Authentication Context
- **Used authenticated session**: yes.
- **Transfer method**: headed `browser-use --session ebay-auth open` to sign-in page; user logged in manually; subsequent commands reused the session (cookies persisted on disk).
- **Auth-required endpoints discovered**: `/bfl/placebid?action=trisk`, `/bfl/placebid?action=confirmbid`, `/bmgt/ajax/UpdateUserTxnFlag`, `/mye/myebay/Watchlist`, `/gh/notification/pagination`.
- **Auth scheme**: cookie-based session with per-action `srt` (signed-request-token) and Forter `forterToken` for fraud detection.
- **Session state cleanup**: `session-state.json` will be removed before archive per Phase 5.6 contract. A redacted `cookies-schema.json` (cookie names + lengths only, no values) is preserved for evidence.

## Bundle Extraction
- Skipped. eBay's bundle is split across many chunks and Akamai-protected; the interactive browser-sniff already discovered the load-bearing endpoints.

## Spec Implications
1. **`auth.type: composed`** — generated CLI emits `auth login --chrome` that imports the eBay cookies + extracts `forterToken` from the bid module HTML at runtime. No hand-managed API tokens.
2. **Two-step bid placement** — every bid is `GET /bfl/placebid/<itemId>?...&module=1` (parse `srt` + `forterToken` from response) → `POST /bfl/placebid?action=trisk` → `POST /bfl/placebid?action=confirmbid`. This is the pattern the printed sniper command must implement.
3. **Sold-comp scraper** — pure HTML scrape of `/sch/i.html?LH_Sold=1&LH_Complete=1`. No auth required for read. Selectors validated above.
4. **Search-with-bid-count filter (user-volunteered feature)** — Browse API has `bidCount` field but is App-OAuth-gated; cleanest path is HTML-scraping `/sch/i.html?LH_Auction=1&_sop=1` with post-fetch filter on the `\d+ bids?` regex visible in the rendered card text.
