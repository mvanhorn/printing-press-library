// Catalog snapshot command adapter.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "coverage"}
	*cmd = *newGovLocalCmd(flags, "coverage")
	return cmd
}
