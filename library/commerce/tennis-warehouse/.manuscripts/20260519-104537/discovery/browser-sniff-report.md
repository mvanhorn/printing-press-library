# Tennis Warehouse browser-sniff report

## Target
- Base URL: `https://www.tennis-warehouse.com`
- Capture method: direct HTTP (curl with Chrome UA — no challenge, no WAF, no JS gating).
- 5 sample HTML pages saved to `discovery/samples/` for spec authoring.

## Reachability
- `probe-reachability`: not strictly required — direct HTTP HEAD against both seed URLs returned `HTTP/2 200`, plain Apache, no Cloudflare/Vercel/Akamai/DataDome/PerimeterX/CAPTCHA headers. Standard HTTP (`mode: standard_http`).
- Sets `SMID=smc_new` cookie (anonymous session identifier; not required to replay endpoints).
- Standard HTTPS, no challenge.

## Endpoint catalog (HTML-extraction surfaces)

| Method | Path | Query/Params | Returns | Notes |
|--------|------|--------------|---------|-------|
| GET | `/usedracquets.html` | none | HTML landing | Lists brand `ccode` codes; grade legend |
| GET | `/usedcatpage.html` | `ccode={BRANDRACS}` | HTML brand-used catalog | Each model card: `data-pcode`, `data-price_low/high`, `data-prod_name`, `data-gtm_impression_*` |
| GET | `/orderusedproduct.html` | `pcode={SKU}` | HTML used-model detail | Spec table + multiple `<tr data-code="UR...">` individual listings with `data-styleref="Grade X"` |
| GET | `/Tennis_Racquets/catpage-RACQUET.html` | none | HTML all-racquets landing | Featured + best-sellers, links to brand catalogs |
| GET | `/{Brand}racquets.html` | none | HTML brand catalog | e.g., `/Wilsonracquets.html`, `/Babolatracquets.html` |
| GET | `/{Brand_Model_Name}/descpageRC{BRAND}-{SKU}.html` | none | HTML new-racquet detail | Full spec table |

All endpoints respond to plain GET with no required headers beyond a normal browser User-Agent (the site does serve degraded content to a generic curl UA, so the CLI must always send a Chrome UA — Surf transport will handle this).

## Extraction patterns (key selectors / regexes)

### Used catalog listing card (`/usedcatpage.html?ccode=...`)

Each model card is a `<div>` with attributes carrying full product metadata:

```html
<... data-pcode="WB9816"
     data-prod_name="Wilson Blade 98 16x19 v9 Racquet"
     data-gtm_impression_brand="Wilson"
     data-gtm_impression_category="Racquets"
     data-gtm_impression_price="269.00"
     data-price_low="..."
     data-price_high="..."
     data-old_price_low="..."
     data-newitem|data-saleitem|data-reduced|data-closeout|data-bestseller
>
```

The model SKU (`pcode`) maps to a detail URL: `/orderusedproduct.html?pcode={pcode}`.

### Used model detail (`/orderusedproduct.html?pcode=...`)

- Page-level: `<h1>` carries the model name; spec table uses `<td class="SpecsLt|SpecsDk">Label:</td><td>Value</td>` rows. Labels: Head Size, Strung Weight, Balance, Swingweight, Stiffness, Beam Width, String Pattern, Length, Composition, Power Level, Stroke Style.
- Each individual used unit is a row: `<tr data-code="UR8A04E" class="subproduct ..."> ... data-styleref="Grade A" ...`. Codes are unique per physical racquet; `data-code` is the stock number.

### New racquet brand catalog (`/{Brand}racquets.html`)

Each card carries:
- Product image
- Title (model name)
- Price (HTML text — `$269.00` style)
- Badges: "New", "Reduced", "Clearance"
- Star rating + review count
- Brief description containing inline mini-specs: `Headsize: 98in². String Pattern: 16x19. Standard Length.`
- Link: `/{Brand_Model_Name}/descpageRC{BRAND}-{SKU}.html`

### New racquet detail (`descpageRC...html`)

Same spec-table shape as used model detail: `<td class="SpecsLt|SpecsDk">Head Size: ...`. All canonical spec fields are present here.

## Brand `ccode` enumeration (from `/usedracquets.html`)

- `BABRACS` — Babolat
- `DUNLOPRACS` — Dunlop
- `HEADRACS` — Head
- `KENNEXRACS` — ProKennex
- `PRINCERACS` — Prince
- `SOLINCORAC` — Solinco
- `TECRACS` — Tecnifibre
- `VOLKLRACS` — Volkl
- `WILSONRACS` — Wilson
- `YONEXRACS` — Yonex
- `RACSBYMAKER` — index page

Note: `NEWLC` appears but is likely a Lacoste/used section variation; treat as discoverable on demand.

## New-racquet brand pages

Direct URL pattern: `/{Brand}racquets.html` (capitalized brand). Confirmed for Wilson and Babolat. Other brands follow the same shape.

## Authentication

None required for catalog browsing. The site has user accounts (saved racquets, order history), but they are NOT in scope — the user did not provide auth context, and the headline workflows (filter/search/compare/track) are all anonymous-readable.

## Replayability verdict

**PASS — standard HTTP replayable**. Every endpoint above can be replayed with a single GET request using a Chrome UA. No JS execution required. HTML is parseable with goquery (or stdlib regex for the data-attribute fast path). No clearance cookie, no session token, no rate-limit handshake observed.

## Recommended transport for the printed CLI

- HTTP client: stdlib `net/http` with a Chrome UA header. `surf` would also work but isn't strictly required since direct HTTP probes succeeded.
- Per-request `User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36`.
- Rate limit: be polite — 1 req/sec default with adaptive backoff on 429 (use `cliutil.AdaptiveLimiter`).
- Parse layer: `goquery` selectors on the data attributes for catalog cards, table-cell labels for spec sheets.

## Traffic analysis hints

- `reachability.mode`: `standard_http`
- `response_format`: `html` for every endpoint
- `client_pattern`: standard (not proxy-envelope, not GraphQL)
- `auth.type`: `none`
- `pii_observed`: none beyond an anonymous session cookie (`SMID`)
