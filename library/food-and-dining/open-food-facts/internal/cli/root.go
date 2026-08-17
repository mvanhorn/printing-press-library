// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"time"

	"github.com/spf13/cobra"
)

var version = "2026.8.2"

type rootFlags struct {
	json    bool
	agent   bool
	compact bool
	timeout time.Duration
}

// Execute runs the CLI.
func Execute() error {
	flags := rootFlags{timeout: 20 * time.Second}
	return newRootCmd(&flags).Execute()
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "open-food-facts-pp-cli",
		Short:         "Open Food Facts product, nutrition, allergen, and category lookup recipes",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	rootCmd.SetVersionTemplate("open-food-facts-pp-cli {{ .Version }}\n")
	rootCmd.PersistentFlags().BoolVar(&flags.json, "json", false, "Print JSON output")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Print compact agent-ready JSON")
	rootCmd.PersistentFlags().BoolVar(&flags.compact, "compact", false, "Print compact JSON")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 20*time.Second, "HTTP timeout")
	rootCmd.AddCommand(newProductCmd(flags))
	rootCmd.AddCommand(newSearchCmd(flags))
	rootCmd.AddCommand(newNutritionCmd(flags))
	rootCmd.AddCommand(newAllergensCmd(flags))
	rootCmd.AddCommand(newCompareCmd(flags))
	rootCmd.AddCommand(newCategoryCmd(flags))
	rootCmd.AddCommand(newSourcesCmd(flags))
	rootCmd.AddCommand(newDoctorCmd(flags))
	rootCmd.AddCommand(newVersionCliCmd())

	return rootCmd
}

// ExitCode extracts exit code from an error (always 1 for now).
func ExitCode(err error) int {
	return 1
}

func newVersionCliCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("open-food-facts-pp-cli %s\n", version)
		},
	}
}
