# Crestron.com Browser-Sniff Discovery Report

Backend: browser-use v0.13.7 (CDP attach to user's Chrome)
Date: 2026-08-02
Runtime classification: `standard_http` (probe-reachability, confidence 0.95)

## Verdict
Site is server-rendered ASP.NET / Kentico CMS. **No resident browser required.**
All shipping surfaces replay over plain HTTP. Browser-sniff confirmed the
statically-derived contract and added product-page structure detail.

## Endpoints (live-confirmed)

| Method | Path | Purpose | Auth | Format |
|---|---|---|---|---|
| GET | `/Support/Search-Results?q=&type=&c=&p=&m=&o=` | Resource search across 8 categories | none | HTML |
| GET | `/CMSPages/ProductSubcategoryItemTemplate.aspx?dId=&nId=&ps=&os=&cult=&sn=&sort=&fltr=` | Product tiles per category, paginated by `os` offset | none | HTML fragment |
| GET | `/Products/Catalog/<path>/<MODEL>` | Product detail (JSON-LD + specs + resources) | none | HTML |
| GET | `/getmedia/<guid>/<filename>` | PDF assets (spec sheets, manuals, certs, drawings, CAD, Revit) | none | PDF/binary |
| GET | `/Handlers/PublicPriceRegionOptionsHandler.ashx` | Pricing region list | none | JSON |
| GET | `/sitemap` | Full category taxonomy (127 product paths) | none | HTML |
| GET | `/Software-Firmware/<Type>/<Family>/<Model>/<Version>` | Firmware/software detail + download | **login** | HTML |

## Search filters (`c` parameter)
0=All, 1=Marketing Resources, 3=Technical Resources, 4=Software & Firmware,
5=Spec Sheets, 6=Guides & Manuals, 7=Specifier Resources, 8=Multimedia,
11=Product Certificates

Sort (`o`): `Created:desc` (last updated), `Name_Search.keyword:asc` (A-Z),
`Name_Search.keyword:desc` (Z-A). Page size (`m`): 10 | 25 | 50.

## Product detail page structure
- `<script type="application/ld+json">` → schema.org Product: `name`, `sku`,
  `description`, `brand`, `image[]`
- Tabs (all server-rendered, no XHR): Overview, Specifications, Resources,
  Models & Accessories
- Spec table in `#panel2`: `td.productSpecTDHead` = section header,
  `<td width="30%">` / `<td width="70%">` = key/value rows
- Per-product resources: `a[href^="/getmedia/"]` with link text as title
- Related models: `~/Products/Catalog/.../<MODEL>` hrefs

## Category page structure
Inline `var request = { documentId, nodeId, pageSize, culture, siteName,
sort, offset, categoryCount }` drives the tile endpoint. `dId`/`nId` are
scrapeable from the server-rendered category page.
Subcategory nav carries live product counts, e.g. "Video Endpoint (20)".

## Auth
User was **not** logged in during capture (`#User_ID` = 65 = anonymous;
"Sign In" link present). Only analytics/consent cookies observed — no
Crestron session cookie captured.
Firmware/software detail pages 302 to `/login?returnurl=...`, so a
cookie-based session would be required to reach downloads.
Firmware **version + release date remain public** in search results.

## Not observed
- Product filter facets (`fltr` param) — the sampled category exposes none;
  other categories may. Format is `name:value;` pairs.
- Compare API (`/Handlers/CompareHandler.ashx`) — requires product selection.
- `PublicPricingHandler.ashx` returned `[]` for sampled models; advertised
  pricing appears to be absent for these SKUs rather than the call being wrong.
- `/handlers/quicksearch.ashx` returns HTTP 500 server-side — broken upstream.

## Replayability
PASS. Every shipping surface is plain-HTTP replayable. No clearance cookie,
no Surf fingerprint, no page-context execution required.

## Auth validation (post-capture, user signed in mid-run)
Cookie replay **VERIFIED** over plain HTTP. Required cookies (names only):

| Cookie | HttpOnly | Domain | Role |
|---|---|---|---|
| `.ASPXFORMSAUTH` | yes | www.crestron.com | ASP.NET forms auth ticket (primary) |
| `.WebApp.Cookies` | yes | .crestron.com | app identity |
| `ASP.NET_SessionId` | yes | www.crestron.com | session |
| `CMSCsrfCookie` | yes | www.crestron.com | Kentico CSRF |
| `cf_clearance` | yes | .crestron.com | Cloudflare clearance (present, not required) |

All are `HttpOnly`, so `document.cookie` cannot read them — capture must use
CDP `Network.getAllCookies` (i.e. the generated CLI's `auth login --chrome`
must read Chrome's cookie store, not JS).

Proven with replay:
- `GET /Software-Firmware/Firmware/DigitalMedia/DM-NVX-384(C)/7-4-0255-22319`
  → 200, yields version, Last Modified, **release notes**, **change log**, and a
  direct download href.
- `GET /firmware_files/dm-nvx/dm-nvx-38x_7.4.0255.22319_r598399.zip`
  → 206 `application/x-zip-compressed`, **range requests supported**
  (resumable downloads). Same URL anonymously → 302 to `/login`.

Auth mode for spec: `type: cookie`, browser-imported, no header composition
needed — cookies replay directly.

## Resource Library filter contract (refined)
`/Support/Resource-Library` is the hub page; it links into the same
`/Support/Search-Results` endpoint. Full param contract:

- `q` — free-text query (model number, product family, keyword)
- `c` — category: 0 All, 1 Marketing, 3 Technical, 4 Software & Firmware,
  5 Spec Sheets, 6 Guides & Manuals, 7 Specifier, 8 Multimedia, 11 Certificates
- `type` — sub-type facet **within** a category. Verified values seen:
  `Firmware`, `Software`, `Advanced-Support-Tools` (within c=4);
  `Manuals-Guides`, `Spec Sheets`, `Web CAD Drawings` (cross-category c=0).
  An out-of-category `type` yields 0 rows, so `type` must be paired with a
  compatible `c` (or `c=0`).
- `o` — sort: empty = **Relevance**, `Created:desc` = Last Updated,
  `Name_Search.keyword:asc` = A-Z, `Name_Search.keyword:desc` = Z-A
- `p` — page (1-based), `m` — page size (10 | 25 | 50; 25 verified to return 25 rows)

Result row fields: title, detail/asset href, type, date. `/getmedia/` hrefs are
direct public assets; `/Software-Firmware/` hrefs are login-gated detail pages.

## Sync path proof (end-to-end, live)
Verified the complete catalog crawl chain works over plain HTTP:

1. `GET /sitemap` → **79** `/Products/Catalog/...` category paths
2. `GET <category-path>` → inline `var request` yields `documentId`, `nodeId`,
   `categoryCount`, plus a subcategory `<option>` list
3. `GET /CMSPages/ProductSubcategoryItemTemplate.aspx?dId=&nId=&ps=50&os=0&...`
   → `<span id="productCount">N</span>` + `<p class="model-number">MODEL</p>` tiles

Measured results:

| Category | dId | nId | count | parsed (ps=50, os=0) |
|---|---|---|---|---|
| AV-Over-IP/DM-NVX-AV-Over-IP | 30989 | 31385 | 42 | 42 |
| AV-Over-IP/h-264-Streaming | 30999 | 31395 | 0 | 0 |
| Accessories/Keypad-Faceplates-Covers | 30983 | 31379 | 73 | 52 |

The 73→52 case confirms page size caps below the category total, so the syncer
must page with `os` until `parsed >= productCount`. `productCount` gives the
loop a definite termination condition — no guessing.

## Firmware family mapping (Crestron-specific pattern)
One firmware release covers an entire model **family**. Live example:
`"TSW-570/TSW-770/TSW-1070/TSS-770/TSS-1070/TS-770/TS-1070 3.0.x"` — a single
release row covering **7 distinct models**. Release titles embed a
slash-delimited model list followed by the version string.

Consequence: model → firmware is many-to-many and must be normalized at sync
time (split the title on `/`, strip the trailing version). No single page on the
site answers "which release covers model X" — this is a local-join-only query.

## Product-page AJAX handlers (captured via Performance API)
The first interceptor pass missed these because they fire during page load.
Re-captured with `performance.getEntriesByType('resource')`, then each was
re-verified over plain anonymous curl.

| Handler | Params | Returns | Verified |
|---|---|---|---|
| `/Handlers/ResourceHandler.ashx` | `dID=<DocumentID>` | **20 per-product resources** — CAD, Revit, Guide Spec, Product Info, Security Reference Guide, Product Manual, Spec Sheet, User Guide | anon 200 |
| `/Handlers/VariantProduct.ashx` | `load=list&Product_Name=<Series>&ClassName=&DocumentCulture=` | "Available Models" table for the series: model, internal id, description, price column | anon 200 |
| `/Handlers/VariantProduct.ashx` | `load=dropdown&Model_Number=&DocumentID=&Product_Name=&ClassName=` | model switcher for the series | anon 200 |
| `/Handlers/OptionalAccessoriesHandler.ashx` | `ids=<semicolon-separated>` | accessories table: model, description, price | anon 200 |
| `/Handlers/RelatedProducts.ashx` | `NodeGUID=&DocumentCulture=&DocumentID=&ParentNodeID=&hasRelatedProduct=True&ParentRelativeURL=` | related products block | anon 200 |
| `/Handlers/ReplacementProductsHandler.ashx` | `ids=&label=&culture=` | **replacement products for discontinued items** | anon 200 |
| `/handlers/Header.ashx` | `p=authlinks` | auth-state fragment (useful for `auth status`) | anon 200 |

**Both handler families are GET with query params**, not POST — an early POST
attempt against `VariantProduct.ashx` returned an empty body and was corrected
by reading the jQuery `$.ajax({type:"GET", ...})` call site in `bundle-dev.min.js`.

**DocumentID discovery:** the product page carries `data-id="<DocumentID>"`
(e.g. `21965` for DM-NVX-360) and the same value appears in the tile markup's
`data-id` and in `VariantProduct`'s `DocumentID` param. So the chain is:
product page → `data-id` → `ResourceHandler.ashx?dID=` → per-product asset list.

**Series grouping:** `Product_Name` is a *series* (e.g. `DM-NVX-35 Series`), not
a model. Variants resolve series → member models, mirroring the same
family-level grouping seen in firmware releases.

## Final replayability verdict
**PASS — no browser required at runtime.** Every surface above was re-verified
over plain anonymous `curl` after browser discovery. The browser was needed only
to *discover* the page-load handlers and to *import cookies* for the gated
firmware paths. The printed CLI ships plain `net/http`.

## Product lifecycle surface (verified)
Discontinued products stay reachable under `/Products/Catalog/Inactive/Discontinued/<Letter>/<MODEL>`
and keep their full resource list.

- **End-of-Sale notices are a dated, searchable stream.** `c=6` + `q=End-of-Sale`
  + `o=Created:desc` returns a chronological EOS feed (verified: IV-CAMHK, PC-300,
  SFP-1G-BX-D, ZUMMESH JBOX, SFP-1G-BX-U all dated Jul 31, 2026). This makes
  "what got end-of-saled this month?" answerable — the site itself offers no
  such view.
- Discontinued product pages carry `id="replacement-products"` with a
  `data-ids="22053;22054;22052;22883;22885"` attribute feeding
  `ReplacementProductsHandler.ashx`, so discontinued → replacement is traversable.
- A discontinued product's `ResourceHandler.ashx?dID=` still returns its assets
  including its own End-of-Sale Notice PDF (verified for UC-FCM-Z, dID 21931).

Combined with the family-scoped firmware mapping, this gives three independent
lifecycle signals per model: latest firmware release, EOS announcement date, and
replacement chain — none of which the website surfaces together.
