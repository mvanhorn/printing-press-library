package cli

import "github.com/spf13/cobra"

// Preserve the generated constructor path as a delegate to its full implementation.
func newNovelListsAuditCmd(flags *rootFlags) *cobra.Command { return newTabelogListAuditCmd(flags) }
