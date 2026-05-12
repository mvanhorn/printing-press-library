# Kickstarter CLI Brief

## API Identity
- **Domain:** Crowdfunding launch + project funding intelligence
- **Users:** Market researchers, trend analysts, agents harvesting product launch signal (this CLI's primary user is **Scout**, an autonomous weekly market-research agent across ai-agents, frontier-ai, smb-saas, geopolitics, aus-tech, india-tech verticals)
- **Data profile:** Projects (live + funded + cancelled), Creators, Categories (15 main / 36 sub), MagazineArticles (editorial), Updates (creator-posted news to backers), Comments, Rewards. ~300K live + millions of historical projects.

## Reachability Risk
**High direct-HTTP risk; mitigated by Surf runtime.** Plain stdlib HTTP returns 403 with `cf-mitigated: challenge` (Cloudflare bot detection). `printing-press probe-reachability` returned `mode: browser_http` with 0.85 confidence — Surf with Chrome TLS fingerprint cleared the challenge and got a clean 200 against both `kickstarter.com` and `/discover/advanced?format=json`. Generator will be told to ship Surf transport; no clearance cookie, no browser sidecar, no chrome-attach needed at runtime.

Evidence: probe artifacts captured in this run's `proofs/`. Apify, ScrapingBee, Piloterr all sell Kickstarter scrapers — confirms it's a frequently-scraped target where direct curl fails but TLS-fingerprinted clients succeed.

## Top Workflows
1. **Latest tech/AI launches** — newly launched projects in Technology + Design categories (or specific subcategories: AI, Hardware, Software, Robots, Apps), sorted by `launched_at`. This is Scout's primary signal.
2. **Trending + most-funded across Discover** — `sort=popularity` and `sort=most_funded` across all categories, surfaces what's catching real money.
3. **Magazine editorial roll-up** — `/magazine` posts from Kickstarter's editorial team (Maker Monday, Creator Spotlight, trend recaps). Lower volume, higher signal-to-noise for "what Kickstarter itself thinks is worth amplifying."
4. **Project deep-dive** — fetch a single project's full data (creator, rewards, updates, video, funding-curve) by slug.
5. **Cross-category search** — text search across keyword + category + status (live/successful/failed) for niche signal hunting (e.g., "AI agent" across all categories live).

## Table Stakes
- Public search via `/discover/advanced?format=json` (works with Surf, returns paginated JSON)
- Filter by `category_id` (main) and `subcategory_id`
- Sort: `magic`, `newest`, `end_date`, `popularity`, `most_funded`, `most_backed`
- Status filter: `live`, `successful`, `failed`, `cancelled`, `submitted`
- Pagination via `page=N`
- Project lookup by `/projects/<creator>/<slug>?format=json`
- Magazine HTML scraping (no JSON endpoint; clean URLs at `/magazine/...`)
- oEmbed at `/services/oembed?url=...` (lightweight project metadata, no auth)

## Data Layer
- **Primary entities:** Project, Creator (User), Category, Subcategory, MagazineArticle, Update, Reward
- **Sync cursor:** `launched_at` (projects), `published_at` (Magazine articles), `updated_at` (project metadata refresh)
- **FTS/search:** `projects_fts` over name + blurb + creator name; `magazine_fts` over title + body + tags
- **Composite tables:** `vertical_match` (project_id → vertical_slug + match_score) for Scout's vertical-mapping queries

## Codebase Intelligence
- **markolson/kickscraper** (Ruby) — most mature unofficial client. Confirms base URLs: `https://api.kickstarter.com` (OAuth, partner-only) vs `https://www.kickstarter.com` (public search/discover with `format=json`). Middleware appends `format=json` automatically for public endpoints. Headers: `Content-Type: application/json`, `Accept: application/json; charset=utf-8`. SSL verify disabled in their default config (Faraday HTTP middleware).
- **rabidlogic/PyKickstarter** (Python) — narrower scope but confirms entity shape (Project, Reward, Category, Update).
- **gippy/kickstarter-search** (Apify Actor) — explicitly uses search.json with Chrome TLS impersonation to clear bot detection. Confirms the Surf path is the production-standard way.
- **No existing MCP server** for Kickstarter. Net-new MCP surface.

## User Vision
Scout (weekly autonomous market-research agent across ai-agents, frontier-ai, smb-saas, geopolitics, aus-tech, india-tech) needs a fast, terse, agent-readable Kickstarter source. Ingestion target: JSONL records that Scout's vertical-mapping stage can score. Three angles in one binary: (1) newly launched campaigns filtered by Technology/AI/Design verticals, (2) trending + most-funded across Discover, (3) Kickstarter Magazine editorial posts. Plus a unified `latest-news` roll-up that fans out across all three with one invocation.

## Product Thesis
- **Name:** kickstarter-pp-cli
- **Why it should exist:** Every existing Kickstarter scraper is either (a) a one-off Python notebook for academic analysis, (b) a commercial SaaS scraper (Apify/ScrapingBee, $$/call), or (c) a Ruby gem that requires Kickstarter email/password and breaks against modern bot defenses. None ship: agent-native JSONL output, a local SQLite mirror you can SQL across, Surf transport that clears Cloudflare without a paid service, or a Magazine ingestion path. None have a unified "latest news" roll-up. None pre-classify projects against Scout-style verticals. This CLI fills all five gaps in one binary.

## Build Priorities
1. **P0 — Discover JSON client + project sync** — `/discover/advanced?format=json` with category/sort/page/status filters; sync hot pages into local SQLite. Surf transport from day one.
2. **P0 — Magazine ingestion** — list+fetch Magazine articles by parsing `/magazine` index HTML + article pages.
3. **P0 — Local SQLite store with FTS** — projects, creators, magazine_articles, categories tables + FTS5.
4. **P1 — Absorbed-feature commands** — match every kickscraper/PyKickstarter command (search, fetch, list-categories, project-detail, updates).
5. **P2 — Transcendence commands** — `latest-news` roll-up, `tech-radar`, `funding-rank`, `vertical agentic`, `creator-portfolio`, `category-velocity`, `magazine search`.
