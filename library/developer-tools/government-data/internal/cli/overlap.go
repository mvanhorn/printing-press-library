// Catalog snapshot command adapter.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelOverlapCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "overlap"}
	*cmd = *newGovLocalCmd(flags, "overlap")
	return cmd
}
