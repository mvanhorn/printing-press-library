// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/config"
	"github.com/spf13/cobra"
)

var version = "2026.8.2"

// Execute runs the CLI.
func Execute() error {
	var flags rootFlags
	rootCmd := newRootCmd(&flags)
	return rootCmd.Execute()
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "notebooklm-pp-cli",
		Short:        "Gemini Notebook CLI with notebooks, sources, chat, Studio artifacts, and offline search",
		SilenceUsage: true,
		Version:      version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			applyAgentDefaults(flags)
			if os.Getenv("NO_COLOR") != "" {
				flags.noColor = true
			}
			maybeAutoRefresh(cmd, flags)
		},
	}
	rootCmd.SetVersionTemplate("notebooklm-pp-cli {{ .Version }}\n")
	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Emit machine-readable JSON")
	rootCmd.PersistentFlags().BoolVar(&flags.plain, "plain", false, "Plain-text output without formatting")
	rootCmd.PersistentFlags().StringVar(&flags.selectFields, "select", "", "Comma-separated JSON fields to include in output")
	rootCmd.PersistentFlags().BoolVar(&flags.asCSV, "csv", false, "Emit CSV when output is tabular")
	rootCmd.PersistentFlags().BoolVar(&flags.quiet, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Strip verbose metadata fields from JSON output")
	rootCmd.PersistentFlags().BoolVar(&flags.stdin, "stdin", false, "Read additional input from stdin when supported")
	rootCmd.PersistentFlags().BoolVar(&flags.noColor, "no-color", false, "Disable ANSI color output")
	rootCmd.PersistentFlags().StringVar(&flags.configPath, "config", "", "Path to config file")
	rootCmd.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "Show what would run without calling the API")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent-friendly defaults (--json --no-input --yes)")
	rootCmd.PersistentFlags().BoolVar(&flags.noInput, "no-input", false, "Disable interactive prompts")
	rootCmd.PersistentFlags().BoolVar(&flags.yes, "yes", false, "Skip confirmation prompts")
	flags.dataSource = "auto"

	rootCmd.AddCommand(newNotebookCmd(flags))
	rootCmd.AddCommand(newSourceCmd(flags))
	rootCmd.AddCommand(newChatCmd(flags))
	rootCmd.AddCommand(newStudioCmd(flags))
	rootCmd.AddCommand(newShareCmd(flags))
	rootCmd.AddCommand(newWhoamiCmd(flags))
	rootCmd.AddCommand(newSyncCmd(flags))
	rootCmd.AddCommand(newSearchCmd(flags))
	rootCmd.AddCommand(newSQLCmd(flags))
	rootCmd.AddCommand(newExportCmd(flags))
	rootCmd.AddCommand(newStatsCmd(flags))
	rootCmd.AddCommand(newAuthCmd(flags))
	rootCmd.AddCommand(newDoctorCmd(flags))
	rootCmd.AddCommand(newAgentContextCmd(rootCmd))
	rootCmd.AddCommand(newProfileCmd(flags))
	rootCmd.AddCommand(newFeedbackCmd(flags))
	rootCmd.AddCommand(newMCPCmd(flags))
	rootCmd.AddCommand(newVersionCliCmd())
	return rootCmd
}

// ExitCode extracts exit code from an error.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	return exitCodeFor(err)
}

func newVersionCliCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print CLI version string for install verification",
		Example: `  notebooklm-pp-cli version`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("notebooklm-pp-cli %s\n", version)
		},
	}
}

// defaultConfigPath helper for doctor and profile commands.
func defaultConfigPath() string {
	p, err := config.DefaultPath()
	if err != nil {
		return ""
	}
	return p
}
