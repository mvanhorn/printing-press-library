# Slickdeals CLI Absorb Manifest

## Source Tools Surveyed (Step 1.5a)

| Tool | URL | Type | Stars | Last Active | Status |
|---|---|---|---|---|---|
| `Unwoundd/Slickdeals-Discord-Bot` | github.com/Unwoundd/Slickdeals-Discord-Bot | Discord bot (RSS-driven) | 7 | 2024-12 | Maintained |
| `karthiksivaramms/bargainer-mcp-client` | github.com/karthiksivaramms/bargainer-mcp-client | MCP **client** (chat UI) | 4 | 2026-03 | Active |
| `carvalhe/SlickDealsScraper` | github.com/carvalhe/SlickDealsScraper | Python webscraper for Discord | 4 | 2024-10 | Maintained |
| `MinweiShen/slickdeals` | github.com/MinweiShen/slickdeals | Python CLI | 2 | 2023-11 | Abandoned |
| `vanowm/slickdealsPlus` | github.com/vanowm/slickdealsPlus | Browser userscript | 2 | 2026-02 | Active |
| `Amberganda/slickdeals_scraper` | github.com/Amberganda/slickdeals_scraper | Ruby scraper | 2 | 2019-12 | Abandoned |
| `sparkyfen/SlickDeals-Parser` | github.com/sparkyfen/SlickDeals-Parser | RSS parser | 2 | 2017-10 | Abandoned |
| `norrism/slickdeals-affiliate-link-remover` | github.com/norrism/slickdeals-affiliate-link-remover | JS userscript | 4 | 2023-11 | Maintained |
| `schrauger/slickdeals-clean-url` | github.com/schrauger/slickdeals-clean-url | JS userscript | 11 | 2024-09 | Active |
| Apify `Slickdeals Forum Threads Scraper` | apify.com/scralab/slickdeals-forum-threads-scraper | Commercial scraper | — | — | Active |
| Apify `Slickdeals Hot Deals Scraper` | apify.com | Commercial scraper | — | — | Active |
| Slickdeals official RSS feeds | slickdeals.net/newsearch.php?...&rss=1 | First-party feed (ToS-permitted) | — | live | Stable |
| IFTTT/Zapier RSS recipes | community recipes | Notification automation | — | — | Active |
| Slickdeals mobile app | iOS/Android | First-party | — | live | Active |

**Notable absences:** No npm packages, no PyPI packages, no MCP server (only the bargainer-mcp-client which is a UI shell). This is a greenfield catalog slot.

---

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Frontpage deals fetch | Apify Hot Deals Scraper | `deals` via RSS `newsearch.php?forum=9&rss=1` | Local SQLite snapshot, FTS5 search, `--json --select`, no scraping |
| 2 | Hot deals (≥3 thumbs filter) | Apify Hot Deals Scraper | `hot` via `newsearch.php?forum=9&hotdeals=1&rss=1` | `--min-thumbs N` flexible threshold, snapshot history |
| 3 | Keyword search | `MinweiShen/slickdeals` Python CLI | `search "<query>"` via search RSS, with FTS5 fallback to local | Offline FTS5 across snapshotted history; live + local routing via `--data-source` |
| 4 | Category browse | `Unwoundd/Slickdeals-Discord-Bot` | `category <id\|name>` resolves to forum ID, fetches RSS | Built-in category-name → forum-ID map; works for Tech, Home, Apparel, Garden, etc. |
| 5 | Freebies / coupons / price-drop filter | RSS `&filter=` | `deals --freebie`, `deals --coupon`, `deals --price-drop` | Composable with all other filters |
| 6 | Email/Discord/Slack alert delivery | Discord-Bot, IFTTT/Zapier | `alerts add --keyword "..." --deliver webhook:...` | Multiple sinks (stdout/file/webhook), keyword + price-cap + category |
| 7 | New-deal notifications | Mobile app push | `watch <deal-id>` + alerts loop | Local cron-style poll; emits when delta detected |
| 8 | Affiliate link cleanup | `norrism/slickdeals-affiliate-link-remover` | `--clean-links` opt-in flag | Built into all `deals` commands; defaults off |
| 9 | Forum thread scraping | Apify Forum Threads Scraper | `forum-thread <id>` via deal page HTML (Surf transport) | One scrape feeds local store; agent-native output |
| 10 | Personalized frontpage (logged-in) | Slickdeals web/app | `personalized` (auth-required) | Auth via cookie import (no password automation) |
| 11 | Saved deals list | Slickdeals web/app | `saved list` (auth-required) | Local snapshot + delta tracking |
| 12 | View profile (any user) | Slickdeals web/app | `profile <username>` | Public profile lookup; deals posted, votes given |
| 13 | Submit a new deal | Slickdeals web/app | `submit --url <merchant> --price N --title "..." --category tech` (auth-required, **--accept-tos-risk**) | CSRF-aware POST; rate-limited ≤2/day agent-safe default |
| 14 | Vote thumbs up/down | Slickdeals web/app | `vote up/down <deal-id>` (auth-required, **--accept-tos-risk**) | Idempotent (checks current vote state); rate-limited ≤10/hr |
| 15 | Comment on a deal | Slickdeals web/app | `comment add <deal-id> --text "..."` (auth-required, **--accept-tos-risk**) | Markdown→BBCode conversion; rate-limited ≤5/hr |
| 16 | List comments on a deal | Slickdeals web/app | `comment list <deal-id>` | Local cache; agent-native output |
| 17 | Send DM to user | Slickdeals web/app PM system | `dm send <user> --text "..."` (auth-required, **--accept-tos-risk**) | CSRF-aware; rate-limited ≤20/day |
| 18 | DM inbox / threads | Slickdeals web/app PM system | `dm inbox`, `dm thread <id>`, `dm reply <id>` | Local cache for offline browsing |
| 19 | Coupons section browse | Slickdeals coupons section | `coupons --store target --since 7d` | RSS-eligible read, snapshot |
| 20 | Account info / dashboard | Slickdeals web/app | `me` | Authenticated overview |

20 absorbed features. Every row IS shipping scope unless explicitly marked `(stub)` below.

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| 1 | Compound query: store + freshness + thumbs | `deals --store costco --since 24h --min-thumbs 50` | Requires SQL JOIN over snapshotted deals + merchant-extracted store + thumbs over time. No competitor stores this locally. | 9/10 |
| 2 | Price-drop velocity tracker | `deals --price-drop 20% --since 7d` | Requires `deal_snapshots` history; computes delta as SQL window function over snapshot rows. | 9/10 |
| 3 | Watch lists with delta detection | `watch <deal-id>`, `watch list`, `watch history <deal-id>` | Local poll loop comparing snapshots; emits structured event on price/stock/thumbs delta. No competitor offers this for non-frontpage deals. | 8/10 |
| 4 | Daily digest with deduplication | `digest --since 24h --top 20 --merchant-cap 3` | Aggregation across snapshotted RSS pulls; cap per-merchant prevents one store from dominating. | 8/10 |
| 5 | Top-stores analytics | `analytics --top-stores --window 30d` | SQL aggregation over local snapshot store. No web UI surfaces this. | 7/10 |
| 6 | Cross-cycle thumbs trajectory | `analytics --thumbs-velocity <deal-id>` | Snapshot history makes "thumbs/hour over the deal's lifetime" computable. | 7/10 |
| 7 | "What did I miss" | `digest --since 4h --grouped-by category` | Time-windowed aggregation no single RSS query provides; categorized rollup. | 7/10 |
| 8 | Auto-snipe expiring deals | `watch --expiring-within 2h --notify-when "thumbs > 100"` | Compound condition: time-to-expiration AND community velocity. SQL-backed. | 6/10 |
| 9 | Personal vote/comment audit log | `me --votes --since 30d`, `me --comments --since 30d` | Authenticated scrape + local cache; lets users review their own activity history. | 6/10 |
| 10 | Doppelganger detection | `deals --duplicate-of <deal-id>` | Local FTS5 on title+merchant+price reveals reposts/duplicates. | 5/10 |
| 11 | Merchant reliability score | `analytics --merchant-stats target` | Aggregation of past deals from a merchant: avg thumbs, expiration rate, price-drop frequency. | 5/10 |
| 12 | Cross-deal price comparison | `compare <deal-id-1> <deal-id-2>` | Side-by-side using local snapshots. | 5/10 |

12 transcendence features (all ≥5/10). 8 score ≥7/10 (the differentiators).

---

## Stubs

| # | Feature | Reason |
|---|---|---|
| (none in v1) | — | All approved features are full implementation. |

---

## Surface counts

- **Absorbed:** 20 features (10 read-only / RSS, 10 auth-required including 4 ToS-gated writes)
- **Transcendence:** 12 features (all SQL-backed compound or analytics)
- **Total user-facing commands:** ~32
- **Estimated MCP tool surface:** ~32 + ~13 framework = **~45 tools** → triggers Phase 2 MCP enrichment recommendation (`mcp.transport: [stdio, http]` + intents)

## Comparison to incumbents

| Tool | Features | Local store | Agent-native | MCP | Auth | Compound queries |
|---|---|---|---|---|---|---|
| Apify scrapers | ~3 each | ❌ | partial | ❌ | ❌ | ❌ |
| `Unwoundd/Slickdeals-Discord-Bot` | ~3 (alerts only) | ❌ | ❌ | ❌ | ❌ | ❌ |
| `karthiksivaramms/bargainer-mcp-client` | UI only | ❌ | ❌ | client | ❌ | ❌ |
| `MinweiShen/slickdeals` (abandoned) | ~5 | ❌ | partial | ❌ | ❌ | ❌ |
| **`slickdeals-pp-cli` (this CLI)** | **32** | **✅ SQLite + FTS5** | **✅ `--json --select --csv`** | **✅ server emitted** | **✅ cookie import** | **✅ 12 SQL-backed** |
