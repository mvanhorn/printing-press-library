# Kalshi CLI Absorb Manifest (REDO, 2026-05-04)

## Sources Cataloged
1. austron24/kalshi-cli (Python, 14★) — most complete general CLI
2. JThomasDevs/kalshi-cli (Python, 1★) — smart search, interactive drill-down
3. OctagonAI/kalshi-deep-trading-bot (TypeScript, 168★) — AI research, Kelly sizing, SQLite
4. newyorkcompute/kalshi (TypeScript, 3★) — 14 MCP tools + TUI + market maker
5. yakub268/kalshi-mcp (TypeScript, 0★) — production-grade MCP, trending markets
6. fsctl/go-kalshi (Go, 2★) — Go client, semantic order types
7. ammario/kalshi (Go) — alternate Go client
8. AndrewNolte/KalshiPythonClient — OpenAPI-generated Python client
9. kurushdubash/kalshi-sdk (TypeScript) — markets/exchange/collections client
10. humz2k/kalshi-python-unofficial — lightweight Python wrapper
11. **NEW: 9crusher/mcp-server-kalshi (TypeScript)** — Docker-deployable Kalshi MCP, BASE_URL env switch
12. **NEW: joinQuantish/kalshi-mcp** — DFlow on Solana integration MCP
13. **NEW: kalashdotai/mcp** — research/analysis tools MCP
14. **NEW: JamesANZ/prediction-market-mcp** — multi-source MCP (Polymarket + PredictIt + Kalshi)
15. **NEW: shaanmajid/prediction-mcp** — multi-source Kalshi+Polymarket MCP
16. austron24/kalshi-trader-plugin — Claude Code plugin for AI-assisted trading
17. eishan05/kalshi-agent-skills — agent skills for Kalshi (auth, trading, FIX protocol)
18. machina-sports/sports-skills — Kalshi public market data skills
19. joinQuantish/skills — Claude Code prediction-market trading skills
20. ratacat/claude-skills — kalshi-prediction-market skill
21. kalshi-python (official, semi-public) — auto-generated from OpenAPI

Net delta vs prior absorb (April 2026): +5 MCP servers, +4 agent skills, +2 SDK wrappers. Ecosystem nearly doubled in one month — a real signal that prediction markets are an emerging surface.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Market search by keyword | austron24 `find`, JThomasDevs `search` | `markets search <query>` + FTS5 | Works offline, regex, SQL composable |
| 2 | List markets with filters | austron24 `markets`, newyorkcompute `get_markets` | `markets list --status --category --series` | Offline + --json/--csv/--select |
| 3 | Get market details | austron24 `market`, yakub268 `get_market_details` | `markets get <ticker>` | Includes cached orderbook depth |
| 4 | Orderbook depth | austron24 `orderbook`, newyorkcompute `get_orderbook` | `markets orderbook <ticker>` | Snapshot stored in SQLite for spread tracking |
| 5 | Trade history | austron24 `trades`, newyorkcompute `get_trades` | `markets trades <ticker>` | Persisted for volume analysis |
| 6 | Market candlesticks | Kalshi API (historical) | `markets candles <ticker> --period` | Local OHLC data for charting |
| 7 | List events | newyorkcompute `get_events` | `events list --category --status` | FTS5 search, offline browse |
| 8 | Get event details | newyorkcompute `get_event` | `events get <ticker>` | Includes all child markets |
| 9 | List series | austron24 `series` | `series list` | FTS5 search |
| 10 | Get series details | austron24 `series` | `series get <ticker>` | Includes child events + markets |
| 11 | Portfolio balance | austron24 `balance`, yakub268 `get_portfolio` | `portfolio balance` | Historical tracking in SQLite |
| 12 | Portfolio positions | austron24 `positions`, JThomasDevs enriched | `portfolio positions` | Return %, market titles, P&L |
| 13 | Portfolio fills | austron24 `fills`, newyorkcompute `get_fills` | `portfolio fills` | Persisted for win rate analysis |
| 14 | Portfolio settlements | austron24 `settlements`, newyorkcompute `get_settlements` | `portfolio settlements` | P&L attribution by category |
| 15 | Portfolio summary | austron24 `summary` | `portfolio summary` | Total value, resting orders, exposure |
| 16 | Place order | austron24 `buy`/`sell`, newyorkcompute `create_order` | `orders create --side --type --price --count` | --dry-run, cost preview, --json, scope check |
| 17 | Cancel order | austron24 `cancel`, newyorkcompute `cancel_order` | `orders cancel <order_id>` | --dry-run confirmation |
| 18 | Batch cancel | austron24 `cancel-all`, newyorkcompute `batch_cancel_orders` | `orders cancel-all --market --side` | Scoped batch with preview |
| 19 | Amend order | Kalshi API | `orders amend <order_id> --price --count` | --dry-run |
| 20 | List orders | austron24 `orders`, newyorkcompute `get_orders` | `orders list --status --market` | Historical orders in SQLite |
| 21 | Order queue position | Kalshi API | `orders queue <order_id>` | Shows queue depth |
| 22 | Order groups | Kalshi API | `order-groups list/create/delete/trigger` | Full lifecycle management |
| 23 | Close position | austron24 `close`, fsctl semantic types | `positions close <ticker> --side` | Validates position exists first |
| 24 | Human-readable prices | JThomasDevs ($0.68 / 68%) | All price output: $0.68 (68%) | Default human, --json for raw |
| 25 | Human-readable expiry | JThomasDevs (8h 35m) | Relative time on all market output | Contextual: "8h 35m", "3 days" |
| 26 | Market type detection | JThomasDevs (binary/range/multi/parlay) | Auto-detected from event structure | Adaptive table display |
| 27 | Interactive drill-down | JThomasDevs series→events→markets | `browse` command with numbered selection | Navigate the hierarchy |
| 28 | Search alias expansion | JThomasDevs ("nfl"→"football") | Alias config in SQLite | User-extensible aliases |
| 29 | Trending markets | yakub268 `get_trending_markets` | `markets trending` | By volume, movers, new listings |
| 30 | Exchange status | Kalshi API | `exchange status` | Health check for trading hours |
| 31 | Exchange announcements | Kalshi API | `exchange announcements` | Latest platform news |
| 32 | Exchange schedule | Kalshi API | `exchange schedule` | Trading hours/holidays |
| 33 | API key management | Kalshi API | `api-keys list/create/delete` | Key lifecycle from CLI |
| 34 | Account limits | Kalshi API | `account limits` | Position/order limits |
| 35 | Historical markets | Kalshi API | `historical markets --status settled` | Browse settled markets |
| 36 | Historical fills/orders | Kalshi API | `historical fills/orders` | Full trade history |
| 37 | RFQ/Quotes | Kalshi API (communications) | `rfq list/create/delete`, `quotes list/create/accept` | Block trade negotiation |
| 38 | Subaccounts | Kalshi API | `subaccounts create/transfer/balances` | Multi-account management |
| 39 | Live data/milestones | Kalshi API | `live-data milestone <id>` | Real-time event data |
| 40 | Game stats | Kalshi API | `live-data game-stats <milestone_id>` | Sports data integration |
| 41 | Structured targets | Kalshi API | `targets list/get` | Target-based market sets |
| 42 | Tags/categories search | Kalshi API | `search tags --category` | Browse market taxonomy |
| 43 | Sport filters | Kalshi API | `search filters --sport` | Sport-specific market filters |
| 44 | Fee changes | Kalshi API | `series fee-changes` | Track fee schedule updates |
| 45 | Incentive programs | Kalshi API | `incentive-programs list` | Active reward programs |
| 46 | Multivariate events | Kalshi API | `multivariate list/get/create` | Complex event collections |
| 47 | Doctor/health check | Standard PP CLI | `doctor` | Auth, connectivity, env, **scope-tier detection** |
| 48 | JSON output | austron24 | `--json` on all commands | Valid JSON, pipes to jq |
| 49 | OpenAPI browser | austron24 `endpoints`/`show`/`schema` | `api endpoints/show/schema` | Built-in API reference |
| 50 | Demo environment | Kalshi API | `--demo` flag or `KALSHI_ENV=demo` | Switch to sandbox instantly |
| 51 | **NEW: Read-only vs read/write key scope** | None of the catalogued tools surface this honestly | `auth status` reports detected scope; mutating commands warn pre-flight; doctor includes scope check | User-flagged need; current tools all assume RW or fail mid-script with 403 |

## Transcendence (only possible with our local data layer)

| # | Feature | Command | Why Only We Can Do This | Score (April 2026) |
|---|---------|---------|------------------------|-------|
| 1 | Portfolio attribution | `portfolio attribution --by category --period 30d` | Joins fills + settlements + events + series for time-windowed P&L decomposition | 9/10 |
| 2 | Odds history tracker | `markets history <ticker> --since 7d` | Periodic price snapshots stored locally; renders the price progression with rendered text chart | 8/10 (was dropped in prior build — must rebuild this round) |
| 3 | Win rate analytics | `portfolio winrate --by category` | W/L ratio, EV, ROI across all settled positions joined to category metadata | 8/10 |
| 4 | Settlement calendar | `portfolio calendar` | Upcoming expirations + your positions + expected payouts joined to event expiry | 8/10 |
| 5 | Market movers | `markets movers --period 24h` | Biggest price deltas computed against last-sync price snapshot | 7/10 |
| 6 | Cross-market correlation | `markets correlate <ticker1> <ticker2>` | Pearson r over historical price series stored locally for both markets | 7/10 |
| 7 | Event lifecycle | `events lifecycle <ticker>` | Price/volume progression from event creation through settlement | 7/10 (not built in prior; subagent should re-validate) |
| 8 | Category heatmap | `markets heatmap` | Volume/OI/price-movement aggregated by category to surface hot zones | 7/10 (not built in prior; subagent should re-validate) |
| 9 | Exposure analysis | `portfolio exposure` | Total risk by category + concentration warnings joined to position metadata | 8/10 |
| 10 | Stale position finder | `portfolio stale --days 30` | Positions near expiry where the user hasn't acted recently | 7/10 |

(Subagent will re-score against current personas in the next step.)

## Reprint Reconciliation Notes (Pass 2(d) hints for the subagent)

Prior CLI shipped 7 of these 10. The dropped 3 were:
- **Odds History** — dropped because the snapshot table was missing in v1.2.2-dev's data-layer template. v3.9.0 generator emits the snapshot pattern correctly; rebuild this round.
- **Event Lifecycle** — not built; was in absorb manifest but not in research.json novel_features.
- **Category Heatmap** — not built; was in absorb manifest but not in research.json novel_features.

The user's read-only-vs-read-write call-out in the briefing is a NEW dimension not present in prior research. It is absorbed at row 51 (above) but the subagent should also consider whether scope-aware UX (e.g., a `--scope` filter on `auth status`, or a "what can my key do" rendering) should be elevated to a transcendence feature.
