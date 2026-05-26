package cli

import "github.com/spf13/cobra"

func newNotebooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notebooks",
		Short: "Manage NotebookLM notebooks",
	}
	cmd.AddCommand(newNotebooksListCmd())
	cmd.AddCommand(newNotebooksGetCmd())
	cmd.AddCommand(newNotebooksCreateCmd())
	cmd.AddCommand(newNotebooksDescribeCmd())
	cmd.AddCommand(newNotebooksRenameCmd())
	cmd.AddCommand(newNotebooksDeleteCmd())
	return cmd
}
