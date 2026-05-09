package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const transactionsQuery = `query GetTransactionsList($offset: Int, $limit: Int, $filters: TransactionFilterInput, $orderBy: TransactionOrdering) {
  allTransactions(filters: $filters) {
    totalCount
    results(offset: $offset, limit: $limit, orderBy: $orderBy) {
      id
      amount
      pending
      date
      plaidName
      notes
      category { id name }
      merchant { id name }
      account { id displayName }
      tags { id name color order }
    }
  }
}`

func newTransactionsCmd() *cobra.Command {
	var jsonOut bool
	var limit, offset, days int
	var startDate, endDate, search, tagID, accountID string
	cmd := &cobra.Command{
		Use:   "transactions",
		Short: "List recent transactions with date, merchant, category, account, amount, and tags.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			filters := map[string]any{"search": search, "categories": []string{}, "accounts": []string{}, "tags": []string{}}
			if accountID != "" {
				filters["accounts"] = []string{accountID}
			}
			if tagID != "" {
				filters["tags"] = []string{tagID}
			}
			if days > 0 && startDate == "" && endDate == "" {
				end := time.Now()
				start := end.AddDate(0, 0, -days)
				startDate = start.Format("2006-01-02")
				endDate = end.Format("2006-01-02")
			}
			if startDate != "" || endDate != "" {
				if startDate == "" || endDate == "" {
					return fmt.Errorf("--start and --end must be provided together")
				}
				filters["startDate"] = startDate
				filters["endDate"] = endDate
			}
			vars := map[string]any{"offset": offset, "limit": limit, "orderBy": "date", "filters": filters}
			data, err := graphql("GetTransactionsList", transactionsQuery, vars)
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(data)
			}
			root := asMap(data["allTransactions"])
			rows := [][]string{}
			for _, v := range asSlice(root["results"]) {
				txn := asMap(v)
				merchant := str(field(txn, "merchant", "name"))
				if merchant == "" {
					merchant = str(txn["plaidName"])
				}
				tags := []string{}
				for _, tv := range asSlice(txn["tags"]) {
					tags = append(tags, str(asMap(tv)["name"]))
				}
				rows = append(rows, []string{
					str(txn["date"]),
					merchant,
					str(field(txn, "category", "name")),
					str(field(txn, "account", "displayName")),
					money(txn["amount"]),
					strings.Join(tags, ", "),
				})
			}
			return table([]string{"Date", "Merchant", "Category", "Account", "Amount", "Tags"}, rows)
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output raw JSON")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum transactions to fetch")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	cmd.Flags().IntVar(&days, "days", 30, "Look back this many days when --start/--end are omitted")
	cmd.Flags().StringVar(&startDate, "start", "", "Start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endDate, "end", "", "End date YYYY-MM-DD")
	cmd.Flags().StringVar(&search, "search", "", "Search text")
	cmd.Flags().StringVar(&tagID, "tag-id", "", "Filter by Monarch tag ID")
	cmd.Flags().StringVar(&accountID, "account-id", "", "Filter by Monarch account ID")
	return cmd
}
