# Crestron.com CLI Brief

## API Identity
- **Domain:** Crestron.com website content — product catalog, firmware/software
  releases, spec sheets, manuals, certificates, CAD/Revit drawings.
- **Users:** AV integrators, Crestron programmers, field technicians, system
  designers, spec writers, IT/AV managers maintaining installed fleets.
- **Data profile:** ~thousands of SKUs across 10 top-level categories and 127
  category paths; a large, dated resource library (firmware, spec sheets,
  manuals, certs, drawings) that changes continuously. Highly cacheable —
  entities are stable, releases are append-mostly.
- **Not an official API.** Crestron publishes no public developer API for
  website/catalog content. Every surface below is a server-rendered page or an
  internal AJAX handler, discovered and verified live during this run.

## Reachability Risk
- **None.** `probe-reachability` → `mode: standard_http`, confidence 0.95.
  Plain stdlib HTTP returns 200. No Cloudflare challenge, no bot detection, no
  TLS-fingerprint requirement. (A `cf_clearance` cookie exists in-browser but is
  not required for replay.)
- Probe-safe endpoint used: `GET /Support/Search-Results?q=DM-NVX&c=4&p=1&m=10&o=0`
- One upstream defect noted: `/handlers/quicksearch.ashx` returns HTTP 500. Do
  not build on it.

## Verified Contract

| Surface | Endpoint | Auth | Format |
|---|---|---|---|
| Resource search (8 categories) | `/Support/Search-Results?q=&type=&c=&p=&m=&o=` | none | HTML |
| Product tiles per category | `/CMSPages/ProductSubcategoryItemTemplate.aspx?dId=&nId=&ps=&os=&cult=&sn=&sort=&fltr=` | none | HTML fragment |
| Product detail | `/Products/Catalog/<path>/<MODEL>` | none | HTML + JSON-LD |
| PDF/CAD assets | `/getmedia/<guid>/<file>` | none | PDF/binary |
| Category taxonomy | `/sitemap` (127 product paths) | none | HTML |
| Pricing regions | `/Handlers/PublicPriceRegionOptionsHandler.ashx` | none | JSON |
| Firmware detail + release notes | `/Software-Firmware/<Type>/<Family>/<Model>/<Version>` | **cookie** | HTML |
| Firmware binary | `/firmware_files/...zip` | **cookie** | zip, range-capable |

Search filters — `c`: 0 All, 1 Marketing, 3 Technical, 4 Software & Firmware,
5 Spec Sheets, 6 Guides & Manuals, 7 Specifier, 8 Multimedia, 11 Certificates.
Sort `o`: `Created:desc`, `Name_Search.keyword:asc|desc`. Page size `m`: 10/25/50.

Product detail pages carry schema.org JSON-LD (`name`, `sku`, `description`,
`brand`, `image[]`), a spec table (`#panel2`, `td.productSpecTDHead` section
headers with 30%/70% key/value rows), per-product `/getmedia/` resource links,
and related-model links. Zero same-origin XHR — fully server-rendered.

## Top Workflows
1. **"What's the latest firmware for `<model>`, and what changed?"** — the single
   most common question. Version + date are public; release notes and changelog
   need a session. Today this is 5+ clicks through a slow site.
2. **"Get me the spec sheet / manual / CAD / Revit for `<model>`."** — spec
   writers and designers pulling assets into submittals and drawing sets.
3. **"What are the specs for `<model>`?"** — resolution support, bit rates,
   power, dimensions, without opening a PDF.
4. **"Is this product discontinued, and what replaced it?"** — the catalog
   surfaces `Inactive/Discontinued` paths and End-of-Sale notices.
5. **"What's new across my installed fleet since last month?"** — no such view
   exists on the site at all; requires per-model manual checking.

## Table Stakes
- Search resources by keyword, filter by category/type, sort, paginate.
- Look up a product by model number; show description, specs, images.
- List every downloadable asset for a product.
- Download PDFs and firmware.
- Browse the category taxonomy.

## Data Layer
- **Primary entities:** `product` (model, sku, description, category path,
  specs, images), `resource` (title, type, date, url, product refs),
  `firmware_release` (model family, version, date, release notes, download url),
  `category` (path, parent, dId/nId, product count).
- **Sync cursor:** resource/firmware `date` field (`Created:desc` sort makes
  incremental sync natural — page until dates predate the cursor).
- **FTS/search:** full-text over product name + description + spec text +
  resource titles + firmware release notes. Release-note FTS is the standout:
  that text is behind a login and has never been searchable across versions.

## Codebase Intelligence
Discovered by reading the site's own JS during this run:
- `App_Themes/Crestron/js/CategoryPage.min.js` defines the product-tile call and
  its `fltr` filter format (`name:value;` pairs).
- `bundle.min.js` / `bundle-dev.min.js` enumerate all `/Handlers/*.ashx`
  endpoints (pricing, related products, compare, dealer, resource list).
- Category pages embed `var request = { documentId, nodeId, ... }` — the
  `dId`/`nId` pair needed to page a category is scrapeable server-side.

## Auth
`type: cookie`, browser-imported. Required cookies are all `HttpOnly`
(`.ASPXFORMSAUTH`, `.WebApp.Cookies`, `ASP.NET_SessionId`, `CMSCsrfCookie`), so
capture must read Chrome's cookie store via CDP — `document.cookie` cannot see
them. Verified: replaying these over plain curl unlocks firmware release notes
and yields a 206 range-capable binary download. Anonymous access to the same
URL 302s to `/login`.

**Design consequence:** the CLI must be fully useful logged-out. Product
catalog, specs, spec sheets, manuals, certificates, drawings, and firmware
*version + date* are all public. Only release notes and the binary need a
session. Auth is an upgrade, not a gate.

## User Vision
User scoped this explicitly to **Crestron.com website content** (product
catalog, firmware/downloads, documentation lookup) rather than device/runtime
control (XiO Cloud, control processors). Cookie-login support was approved so
the CLI can reach gated firmware downloads.

## Product Thesis
- **Name:** `crestron-pp-cli`
- **Why it should exist:** Crestron.com is a slow, click-heavy Kentico site with
  a broken quick-search endpoint and no API. The information integrators need
  most — *what firmware is current, what changed, and where's the spec sheet* —
  takes minutes of navigation per model and cannot be scripted, diffed, watched,
  or asked about in bulk. Every existing community tool (see absorb manifest)
  handles firmware *transfer to devices*; none makes the *catalog and release
  library* queryable. A local SQLite mirror turns a fleet-wide question like
  "which of my 40 models have new firmware since March?" from an afternoon of
  tab-opening into one command.

## Build Priorities
1. Data layer + sync for products, resources, firmware releases, categories.
2. Resource search, product lookup, spec display, asset download (public path).
3. Cookie auth (`auth login --chrome`) → release notes + firmware download.
4. Transcendence: fleet firmware-status checks, release-note diffing across
   versions, offline FTS over release notes, discontinued/replacement tracing.
