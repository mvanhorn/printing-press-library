package cli

import "github.com/spf13/cobra"

func newSourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage sources in notebooks",
	}
	cmd.AddCommand(newSourcesListCmd())
	cmd.AddCommand(newSourcesAddCmd())
	cmd.AddCommand(newSourcesGetCmd())
	cmd.AddCommand(newSourcesDescribeCmd())
	cmd.AddCommand(newSourcesContentCmd())
	cmd.AddCommand(newSourcesSyncCmd())
	cmd.AddCommand(newSourcesDeleteCmd())
	return cmd
}
