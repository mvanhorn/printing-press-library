# Awwwards CLI Brief

## API Identity
- Domain: Web design awards & inspiration. Awwwards is the canonical arbiter of "best web design in the world" — daily jury-scored awards (SOTD/SOTM/SOTY, Honorable Mention, Developer Award) with quantified quality scores no competitor has.
- Users: designers, creative agencies, front-end developers — and now AI agents that need grounded design context.
- Data profile: ~65k award entries. Per site: id, slug, title, createdAt, thumbnail, tags (merged style+category+tech), award tier, overall + 4-dimension jury scores (Design 40% / Usability 30% / Creativity 20% / Content 10%), ~18-24 individual jury votes with juror name+country, community score, Developer Award 6-dimension scores, color palette hexes, agency/designer credits. Plus: elements (section-level screenshots typed hero/footer/404...), curated collections, agency directory.
- Surface: 100% server-rendered HTML, structured JSON embedded in `data-collectable-model-value` card attributes; scores/palette in class-structured markup. No JSON API exists.

## Reachability Risk
- **None.** All surfaces return HTTP 200 to anonymous plain curl (even default curl UA). `probe-reachability` → `standard_http` @ 0.95 (stdlib 200 + surf-chrome 200). No Cloudflare/WAF observed. No GitHub issues reporting 403s. Historical scrapers died from markup churn, not blocking. Adjacent galleries (lapa.ninja 403, siteinspire 429) actively bot-block — Awwwards is the least protected in the category.
- Probe-safe endpoint used: GET /websites/ (and 13 other GET pages, all 200)
- Care point: robots.txt disallows `/websites/?` query-crawls, `/elements/*`, search endpoints (they serve 200 anyway). CLI self-rate-limits (~2 req/s ceiling) and fetches interactively, not bulk-crawls. Historical community scrapers self-imposed 250ms delay.

## Top Workflows
1. **"Show me the best of X"** — winners filtered by award tier / category / style tag / tech / color / font / country, with title+tags+date+thumbnail. One-filter-at-a-time server constraint → client-side AND-intersection is the killer feature.
2. **Deep-dive a reference** — full detail for one site: overall + dimension scores, jury critique numbers, palette hexes, tech stack, live URL. "Why did this score 9.2 on design?"
3. **Pattern research by section** — `/elements/hero/`, `/elements/footer/`, `/elements/404_page/`: tagged screenshots at exactly the granularity an agent designing a hero needs.
4. **Trend snapshot** — aggregate tag/color/tech frequency over the latest N pages: "what's current in award-winning design right now."
5. **Curated theme boards** — `/awwwards/collections/dark-mode/`, `hot-right-now`, `ai-powered-web-projects` + directory of top agencies by country.

## Table Stakes
- Browse by award tier, category, tag/tech, color (hex), font, country (all confirmed live filter URLs)
- Latest winners feed (the "RSS that never existed" — #1 historical community ask)
- Per-site detail with scores + palette + tags
- Pagination (?page=N, 30 cards/page), text search (?text=)
- Thumbnail URL construction (thumb_440_330 / thumb_880_660)
- Sub-score ranking (awwwards-best-of's sole feature: rank last N SOTD by design/usability/creativity/content)
- JSON/CSV output, collections, directory

## Data Layer
- Primary entities: websites (card + detail), elements, collections, directory profiles, jury votes
- Sync cursor: createdAt (unix) from card JSON; page-based fetch
- FTS/search: title + tags full-text; tag/color/tech columns for SQL composition
- The local store is what unlocks transcendence: multi-filter intersection, trend aggregation, palette queries, and score ranking all need N cards + N detail fetches persisted locally.

## Codebase Intelligence
- No official API, no SDK, no maintained wrapper (npm/PyPI all dead 2016-2018, endpoints stale), no real MCP server. Apify actors ($2.99/1k results) are the only living tools — they parse the same embedded card JSON.
- Most stable parse target: `data-collectable-model-value` JSON (survived across redesigns; markup classes churn more).
- jam3/awwwards-stream (dead): streamed entries with rate=250ms default — rate-limit precedent.
- awwwards-best-of (dead): score-ranking workflow precedent.
- koelll/awwwards (dead): filter matrix precedent {technology, category, country, color, tag} + RSS-generator server (pain point: "no RSS feed... never have").

## User Vision
- "Allow an agent to gather context about the best web designs in the world to then use in its own designs."
- Implications: agent-native output first (--json/--select/--compact); design-language data (palettes, tags, fonts, tech, scores) over social features (voting, submissions out of scope); local mirror so an agent can ask design questions offline mid-build; features that answer "what does great look like for <X>?"

## Product Thesis
- Name: awwwards-pp-cli
- Why it should exist: The design world's only quantified quality signal (jury scores) is locked in HTML nobody can query. Every scraper that tried is dead; nosh-cli explicitly punted ("out of scope until they have a usable public API"). Nothing occupies this space. An agent-native CLI with a local SQLite mirror turns Awwwards into a queryable design-intelligence database: "top-scoring dark e-commerce sites using GSAP, with palettes" is one command instead of an afternoon of clicking.

## Build Priorities
1. Card parser + local store + sync (foundation for everything)
2. Browse/filter/search commands with client-side multi-filter intersection
3. Site deep-dive (scores, jury votes, palette, tech) 
4. Trend aggregation (tags/colors/tech frequency over time windows)
5. Elements section research + collections + directory

## Source Priority
- Single source (awwwards.com). No combo gate.

## Spec Source
- Hand-authored internal YAML from browser-sniff + curl evidence: `research/awwwards-browser-sniff-spec.yaml` (6 resources, 10 endpoints, response_format: html). Analyzer auto-spec preserved at `.auto.yaml`. Traffic analysis repaired: false CAPTCHA marker from `.legal-recaptcha` CSS class → standard_http (see discovery/browser-sniff-report.md).

## Gate Log
- Phase 1.7 Browser-Sniff: pre-approved (Phase 0 website choice), capture completed via browser-use v0.13.3 + headless scratch Chrome. Marker written.
- Phase 1.8 Crowd-Sniff: **skip silently** — spec resolved and complete relative to community knowledge; the ecosystem sweep already read every known community tool's endpoints (all dead, all stale-URL variants of the same /websites/ listing surface already in the spec). npm/GitHub code search would rediscover a subset.
- Phase 0.5 API Key Gate: skipped — no authentication required for Awwwards public surfaces.
