package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	compact bool
	dryRun  bool
)

var rootCmd = &cobra.Command{
	Use:   "ghost-layer",
	Short: "Proprietary dual-chain XRPL/Base toll gateway for AI agents",
	Long: `Ghost Layer CLI — Script Master Labs

Agent-native CLI for Ghost Layer — the proprietary Web3 checkout and
XRPL-native licensing infrastructure. Bridge RLUSD (XRPL) and USDC (Base/EVM),
mint X402-BEAST-KEY URITokens on Xahau, and query the x402 product catalog.

Environment:
  GHOST_LAYER_BASE_URL   override the base URL (default: https://ghost-layer.onrender.com)
  GHOST_LAYER_WALLET     your XRPL wallet address (rXXX...)

Exit codes: 0 success · 2 usage · 3 not found · 4 auth · 5 API error`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(2)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&compact, "compact", false, "compact JSON output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "print request details without executing")

	rootCmd.AddCommand(
		bridgeCmd,
		x402Cmd,
		agentCmd,
		cubeCmd,
		statusCmd,
	)
}
