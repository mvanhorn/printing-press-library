// Package cli wires the Cobra command tree for mt5-pp-cli.
package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the binary version reported by `mt5-pp-cli --version`. Release
// builds override via -ldflags:
//
//	go build -ldflags "-X github.com/.../internal/cli.Version=v0.1.0" ./...
//
// The MCP server reads the same variable via mcp.ServerVersion().
var Version = "0.1.0-dev"

// ExitErr lets handlers signal a specific exit code documented in the spec.
type ExitErr struct {
	Code int
	Err  error
}

func (e *ExitErr) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitErr) Unwrap() error { return e.Err }

// Exit constants documented in cmd/mt5-pp-cli/main.go.
const (
	ExitOK             = 0
	ExitUsage          = 2
	ExitNotFound       = 3
	ExitAuth           = 4
	ExitBrokerRejected = 5
	ExitSafetyRejected = 6
	ExitRateLimited    = 7
	ExitConfig         = 10
	ExitTerminalDown   = 11
)

// ErrNotImplemented marks scaffolded commands whose handler hasn't landed yet.
var ErrNotImplemented = errors.New("not implemented — scaffolded only; see library/other/mt5/STATUS.md for which phase delivers this")

// NewRootCmd builds the root command and wires every subcommand from the spec.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "mt5-pp-cli",
		Short: "Printing Press CLI for MetaTrader 5 (live + algo + quant)",
		Long: `mt5-pp-cli — Printing Press CLI for MetaTrader 5.

One binary, three audiences:
  - Live discretionary traders: quote, book, positions, order send, close all
  - Algorithmic traders:        history, stats, magic audit, drawdown, r-multiples
  - Quant developers:           bars/ticks copy, features build, replay, backtest

Architecture:
  Go CLI ←(JSON-RPC over stdio)→ Python bridge ←→ MetaTrader5 package

Safety (writes only):
  All write commands are DRY-RUN by default. To send a real order:
    1. Set MT5_LIVE=1 in your environment (unlocks the capability)
    2. Pass --i-understand-this-is-live (arms the specific command)
    3. Re-invoke with --confirm <hash> from the dry-run within 60s

Get started:
  mt5-pp-cli doctor                       # verify Python, MT5 package, terminal
  mt5-pp-cli connect login --account 123 --server Broker-Live --password-env MT5_PASSWORD
  mt5-pp-cli sync all --since 2024-01-01  # mirror everything into local SQLite
  mt5-pp-cli sql "select count(*) from deals"

See mt5-pp-cli doctor for any setup issue and mt5-pp-cli <command> --help per command.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Persistent flags — present on every subcommand per press conventions.
	pf := root.PersistentFlags()
	pf.Bool("json", false, "Force JSON output (default in non-TTY)")
	pf.Bool("agent", false, "Agent mode: --json --compact --no-color --no-input --yes")
	pf.Bool("dry-run", false, "Preview without executing; for writes implies safety hash flow")
	pf.String("select", "", "Comma-separated dotted paths to include in JSON output")
	pf.Bool("human-friendly", false, "Force human-formatted output (tables, colors) even when piped")
	pf.Bool("no-color", false, "Disable colors in output")
	pf.Bool("compact", false, "Compact JSON (no indent)")
	pf.Bool("no-input", false, "Never prompt; fail if input would be needed")
	pf.Bool("yes", false, "Auto-confirm interactive prompts (does NOT bypass safety hash)")
	pf.String("profile", "", "Named connection profile from ~/.config/mt5-pp-cli/config.toml")
	pf.Int64("account", 0, "Restrict mirror reads to this account_login (default: most recently synced account)")
	pf.Duration("timeout", 0, "Per-command timeout (0 = use default)")
	pf.Bool("verbose", false, "Verbose diagnostic output to stderr")

	root.AddCommand(
		// Foundation
		newDoctorCmd(),
		newConnectCmd(),
		newAccountCmd(),
		newTerminalCmd(),
		newSyncCmd(),
		newSQLCmd(),

		// Live traders
		newSymbolsCmd(),
		newQuoteCmd(),
		newBookCmd(),
		newPositionsCmd(),
		newOrdersCmd(),
		newOrderCmd(),
		newPositionCmd(),
		newCloseAllCmd(),
		newRiskCmd(),

		// Algo
		newHistoryCmd(),
		newStatsCmd(),
		newRMultiplesCmd(),
		newCorrelationCmd(),
		newMagicCmd(),

		// Quant
		newBarsCmd(),
		newTicksCmd(),
		newFeaturesCmd(),
		newCalendarCmd(),
		newReplayCmd(),
		newBacktestCmd(),

		// Phase 2 helper
		newHelperCmd(),

		// Phase 2 live event tail (stub today; documents the polling workaround)
		newWatchCmd(),

		// Phase 6 config tooling
		newConfigInitCmd(),
		newAuditCmd(),
	)

	return root
}

// notImpl returns a standard ErrNotImplemented for stubbed handlers, optionally
// annotated with a phase hint shown to the user.
func notImpl(phase string) error {
	if phase == "" {
		return ErrNotImplemented
	}
	return fmt.Errorf("%w (phase: %s)", ErrNotImplemented, phase)
}
