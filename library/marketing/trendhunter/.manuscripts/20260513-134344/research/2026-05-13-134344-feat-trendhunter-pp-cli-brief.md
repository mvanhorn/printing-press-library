# TrendHunter CLI Brief

## API Identity

- Domain: trendhunter.com - "#1 in Trends, Trend Reports, Fashion Trends, Tech, Design"
- Users: marketers, brand strategists, product managers, futurists, agency
  researchers, designers, anyone running competitive scans. The site sells
  premium reports/PDF/advisory; the free public surface is the daily firehose
  of curated "micro-trend" cards (~600K trends archived, ~30 new/day).
- Data profile: 100% server-rendered HTML + one global RSS feed; no JSON API
  on the public surface. There IS a paid `developer.trendhunter.com` API but
  the user is explicitly building a free scraper - no key, no login.

## Reachability Risk

None. `probe-reachability` returned `mode: standard_http` with 0.95 confidence.
The 403s curl saw at first are pure User-Agent / Accept-header filtering -
stdlib Go with a Chrome-imitating header set returns 200 across every endpoint.
No Cloudflare, no clearance cookie, no JS rendering, no CAPTCHAs.

## Top Workflows

1. "What's trending right now?" - pull the global RSS feed or `/popular`, get
   30 latest trend cards with title, image, link, category.
2. "Find trends about <topic>" - hit `/results?search=<term>`, get 20-30
   matching trend slugs.
3. "Show me everything in <category>" - hit `/<category>` (tech, ai, fashion,
   food, eco, marketing, ...), get the ~30-card index page.
4. "Open the full trend page for <slug>" - fetch `/trends/<slug>`, extract
   title, description, image, keywords, author, related slugs, FAQ Q&A.
5. "Sync everything I've seen and search it offline" - run a recurring
   `sync` against `/rss` + a chosen category list, persist to SQLite, search
   via FTS without re-hitting the network.

## Table Stakes (from competing tools)

Only one real competitor exists: `jbal/trendhunter` (PyPI / GitHub, 2 stars,
last commit Sept 2024, 23 commits total).

It ships these features. We must match every one, then beat them:

- `trends` - fetch global trending ideas (their flagship).
- `lists` - query list-based content.
- `categories` - browse by category.
- `search` - full-text search.
- `assortment` - combined query across the four above.
- Output to console (text) or PowerPoint file.
- Concurrent fetch (default 5, max 100).
- HTTP proxy support.
- Token-bucket rate limiter.
- Image download + resize (PIL).

It does NOT have: JSON output, `--select`, CSV, SQLite persistence, offline
search, FTS, MCP, agent-native output, sync command, watch mode, dedup,
related-trend graph, FAQ extraction, sitemap-driven catalog, contributor
profiles, futurist profiles, category-cluster aggregation.

## Data Layer

Primary entities:

- **trend** (slug PK, title, description, image_url, keywords, author,
  category, trend_id, pub_date, body_text, related_slugs JSON, faq JSON,
  first_seen, last_seen, source ('rss'|'category'|'search'|'detail'))
- **category** (slug PK, label, last_synced)
- **author** (name PK, profile_slug, trend_count)
- **report** (slug PK, title, kind ('megatrend'|'pattern'|'trendreport'),
  description, image_url, last_seen)
- **sync_run** (id PK, run_at, source, items_seen, items_new)

Sync cursor: `pub_date` for RSS items + last-seen URL hash for HTML pages.
FTS5 over `trend.title`, `trend.description`, `trend.body_text`,
`trend.keywords`, and `trend.faq`.

## Codebase Intelligence

- jbal/trendhunter source uses Python `requests` with concurrent.futures, a
  custom monkey-patch to bypass 429, PIL for image resize, python-pptx for
  PowerPoint emit. Author handling is fragile (regex over a single CSS class).
- No FAQ JSON-LD extraction in any prior tool.
- No tool tracks the sitemap or treats trends as a synced corpus.

## User Vision

User asked verbatim: "i don't have an api / logged in. want to scrape /
figure out best way to build CLI from what's out there on sniffing." So:

- No paid API, no Pro account.
- Scrape-first design.
- Best replayable surface = stdlib HTTP + RSS + HTML extraction.
- They want the agent to make the architecture call and recommend the right
  feature set; they did not pre-specify the command shape.

## Source Priority

Single-source CLI. The free public trendhunter.com surface is the only
source. The paid `developer.trendhunter.com` API is excluded by user choice.

## Product Thesis

- Name: `trendhunter-pp-cli` (binary), library slug `trendhunter`.
- Why it should exist: The only existing tool is stale, Python-only, no JSON,
  no offline search, no agent surface. TrendHunter is a research firehose;
  anyone using it programmatically today either (a) eyeballs the website,
  (b) pays for the developer API, or (c) wires up the jbal Python script
  and ships PowerPoint files. None of those plug into an agent loop. A Go
  CLI with stdlib HTTP, a local SQLite corpus, FTS5 search, JSON-native
  output, FAQ extraction, and MCP exposure plugs in immediately and runs
  for free.

## Build Priorities

1. Foundation: SQLite store with trends/categories/authors/reports tables,
   FTS5 over the trend corpus, sync engine driven by /rss + /sitemap.xml.
2. Absorb (match jbal/trendhunter): `trends`, `lists`, `categories`,
   `search`, `assortment`. Add JSON, --select, --csv, --compact, --dry-run.
3. Transcend: features only possible because we have a local corpus,
   FAQ extraction, related-trend graph, and sitemap awareness. See Phase 1.5.

## Notes for the generator

- Auth: none. Spec must declare `auth: {type: none}`.
- HTTP transport: stdlib with Chrome UA + standard Accept header. NOT
  Surf, NOT browser-clearance.
- Response formats: mix of RSS (xml) and HTML.
- The CLI ships an MCP server because absorbed + transcendence commands
  total well under 30 - default endpoint-mirror surface is fine; no
  enrichment needed.
