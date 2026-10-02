package cli

import (
	"strconv"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var category string
	var sortField string
	var limit int
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List fonts",
		Example: `  gfonts-pp-cli list --category sans-serif --sort trending --limit 10
  gfonts-pp-cli list --sort alpha --limit 5`,
		Args: cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cmdList([]string{"--category", category, "--sort", sortField, "--limit", strconv.Itoa(limit)})
		},
	}
	cmd.Flags().StringVarP(&category, "category", "c", "", "filter by category")
	cmd.Flags().StringVarP(&sortField, "sort", "s", "popularity", "sort by popularity, alpha, date, or trending")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "number of results")
	return cmd
}
