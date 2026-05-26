package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/nlm"
)

func newNotebooksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all notebooks",
		Example: `  notebooklm-pp-cli notebooks list
  notebooklm-pp-cli notebooks list --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			var notebooks []nlm.Notebook
			if err := r.RunJSON(&notebooks, "notebook", "list"); err != nil {
				return err
			}
			if flags.json {
				return printJSON(notebooks)
			}
			for _, nb := range notebooks {
				fmt.Printf("%s  %s  (%d sources)\n", nb.ID, nb.Title, nb.SourceCount)
			}
			return nil
		},
	}
}
