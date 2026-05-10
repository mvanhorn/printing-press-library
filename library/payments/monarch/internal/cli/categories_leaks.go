// Hand-authored novel feature: find merchants whose transactions split
// across 3+ categories (likely miscategorized) and Uncategorized merchants
// with prior labeled history.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newCategoriesLeaksCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var flagMinCategories int
	cmd := &cobra.Command{
		Use:   "leaks",
		Short: "Find merchants split across multiple categories (likely miscategorized)",
		Long: `Surfaces the cleanup work that compounds over years of inconsistent tagging.
A merchant whose transactions split across 3+ categories almost certainly
needs a rule. An Uncategorized merchant with prior labeled history needs
a single click to backfill.`,
		Example:     "  monarch-pp-cli categories leaks --window 90d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command:    "categories leaks",
					Persona:    "Priya — Sunday-night CFO",
					Plan:       fmt.Sprintf("Build merchant×category histogram for window %s, flag merchants split across >=%d categories", flagWindow, flagMinCategories),
					Operations: []string{"Web_GetTransactionsList"},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start, end := windowFromSince(flagWindow)
			vars := map[string]any{
				"limit":   1000,
				"offset":  0,
				"orderBy": "DATE_DESC",
				"filters": map[string]any{"startDate": start, "endDate": end},
			}
			data, err := fetchGraphQL(c, client.TransactionsListQuery, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				All struct {
					Results []struct {
						Merchant *struct{ ID, Name string } `json:"merchant"`
						Category *struct{ ID, Name string } `json:"category"`
					} `json:"results"`
				} `json:"allTransactions"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing transactions: %w", err))
			}
			// Build merchant -> category counts
			merchantHist := map[string]map[string]int{}
			merchantNames := map[string]string{}
			for _, r := range resp.All.Results {
				if r.Merchant == nil || r.Merchant.ID == "" {
					continue
				}
				merchantNames[r.Merchant.ID] = r.Merchant.Name
				if _, ok := merchantHist[r.Merchant.ID]; !ok {
					merchantHist[r.Merchant.ID] = map[string]int{}
				}
				if r.Category != nil {
					merchantHist[r.Merchant.ID][r.Category.Name]++
				} else {
					merchantHist[r.Merchant.ID]["(uncategorized)"]++
				}
			}
			leaks := []map[string]any{}
			for mid, hist := range merchantHist {
				if len(hist) < flagMinCategories {
					continue
				}
				type kv struct {
					Cat   string
					Count int
				}
				var ranked []kv
				total := 0
				for cat, count := range hist {
					ranked = append(ranked, kv{Cat: cat, Count: count})
					total += count
				}
				sort.Slice(ranked, func(i, j int) bool { return ranked[i].Count > ranked[j].Count })
				leaks = append(leaks, map[string]any{
					"merchant":          merchantNames[mid],
					"merchant_id":       mid,
					"category_count":    len(hist),
					"transaction_count": total,
					"distribution":      ranked,
				})
			}
			sort.Slice(leaks, func(i, j int) bool {
				return leaks[i]["transaction_count"].(int) > leaks[j]["transaction_count"].(int)
			})
			return printJSONOrTable(flags, map[string]any{
				"window":               map[string]string{"start": start, "end": end},
				"min_categories":       flagMinCategories,
				"merchants_with_leaks": leaks,
				"count":                len(leaks),
			})
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "90d", "Lookback window (e.g. 30d, 90d, 6mo, 1y)")
	cmd.Flags().IntVar(&flagMinCategories, "min-categories", 3, "Minimum distinct categories to flag a merchant")
	return cmd
}
