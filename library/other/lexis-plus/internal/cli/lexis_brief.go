// Hand-written: brief command group.

package cli

import "github.com/spf13/cobra"

func newBriefCmd(flags *rootFlags) *cobra.Command {
	root := &cobra.Command{Use: "brief", Short: "Work from the citations in your draft brief (.docx or .txt)"}
	root.AddCommand(newBriefCitesCmd(flags), newBriefCheckCmd(flags), newBriefGapsCmd(flags))
	return root
}
