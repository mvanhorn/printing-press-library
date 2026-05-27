package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var compact bool

var rootCmd = &cobra.Command{
	Use:   "tipmaster",
	Short: "Zero-custody Farcaster RLUSD tip bot — resolve wallets, check leaderboard",
	Long: `TipMaster CLI — Script Master Labs

Agent-native CLI for TipMaster — the zero-custody RLUSD tip bot for Farcaster.
Resolve Farcaster usernames to XRPL wallet addresses, browse the weekly
leaderboard, and query user account status. Ideal for AI agents that need to
route RLUSD tips to creators programmatically.

Environment:
  TIPMASTER_BASE_URL   override the base URL (default: https://tipmaster.onrender.com)

Exit codes: 0 success · 2 usage · 3 not found · 5 API error`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(2)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&compact, "compact", false, "compact JSON output")

	rootCmd.AddCommand(
		resolveCmd,
		leaderboardCmd,
		userCmd,
		statusCmd,
	)
}
