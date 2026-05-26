package cli

import (
	"github.com/spf13/cobra"
	"notebooklm-pp-cli/internal/nlm"
)

func newCrossQueryCmd() *cobra.Command {
	var notebooks string
	cmd := &cobra.Command{
		Use:   "cross-query <question>",
		Short: "Query across multiple notebooks with aggregated citations",
		Example: `  notebooklm-pp-cli cross-query "compare approaches" --notebooks "id1,id2"`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := requireRunner()
			if err != nil {
				return err
			}
			nlmArgs := []string{"cross", "query", args[0]}
			if notebooks != "" {
				nlmArgs = append(nlmArgs, "--notebooks", notebooks)
			}
			var resp nlm.CrossQueryResponse
			if err := r.RunJSON(&resp, nlmArgs...); err != nil {
				return err
			}
			return printJSON(resp.Value)
		},
	}
	cmd.Flags().StringVar(&notebooks, "notebooks", "", "Comma-separated notebook IDs")
	return cmd
}
