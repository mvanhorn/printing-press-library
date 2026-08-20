package cli

import "github.com/spf13/cobra"

const tagsQuery = `query GetHouseholdTransactionTags($search: String, $limit: Int, $bulkParams: BulkTransactionDataParams) {
  householdTransactionTags(search: $search, limit: $limit, bulkParams: $bulkParams) {
    id
    name
    color
    order
    transactionCount
  }
}`

func newTagsCmd() *cobra.Command {
	var jsonOut bool
	var search string
	var limit int
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "List household transaction tags and transaction counts.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vars := map[string]any{"limit": limit}
			if search != "" {
				vars["search"] = search
			}
			data, err := graphql("GetHouseholdTransactionTags", tagsQuery, vars)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			rows := [][]string{}
			for _, v := range asSlice(data["householdTransactionTags"]) {
				tag := asMap(v)
				rows = append(rows, []string{str(tag["name"]), str(tag["transactionCount"]), str(tag["id"])})
			}
			sortRows(rows, 0)
			return table([]string{"Tag", "Transactions", "ID"}, rows)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON")
	cmd.Flags().StringVar(&search, "search", "", "Search tags by name")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum tags to fetch")
	return cmd
}
