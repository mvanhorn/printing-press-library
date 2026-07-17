# StarterStory CLI Brief

## API Identity
- Domain: starterstory.com — Rails, server-rendered HTML, NO JSON API (`.json` → 406), plain HTTP 200 (standard_http, no bot wall)
- Users: indie hackers / founders researching proven business ideas + revenue case studies
- Data profile (from CloudFront sitemap): /ideas 5,950 · /stories 3,260 (case studies, revenue in og:title, og:type=article) · /breakdowns 1,520 · /businesses 389 · /tools 347 · /data/<category> 40 curated idea lists · /episodes podcast

## Reachability Risk
- None. stdlib + surf both 200. Public detail/list/sitemap pages readable without auth. (Deep premium playbooks are paywalled but out of scope.)

## Surface
- Discovery backbone: CloudFront gzip sitemap (d1coqmn8qm80r4.cloudfront.net/sitemaps/sitemap.xml.gz) → ~13k URLs, classifiable by path segment
- List pages that work: /data/<category>, homepage, /explore
- Detail pages: /stories|ideas|businesses|breakdowns|tools/<slug> → og:title/description/image, canonical, page text (revenue baked into story og:title)
- Search: /search?q= returns HTML 200 (results markup low-confidence; treat as best-effort)
- No JSON-LD, no typeahead JSON (the "autocomplete" tokens were form attrs)

## Data Layer
- Primary entities: story, idea, business, breakdown, tool, data_list (category)
- Sync cursor: sitemap fetch → upsert url+section+slug+title; re-sync diffs new URLs
- FTS/search: FTS5 over slug/title/section → offline search beats site search; revenue parsed from story titles for ranking

## Product Thesis
- Name: starterstory-pp-cli
- Why: StarterStory has no API and weak on-site filtering. A local SQLite index of all 13k stories/ideas/businesses with parsed revenue lets you rank case studies by $/month, filter ideas by keyword offline, and diff "what's new since last sync" — things the website itself can't do.

## Build Priorities
1. Sitemap sync → SQLite index (all sections) + FTS5
2. Detail fetch (HTML extract: og + text) for story/idea/business/breakdown/tool
3. Category list fetch (/data/<category>)
4. Transcendence: revenue-rank stories, offline search/filter, new-since-sync diff, category browse
