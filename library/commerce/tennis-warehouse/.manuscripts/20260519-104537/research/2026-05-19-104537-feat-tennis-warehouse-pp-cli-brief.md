# Tennis Warehouse CLI Brief

## API Identity
- Domain: `tennis-warehouse.com` — premier online tennis-gear retailer (United States).
- Users: serious recreational and competitive tennis players who care about racquet specs (head size, weight, swingweight, stiffness, string pattern) and price.
- Data profile: static SSR HTML — large category pages, brand catalogs, product detail pages, used-racquet inventory listings. No documented API.

## Reachability Risk
- **Low.** HEAD probes against `usedracquets.html` and `Tennis_Racquets/catpage-RACQUET.html` returned `HTTP/2 200`, `text/html;charset=ascii`, no Cloudflare/WAF/bot-detection headers. Plain `curl` with a Chrome UA is enough to reach the pages. No GitHub issues to scan because there is no community wrapper.
- `probe-reachability` should classify this as `standard_http`. No clearance cookie required.

## Top Workflows
1. **Browse used racquets by brand and condition grade** — "Show me Grade A Wilson Blade 98 listings under $150."
2. **Compare new racquet specs side-by-side** — "Which Babolat Pure Aero models have a strung weight under 11.5oz and a 16x19 string pattern?"
3. **Track price drops** — "Watch this used Yonex EZONE 100 and tell me when a new listing drops below $130."
4. **Find a substitute for a discontinued racquet** — "I love the Wilson Blade 98 v7 — what current racquet matches its head size, weight, swingweight, and string pattern?"
5. **Demo program planning** — "List demo-eligible racquets from Head and Yonex that match my swingweight target of 325."

## Table Stakes
- Filter racquets by brand, head size, weight, string pattern, length, price range.
- Drill from listing to detail page (spec sheet).
- Browse used inventory by condition grade (Unused / A / B / C).
- Read full spec sheet on a single racquet.
- Sort by price.

## Data Layer
- **Primary entities:**
  - `racquet` — a new (catalog) racquet model. Fields: brand, model, sku, head_size, length, strung_weight, unstrung_weight, balance, swingweight, stiffness, beam_width, string_pattern, composition, grip_sizes, color, price, msrp, url, image_url, power_level, status (new/reduced/clearance), rating, review_count, description, last_seen_at.
  - `used_listing` — a specific used racquet listing. Fields: pcode (sku), brand, model, grip_size, condition_grade (Unused/A/B/C), price, msrp, url, image_url, notes, first_seen_at, last_seen_at, sold (bool).
  - `brand` — Wilson, Babolat, Head, Prince, Yonex, Tecnifibre, Dunlop, Volkl, ProKennex, Solinco, Mizuno, Lacoste. Used-only brands subset.
  - `price_snapshot` — historical (used_listing_id or sku, captured_at, price). Enables drop tracking.
- **Sync cursor:** none provided — full crawl per brand catalog + used-by-brand page. Use ETag / If-Modified-Since when supported; cache rendered HTML by URL.
- **FTS/search:** SQLite FTS5 over `racquet (brand, model, composition, description)` and `used_listing (model, notes)`.

## Codebase Intelligence
- No public SDK, MCP, or wrapper exists. This is greenfield retail-HTML CLI work.

## Reverse-engineering signals (from direct probes)
- **URL patterns confirmed:**
  - Used by brand: `https://www.tennis-warehouse.com/usedcatpage.html?ccode=<CCODE>` where `<CCODE>` ∈ {`BABRACS`, `DUNLOPRACS`, `HEADRACS`, `KENNEXRACS`, `PRINCERACS`, `SOLINCORAC`, `TECRACS`, `VOLKLRACS`, `WILSONRACS`, `YONEXRACS`, `NEWLC` (new/Lacoste?), `RACSBYMAKER` (index)}.
  - Used product detail: `https://www.tennis-warehouse.com/orderusedproduct.html?pcode=<SKU>` (e.g., `PS9818`, `PS1619`).
  - New racquet brand catalog: `/Wilsonracquets.html`, `/Babolatracquets.html`, etc.
  - New racquet detail: `/<Brand>_<Model_Name>/descpageRC<BRAND>-<SKU>.html` (e.g., `/Wilson_Blade_98_16x19_v10/descpageRCWILSON-WB9810.html`).
  - All-racquets root: `/Tennis_Racquets/catpage-RACQUET.html`.
- **Page envelope:** static SSR HTML; pages render product cards with title + price + thumbnail. Detail pages carry the full spec sheet. Embedded `<script type=` blocks suggest some JSON-LD or analytics — Phase 1.7 to confirm.
- **No bot protection** on HEAD probes; user-agent gating likely but not strict.

## Condition grade definitions (from `usedracquets.html`)
- **Unused** — not been hit with but may have minor cosmetic defects.
- **Grade A** — very little use; wear evident on grip and grommets only.
- **Grade B** — used and shows some minor cosmetic wear.
- **Grade C** — clear wear from groundstrokes in multiple places.

## Demo program
- Tennis Warehouse runs a paid-deposit demo ($25 for up to 3 racquets, 1 week). Demo-eligibility per racquet is a useful field if browser-sniff can extract it.

## Top competitors / inspiration
- **Tennis Express** (`tennisexpress.com`) — similar retail catalog, also HTML SSR.
- **Tennis Plaza** — catalog + reviews.
- **Holabird Sports**, **Racquet World**, **Amazon Tennis** — adjacent retail.
- No competing CLI or MCP for any of these exists (verified via Step 1.5a). This is a greenfield agent surface.

## User Vision
- (User chose "Let's go" — no explicit vision given. Default to the workflows above.)
- Two seed URLs explicitly supplied by user: `usedracquets.html` (used inventory) and `Tennis_Racquets/catpage-RACQUET.html` (all new racquets). Both must be reachable as headline command roots in the CLI.

## Product Thesis
- **Name:** `tennis-warehouse-pp-cli`.
- **Why it should exist:** Tennis Warehouse has the best racquet catalog and used-inventory selection in the U.S., but the web UI is browse-only and the data is locked behind page navigation. A CLI with a local SQLite cache lets a player (or an agent) ask spec-driven and price-driven questions that the website itself can't answer: "find a Wilson 98sq under 11.5oz with a 16x19 pattern, sort by swingweight" or "alert me when a Grade A Babolat Pure Drive drops below $120." No competing tool — agentic or otherwise — exists.

## Build Priorities
1. **Sync used-racquet inventory by brand** — crawl `/usedcatpage.html?ccode=<X>RACS` for each brand, then `/orderusedproduct.html?pcode=<sku>` for spec/condition extraction. Populate `used_listing` + `price_snapshot`.
2. **Sync new-racquet catalog by brand** — crawl `/<Brand>racquets.html` and `descpageRC<BRAND>-<SKU>.html`. Populate `racquet`.
3. **Filter + search commands** for both inventories — `racquets list --brand wilson --head-size 98 --string-pattern 16x19`, `used list --brand babolat --grade A --max-price 150`.
4. **Spec-compare** — `racquets compare WB9810 WB9818` (two SKUs).
5. **Substitute finder** — given an SKU, find current models with similar specs (head_size ±2sq, strung_weight ±5g, string_pattern match).
6. **Price drop watch** — `used watch <pcode>` records to `price_snapshot`; `used drops --since 7d` lists recent drops.
7. **Demo eligibility** — surface demo-program flag if discoverable from product pages.

## Anti-reimplementation note
All commands must call the live site (HTTP) or read from the local store populated by sync. No hand-rolled spec data or fake endpoint stubs.
