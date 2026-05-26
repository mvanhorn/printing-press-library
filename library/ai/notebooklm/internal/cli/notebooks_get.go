package cli

import (
	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/nlm"
)

func newNotebooksGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <notebook-id>",
		Short: "Get notebook details with source list",
		Example: "  notebooklm-pp-cli notebooks get <notebook-id>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			var resp nlm.NotebookGetResponse
			if err := r.RunJSON(&resp, "notebook", "get", args[0]); err != nil {
				return err
			}
			return printJSON(resp.Value)
		},
	}
}
