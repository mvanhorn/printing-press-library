# Absorb Manifest — robinhood-agentic-pp-cli

Sources absorbed: official Agentic Trading MCP (49-50 live tools), the library's existing `robinhood` print (pre-MCP), robin-sdk (gordil), robinhood-for-agents (kevin1chun), alpheus rhmcp Go client (JackZhao98), rel-str tool catalog (rinebob), zaydiscold robinhood-cli-mcp-api, verygoodplugins/robinhood-mcp, rhx, rhood-rs, trayd, open-stocks-mcp, robin_stocks/pyrh legacy surface, alpaca-mcp-server, jkoelker/schwab-mcp, SnapTrade MCP, ticker/cointop/stocksTUI/tstock/wallstreet terminal-UX field, pp-kalshi + pp-prediction-goat library conventions, SecProve guardrail kit, agentic-trading-desk (Oft3r).

### Absorbed (match or beat everything that exists)

Group A — official MCP surface (every live tool becomes a typed command; all reads work offline-first after sync):

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List accounts w/ agentic_allowed + option_level | MCP get_accounts | `accounts list` | flags the one order-capable account; local cache; --json/--select |
| 2 | Portfolio value breakdown + authoritative buying power | MCP get_portfolio | `portfolio show <acct>` | snapshot persisted → local time series (API has none) |
| 3 | Realized P&L buckets | MCP get_realized_pnl | `pnl realized --span` | encodes rhs-account + required asset_classes gotchas; CSV export |
| 4 | Trade-by-trade P&L history | MCP get_pnl_trade_history | `pnl trades` | cursor auto-walk w/ loop guard; synced to SQLite journal |
| 5 | Symbol/name/index search | MCP search | `search <q> --type` | one result-shape (server returns 3 different arrays) |
| 6 | Real-time quotes (≤20) | MCP get_equity_quotes | `quotes <syms>` | transparent batching >20; snapshot history table |
| 7 | Level-2 price book (≤4) | MCP get_equity_price_book | `quotes book <syms>` | undocumented-in-docs tool surfaced; batching |
| 8 | Fundamentals (≤10) | MCP get_equity_fundamentals | `fundamentals <syms>` | batching; offline cache |
| 9 | Quarterly/annual financials | MCP get_financials | `financials <syms>` | undocumented tool surfaced; --period/--limit |
| 10 | Earnings calendar (≤31d window) | MCP get_earnings_calendar | `earnings calendar` | joins vs local positions (see transcendence) |
| 11 | Per-symbol earnings history | MCP get_earnings_results | `earnings results <sym>` | 8-quarter EPS table |
| 12 | OHLCV historicals (15s→50y) | MCP get_equity_historicals | `historicals <syms>` | hyphenated-tool-name quirk hidden; bounds/adjustment enums typed |
| 13 | 18 technical indicators | MCP get_equity_technical_indicators | `indicators <sym> --type` | full enum + per-indicator params as flags |
| 14 | Per-account tradability flags | MCP get_equity_tradability | `tradability <syms>` | pre-order check wired into order preflight |
| 15 | Index lookup + live index quotes | MCP get_indexes + get_index_quotes | `indexes [quotes]` | auto symbol→UUID chaining (server wants UUIDs) |
| 16 | Equity positions | MCP get_equity_positions | `positions list` | pairs quotes for market value; offline; --nonzero |
| 17 | Order history + single order | MCP get_equity_orders | `orders list/get` | state/placed_agent/symbol filters; local audit mirror |
| 18 | Tax lots per symbol | MCP get_equity_tax_lots | `taxlots <sym>` | feeds specified-lot sells (≤30 lots); term/CB columns |
| 19 | Server-side order simulation | MCP review_equity_order | `orders review` (= `orders place --dry-run`) | the CLI's default; prints warnings verbatim |
| 20 | Equity order placement | MCP place_equity_order | `orders place` [write-gated] | ref_id idempotency auto-managed; review-fingerprint required; agentic_allowed pre-verified |
| 21 | Equity order cancel | MCP cancel_equity_order | `orders cancel` [write-gated] | re-reads terminal state after {accepted} race |
| 22 | Options approval status/upgrade | MCP get_option_level_upgrade_info | `options upgrade-info` | plain-English option_level explainer |
| 23 | Option chains | MCP get_option_chains | `options chains <sym>` | expiration_dates flattened |
| 24 | Option contract discovery | MCP get_option_instruments | `options instruments` | expiry/strike/type filters; cursor walk |
| 25 | Option quotes | MCP get_option_quotes | `options quotes` | chain→contract UUID chaining |
| 26 | Option positions | MCP get_option_positions | `options positions` | nonzero/type/expiry filters |
| 27 | Option orders | MCP get_option_orders | `options orders` | state filters; audit mirror |
| 28 | Option order review | MCP review_option_order | `options review` (= place --dry-run) | legs[] built from flags, no raw JSON |
| 29 | Option order placement | MCP place_option_order | `options place` [write-gated] | typed single-leg builder + ref_id |
| 30 | Option order cancel | MCP cancel_option_order | `options cancel` [write-gated] | race-aware re-read |
| 31 | Option watchlist (get/add/remove) | MCP get_option_watchlist, add/remove_option_from_watchlist | `options watchlist` | position_type match enforced |
| 32 | Watchlists list/items | MCP get_watchlists + get_watchlist_items | `watchlists list/items` | owner_type-aware (custom vs curated) |
| 33 | Popular watchlists + follow/unfollow | MCP get_popular_watchlists, follow/unfollow_watchlist | `watchlists popular/follow/unfollow` | follow-limit error explained |
| 34 | Watchlist create/update | MCP create/update_watchlist | `watchlists create/update` | custom-only guard |
| 35 | Watchlist add/remove members | MCP add/remove_from_watchlist | `watchlists add/remove` | symbols XOR pair/index ids typed |
| 36 | Saved scans list | MCP get_scans | `scans list` | local mirror for FTS |
| 37 | Scan-filter DSL discovery | MCP get_scanner_filter_specs | `scans specs` | undocumented tool surfaced; specs cached for validation |
| 38 | Run scanner | MCP run_scan | `scans run` | results → SQLite (screen history) |
| 39 | Create scan / update filters / update config | MCP create_scan, update_scan_filters, update_scan_config | `scans create/set-filters/set-config` [safe writes] | filters validated against cached specs before send |

Group B — features absorbed from community/competitor tools (beyond raw MCP mirroring):

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 40 | OAuth login with PKCE + localhost callback | alpheus auth.go / generated auth.go.tmpl | `auth login` | + RFC 7591 dynamic self-registration (no shipped client secret); robin-sdk has NO refresh — we do |
| 41 | Token refresh + expiry handling | alpheus (4-day tokens) | automatic refresh in transport | headless-safe; `auth status` shows expiry |
| 42 | Write gate: dry-run default + env + flag | existing robinhood print (ROBINHOOD_PP_ALLOW_WRITES) | same convention: `ROBINHOOD_AGENTIC_PP_ALLOW_WRITES=1` + `--live-write` | library house style preserved; review-first adds a third layer |
| 43 | Place-by-preview reference | jkoelker/schwab-mcp place_previewed_order | `orders place --from-review <id>` | review result fingerprint (60s TTL) stops parameter drift between review and place |
| 44 | Session/idle management, read/mutation client split | alpheus rhmcp | transport: reads retry once, mutations never auto-retry; ref_id persisted before submit | ambiguous-retry safety |
| 45 | Multi-account selection everywhere | verygoodplugins #12/#21, robin_stocks #1652 | `--account` flag + default-account config | rollup commands aggregate all accounts |
| 46 | Response slimming for agents | verygoodplugins #13, Alpaca #45 | `--compact` high-gravity fields + `--select` | context-bloat killer, library convention |
| 47 | CSV/portfolio export | robinhood-to-csv, ticker print | `--csv` on list commands + `pnl trades --csv` | tax-season workflow |
| 48 | Cost-basis/lot awareness in views | ticker (multi-lot), robinhood-for-agents tax lots | positions view joins tax lots on demand | term (short/long) surfaced |
| 49 | Watchlist quick-quote board | ticker/stocksTUI watch boards | `watchlists quotes <list>` | one command: items→quotes join |
| 50 | Dividend/income views | zaydiscold dividends/income, robin_stocks demand | `pnl income` (realized P&L asset-class slices) | honest: MCP has no dividend feed; documented boundary vs private-API tools |
| 51 | Rate-limit self-observability | open-stocks-mcp rate_limit_status | `doctor --json` includes limiter state | 429/Retry-After visibility, typed exit 7 |
| 52 | Natural-language command routing | pp house style `which` | `which "<capability>"` | maps asks → commands, exit 0/2 |
| 53 | Health/auth doctor | pp house style + open-stocks-mcp health | `doctor` (metadata probe, token expiry, session, store) | first-run triage |
| 54 | MCP server exposure of the CLI | pp house style (runtime cobratree) | stdio+http transports, code orchestration, hidden endpoint mirrors | Alpaca #45 context pain solved by design |
| 55 | Scan→watchlist promotion | trayd/stocksTUI screen-to-watch flows | `scans run --to-watchlist <list>` [safe write] | one-step screen→track ritual |

Stubs: **none planned.** Every row above ships fully implemented. (Rows 20/21/29/30/39/55 are write commands shipping behind the write gate with review-first defaults — fully functional, not stubs.)

### Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Portfolio history | `portfolio history --sparkline --since 30d` | 10/10 | Queries the local append-only portfolio_snapshots table (captured on every sync/show via get_portfolio) to render a time series with no external dependencies. | MCP has no portfolio-history endpoint; robin_stocks' get_historical_portfolio broke and users miss it; kalshi house bar: snapshot store + sparkline history. |
| 2 | Guard — client-side trade policy | `guard set --max-order 500 --daily-cap 2000` / `guard status` (enforced inside place paths) | 10/10 | Checks a local policy table (per-trade cap, daily cap from the local order journal, concentration from positions, allow/denylist, kill switch) before any mutating tool call. | SecProve sells exactly these guardrail configs because Robinhood ships none natively; scope is all-or-nothing so limits must live client-side. |
| 3 | Order settle | `equities settle <order-id> --wait` | 10/10 | Polls order state to verified terminal truth, past the cancel {accepted} race and the null-until-backfilled market-order price. | alpheus: price null while working, backfilled later; cancel acknowledgement races the fill. |
| 4 | Morning brief | `brief --agent` | 9/10 | One command joining authoritative portfolio, open orders, watchlist quotes, earnings for held symbols, and the local snapshot delta. | Brief Workflow #1 is 4+ round-trips today; get_accounts buying power unreliable vs get_portfolio. |
| 5 | Mutation audit | `audit --since 7d --denied` | 9/10 | Queries the local write journal: reviews, ref_ids, review fingerprints, placements, cancels, guard denials, watchlist/scan outcomes. | Wild agent failure modes incl. silent watchlist failures; no server-side audit trail exists. |
| 6 | P&L win rate | `portfolio winrate --by-symbol` | 9/10 | Pairs synced P&L trade rows into round trips locally; win rate, avg win/loss, per-symbol stats. | zaydiscold ships this only on the private API; kalshi house bar: winrate. |
| 7 | Surface diff | `surface diff` (auto-warn on sync) | 9/10 | Snapshots tools/list names + input schemas into SQLite each sync; diffs consecutive snapshots with dates. | Surface churned 49→50 tools mid-July 2026 unannounced; zero official schema docs. |
| 8 | Wheel status | `wheel status AAPL` | 8/10 | Joins synced option_orders × equity_positions × tax_lots; infers assignment/exercise from expired ITM shorts + position deltas. | No assignment/exercise tool on the MCP; rel-str documents the manual three-way correlation. |

Killed candidates + full customer model: `2026-07-20-170351-novel-features-brainstorm.md`.

**Gate note (autonomous run):** Phase Gate 1.5 approval is carried by the user's standing goal directive ("run through all the printing press steps… don't stop until it will pass the printing press review") captured in the brief's `## User Vision`. Scope = all 55 absorbed rows + all 8 transcendence features, zero stubs, writes triple-gated, live testing read-only. The full showcase is included in the session report for after-the-fact review.
