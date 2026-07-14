# MT5PPHelper EA — Phase 2

Status: **stub directory** — the helper EA isn't wired yet. v1 of mt5-pp-cli ships without it; the workarounds below cover the gap.

## Why an EA at all?

The Python `MetaTrader5` package is request/response only. There is no callback when a trade event happens — you have to poll. For three workflows that hurts:

1. **Live trade event streaming** (`mt5-pp-cli watch trades --tail`). Polling `positions_get` every N seconds works but is wasteful and misses sub-second fills.
2. **EA lifecycle control** — start/stop an EA on a chart from the CLI. No Python API for this; only MetaEditor and chart UI can attach EAs.
3. **Strategy Tester orchestration** — drive the in-terminal optimizer from the CLI. The Python API can't open the tester; only the GUI can.

A small MQL5 EA running inside the terminal closes all three gaps. It listens on a named pipe / file watcher and emits trade-transaction events back to mt5-pp-cli over the same channel.

## v1 workarounds (use these until Phase 2)

| Need                          | Workaround                                                  |
|-------------------------------|-------------------------------------------------------------|
| Live trade event tail         | `while true; mt5-pp-cli positions list --json; sleep 2; done` and diff in your tool of choice |
| Start/stop an EA from the CLI | Attach by hand in the terminal; the CLI cannot do this in v1 |
| Strategy Tester from the CLI  | Use the separate `mt5-backtester-pp-cli` CLI (terminal `.ini` mode) |

## Phase 2 design sketch

- `mt5-pp-cli helper install` writes `MT5PPHelper.mq5` into `<terminal data>\MQL5\Experts\` and asks the user to attach it to any chart.
- The EA hooks `OnTradeTransaction` and writes one JSONL line per event to `<terminal data>\Files\mt5-pp-events.jsonl` (or a named pipe on Windows where supported).
- `mt5-pp-cli watch trades --tail` tails that file.
- A second EA command channel reads `<terminal data>\Files\mt5-pp-cmd.jsonl` to receive `attach-ea`, `detach-ea`, `tester-run` instructions.

Out of scope for v1. Spec'd here so the next session has the design at hand.
