# TradingView CLI — Shipcheck

## Result: PASS (7/7 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS |

## Scorecard: 77/100 (Grade B)
- Strong: Output Modes 10, Auth 10, Terminal UX 10, README 10, Doctor 10, Agent Native 10, MCP Remote Transport 10, Local Cache 10, Sync Correctness 10.
- Sample Output Probe: 2/2 (100%) after fixing the quote example to include `symbol` in `--select`.

## Gaps (non-blocking)
- mcp_token_efficiency 4/10 — the cobratree mirror exposes all commands as MCP tools; minor for a 2-endpoint CLI. Candidate for polish.
- Type Fidelity 2/5, Path Validity 5/10 — inherent to a reverse-engineered spec with loose types; hand-coded commands parse responses directly.

## Behavioral correctness (verified live)
- quote NASDAQ:AAPL -> 308.63 USD / 269.86 EUR (correct).
- quote AAPL -> auto-resolves NASDAQ:AAPL.
- quote BINANCE:BTCUSDT -> USDT peg noted, USD+EUR computed.
- convert 100 USD EUR -> 87.41 EUR.
- lookup / --select / --agent all correct.

## Ship recommendation: ship
No functional bugs in shipping-scope features. All flagship features return correct output.
