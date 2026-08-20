# NEJM CLI Brief

## API Identity
- Domain: New England Journal of Medicine (nejm.org) — the world's most-cited general medical journal. Atypon/Literatum-powered website. No public developer API.
- Users: clinicians, researchers, med students, journal-club organizers, and AI agents that need current peer-reviewed medical literature metadata.
- Data profile: articles (research, reviews, case reports, perspectives, editorials), organized into weekly issues (volume/issue), tagged by specialty and topic, each with a DOI, authors, abstract, publication date, article type, and free/paywalled flag.

## Reachability Risk
- **Low-Medium / mixed by path.** NEJM is fronted by Cloudflare, but protection varies:
  - RSS feeds (`/action/showFeed`): **plain HTTP 200**, no challenge — richest, most reliable source.
  - Article pages (`/doi/full/{doi}`), issue TOC pages, homepage: **403 to plain stdlib, 200 via Surf Chrome-TLS** (`browser_http`). Surf transport clears it; no clearance cookie needed.
  - Search (`/action/doSearch`): **403 even via Surf** (`browser_clearance_http`). Live search is NOT viable → search is offline FTS over synced content.
- Probe-safe endpoint used: `GET /action/showFeed?jc=nejm&type=etoc&feed=rss` → 200; `GET /doi/full/10.1056/NEJMoa2506905` → 200 via Surf.
- Tier/permission hints: full article text is subscription-gated (`isFree` meta flag distinguishes free vs paywalled); abstracts + metadata are public. CLI targets public surfaces only — no paywall circumvention.

## Discovery method (browser tools skipped by user)
Manual discovery via `cli-printing-press probe-reachability` (transport classification) + direct Surf/browser-UA fetches + HTML/RSS structure analysis. No browser-use/agent-browser automation. Findings:
- **eTOC RSS** `https://www.nejm.org/action/showFeed?jc=nejm&type=etoc&feed=rss` — RSS 1.0/RDF + PRISM. ~70 items = current issue + ahead-of-print. Per item: title, link, dc:creator (authors), dc:date, prism:doi, prism:url, prism:volume, prism:number, prism:startingPage, prism:endingPage, prism:coverDate, description (citation string, no abstract).
- **Recently-published RSS** `...&type=axatoc&feed=rss` — recently published articles, same shape.
- **Article detail** `/doi/full/{doi}` (Surf) — server-rendered meta tags: `dc.Title`, `dc.Creator`, `Description`/`shortAbstract`/`og:description` (abstract), `dc.Date`, `articleType`, `articleCategory`, `Specialties`, `topics`, `isFree`, `isOnlineFirst`, `og:image`.
- **Specialty index** `/browse/specialty` — static links to 14 specialties.
- **Issue TOC / specialty-article pages** — SPA shells (article lists are JS-rendered; not in server HTML). NOT used for list extraction; RSS is the list source.
- **14 specialties:** cardiology, climate-change, clinical-medicine, endocrinology, gastroenterology, health-policy, hematology-oncology, infectious-disease, nephrology, neurology-neurosurgery, obstetrics-gynecology, pediatrics, pulmonary-critical-care, surgery.

## Top Workflows
1. "What's in the current issue?" — list current-issue + ahead-of-print articles with authors, type, pages.
2. "What did NEJM publish recently?" — recently-published feed, filterable.
3. "Show me this article" — fetch an article by DOI: title, authors, abstract, type, specialty, free/paywalled.
4. "Find NEJM articles about X" — offline full-text search over synced article metadata/abstracts.
5. "Filter to my specialty / only free / by type" — slice synced articles by specialty, article type, free flag, date.

## Table Stakes
- Browse current issue / recently published.
- Fetch article metadata + abstract by DOI.
- Search/filter the corpus (offline FTS).
- List specialties; filter by specialty.
- `--json` / agent-native output, local SQLite cache, `doctor` health check.

## Data Layer
- Primary entity: **article** (doi PK, title, authors, abstract, article_type, specialties[], topics[], volume, issue, start_page, end_page, pub_date, cover_date, is_free, url, image_url, source_feed).
- Secondary: **issue** (volume/issue, cover_date, article DOIs) derived from synced articles.
- Sync cursor: feed re-fetch (eTOC + axatoc) is idempotent upsert by DOI; `dc:date`/`coverDate` for recency ordering.
- FTS/search: FTS over title + authors + abstract; filters on specialty/type/is_free/date.

## Why install this instead of the website
- Agent-native: structured JSON + field selection for current medical literature, no Cloudflare-challenge browser needed (Surf transport built in).
- Offline + composable: synced corpus is queryable with SQL/FTS, pipes to jq, works without re-hitting the site.
- Honest about the paywall: surfaces abstracts + metadata + a free/paywalled flag, never pretends to deliver gated full text.

## Source Priority
- Single source (NEJM website itself). No combo. Multi-source gate skipped.

## Product Thesis
- Name: **nejm-pp-cli** ("nejm")
- Why it should exist: NEJM has no API. Clinicians and agents who want current peer-reviewed medical literature programmatically are stuck with a Cloudflare-gated SPA. This CLI turns NEJM's public surfaces (RSS + server-rendered article metadata) into a fast, offline-queryable, agent-native corpus — current issue, recently published, per-article abstracts, and FTS search — with a built-in browser-compatible transport so it just works.

## Build Priorities
1. Data layer: `article` store + RSS sync (eTOC + axatoc) populating SQLite.
2. Core reads: `current` (current issue), `latest`/`recent` (recently published), `article get <doi>` (metadata + abstract), generated `specialty list`.
3. Offline search/filter: `search`, filter by specialty / type / free / date.
4. Transcendence: local-store features (see absorb manifest).
