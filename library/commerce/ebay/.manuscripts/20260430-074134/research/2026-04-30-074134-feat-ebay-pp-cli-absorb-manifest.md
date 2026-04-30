# eBay CLI Absorb Manifest

## Source Tools Surveyed
| Tool | Type | Stars | Coverage |
|------|------|-------|----------|
| hendt/ebay-api | TypeScript SDK | ~200 | Most comprehensive: Buy (Browse, Deal, Feed, Marketplace Insights, Offer, Order), Commerce (all), Sell (all), Trading XML, Shopping XML |
| pajaydev/ebay-node-api | JS SDK | ~1,000 | Browse, Finding, Shopping, Trading basics |
| timotheus/ebaysdk-python | Python SDK | ~700 | Trading XML, Finding, Shopping, HTML scraping |
| asherAgs/ebSear | Python CLI | 11 | Active-listings web search only |
| Apophenic/FindItFirst | Java alert tool | n/a | Saved-search alerts via Browse API |
| YosefHayim/ebay-mcp | MCP server | n/a | 325 seller-side tools, no buyer features |
| ruippeixotog/ebay-snipe-server | Scala sniper | 139 | Full sniper: contingency groups, T-N fire, web session replay |
| VincenzoImp/ebay-sniper-bot | Python sniper | 2 | T-1s fire via DrissionPage |
| Gixen | SaaS sniper | n/a | T-3/6/8/10/12/15s fire, contingency groups, multi-win groups, mirror service |
| Auction Sniper | SaaS sniper | n/a | Bid groups, group stop on win |
| BidSlammer / EZSniper / BidNapper | SaaS snipers | n/a | One snipe per group, alert notifications, multi-site (EZ) |
| JBidWatcher | OSS desktop | n/a | Offline operation, auto-bid, failure notifications |
| 130point.com | Sold-comp SaaS | n/a | Best-offer-accepted price recovery, keyword search |
| Terapeak | eBay native | n/a | 2-3 yr sold history, sell-through rate (seller-only) |
| WorthPoint | $29-47/mo | n/a | 545M historical prices |
| driscoll42/ebayMarketAnalyzer | Python | 254 | Trendline + rolling avg over scraped sold listings |

## Absorbed Features (match or beat everything that exists)

### Search & Discovery
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Keyword search active listings | hendt/ebay-api Browse.search | `ebay search <query>` (HTML scrape `/sch/i.html`) with --json/--csv/--select | Works without App ID; offline rerun via local cache |
| 2 | Filter by category | Browse API category_ids | `--category <id>` flag; `ebay category list` for taxonomy lookup | Local taxonomy cache, fuzzy category match |
| 3 | Filter by price range | Browse API price filter | `--min-price`, `--max-price`, `--condition new\|used\|refurb` | Single short flag set |
| 4 | Filter Buy It Now / auctions | LH_BIN / LH_Auction | `--type bin\|auction\|all` | Default sane: all, but auctions ranked by ending-soonest |
| 5 | Sort: ending soonest, newest, price | Browse API sort | `--sort ending\|newest\|price-asc\|price-desc\|distance` | Maps human strings to eBay codes |
| 6 | Search by image | Browse API searchByImage | `ebay search --image <path>` (Base64 POST when App ID present; HTML fallback) | Almost no other CLI exposes this |
| 7 | Item detail | Browse.getItem | `ebay item <id>` (HTML scrape + cache) | Hydrates local store; offline reread |
| 8 | Item compatibility | Browse.checkCompatibility | `ebay item compat <id> --year 2010 --make Toyota --model Camry` | One-shot vehicle compat |
| 9 | Deal feed | Deal API getDeals | `ebay deals` | Filter by category; emits to feed |
| 10 | Category browse | Commerce.Taxonomy | `ebay category tree`, `ebay category find <name>` | FTS over local taxonomy |

### Buyer Account
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 11 | List watchlist | Trading API GetMyeBayBuying | `ebay watch list` (HTML scrape /mye/myebay/Watchlist) | Local mirror with tags |
| 12 | Add to watchlist | Trading API AddToWatchList | `ebay watch add <id>` | Composable from `ebay search ... \| xargs ebay watch add` |
| 13 | Remove from watchlist | Trading API RemoveFromWatchList | `ebay watch remove <id>` | Idempotent |
| 14 | List active bids | GetMyeBayBuying BiddingData | `ebay bid list` | Status: leading/outbid/won/lost |
| 15 | Best Offer history | Trading GetBestOffers | `ebay offer history <id>` | Includes accepted offer prices when visible |
| 16 | Place a bid | /bfl/placebid?action=confirmbid | `ebay bid place <id> --amount 3.25` | Replays the trisk + confirmbid flow with captured forterToken |
| 17 | Submit best offer | Trading MakeOffer | `ebay offer submit <id> --amount 5.00 --message "..."` | Optional message; --dry-run supported |
| 18 | Buying history (won/lost) | GetMyeBayBuying | `ebay history won`, `ebay history lost`, `ebay history paid` | Default 90 days; --days N |

### Saved Searches & Alerts
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 19 | Saved search CRUD | eBay-native saved searches | `ebay saved-search create/list/delete/run` | Local-first; not tied to eBay's ephemeral saved-search UI |
| 20 | New-listing alerts | FindItFirst | `ebay feed <saved-search>` runs differential against local store | Sub-15-min latency; agent-friendly JSONL output |
| 21 | Price-drop alerts | n/a (eBay doesn't offer for buyers) | `ebay watch threshold <id> --below 50.00` + cron-friendly check | Buyer-side feature eBay never built |
| 22 | Auction-end alerts | n/a (eBay's email is poor) | `ebay watch end-alert <id> --minutes 5` | Local notification or stdout |

### Output / Composition
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 23 | JSON output | hendt/ebay-api .json() | `--json` on every command | Every command, every subcommand |
| 24 | CSV output | n/a | `--csv` on list commands | Pipes to spreadsheets |
| 25 | Field selection | n/a | `--select 'title,price,bids,end_time'` | Reduces token cost for agents |
| 26 | Dry-run | n/a | `--dry-run` on every mutation | Shows what would happen, no submit |
| 27 | Idempotent retry | n/a | Mutation commands store attempt-id; safe to re-run | No double-bid |
| 28 | Local SQLite store | n/a | `~/.config/ebay-pp-cli/ebay.db` with FTS5 | Sync once, query forever |
| 29 | SQL escape hatch | n/a | `ebay sql 'SELECT * FROM sold_items WHERE price > 100'` | Power-user analytics |

## Transcendence Features (only possible with our approach)

| # | Score | Feature | Command | Why Only We Can Do This |
|---|-------|---------|---------|-------------------------|
| T1 | 10/10 | **Sold-comp intelligence** | `ebay comp <query> [--days 90] [--condition raw\|graded] [--outlier-trim] [--group-by-edition]` | Marketplace Insights API is partners-only — every other tool can give individual sold listings, none normalize / dedupe / trim outliers / group by edition / show trendline. |
| T2 | 10/10 | **True sniper bidding** | `ebay snipe <id> --max <amount> [--lead 25s]` | Holds max client-side. Replays the captured /bfl/placebid?action=confirmbid flow at T-N seconds. eBay's proxy bidder gets the max only at fire time, hidden from other bidders. |
| T3 | 9/10 | **Bid groups** (single-win, multi-win, contingency) | `ebay bid-group create <name> --type single-win\|multi-win=N\|contingency` then `ebay snipe ... --group <name>` | Coordinated multi-item snipe management — every paid SaaS sniper has this; no OSS CLI does. |
| T4 | 9/10 | **Has-bids + ending-window search** (user-volunteered) | `ebay auctions --has-bids --min-bids 3 --ending-within 1h --query "Steph Curry"` | Browse API lost bidCount filter when Finding API died. Combines HTML scrape filter + time-window math; nothing else does. |
| T5 | 8/10 | **Watchlist intelligence** (folders, tags, thresholds) | `ebay watch add <id> --folder cards --tag rookie --threshold 200`; `ebay watch ls --folder cards --status alertable` | eBay's watchlist is a flat dump — folders and tags are local-only; threshold alerts cross-reference live price polling. |
| T6 | 8/10 | **Saved-search-to-feed** | `ebay feed <saved-search>` produces stream of new matches with sold-comp context appended | New listings + price-history at the same time = single-pane buyer cockpit. |
| T7 | 7/10 | **Offer hunter** | `ebay offer-hunter <saved-search> --at-percent 80 [--auto-submit]` | Auto-submit best offers at N% of asking across a saved search. Tracks acceptance rate per seller, learns floors. |
| T8 | 7/10 | **Sold-comp by image** | `ebay comp image <path>` | Combines Browse.searchByImage (active matches) with /sch?LH_Sold=1 lookup of identified product. No tool unifies image-search and sold-comps. |
| T9 | 7/10 | **Best-offer-accepted price recovery** | `ebay comp <q> --include-best-offers` | 130point's value prop, applied to any product. Parses sold-listing description for "Accepted $X offer" or `BIN-OBO` indicator. |
| T10 | 6/10 | **Cross-condition normalization** | `ebay comp <q> --normalize-condition` | Buckets results: New / Used / Graded-9 / Graded-10 etc. Surfaces per-bucket median + range. |
| T11 | 6/10 | **Title-variant deduplication** | `ebay comp <q> --dedupe-variants` | Smart matching collapses near-identical titles ("Cooper Flagg /50 Topps Chrome" ↔ "2025-26 Topps Chrome Cooper Flagg #251 Gold /50"). |
| T12 | 6/10 | **Trendline & rolling average** | `ebay comp <q> --trend [--weeks 4]` | Linear/polynomial trendline + rolling average; ASCII chart. driscoll42's pattern in a CLI. |
| T13 | 5/10 | **Auction-end-soon alerts** | `ebay watch end-alert <id> --minutes 5 --notify osascript` | Local notification when watched auction enters final 5 min. |
| T14 | 5/10 | **Snipe simulator** | `ebay snipe <id> --simulate --max 50 --lead 25s` | Show what would happen at fire time without placing the bid. Useful for testing strategy. |
| T15 | 5/10 | **Forter-token refresh** | `ebay auth refresh` | Refreshes Forter token by replaying the item-page bid module GET; required before high-stakes snipes. |

## Stub Disclosure
- T8 (sold-comp by image) ships full when `EBAY_APP_ID` is present (uses Browse.searchByImage). Without it, it ships as `(stub — requires App OAuth for image search)` and prints honest messaging. T1-T7, T9-T15 ship full with browser-session auth alone.

## Auth Flow Summary
- **Primary**: `ebay auth login --chrome` imports cookies from logged-in Chrome profile. Captures cid, s, nonsession, dp1, ebaysid, ds1, ds2, shs, npii. Stores `~/.config/ebay-pp-cli/session.json`.
- **Bid placement specifically**: each `ebay bid place` and each `snipe` fire executes:
  1. `GET /bfl/placebid/<itemId>?currencyId=USD&module=1` — parse the embedded `srt` and `forterToken`.
  2. `POST /bfl/placebid?action=trisk` with the captured tokens.
  3. `POST /bfl/placebid?action=confirmbid` with `{price.value, itemId, srt, autoPayContext.attemptId}`.
- **Read-only paths** (search, sold, item, deals, category, watchlist read): cookie-only, no Forter token refresh needed.

## Provenance
- Phase 1 web research (compound-engineering:ce-web-researcher) produced the source-tool inventory.
- Phase 1.7 authenticated browser-sniff captured live bid placement via `POST /bfl/placebid?action=confirmbid` (full request body recorded in `discovery/browser-sniff-report.md`).
- User-volunteered feature T4 (has-bids + ending-window search) added during Phase 1.7 walk-through.
