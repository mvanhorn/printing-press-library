package cli

import (
	"github.com/spf13/cobra"
)

// RootCmd is the top-level obs-pp-cli command.
var RootCmd = &cobra.Command{
	Use:   "obs-pp-cli",
	Short: "OBS Studio control CLI — scenes, layouts, guests, streaming, recording",
	Long: `obs-pp-cli controls OBS Studio via its built-in WebSocket server.

Configure once with: obs-pp-cli configure
Then use subcommands to control OBS from your terminal or agent.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Agent-mode pre-run: in agent mode, default format to JSON unless
		// the caller explicitly chose a different format.
		agent, _ := cmd.Root().PersistentFlags().GetBool("agent")
		if agent {
			if f, _ := cmd.Root().PersistentFlags().GetString("format"); f == "plain" {
				_ = cmd.Root().PersistentFlags().Set("format", "json")
			}
		}
	},
}

func init() {
	// Output format: plain (default), json, select, csv, quiet
	RootCmd.PersistentFlags().String("format", "plain",
		`Output format: "plain", "json", "select", "csv", or "quiet"`)
	RootCmd.PersistentFlags().String("select", "",
		`Comma-separated field names to include in --format=select output`)

	// Agent and automation flags
	RootCmd.PersistentFlags().Bool("agent", false,
		`Enable agent mode: JSON output, no prompts, structured exit codes`)
	RootCmd.PersistentFlags().Bool("dry-run", false,
		`Print what would happen without making changes to OBS`)
	RootCmd.PersistentFlags().Bool("no-color", false,
		`Disable ANSI colour output (also honoured via NO_COLOR env var)`)

	// stdin/non-interactive control
	RootCmd.PersistentFlags().Bool("yes", false,
		`Skip confirmation prompts; assume "yes" to all questions`)
	RootCmd.PersistentFlags().Bool("stdin", false,
		`Read resource names from stdin (one per line) when no args are given`)

	// Register all subcommands
	RootCmd.AddCommand(configureCmd)
	RootCmd.AddCommand(doctorCmd)
	RootCmd.AddCommand(sceneCmd)
	RootCmd.AddCommand(streamCmd)
	RootCmd.AddCommand(recordCmd)
	RootCmd.AddCommand(layoutCmd)
	RootCmd.AddCommand(guestCmd)
	RootCmd.AddCommand(preflightCmd)
	RootCmd.AddCommand(statsCmd)
	RootCmd.AddCommand(healthCmd)
	RootCmd.AddCommand(searchCmd)
	RootCmd.AddCommand(exportCmd)
	RootCmd.AddCommand(profileCmd)
	RootCmd.AddCommand(deliverCmd)
	RootCmd.AddCommand(feedbackCmd)
	RootCmd.AddCommand(trendsCmd)
	RootCmd.AddCommand(analyticsCmd)
}
