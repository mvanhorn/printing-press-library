# AVer USA CLI Brief

## API Identity
- **Domain:** averusa.com — the US arm of AVer Information Inc. (Taiwanese AV manufacturer). Product lines: document cameras (AVerVision F-series), conference cameras / video bars (CAM, VC, VB series), professional PTZ cameras (TR, PTC, RZ series), charging carts, audio solutions, room solutions, accessories.
- **Users:** AV integrators and installers, IT/AV admins, education technology coordinators, procurement/RFP writers, field techs who need the right manual or spec sheet fast, often offline or in the field.
- **Data profile:** A product catalog (categories → models, each a static HTML page) plus a PDF document library. Document types observed: **user manual**, **spec sheet**, **datasheet**, **white paper**, **quick start guide**, **software manual**, **brochure**, **comparison chart**. PDFs are static and directly downloadable over plain HTTP.

## Reachability Risk
- **Low-Medium.** `https://www.averusa.com` answers plain curl with HTTP 200 and real HTML (no WAF/bot wall observed on homepage, product pages, or PDFs). Two 61301-byte soft-404 shells exist for guessed paths (site returns 200 + "404 | AVer USA" title).
- Direct PDF fetch verified (200, `application/pdf`): `/pro-av/downloads/user-manual/AVer PTZ Link User Manual EN v1.02_2021.07.12.pdf`, `/pro-av/downloads/software/Q-SYS Plugin for AVer PTZ Cameras User Maual.pdf`.
- **Salesforce support portal** `https://averusa.my.site.com/support/s/` returns a 200 Aura/Experience-Cloud SPA skeleton (JS-driven; article/search/`s/` subpaths 404 via plain HTTP). Manuals/spec sheets per product are served from this portal — endpoint discovery requires browser-sniff or JS execution.
- Directory listings are blocked: `/business/downloads/` → 403; `/pro-av/downloads/<type>/` → 403 (only individual file URLs work). Document catalog **cannot be enumerated by directory walking**; enumeration needs the support portal, product pages, or a search surface.
- No community wrapper/CLI ecosystem exists (GitHub search for AVer CLI/wrapper: none), so no 403-issue signal. main site reachability is good.

## Top Workflows
1. **"Get the manual"** — AV tech needs the user manual for a specific model (e.g., CAM570) to set it up at a job site; wants a direct PDF, not the portal maze.
2. **"Get the spec sheet"** — integrator writing an RFP/bid needs the spec sheet (datasheet) for a model; wants it as PDF or as extractable spec fields.
3. **"Compare models"** — spec-sheet comparison across models in a category (VC vs CAM vs PTZ) before recommending a purchase.
4. **"Background paper"** — evaluation of AVer technology via white papers (e.g., for education/AV decisions).
5. **"Offline library"** — maintain a local, searchable catalog of AVer documents (title/type/model) with on-demand PDF download, usable without the portal.

## Table Stakes
- **manualslib.com / manualzz.com / manualsonline.com** — search manuals by brand+model, browse pages, download PDF. Weaknesses: ad-heavy, stale/mislabeled documents, no direct vendor link, no spec-sheet or white-paper coverage, no CLI.
- **AVer's own support portal** — official but JS-heavy, click-through per product, no bulk/type filtering, no offline.
- No existing CLI, MCP server, or SDK for AVer documents → every feature is a beat.

## Data Layer
- Primary entities: `products` (category, model slug, name, product URL), `documents` (product/model, doc type, title, PDF URL, file size, published date), `categories`.
- Doc-type enum (from the user's ask + observed surface): `user-manual`, `spec-sheet`, `datasheet`, `white-paper`, `quick-start`, `software`, `brochure`, `comparison-chart`.
- PDFs are binary → store metadata + local file path in SQLite; do not inline blobs.
- Sync cursor: catalog crawl of `/products/<category>/<model>/` pages + support-portal article walk; PDF HEAD for size/date.
- FTS: product names + document titles + model slugs.

## Product Thesis
- **Name:** `averusa-pp-cli`
- **Why it should exist:** Every AVer document, one command away. Type-filtered search across user manuals, spec sheets, and white papers with a synced offline catalog and one-command PDF download — no portal clicking, no manualslib ads, no waiting for the Salesforce SPA.

## Build Priorities
1. `products list` / `products get` — category/model catalog from `www.averusa.com` (plain HTTP crawl).
2. `docs search <query> --type user-manual|spec-sheet|white-paper` — type-filtered document search over the local catalog.
3. `docs download <doc-id>` — fetch the PDF to a local folder (dry-run friendly, `--out`).
4. `docs sync` — refresh catalog metadata from product pages + support portal.
5. Novel: `compare <modelA> <modelB>` — side-by-side spec fields from spec sheets; `doctor` reachability check.
