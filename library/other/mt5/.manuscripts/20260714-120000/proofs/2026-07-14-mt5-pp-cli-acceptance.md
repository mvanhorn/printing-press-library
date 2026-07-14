# mt5-pp-cli — Acceptance Record (packaging run 20260714-120000)

Two layers of evidence: (A) structural validation executed on the packaging machine on 2026-07-14 (Go 1.26.5, windows/amd64, no MT5 terminal present), and (B) runtime verification performed during the phased build against a live JustMarkets demo account (recorded per-phase in `STATUS.md`).

## A. Packaging-run validation (2026-07-14)

| Check | Result | Evidence |
|---|---|---|
| `go build ./...` | PASS | clean; see `build-log.txt` |
| `go vet ./...` | PASS | clean |
| `go test ./...` | PASS | internal/cli, internal/mcp, internal/safety, internal/store all ok |
| `TestOpenAndMigrateConcurrent -count=10` | PASS | after busy-retry fix (below) |
| `mt5-pp-cli --version` | PASS | `mt5-pp-cli version 0.1.0-dev` |
| `mt5-pp-cli --help` | PASS | full tree renders; capture in `mt5-pp-cli-help.txt` |
| `mt5-pp-mcp --list-tools` | PASS | 18 tools, matches `tools-manifest.json` |
| `verify_skill.py` | PASS | flag-names, flag-commands, positional-args, unknown-command |
| `govulncheck ./...` | PASS | 0 reachable vulnerabilities |

**Fix landed during this run:** `internal/store` migration bootstrap could hit an immediate `SQLITE_BUSY` (bypassing `busy_timeout`) when concurrent opens raced the fresh-file delete→WAL journal-mode conversion — observed as a ~1-in-3 flake in `TestOpenAndMigrateConcurrent`. Fixed with a bounded busy-retry (`retryBusy`, 5 s deadline / 25 ms step) around the `schema_migrations` bootstrap and the `BEGIN IMMEDIATE` lock acquisition. Deterministic pass at `-count=10` after the fix.

## B. Runtime verification during the phased build (from STATUS.md)

Against a JustMarkets demo account:

- **Sync/mirror:** 301 symbols, 120 H1 bars (EURUSD.s), 3,969 ticks pulled and queryable via `sql`.
- **Live reads:** live EURUSD.s spread 6 pts; `risk preview` at 0.10 lots → 384 ZAR margin, ±100 pip P&L of ±1,654.82 ZAR.
- **Writes through the full safety pipeline:** 0.01 EURUSD.s buy placed (retcode 10009, ticket 2329038588), then closed via `close all --filter "profit < 0"` (retcode 10009). Dry-run → exit 6 + hash → confirm flow verified; every attempt audited.
- **Algo stats:** seeded-data checks — win rate 66.7%, max DD 41.00 (78.8%), Sharpe 5.28; live-mirror correlation EURUSD.s/GBPUSD.s H1 7d = +0.77.
- **Quant:** features build (ret, ATR-14, RSI-14, realized vol) on 120 bars; sma-cross(10/30) backtest persisted (2 trades, row id=1).
- **MCP:** stdio JSON-RPC smoke — initialize → tools/list (18) → `mt5_sql "select count(*) n from deals"` → `[{"n":0}]`.
- **Integration suite:** `go test -tags=integration` (triple-gated: build tag + `MT5_PAPER=1` + demo-account check) covering doctor, account/terminal info, sync, reads, dry-run writes, sql, audit tail.

Five post-build review passes (2 external) fixed all 🔴/🟡 findings; see the review table in `STATUS.md`.

## Verdict

**PASS** — structurally validated on the packaging machine; runtime behavior verified on a demo account during the phased build. Live-terminal re-verification on the operator's MT5 host is recommended before first live use (as it is for any install): `mt5-pp-cli doctor`.
