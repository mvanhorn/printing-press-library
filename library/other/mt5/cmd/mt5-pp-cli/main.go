// mt5-pp-cli is the Printing Press CLI for MetaTrader 5.
//
// One binary serves three audiences:
//   - Live discretionary traders (quote, book, positions, order send, close all)
//   - Algorithmic traders (history, stats, magic audit, drawdown, r-multiples)
//   - Quant developers (bars/ticks copy, features build, replay, backtest)
//
// All commands route through a Python subprocess (bridge/mt5_bridge.py)
// that wraps the official MetaTrader5 package. A local SQLite mirror at
// ~/.local/share/mt5-pp-cli/store.db keeps queries off the hot path.
//
// Safety: any write command (order send, position close/modify, close all)
// requires BOTH `MT5_LIVE=1` in the environment AND `--i-understand-this-is-live`
// on the command line. Every write also dry-runs and emits a SHA-256 hash;
// the user must re-invoke with `--confirm <hash>` within 60s to actually fire.
package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCodeFor(err))
	}
}

// exitCodeFor preserves explicit codes set by command handlers via cli.ExitErr.
// Anything else returns 1 (generic failure). Documented codes:
//
//	0  ok
//	2  usage
//	3  not found
//	4  auth
//	5  broker rejected
//	6  safety-layer rejected
//	7  rate limited
//	10 config
//	11 MT5 terminal unreachable
func exitCodeFor(err error) int {
	if ee, ok := err.(*cli.ExitErr); ok {
		return ee.Code
	}
	return 1
}
