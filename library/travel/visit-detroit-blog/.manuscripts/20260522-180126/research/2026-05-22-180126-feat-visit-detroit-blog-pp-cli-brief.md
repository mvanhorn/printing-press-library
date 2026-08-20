# Inside the D CLI Brief

## API Identity
- **Domain:** Editorial / city-guide content. "Inside the D" is the official blog of the Detroit Metro Convention & Visitors Bureau (DMCVB / Visit Detroit), published at `https://visitdetroit.com/inside-the-d/`.
- **Users:** Travelers planning a Detroit trip, locals looking for things to do, meeting/event planners, and AI agents answering "what should I do/eat/see in Detroit" questions.
- **Data profile:** **748 blog articles** indexed in Algolia (`prod-visit-detroit-listings`, `sectionName:Blogs`). Each record carries the full article body (`content`), a `snippet` summary, `title`, `uri` (the live page path), `blogCategories[]` (28 categories), `partnerRegions[]` (Detroit neighborhoods/counties), `postDate`/`dateUpdated` (unix), `primaryImageUrl`, and `sponsoredContent`. No author/byline field is indexed.

## Backend (confirmed live, HTTP 200)
- **Transport:** Algolia search REST API. `POST https://EYQHJ2IY2M-dsn.algolia.net/1/indexes/prod-visit-detroit-listings/query`.
- **Auth:** Public **search-only** Algolia key embedded in visitdetroit.com frontend JS (App `EYQHJ2IY2M`). Not a private secret — served to every browser. CLI auth type is effectively `none`; the search key ships as a fixed public client credential (matching how the site itself works), overridable via env var.
- **Blog filter:** `facetFilters: [["sectionName:Blogs"]]`. Category filter: `facetFilters` on `blogCategories`. Region filter: `facetFilters` on `partnerRegions`. Date range: `numericFilters` on `postDate`. Full-text: `query`.
- **Pagination:** `hitsPerPage` (max 1000), `page` (0-indexed). 748 hits ≈ 1 page at 1000/page for full sync.

## Reachability Risk
- **None.** Live Algolia query returned HTTP 200 with 748 blog hits and full record bodies. The endpoint is a public Algolia DSN that replays cleanly over plain HTTP with no browser, cookies, or clearance. Prior reconnaissance of the site's Algolia backend confirmed this contract; this run re-verified it against the live index.

## Source Priority
- Single source. No combo. Primary = Algolia `prod-visit-detroit-listings` (Blogs section).

## Top Workflows
1. **"Find me a Detroit article about X"** — full-text search across all 748 article bodies (e.g. "ethiopian food", "patio season", "free things to do").
2. **"What's the editorial take on <category/neighborhood>?"** — browse by `blogCategories` (Dining, Culture, Outdoors, Nightlife…) and `partnerRegions` (Corktown, Greektown, Eastern Market, Midtown…).
3. **"What's new on the blog?"** — recent posts sorted by `postDate`.
4. **"Read this article"** — pull the full body text for an article by slug/uri/id, offline.
5. **"What else should I read?"** — related-article discovery via shared categories + regions (the website has no "related posts" surface that crosses both axes).

## Table Stakes (what the website + any generic content tool offers)
- Browse the blog index, filter by category, open an article, see its image.
- Full-text search box (the site uses Algolia's instant-search widget).
- These are matched and beaten with: offline SQLite store of all 748 posts incl. full body, agent-native `--json`/`--select`, typed exit codes, `--dry-run`, regex/SQL composability, and cross-axis filtering the instant-search UI can't express.

## Data Layer
- **Primary entity:** `blog_posts` (id, objectID, title, uri, url, snippet, content, blog_categories JSON, partner_regions JSON, post_date, date_updated, primary_image_url, sponsored bool). FTS5 over title+snippet+content.
- **Sync cursor:** full re-sync is cheap (748 records, one paged Algolia pull). Incremental by `dateUpdated` is possible but unnecessary at this size — sync `--full` is the default.
- **FTS/search:** SQLite FTS5 virtual table on title/snippet/content enables offline ranked search, phrase queries, and SQL composition (`category = 'Dining' AND content MATCH 'patio'`).

## Codebase Intelligence
- Discovered from the site's public Algolia backend (a two-index model). The content index `prod-visit-detroit-listings` holds 3,680 records across 10 sections; `Blogs` (748) is our slice. Other sections (Events 205, Itineraries 43, Local Business 2151) are out of scope for this CLI but share the index, so the same client can optionally enrich blog↔event/itinerary cross-references later.

## User Vision
- User directive: build a focused editorial-content CLI targeting the blog posts under /inside-the-d/.
- Interpretation: a focused editorial-content CLI for the "Inside the D" blog. Slug `visit-detroit-blog`, binary `visit-detroit-blog-pp-cli`.

## Product Thesis
- **Name:** Inside the D CLI (`visit-detroit-blog-pp-cli`)
- **Why it should exist:** Detroit's official editorial blog has 748 articles of curated local knowledge locked behind a browse-only website with an instant-search box. There is no way to search the full text offline, filter across category *and* neighborhood at once, find related reads, or let an agent answer "give me 3 Detroit dining articles in Corktown from this year." This CLI turns that editorial corpus into a queryable, offline, agent-native local store — the differentiator no existing tool (or the site itself) offers.

## Build Priorities
1. **Data layer + sync** — pull all 748 Blogs records from Algolia into SQLite with full body + FTS5.
2. **Absorbed (table-stakes) commands** — `search`, `blogs list` (with `--category`, `--region`, `--since`/`--until`, `--limit`), `blogs get` (read full article by slug/id/uri), `categories`, `regions`, `recent`.
3. **Transcendence commands** — cross-axis filtering, related-article discovery, category×region content maps, reading lists, and other aggregations only possible with the full corpus in SQLite. (Finalized in the absorb manifest + novel-features subagent.)
