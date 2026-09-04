# Creative Fabrica CLI Brief

## API Identity
- Domain: Digital design-asset marketplace. Fonts, graphics (illustrations, icons,
  clip art, patterns, textures, backgrounds, digital papers), crafts/POD files
  (SVG, PNG, DXF, EPS, sublimation, embroidery PES/etc, laser-cut), and templates
  (planners, invitations, worksheets, social graphics). 1M+ premium designs.
  Also an "AI Studio" (generative tools) and a designer/seller side.
- Users: Cricut/Silhouette crafters (cut files), print-on-demand sellers
  (commercial-license assets, sublimation), graphic designers (fonts/graphics/
  templates), embroidery hobbyists. Mostly subscription members ($3.99-$29/mo:
  Crafts / Fonts / Graphics / All-Access tiers).
- Data profile: searchable catalog keyed on product → {designer, category, file
  formats, license, tags, price/subscription-eligibility, thumbnail, slug/url}.
  Strong secondary entities: designers, categories/taxonomy, daily freebies
  (with 24h expiry), and (auth-gated) the member's library/downloads/favorites.

## Reachability Risk
- [High] www.creativefabrica.com returns HTTP 403 + Cloudflare "Just a moment..."
  challenge to plain HTTP. api.creativefabrica.com resolves (Cloudflare IP) and
  404s on guessed paths — a real internal JSON gateway with undocumented routes.
- No official/public API, no developer portal (developers./docs./partner. all
  NXDOMAIN), no SDK, no community wrapper. This is a website-itself build.
- Runtime transport TBD by `probe-reachability`: expect browser_http (Surf clears
  challenge) or browser_clearance_http (needs Chrome clearance cookie). Real
  endpoints + clearance strategy come from Phase 1.7 browser-sniff.
- Probe-safe endpoint: none declared yet; discover GET search/listing routes.

## Top Workflows
1. Daily freebies harvest — find today's free font + craft + graphic before the
   24h window closes; grab all formats (SVG/PNG/DXF/EPS). Crafters do this daily.
2. Search + filter the catalog by category, file format, license (commercial),
   style/keyword; sort by new/popular; then open/download.
3. Manage the member library — track downloads, organize favorites/collections,
   see what's new since last visit (auth-gated).
4. Designer/seller research — browse a designer's catalog, spot bestsellers and
   new releases (for buyers tracking favorite creators; for sellers, competitor
   research).
5. POD license verification — confirm an asset is commercial-use before putting
   it on a product.

## Table Stakes
- Category/keyword search with filters (file type, license, style/color).
- New / trending / popular sorting; pagination.
- Daily freebies listing.
- Favorites / collections / download history (auth).
- Designer profile + their products.
- Asset detail (formats, license, designer, tags).

## Data Layer
- Primary entities: products, designers, categories, freebies. Auth: library
  (downloads), favorites/collections.
- Sync cursor: per-category "new since" timestamp; freebies by day; favorites by
  updated-at. Snapshot search results for offline + diffing.
- FTS/search: product title + tags + designer name + category → offline search.
- Time-boxed data: freebies carry an expiry — store first-seen + expires-at so a
  command can answer "what's free today / expiring soon / new since last sync".

## Source Priority
- Single source (creativefabrica). No combo. BROWSER_SNIFF_TARGET_URL set;
  spec comes from Phase 1.7 capture of www + api.creativefabrica.com.

## Product Thesis
- Name: creativefabrica-pp-cli  (display: "Creative Fabrica")
- Why it should exist: No CLI exists for Creative Fabrica. The web UI is heavy,
  upsell-laden, and forgets the 24h freebie window. A CLI gives fast scriptable
  search, agent-native JSON, a local SQLite mirror of your library/favorites, and
  three things the site cannot: (a) daily-freebie tracking + "expiring soon"
  alerts, (b) download/library diffing ("what's new since last sync"), and
  (c) POD-license filtering (only commercial-use assets). Offline, composable,
  agent-native.

## Build Priorities
1. Data layer for products / designers / categories / freebies (+ auth library,
   favorites) with FTS search.
2. Absorbed surface: search + filters, category/new/popular listings, freebies
   listing, designer pages, asset detail, (auth) downloads/favorites.
3. Transcendence: freebie tracker ("today", "expiring", "new"), library/search
   diffing, POD-commercial-license filter, designer-watch, collection export.
