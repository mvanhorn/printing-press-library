package cli

import (
	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/nlm"
)

func newNotebooksDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <notebook-id>",
		Short: "AI-generated summary and suggested topics",
		Example: "  notebooklm-pp-cli notebooks describe <notebook-id>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			var resp nlm.NotebookDescribeResponse
			if err := r.RunJSON(&resp, "notebook", "describe", args[0]); err != nil {
				return err
			}
			return printJSON(resp.Value)
		},
	}
}
