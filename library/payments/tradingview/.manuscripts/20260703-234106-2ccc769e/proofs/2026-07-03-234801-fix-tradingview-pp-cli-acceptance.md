# TradingView CLI — Phase 5 Acceptance

Level: Full Dogfood (live, public API — no key required)
Tests: 43/43 passed, 0 failed, 30 skipped (probe types that don't apply per command)
Gate: PASS

## Flagship feature checks (live)
- quote NASDAQ:AAPL -> 308.63 USD / 269.86 EUR — correct.
- quote AAPL -> auto-resolves NASDAQ:AAPL — correct.
- quote BINANCE:BTCUSDT -> USDT-peg noted, USD+EUR computed — correct.
- quote --agent --select symbol,price_usd,price_eur -> filtered output includes symbol — correct.
- convert 100 USD EUR -> 87.41 EUR — correct.
- convert 250 USD EUR --agent -> 218.525 EUR — correct.
- lookup bitcoin --type crypto / lookup AAPL -> candidates with EXCHANGE:TICKER, primary-first — correct.

## Fixes applied during shipcheck/dogfood
- quote example: added `symbol` to --select so the sample-output probe finds the query token.
- narrative value_prop + troubleshoot: `search` -> `lookup` (command was renamed to avoid collision with the framework FTS `search`).

## Printing Press issues for retro
- None material. (MCP token efficiency 4/10 is a scorecard nit for a 2-endpoint CLI.)

Gate: PASS

## Post-completion amendment (user request)
- Renamed the symbol-research command `lookup` -> `search` (with `lookup` kept as an alias), per user request for a "search feature".
- Removed the generator's framework FTS `search` command: it was wired to GET /symbol_search/v3/ against the ROOT host (scanner.tradingview.com) instead of the symbol-search host, so it 404'd — a generator limitation with cross-host per-resource base_url overrides (retro candidate).
- Re-ran shipcheck (7/7 PASS) and live dogfood (43/43) after the change; re-promoted.
