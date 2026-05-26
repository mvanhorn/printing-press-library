package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/nlm"
)

func newQueryCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "query <notebook-id> <question>",
		Short:   "Ask a question against a notebook's sources",
		Example: "  notebooklm-pp-cli query <notebook-id> \"What is this about?\"",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			var resp nlm.QueryResponse
			if err := r.RunJSON(&resp, "notebook", "query", args[0], args[1]); err != nil {
				return err
			}
			if flags.json {
				return printJSON(resp.Value)
			}
			fmt.Println(resp.Value.Answer)
			return nil
		},
	}
}
