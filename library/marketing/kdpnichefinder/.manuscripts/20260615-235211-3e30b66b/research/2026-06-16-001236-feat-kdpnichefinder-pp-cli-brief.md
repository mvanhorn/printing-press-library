# KDP Niche Finder CLI Brief

## API Identity
- Domain: KDP (Amazon Kindle Direct Publishing) niche/book research. Brand front (kdpnichefinder.com) over a Laravel/Inertia app; niche data sourced via Everbee + Amazon listing data.
- Users: Low-content & full-text KDP self-publishers researching profitable book niches before they write/publish.
- Data profile: Curated niche "buckets" of real Amazon books, each with estimated monthly sales & revenue. Users browse/search, then save favorites into folders.

## Reachability Risk
- None. probe + HAR show standard_http (confidence 0.65), no WAF/challenge. Session-cookie auth.
- Tier/permission hints from 4xx body: none observed (authenticated 200s throughout).
- Probe-safe endpoint: GET /api/categories (read-only).

## Top Workflows
1. **Browse a niche bucket** → `GET /app/category/{evergreen|fresh_money|hidden_gems|high_ticket}` → see books ranked by est. sales/revenue.
2. **Search within a bucket** → `?search=<term>` → filter the curated set to a topic.
3. **Triage & save** → favorite promising books (`toggle-save`) into named folders.
4. **Review saved shortlist** → `GET /app/saved-books` / folders.
5. **(Offline-only, our value-add)** → cross-bucket SQL/search, ASIN extraction, revenue snapshots over time, CSV export for KDP keyword work.

## Table Stakes (vs Publisher Rocket / Book Bolt / Self Publishing Titans / KDSPY)
- Keyword/niche search with demand metric (est. sales/revenue) ✓ (have it)
- BSR→sales estimation ✓ (est. sales/revenue provided directly)
- Reverse-ASIN / competitor inspection (derive: amazon_url → ASIN, group by publisher)
- Category/sub-category discovery ✓ (4 curated buckets + /api/categories)
- CSV export (we add)
- Multi-marketplace (KDP Niche Finder is US-centric; out of scope v1)

## Data Layer
- Primary entities: Book (niche), Category (bucket), Folder, SavedBook, User.
- Sync cursor: page-based per category bucket; full re-sync of buckets is cheap (small curated sets).
- FTS/search: full-text over book title; SQL across all buckets for opportunity ranking.
- Snapshots: store estimated_monthly_sales/revenue with timestamp → trend/drift over time (no upstream history API).

## Codebase Intelligence
- Laravel + Inertia.js. GETs need session cookie only; POSTs need session + X-XSRF-TOKEN (decoded XSRF-TOKEN cookie). Inertia routes return JSON envelope `{component, props, url, version}`; list at props.books.data[]. /api/* routes are plain JSON.

## User Vision
- User confirmed their logged-in dashboard contains the niche/keyword research tools (not just the public Artistly generators). Build for the real dashboard endpoints. No further vision constraints given ("Let's go").

## Product Thesis
- Name: kdpnichefinder (CLI: kdpnichefinder-pp-cli)
- Why it should exist: KDP Niche Finder is a web-only, click-to-browse tool. Serious publishers want to rank niches by opportunity, dedupe across buckets, track whether a niche's revenue is rising or fading, extract ASINs/publishers for competitor study, and export shortlists — none of which the web UI does. A local SQLite mirror + offline SQL/search + revenue snapshots turns a browse tool into a research database an agent can drive.

## Build Priorities
1. Data layer: Book/Category/Folder/SavedBook entities; sync all 4 buckets into SQLite.
2. Absorb: category browse + search, folders CRUD, save/unsave, user, saved-books — all with --json/--select/--limit.
3. Transcend: opportunity ranking, cross-bucket dedup, revenue snapshots/drift, ASIN/publisher extraction, CSV export, offline FTS/SQL.
