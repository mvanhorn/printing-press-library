package cli

import "github.com/spf13/cobra"

// Preserve the generated constructor path as a delegate to its full implementation.
func newNovelListsAlternativesCmd(flags *rootFlags) *cobra.Command {
	return newTabelogListAlternativesCmd(flags)
}
