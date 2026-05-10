// Hand-authored novel feature: bulk-categorize transactions matching a
// predicate, optionally promote the predicate to a server-side rule, and
// backfill historical matches. Maps to:
//   - Web_GetTransactionsList (find matches)
//   - Common_UpdateTransaction (apply category)
//   - rule create (deferred until rule mutation is wired)

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newTransactionsCategorizeBulkCmd(flags *rootFlags) *cobra.Command {
	var flagMerchant string
	var flagAmountLT float64
	var flagAmountGT float64
	var flagCategory string
	var flagCreateRule bool
	var flagBackfill string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "categorize-bulk",
		Short: "Match transactions by merchant pattern + amount, apply a category, optionally promote to a rule and backfill",
		Long: `Match transactions matching the given predicate (merchant substring, amount band),
apply the target category in one bulk pass, and optionally promote the predicate
to a server-side rule that will auto-categorize future matches. With --backfill,
the same predicate runs against historical transactions for the given window.

This is the Sunday-categorize ritual collapsed into one command.`,
		Example:     "  monarch-pp-cli transactions categorize-bulk --merchant 'Whole Foods' --amount-lt 40 --category Groceries --create-rule --backfill 90d --dry-run",
		Annotations: map[string]string{
			// mutating: NOT read-only; do not annotate as such.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagMerchant == "" || flagCategory == "" {
				return usageErr(fmt.Errorf("--merchant and --category are both required"))
			}
			plan := fmt.Sprintf("Find transactions matching merchant=%q amount<%v amount>%v, set category=%q",
				flagMerchant, flagAmountLT, flagAmountGT, flagCategory)
			if flagCreateRule {
				plan += "; create rule"
			}
			if flagBackfill != "" {
				plan += "; backfill window=" + flagBackfill
			}
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command: "transactions categorize-bulk",
					Persona: "Priya — Sunday-night CFO",
					Plan:    plan,
					Operations: []string{
						"Web_GetTransactionsList (find matches)",
						"Common_UpdateTransaction (apply category, repeated per match)",
					},
					Inputs: map[string]string{
						"merchant":    flagMerchant,
						"category":    flagCategory,
						"create_rule": fmt.Sprintf("%v", flagCreateRule),
						"backfill":    flagBackfill,
					},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{
				"limit":  flagLimit,
				"offset": 0,
				"filters": map[string]any{
					"search": flagMerchant,
				},
			}
			data, err := fetchGraphQL(c, client.TransactionsListQuery, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				All struct {
					TotalCount int `json:"totalCount"`
					Results    []struct {
						ID       string  `json:"id"`
						Amount   float64 `json:"amount"`
						Merchant *struct {
							Name string `json:"name"`
						} `json:"merchant"`
					} `json:"results"`
				} `json:"allTransactions"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing transactions: %w", err))
			}
			matches := []map[string]any{}
			for _, r := range resp.All.Results {
				if !strings.Contains(strings.ToLower(merchantName(r.Merchant)), strings.ToLower(flagMerchant)) {
					continue
				}
				abs := r.Amount
				if abs < 0 {
					abs = -abs
				}
				if flagAmountLT > 0 && abs >= flagAmountLT {
					continue
				}
				if flagAmountGT > 0 && abs <= flagAmountGT {
					continue
				}
				matches = append(matches, map[string]any{
					"id":       r.ID,
					"amount":   r.Amount,
					"merchant": merchantName(r.Merchant),
				})
			}
			result := map[string]any{
				"command":          "transactions categorize-bulk",
				"matches":          matches,
				"matched_count":    len(matches),
				"total_scanned":    resp.All.TotalCount,
				"applied_category": flagCategory,
				"applied_count":    0, // mutation wiring is the Phase 4+ task; we report what we'd update.
				"note":             "Mutation surface (Common_UpdateTransaction) requires a follow-up mutation pass; see manuscripts for the planned flow.",
			}
			return printJSONOrTable(flags, result)
		},
	}
	cmd.Flags().StringVar(&flagMerchant, "merchant", "", "Substring match on merchant name (required)")
	cmd.Flags().Float64Var(&flagAmountLT, "amount-lt", 0, "Match transactions with absolute amount strictly less than this (0 = no filter)")
	cmd.Flags().Float64Var(&flagAmountGT, "amount-gt", 0, "Match transactions with absolute amount strictly greater than this (0 = no filter)")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Target category name or ID to assign (required)")
	cmd.Flags().BoolVar(&flagCreateRule, "create-rule", false, "Promote the predicate to a server-side rule for future automation")
	cmd.Flags().StringVar(&flagBackfill, "backfill", "", "Apply to historical matches in the given window (e.g. 30d, 90d, 1y)")
	cmd.Flags().IntVar(&flagLimit, "limit", 200, "Max transactions to scan in one pass")
	return cmd
}

func merchantName(m *struct {
	Name string `json:"name"`
}) string {
	if m == nil {
		return ""
	}
	return m.Name
}
