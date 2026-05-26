package cli

import "github.com/spf13/cobra"

func newResearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research",
		Short: "Research and discover sources",
	}
	cmd.AddCommand(newResearchStartCmd())
	cmd.AddCommand(newResearchStatusCmd())
	cmd.AddCommand(newResearchImportCmd())
	return cmd
}
