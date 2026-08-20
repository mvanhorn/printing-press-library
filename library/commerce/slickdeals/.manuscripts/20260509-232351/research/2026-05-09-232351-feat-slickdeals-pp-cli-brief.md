# Slickdeals CLI Brief

## API Identity
- **Domain:** Deal aggregation, e-commerce discovery, community-curated bargain hunting (US-focused, retailer-agnostic)
- **Users:** bargain hunters, price-conscious shoppers, resellers, deal-tracking automation enthusiasts, agents that filter retail signal for owners
- **Data profile:** ~hundreds of frontpage deals/day, thousands of forum-deals/day, tens of thousands of comments, votes, alerts, coupons, user profiles

## Reachability Risk
- **HIGH** for direct HTML scraping (Cloudflare Turnstile/TLS-fingerprinting, ~80% bypass failure on naked HTTP per ScrapingBee/BrightData benchmarks 2025–2026)
- **LOW** for RSS feeds (`newsearch.php?...&rss=1`) — confirmed live 200 OK with `text/xml` (smoke-tested 2026-05-09)
- **LOW** for HTML when using `enetx/surf` Chrome-fingerprint transport (proven by sibling CLIs `recipe-goat`, `allrecipes`)
- **MEDIUM** for authenticated endpoints — `sd_session` cookie auth + CSRF rotation; account lockout after 3 failed logins; ~5–10 req/min/IP rate limit

## Top Workflows
1. **Track keywords for new deals** — set alert for "rtx 4090", "kitchenaid mixer", get notified when a matching deal hits the frontpage
2. **Browse hot/frontpage deals on demand** — agent or human asks "what's hot today?", gets the top N with prices, stores, thumbs, post age
3. **Watch a specific deal** — track price/stock/thumbs delta over time on a single deal until it expires or hits a price target
4. **Cross-store deal hunt** — "all Costco deals in last 24h with ≥50 thumbs" — compound queries no other Slickdeals tool offers
5. **Submit + manage own deals** — post a deal you found, track its votes, reply to comments, send DM to OPs of related deals (auth-write, gated)

## Table Stakes (matched + beaten)
From competitor analysis (`Unwoundd/Slickdeals-Discord-Bot`, `carvalhe/SlickDealsScraper`, `karthiksivaramms/bargainer-mcp-client`, the abandoned `MinweiShen/slickdeals` Python CLI, browser-extensions, IFTTT/Zapier RSS recipes):
- RSS feed parsing → match (gofeed) + beat with FTS5 search across snapshots
- Hot deals fetch → match + beat with provenance envelope (live vs local) and `--select` field filtering
- Discord/notification delivery → match via `--deliver webhook:<url>` + beat with multiple sinks (file, stdout, webhook)
- Search → match via search RSS + beat with offline FTS5 across snapshotted history
- Affiliate link cleanup → match via opt-in `--clean-links` flag

## Data Layer
- **Primary entities:** `deals`, `deal_snapshots` (price/thumbs/stock over time), `forums`, `categories`, `users`, `comments`, `votes`, `alerts`, `subscriptions`, `coupons`, `merchants`
- **Sync cursor:** RSS `pubDate` + `last-modified` header per feed; deal-page polled on `watch` interval
- **FTS/search:** FTS5 virtual table on `deals(title, description, merchant, category)`; auto-rebuild on sync
- **Snapshot history:** every `sync` writes new row to `deal_snapshots` so price-drop / thumbs-velocity / expiration analysis becomes a SQL query

## Codebase Intelligence
- **Source:** GitHub repo analysis of competitor tools. No DeepWiki entries (community projects too small).
- **Auth pattern from `Unwoundd/Slickdeals-Discord-Bot` source:** RSS-only, no auth. Confirms the read surface is unambiguously RSS-served.
- **`carvalhe/SlickDealsScraper` (Python, BeautifulSoup) source:** scrapes HTML directly with `requests` + `User-Agent` spoofing. Cloudflare blocks this approach intermittently — exactly the failure mode `enetx/surf` solves.
- **No competitor implements:** local SQLite snapshot, FTS5 search, watch lists, compound queries, agent-native `--json --select` output, MCP server.

## User Vision (David)
- Allow user to create deal alerts
- View frontpage deals
- Hot deals
- Personalized deals
- Submit new deals
- Comment and thumbs on deals
- DM other users
- "Any other useful features that you recommend" — covered in transcendence (watch, compound queries, snapshot history, coupons, profile lookup, daily digest)

## Source Priority
- Single source (`slickdeals.net`) — Multi-Source Priority Gate not applicable.

## Product Thesis
- **Name:** `slickdeals-pp-cli` (binary), `pp-slickdeals` (skill), CLI library entry `slickdeals`
- **Why it should exist:** Slickdeals has a 1.5M+ daily active community and zero agent-native tooling. Existing scrapers are abandoned or single-purpose Discord bots. Nobody offers offline SQLite snapshot, FTS5 cross-history search, or compound queries like "Costco + ≤24h + ≥50 thumbs". Cloudflare protection has scared off open-source builders, but `enetx/surf` Chrome-fingerprint transport (proven by `recipe-goat`/`allrecipes`) bypasses it cleanly. RSS handles 80% of the read surface with zero ToS risk; auth-write surface ships gated behind `--accept-tos-risk` for the 20% that needs it.

## Build Priorities
1. **RSS read surface** — `deals`, `hot`, `search`, `category`, `coupons` — backed by `gofeed` + `if-modified-since` caching. Zero ToS risk. Ship-first.
2. **Local snapshot store** — SQLite + FTS5; `sync` command pulls RSS and writes snapshots; foundation for every transcendence feature.
3. **Compound queries + analytics** — `deals --store costco --since 24h --min-thumbs 50`, `analytics --top-stores --window 30d`, `deals --price-drop 20% --since 7d`.
4. **Watch / alerts** — `watch <deal-id>`, `alerts add/list/remove`, `digest` — local poll loop; deliver to stdout/file/webhook.
5. **Authenticated read** — `personalized`, `profile <user>`, `me` — cookie-based via `auth import-cookies` (Chrome/Firefox jar).
6. **Authenticated write** (gated `--accept-tos-risk`) — `vote`, `comment`, `submit`, `dm` — agent-safe rate limits (vote ≤10/hr, submit ≤2/day, DM ≤20/day).
7. **MCP server** — `slickdeals-pp-mcp` — auto-emitted by Printing Press from the Cobra tree.
