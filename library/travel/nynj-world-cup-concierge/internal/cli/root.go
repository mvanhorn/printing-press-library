// Copyright 2026 USER and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "2026.7.1"

// Execute runs the CLI.
func Execute() error {
	rootCmd := &cobra.Command{
		Use:           "nynj-world-cup-concierge-pp-cli",
		Short:         "CLI for nynj-world-cup-concierge",
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	rootCmd.SetVersionTemplate("nynj-world-cup-concierge-pp-cli {{ .Version }}\n")
	rootCmd.AddCommand(newExtractCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newAgentContextCmd(rootCmd))
	rootCmd.AddCommand(newVersionCliCmd())

	return rootCmd.Execute()
}

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit code %d", e.code)
}

// ExitCode extracts exit code from an error.
func ExitCode(err error) int {
	if coded, ok := err.(exitError); ok {
		return coded.code
	}
	return 1
}

func newVersionCliCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("nynj-world-cup-concierge-pp-cli %s\n", version)
		},
	}
}
