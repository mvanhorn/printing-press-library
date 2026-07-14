---
name: pp-mt5
description: "Use this skill whenever the user asks to inspect or operate a MetaTrader 5 account from the shell: live quotes, market depth, positions, orders, history, deals, trading stats (Sharpe, drawdown, win rate, R-multiples), tick/bar export, replay, event-loop backtest, or bulk operations like 'close all losing positions matching ...'. Drives MT5 via a Python subprocess (MetaTrader5 package) with a local SQLite mirror so most reads answer in microseconds. Every write command is dry-run by default, requires a SHA-256 hash to confirm, and live writes need BOTH MT5_LIVE=1 in env AND --i-understand-this-is-live flag. Windows-only for live; mirror-only commands work everywhere. NOT for headless Strategy Tester runs — use /pp-mt5-backtester for that."
author: "ek-labs"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash PowerShell"
metadata:
  openclaw:
    requires:
      bins:
        - mt5-pp-cli
      pythons:
        - MetaTrader5
    install:
      - kind: go
        bins: [mt5-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/mt5/cmd/mt5-pp-cli
      - kind: pip
        packages: [MetaTrader5]
        platform: windows
---

# mt5-pp-cli — Printing Press CLI for MetaTrader 5

## Prerequisites

```bash
go install github.com/mvanhorn/printing-press-library/library/other/mt5/cmd/mt5-pp-cli@latest
py -3 -m pip install MetaTrader5      # Windows only
mt5-pp-cli doctor
```

If `doctor` reports anything red, fix it before invoking any other command — it gives exact remediation per failure.

## When to use

- Live ops: quotes, market depth, positions, orders, place/modify/close orders, bulk close by predicate.
- Performance analysis: deals/orders history, stats summary, drawdown, R-multiples, magic-number audit.
- Quant: tick/bar export to parquet/CSV, tick-accurate replay, event-loop backtest, derived features.
- SQL over a local mirror of everything MT5 knows about your account.

## When NOT to use

- **Headless Strategy Tester runs** with `.ini` files and `.set` parameter files — that's `/pp-mt5-backtester` (the `mt5-backtester-pp-cli` binary). Different binary, different transport, no overlap.
- Order placement on a real-money account before you've practised the safety flow on a demo first. **There is no undo on a filled order.**

## Safety semantics (read before any write)

Every write command is dry-run by default:

1. First invocation prints a SHA-256 of the canonical request and exits **6** (safety-rejected).
2. Re-invoke with `--confirm <hash>` within **60–120 seconds**. The window is the current and previous 60s bucket, so the exact validity depends on when you got the hash.
3. Live writes additionally require **`MT5_LIVE=1`** in env **AND** **`--i-understand-this-is-live`** on the command. Missing either → exit 6.
4. A kill switch file (`kill_switch_file` in `~/.config/mt5-pp-cli/config.toml`) — if present — rejects every write unconditionally.

**As an agent, never set `MT5_LIVE=1` yourself.** That's a user action only.

## Exit codes

`0` ok · `2` usage · `3` not found · `4` auth · `5` broker rejected · `6` safety-rejected · `7` rate limited · `10` config · `11` MT5 terminal unreachable.

`5` vs `6` matters: `5` means the broker rejected (retry might help); `6` means you didn't pass the safety gate (change the command, don't retry).

## Argument Parsing

Given `$ARGUMENTS`:

1. **Empty, `help`, `--help`** → `mt5-pp-cli --help`.
2. **`install`** → `go install ...@latest && py -3 -m pip install MetaTrader5`.
3. **`install mcp`** → `go install github.com/mvanhorn/printing-press-library/library/other/mt5/cmd/mt5-pp-mcp@latest`, then `claude mcp add mt5-pp-mcp -- mt5-pp-mcp`.
4. **Anything else** → resolve to a subcommand from the spec and run with `--agent`. For writes: print the dry-run output, surface the hash to the human, and stop. Re-invoke with `--confirm` only after explicit human approval.

## Output

Default JSON in non-TTY; `--agent` for compact JSON; `--select a,b.c` to narrow.

See `README.md` and `STATUS.md` in this directory.
