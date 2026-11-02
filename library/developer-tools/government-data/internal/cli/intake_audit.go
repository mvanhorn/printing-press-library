// Catalog snapshot command adapter.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelIntakeAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "audit"}
	*cmd = *newGovLocalCmd(flags, "audit")
	return cmd
}
