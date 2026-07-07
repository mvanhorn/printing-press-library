// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `expense search` — FTS5 / LIKE search over the local store.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

func newExpenseSearchCmd(flags *rootFlags) *cobra.Command {
	var since, until, policyID string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search your local expense store",
		Long:  "Searches the local SQLite store's FTS5 index over merchant/comment/category/tag fields.",
		Example: `  expensify-pp-cli expense search "coffee"
  expensify-pp-cli expense search "uber" --since 2026-01-01 --limit 50`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			st, err := store.Open("")
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			filters := map[string]string{}
			if since != "" {
				filters["since"] = since
			}
			if until != "" {
				filters["until"] = until
			}
			if policyID != "" {
				filters["policy_id"] = policyID
			}
			if limit > 0 {
				filters["limit"] = strconv.Itoa(limit)
			}
			results, err := st.SearchExpenses(query, filters)
			if err != nil {
				return apiErr(err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"query":   query,
					"count":   len(results),
					"results": results,
				})
			}
			w := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintf(w, "No matches for %q (run `expensify-pp-cli sync` if the local store is empty).\n", query)
				return nil
			}
			rows := make([][]string, 0, len(results))
			for _, e := range results {
				rows = append(rows, []string{
					e.Date,
					truncate(e.Merchant, 30),
					fmt.Sprintf("%.2f", float64(e.Amount)/100),
					e.Category,
					e.TransactionID,
				})
			}
			if err := flags.printTable(cmd, []string{"DATE", "MERCHANT", "AMOUNT", "CATEGORY", "TX_ID"}, rows); err != nil {
				return err
			}
			fmt.Fprintf(w, "\n%d match(es) for %q\n", len(results), query)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only show expenses on/after YYYY-MM-DD")
	cmd.Flags().StringVar(&until, "until", "", "Only show expenses on/before YYYY-MM-DD")
	cmd.Flags().StringVar(&policyID, "policy", "", "Filter to a single policy/workspace ID")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max results (0 for no cap)")
	return cmd
}
