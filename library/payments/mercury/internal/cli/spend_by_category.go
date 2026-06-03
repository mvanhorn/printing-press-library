// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// categorySpend is one category's outflow this period vs the prior period.
type categorySpend struct {
	Category string  `json:"category"`
	Current  float64 `json:"current"`
	Prior    float64 `json:"prior"`
	Delta    float64 `json:"delta"`
}

func newSpendByCategoryCmd(flags *rootFlags) *cobra.Command {
	var flagAccountID string
	var flagDays int

	cmd := &cobra.Command{
		Use:         "spend-by-category",
		Short:       "Roll up outflow by Mercury category with week-over-week delta",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  mercury-pp-cli spend-by-category
  mercury-pp-cli spend-by-category --days 7 --account-id 550e8400-e29b-41d4-a716-446655440000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagDays <= 0 {
				return fmt.Errorf("--days must be greater than 0")
			}

			// Dry-run: emit nothing (composite read-only contract).
			if flags.dryRun {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			accounts, err := fetchAccounts(ctx, c, flags, flagAccountID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			now := time.Now().UTC()
			splitTime := now.AddDate(0, 0, -flagDays)
			priorStart := now.AddDate(0, 0, -2*flagDays).Format("2006-01-02")
			end := now.Format("2006-01-02")

			var txns []mercuryTxn
			for _, a := range accounts {
				accountTxns, err := fetchAccountTxns(ctx, c, flags, a.ID, priorStart, end)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				txns = append(txns, accountTxns...)
			}

			results, dropped := summarizeByCategory(txns, splitTime)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Surface dropped rows so a table reader doesn't mistake an
				// incomplete breakdown for the full picture (JSON exposes it as
				// undated_dropped).
				if dropped > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"note: %d transaction(s) omitted — their timestamps could not be parsed.\n", dropped)
				}
				headers := []string{"CATEGORY", "THIS", "PRIOR", "DELTA"}
				rows := make([][]string, 0, len(results))
				for _, r := range results {
					rows = append(rows, []string{
						r.Category,
						fmt.Sprintf("%.2f", r.Current),
						fmt.Sprintf("%.2f", r.Prior),
						fmt.Sprintf("%+.2f", r.Delta),
					})
				}
				return flags.printTable(cmd, headers, rows)
			}

			payload := map[string]any{
				"accounts":        len(accounts),
				"period_days":     flagDays,
				"undated_dropped": dropped,
				"count":           len(results),
				"results":         results,
			}
			return flags.printJSON(cmd, payload)
		},
	}

	cmd.Flags().StringVar(&flagAccountID, "account-id", "", "Limit to a single account ID (default: all accounts).")
	cmd.Flags().IntVar(&flagDays, "days", 7, "Length in days of the current period; the prior period of equal length precedes it.")
	return cmd
}

// summarizeByCategory buckets outflows (negative amounts) by Mercury category
// into the current period (at/after splitTime) and the immediately prior period
// (before splitTime), with a per-category delta. Transactions whose timestamp
// does not parse are dropped and counted. Sorted by current spend descending.
func summarizeByCategory(txns []mercuryTxn, splitTime time.Time) ([]categorySpend, int) {
	type bucket struct{ current, prior float64 }
	byCat := map[string]*bucket{}
	dropped := 0

	for _, t := range txns {
		amt := float64(t.Amount)
		if amt >= 0 {
			continue // inflows are not spend
		}
		spend := -amt
		ts, ok := parseTxnTime(t)
		if !ok {
			dropped++
			continue
		}
		cat := t.MercuryCategory
		if cat == "" {
			cat = "uncategorized"
		}
		b := byCat[cat]
		if b == nil {
			b = &bucket{}
			byCat[cat] = b
		}
		if ts.Before(splitTime) {
			b.prior += spend
		} else {
			b.current += spend
		}
	}

	results := make([]categorySpend, 0, len(byCat))
	for cat, b := range byCat {
		results = append(results, categorySpend{
			Category: cat,
			Current:  b.current,
			Prior:    b.prior,
			Delta:    b.current - b.prior,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Current == results[j].Current {
			return results[i].Category < results[j].Category
		}
		return results[i].Current > results[j].Current
	})
	return results, dropped
}
