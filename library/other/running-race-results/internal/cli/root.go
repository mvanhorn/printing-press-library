// internal/cli/root.go
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/catalog"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/provider"
	"github.com/spf13/cobra"
)

// NewRoot builds the root command with all subcommands wired to reg.
func NewRoot(reg *provider.Registry) *cobra.Command {
	// Embedded catalog; parse is covered by catalog package tests.
	entries, _ := catalog.Load()
	root := &cobra.Command{
		Use:           "running-race-results-pp-cli",
		Short:         "Look up a runner's race result by race name + bib",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newLookupCmd(reg, entries))
	root.AddCommand(newAthleteCmd(reg))
	root.AddCommand(newAgentContextCmd(root))
	return root
}
