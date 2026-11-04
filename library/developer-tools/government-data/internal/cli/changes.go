// Catalog snapshot command adapter.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelChangesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "changes"}
	*cmd = *newGovLocalCmd(flags, "changes")
	return cmd
}
