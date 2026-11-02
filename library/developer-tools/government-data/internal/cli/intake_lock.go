// Catalog snapshot command adapter.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelIntakeLockCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "lock"}
	*cmd = *newGovLocalCmd(flags, "lock")
	return cmd
}
