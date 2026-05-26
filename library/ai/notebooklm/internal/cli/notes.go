package cli

import "github.com/spf13/cobra"

func newNotesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Manage notes in notebooks",
	}
	cmd.AddCommand(newNotesCreateCmd())
	cmd.AddCommand(newNotesListCmd())
	cmd.AddCommand(newNotesUpdateCmd())
	cmd.AddCommand(newNotesDeleteCmd())
	return cmd
}
