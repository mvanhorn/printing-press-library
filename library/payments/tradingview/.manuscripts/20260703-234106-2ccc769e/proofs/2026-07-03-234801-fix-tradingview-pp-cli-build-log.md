Manifest transcendence rows: 2 planned, 2 built. Phase 3 passed with all rows shipping.

# TradingView CLI — Build Log

## Built
- Shared helper `internal/cli/tradingview_market.go`: symbol search/resolve, universal /symbol quote fetch, FX rate resolution (direct/inverse/USD-cross), USD-peg handling for USDT/USDC.
- `quote <symbol>` (novel #1): price in native currency + USD + EUR in one call. Auto-resolves bare tickers. Confirmed live: NASDAQ:AAPL 308.63 USD / 269.86 EUR.
- `convert <amount> <from> <to>` (novel #2): FX conversion via TradingView FX_IDC pairs. Confirmed live: 100 USD -> 87.41 EUR.
- `lookup <query>` (absorbed symbol search): resolves to EXCHANGE:TICKER candidates with type/exchange/currency; --type/--exchange/--limit filters. Named `lookup` to avoid collision with the framework `search` (FTS) command.
- Generated scaffold: client with TradingView browser headers (User-Agent/Origin/Referer), config, doctor, promoted endpoint commands `symbols search` and `market quote`.
- Unit tests for pure helpers (stripEm, isUSDLike, fmtNum) + scaffold help-wiring tests.

## Verify-friendly patterns applied
- dryRunOK short-circuit + help-on-bare-invocation on all three novel commands.
- convert uses multi-positional two-check gate (help, then len<3 usageErr, then dryRunOK).
- boundCtx(cmd.Context(), flags) wraps all sibling-client calls (root --timeout honored).
- pp:happy-args annotations for live dogfood fixtures (quote/convert/lookup).
- mcp:read-only on all three (read-only data commands).

## Deferred / not built
- Optional local watchlist (SQLite batch-quote): offered at gate, user declined — out of the two-feature scope.
- Authenticated/real-time WS surfaces: not needed for the requested features; public scanner delivers real-time-ish prices.

## Notes
- USDT/USDC crypto quote currency treated as USD-pegged (surfaced via `note` field).
- Non-USD native currencies convert to USD via FX_IDC:<CUR>USD (with inverse + USD-cross fallback).
