package cli

import "github.com/spf13/cobra"

// Preserve the generated constructor path as a delegate to its full implementation.
func newNovelListsCompareCmd(flags *rootFlags) *cobra.Command { return newTabelogListCompareCmd(flags) }
