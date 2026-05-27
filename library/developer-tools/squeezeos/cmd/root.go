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
	Use:   "squeezeos",
	Short: "Institutional AI market intelligence — pay per call with RLUSD",
	Long: `SqueezeOS CLI — Script Master Labs

Agent-native CLI for the SqueezeOS market intelligence API.
Squeeze scanner, options flow, AI council verdicts, signal marketplace,
futures market, and conditional settlement contracts.

Authentication:
  Premium endpoints require a payment token from 402Proof.
  Set SQUEEZEOS_TOKEN env var after purchasing via:
    https://four02proof.onrender.com

  SQUEEZEOS_BASE_URL  override the API base (default: https://squeezeos-api.onrender.com)
  SQUEEZEOS_TOKEN     X-Payment-Token for premium endpoints

Exit codes: 0 success · 2 usage · 3 not found · 4 auth · 5 API error`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(2)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&compact, "compact", false, "compact JSON output (no indentation)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "print request details without executing")

	rootCmd.AddCommand(
		demoCmd,
		previewCmd,
		councilCmd,
		scanCmd,
		optionsCmd,
		iwmCmd,
		historyCmd,
		marketplaceCmd,
		futuresCmd,
		settlementCmd,
		statusCmd,
	)
}
