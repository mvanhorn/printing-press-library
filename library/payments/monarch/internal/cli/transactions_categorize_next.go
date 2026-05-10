// Hand-authored novel feature: emit the oldest Uncategorized transaction
// with a deterministic top-1 category suggestion derived from the merchant's
// historical category distribution. No LLM. No server round-trip per row.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newTransactionsCategorizeNextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "categorize-next",
		Short: "Emit the oldest Uncategorized transaction + top-1 category suggestion from local merchant history",
		Long: `Returns one transaction at a time so the categorize-confirm-next ritual
becomes a tight pipe instead of clicking through 80 rows in the web UI.
The suggested category is the most-frequently-used category for that
merchant across the last 200 categorized transactions — a deterministic
histogram lookup, not an LLM call.`,
		Example:     "  monarch-pp-cli transactions categorize-next --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command: "transactions categorize-next",
					Persona: "Priya — Sunday-night CFO",
					Plan:    "Fetch oldest Uncategorized transaction, build merchant→category histogram from recent categorized transactions, return top-1 suggestion",
					Operations: []string{
						"Web_GetTransactionsList (Uncategorized filter, ascending date)",
						"Web_GetTransactionsList (recent categorized transactions for histogram)",
					},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Step 1: oldest uncategorized
			uncatVars := map[string]any{
				"limit":   1,
				"offset":  0,
				"orderBy": "DATE_ASC",
				"filters": map[string]any{
					"needsReview": true,
				},
			}
			rawUncat, err := fetchGraphQL(c, client.TransactionsListQuery, uncatVars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var uncatResp struct {
				All struct {
					TotalCount int `json:"totalCount"`
					Results    []struct {
						ID       string  `json:"id"`
						Amount   float64 `json:"amount"`
						Date     string  `json:"date"`
						Merchant *struct {
							ID, Name string
						} `json:"merchant"`
					} `json:"results"`
				} `json:"allTransactions"`
			}
			if err := json.Unmarshal(rawUncat, &uncatResp); err != nil {
				return apiErr(fmt.Errorf("parsing uncategorized list: %w", err))
			}
			if len(uncatResp.All.Results) == 0 {
				return printJSONOrTable(flags, map[string]any{
					"status": "empty",
					"reason": "No uncategorized transactions found",
				})
			}
			next := uncatResp.All.Results[0]
			suggestion := map[string]any{}
			if next.Merchant != nil && next.Merchant.ID != "" {
				// Step 2: histogram
				histVars := map[string]any{
					"limit":   200,
					"offset":  0,
					"orderBy": "DATE_DESC",
					"filters": map[string]any{
						"merchants": []string{next.Merchant.ID},
					},
				}
				rawHist, err := fetchGraphQL(c, client.TransactionsListQuery, histVars)
				if err == nil {
					var histResp struct {
						All struct {
							Results []struct {
								Category *struct{ ID, Name string } `json:"category"`
							} `json:"results"`
						} `json:"allTransactions"`
					}
					_ = json.Unmarshal(rawHist, &histResp)
					counts := map[string]int{}
					names := map[string]string{}
					for _, r := range histResp.All.Results {
						if r.Category != nil && r.Category.ID != "" {
							counts[r.Category.ID]++
							names[r.Category.ID] = r.Category.Name
						}
					}
					if len(counts) > 0 {
						type kv struct {
							ID    string
							Name  string
							Count int
						}
						var ranked []kv
						for id, c := range counts {
							ranked = append(ranked, kv{ID: id, Name: names[id], Count: c})
						}
						sort.Slice(ranked, func(i, j int) bool { return ranked[i].Count > ranked[j].Count })
						suggestion["category_id"] = ranked[0].ID
						suggestion["category_name"] = ranked[0].Name
						suggestion["confidence_count"] = ranked[0].Count
						suggestion["histogram"] = ranked
					}
				}
			}
			return printJSONOrTable(flags, map[string]any{
				"transaction": next,
				"suggestion":  suggestion,
				"remaining":   uncatResp.All.TotalCount,
			})
		},
	}
	return cmd
}
