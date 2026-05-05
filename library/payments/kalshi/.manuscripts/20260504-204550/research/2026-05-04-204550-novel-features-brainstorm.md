# Kalshi novel-features brainstorm (full subagent output)

## Customer model

**Persona 1: Maya — the part-time politics/sports trader**
- Today: keeps Kalshi.com open in three tabs, manually clicks each market to see the orderbook, copies tickers into a Google Sheet to track P&L. Has a fragile Python script she's afraid to touch. Cannot answer "did my politics bets actually make money this quarter, after fees?" without exporting CSVs and pivoting in Sheets.
- Weekly ritual: Sunday evening scans current open markets across 4–5 favorite series, decides what to enter, places limit orders; Monday checks settlement on the prior week's expiries.
- Frustration: Reconciling fills against settlements to know what actually paid.

**Persona 2: Dev — the quant/ML hobbyist building a trading model**
- Today: runs personal Postgres he ETLs from Kalshi's REST endpoints every 15 minutes via a homegrown poller. Snapshots break whenever Kalshi changes a field. Zero good story for "show me how the price of `KXPRES-2028-DJT` moved over the last 30 days."
- Weekly ritual: re-train a price-movement model on persisted snapshots, backtest against fills, identify mispriced markets, place small test orders.
- Frustration: Building and maintaining the snapshot pipeline. Every Kalshi wrapper is a stateless API mirror.

**Persona 3: Riley — the journalist/researcher tracking event probabilities**
- Today: quotes Kalshi prices in articles. Opens Kalshi.com, screenshots the orderbook, re-checks the next morning. Has read-only API credentials issued by the newsroom.
- Weekly ritual: monitor a watchlist of ~15 markets, capture daily price snapshots, cross-reference correlated markets.
- Frustration: No tool serves the read-only key tier well — gets 403 surprises mid-script.

**Persona 4: Sam — the high-volume FCM trader operating subaccounts**
- Today: manages a small fund-style book through Kalshi's FCM/subaccount surface. Juggles `KALSHI_API_KEY` swaps to flip between subaccounts.
- Weekly ritual: daily — sync each subaccount's positions/fills, rebalance exposure, cancel stale resting orders by event when news breaks.
- Frustration: Multi-subaccount aggregation. No tool aggregates across the household.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline kill/keep |
|---|------|---------|-------------|---------|--------|---------|
| C1 | Portfolio Attribution | `portfolio attribution` | P&L by category × series × time period | Maya | (a),(c),(d-prior-keep) | KEEP |
| C2 | Win Rate Analytics | `portfolio winrate` | W/L ratio, EV, ROI across settled positions | Maya | (a),(c),(d-prior-keep) | KEEP |
| C3 | Settlement Calendar | `portfolio calendar` | Upcoming settlements with payouts and category breakdown | Maya, Sam | (a),(c),(d-prior-keep) | KEEP |
| C4 | Market Movers | `markets movers` | Markets with biggest swings since last sync | Dev, Maya | (a),(b),(c),(d-prior-keep) | KEEP |
| C5 | Cross-Market Correlation | `markets correlate <a> <b>` | Pairwise correlation of two markets' price histories | Dev | (b),(c),(d-prior-keep) | KEEP |
| C6 | Exposure Analysis | `portfolio exposure` | Total risk by category with concentration warnings | Maya, Sam | (a),(c),(d-prior-keep) | KEEP |
| C7 | Stale Position Finder | `portfolio stale` | Positions near expiry where you haven't traded recently | Maya | (a),(c),(d-prior-keep) | SOFT-KILL |
| C8 | Odds History Tracker | `markets history <ticker>` | Price progression with snapshot deltas + ASCII sparkline | Dev, Riley | (a),(b),(c),(d-prior-reframe) | KEEP |
| C9 | Read-only safe-mode + dry-run | `--read-only` env lock; `--dry-run` on mutators | Client-side block of mutations regardless of key tier | Riley | (a),(e) | KEEP |
| C10 | Watchlist with snapshot diffs | `watch add/list/diff` | Local watchlist + per-ticker delta vs last sync | Riley, Maya | (a),(c) | KEEP |
| C11 | Fill-vs-order reconciliation | `orders reconcile` | Match fills to orders, flag partials/orphans | Maya, Sam | (a),(c) | SOFT-KILL — monthly action |
| C12 | Event-scoped cancel-all | `events cancel-all <event>` | Cancel every order under one event | Sam | (b) | KILL — thin reframing |
| C13 | Subaccount roll-up | `subaccounts rollup` | Aggregate positions/balance/exposure across subaccounts | Sam | (a),(b),(c) | KEEP |
| C14 | Multi-leg parlay decomposer | `markets explain <parlay>` | Decompose parlay into underlying legs | Dev, Maya | (b) | KILL — reimpl risk |
| C15 | Liquidity scanner | `markets liquidity --min-spread` | Find low-liq/wide-spread opportunities | Maya, Dev | (a),(b),(c) | KILL — fold into list flags |
| C16 | Spread-and-skew snapshot | `markets spread <ticker>` | Yes/no spread + percentile vs series | Dev | (b),(c) | KILL — too thin |
| C17 | Read-only `auth probe` | `auth probe` | Active probe of a write endpoint | Riley, Sam | (e) | KILL — duplicate of absorb #51 |
| C18 | Category leaderboard | `markets leaderboard --category` | Top-volume markets in a category over a window | Maya, Riley | (b),(c) | KILL — fold into trending |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence | Source |
|---|---------|---------|-------|--------------|----------|--------|
| 1 | Portfolio Attribution | `portfolio attribution [--since DATE] [--by category\|series]` | 9/10 | Joins local `fills`, `settlements`, `events`, `series` for realized P&L grouped by category or series over a time window | Brief Workflow #5; Maya's frustration; austron24/JThomasDevs lack this; prior-built | prior (kept) |
| 2 | Win Rate Analytics | `portfolio winrate [--category] [--since]` | 8/10 | Joins `fills` × `settlements` to compute wins, losses, EV, ROI | Brief Workflow #5; no competitor computes; prior-built | prior (kept) |
| 3 | Market Movers | `markets movers [--window 24h] [--category]` | 9/10 | Computes price deltas between snapshots in `market_price_history` populated each `markets` sync | Brief Workflow #4; Dev's persona; prior-built | prior (kept) |
| 4 | Odds History Tracker | `markets history <ticker> [--since] [--sparkline]` | 8/10 | Reads `market_price_history` for ticker; emits time-series + ASCII sparkline | Brief Data Layer notes the snapshot table is the missing piece that caused prior drop; Dev/Riley persona-fit | prior (reframed from `markets history`) |
| 5 | Settlement Calendar | `portfolio calendar [--days 14]` | 7/10 | Joins `positions` × `events.expiration_time` × `markets` to project payouts per settlement date | Brief Workflow #5; Maya's Monday ritual; prior-built | prior (kept) |
| 6 | Exposure Analysis | `portfolio exposure [--by category] [--warn-threshold 0.40]` | 7/10 | Joins `positions` × `markets` × `events` × `series` for category exposure; warns when buckets exceed threshold | Brief Workflow #5; Sam's risk view; prior-built | prior (kept) |
| 7 | Cross-Market Correlation | `markets correlate <a> <b> [--window 30d]` | 6/10 | Pearson r on snapshot price series from `market_price_history` | Dev's signal loop; Riley's CPI/Fed-cut framing; pure local compute; prior-built | prior (kept) |
| 8 | Read-only safe-mode + dry-run | `--read-only` global flag/env lock; `--dry-run` on every mutator | 7/10 | Generator emits `--dry-run` on every mutating endpoint mirror (POST/DELETE/PUT/PATCH) that resolves the request without sending; the env/flag lock short-circuits client-side regardless of detected tier | Brief User Vision; Riley persona; transcends absorb row 51 (which only reports + warns) | new |
| 9 | Subaccount roll-up | `subaccounts rollup [--by category]` | 6/10 | Iterates local `subaccounts`, joins each subaccount's positions/balance, emits aggregate household view | Brief Build Priorities; Sam persona; service-specific to FCM tier; no MCP/CLI aggregates | new |
| 10 | Watchlist with snapshot diffs | `watch add/list/remove <ticker>`; `watch diff [--since]` | 6/10 | Local `watchlist` table joined to `market_price_history` for per-ticker price/volume deltas | Brief Workflow #4 ("price progression on watched markets"); Riley/Maya | new |

### Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| Stale Position Finder | Settlement Calendar covers "what settles soon" — the actual question Maya is asking | Settlement Calendar |
| Fill-vs-order reconciliation | Monthly action, not weekly | Portfolio Attribution |
| Event-scoped cancel-all | Thin reframing of absorbed `orders cancel-all --market`; can be a flag | absorb row 18 |
| Multi-leg parlay decomposer | Reimplementation risk; multivariate-event-collections endpoint already does this | absorb row 46 |
| Liquidity scanner | Folded into `markets list --min-spread --max-volume` flags | absorb row 2 |
| Spread-and-skew snapshot | Thin wrapper of orderbook | absorb row 4 |
| `auth probe` standalone | Duplicate of absorb row 51 (doctor-time tier detection + warn) | absorb row 51 |
| Category leaderboard | Fold into `markets trending --category` | absorb row 29 |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| Portfolio Attribution | Keep | Maya's central frustration; local-only join; prior-built. Same command. |
| Odds History Tracker | Reframe | Right idea, prior was dropped because no snapshot table existed. v3.9.0 generator emits the snapshot pattern correctly; rebuild this round. Same command. |
| Win Rate Analytics | Keep | Workflow #5; Maya; prior-built; thesis unchanged. |
| Settlement Calendar | Keep | Maya's Monday ritual; prior-built; thesis unchanged. |
| Market Movers | Keep | Workflow #4; Dev; prior-built; snapshot table now first-class. |
| Cross-Market Correlation | Keep | Dev/Riley fit; pure local compute on snapshot table. |
| Exposure Analysis | Keep | Sam's risk view; prior-built. |
| Stale Position Finder | Drop | Soft persona-fit; Settlement Calendar already covers "what settles soon." Score 4/10. |
