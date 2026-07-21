# Novel-features brainstorm — robinhood-agentic (subagent audit trail, 2026-07-20)

Full three-pass output of the Phase 1.5c.5 novel-features subagent (customer model → candidates → adversarial cut). Survivors feed the absorb manifest's transcendence table; killed candidates and personas are preserved here for retro/dogfood debugging.

## Customer model

**Marcus — the cron-job scripter.** Today: scripts around the app's edges; morning check is four round-trips; pastes results into a spreadsheet because nothing persists; cannot answer "what was my portfolio worth on any given Tuesday" (the MCP has no portfolio-history endpoint). Weekly ritual: pre-open check (positions, day/total P&L, open orders, watchlist movers) plus a weekend week-over-week pass. Frustration: the morning check should be one command, and week-over-week comparison is impossible without hand-kept records.

**Priya — the agent operator.** Today: Claude wired to the MCP running loops from cron/CI; has read the horror stories (agents targeting non-agentic accounts, improvising nonexistent tools, silent watchlist failures); Robinhood's safety guidance is "prompt the LLM to be careful"; scope is all-or-nothing (`internal`); no paper trading so write-path tests cost real money. Weekly ritual: review everything her agent did before adjusting its autonomy. Frustration: can neither constrain the agent (no caps or kill switch exist natively) nor reconstruct what it did after the fact.

**Dave — the robin_stocks refugee.** Today: ran the wheel on robin_stocks until login hardening broke it; misses `get_historical_portfolio`; on the official MCP must manually correlate option orders × equity positions × tax lots to infer assignments (rel-str documents the grind). Weekly ritual: Friday post-expiration wheel bookkeeping (assignments, stages, round trips, win rate). Frustration: the assignment-inference join and round-trip bookkeeping are pure manual labor.

**Sam — the MCP integrator.** Today: builds agent tooling on a surface with zero request/response docs that churns without notice (22 tools counted in June, 49 on Jul 17, 50 on Jul 18); breakage is discovered by eye-diffing saved tools/list JSON. Weekly ritual: re-export tools/list after breakage and before releases. Frustration: change detection is manual and unrecorded.

## Candidates (pre-cut)

16 candidates generated across sources (a) persona-driven, (b) service-specific patterns, (c) cross-entity local queries, (e) user briefing, (f) transport/schema intelligence: portfolio history, morning brief, guard, mutation audit, order settle, wheel status, P&L win rate, surface diff, P&L attribution, explain (observed-schema docs), exposure, held-earnings calendar, buying-power reconcile, watchlist write-verify, read-only transport mode (reframed into guard), dividend projection (cut: no dividends feed on the MCP — unverifiable estimation).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Portfolio history (Marcus, Dave) | `portfolio history --sparkline --since 30d` | 10/10 | Queries the local append-only portfolio_snapshots table (captured on every sync/show via get_portfolio) to render a time series with no external dependencies. | MCP has no portfolio-history endpoint; robin_stocks' get_historical_portfolio broke and users miss it (rel-str); kalshi house bar: snapshot store + sparkline history. |
| 2 | Guard — client-side trade policy (Priya) | `guard set --max-order 500 --daily-cap 2000` / `guard status`, enforced in place paths | 10/10 | Checks a local policy table (per-trade cap, daily cap from the local order journal, concentration from positions, symbol allow/denylist, kill switch) before any mutating tool call; denial path testable without live writes. | SecProve sells exactly these guardrail configs because Robinhood ships no native limits; official guidance is "prompt the LLM to be careful"; OAuth scope is all-or-nothing so limits must be client-side. |
| 3 | Order settle (Marcus, Priya) | `equities settle <order-id> --wait` | 10/10 | Polls order state until terminal, past the cancel {accepted} acknowledgement and until the backfilled market-order price appears, then reports verified fill truth. | alpheus: market-order price is null while working and backfilled later; cancel returns {accepted} racing the fill. |
| 4 | Morning brief (Marcus) | `brief --agent` | 9/10 | Calls get_portfolio (authoritative), open orders, watchlist items + batched quotes, and the earnings calendar, joins with the last local snapshot for deltas, in one command. | Brief Workflow #1 is four round-trips today; rel-str: get_accounts buying power unreliable, get_portfolio authoritative. |
| 5 | Mutation audit (Priya) | `audit --since 7d --denied` | 9/10 | Queries the local write journal (reviews, ref_ids, review fingerprints, placements, cancels, guard denials, watchlist/scan outcomes). | Wild agent failure modes (improvised tools, silent watchlist failures, wrong-account attempts); ref_ids persisted but nothing queries them. |
| 6 | P&L win rate (Dave, Marcus) | `portfolio winrate --by-symbol` | 9/10 | Pairs synced pnl-trade rows into round trips locally; win rate, avg win/loss, per-symbol stats. | zaydiscold ships round-trip win rate only on the private API; kalshi house bar: winrate. |
| 7 | Surface diff (Sam) | `surface diff` (auto-warn on sync) | 9/10 | Snapshots tools/list names + input schemas into SQLite each sync and diffs consecutive snapshots: added/removed/changed tools with dates. | Surface churned 49→50 tools mid-July 2026 without notice; zero official schema docs. |
| 8 | Wheel status (Dave) | `wheel status AAPL` | 8/10 | Joins synced option_orders × equity_positions × tax_lots in SQLite; infers assignment/exercise from expired ITM shorts + position deltas to label wheel stage. | No assignment/exercise history tool on the MCP; rel-str wheel analysis documents the manual three-way correlation; zaydiscold's wheel detection is private-API only. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| P&L attribution | Derived view of the snapshot store, house-style precedent only; ship later as a portfolio-history flag if asked. | Portfolio history |
| Explain (observed-schema docs) | Doc generation without a weekly command; the actionable half is change detection. | Surface diff |
| Exposure | Concentration read-back ships as `guard status` output, not a separate row. | Guard |
| Held-earnings calendar | One-line local filter on an absorbed endpoint; ships as a flag and inside brief. | Morning brief |
| Buying-power reconcile | Not a weekly command; encoded as brief's authoritative-source rule + a doctor check. | Morning brief |
| Watchlist write-verify | Generalized into settle's re-read convention with outcomes recorded in audit. | Order settle |
| Read-only transport mode | Config, not a feature: covered by the write gate + guard's kill switch. | Guard |
| Dividend projection | The MCP has no dividends feed; yield-based projection is unverifiable estimation. | Wheel status |

(Command paths normalized to the generated command tree during manifest merge: `pnl winrate` → `portfolio winrate`; `orders settle` → `equities settle`.)
