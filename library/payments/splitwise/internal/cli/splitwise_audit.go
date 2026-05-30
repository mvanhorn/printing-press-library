package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type auditDuplicateCluster struct {
	Description  string  `json:"description"`
	Cost         float64 `json:"cost"`
	CurrencyCode string  `json:"currency_code"`
	Date         string  `json:"date"`
	GroupID      int     `json:"group_id"`
	ExpenseIDs   []int   `json:"expense_ids"`
	Count        int     `json:"count"`
}

type auditCostOutlier struct {
	ExpenseID    int     `json:"expense_id"`
	Description  string  `json:"description"`
	Cost         float64 `json:"cost"`
	CurrencyCode string  `json:"currency_code"`
	Category     string  `json:"category"`
	Date         string  `json:"date"`
	CategoryMean float64 `json:"category_mean"`
}

type auditResult struct {
	Duplicates      []auditDuplicateCluster `json:"duplicates"`
	Outliers        []auditCostOutlier      `json:"outliers"`
	ScannedExpenses int                     `json:"scanned_expenses"`
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	limit := 50
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Audit synced expenses for likely duplicates and per-category cost outliers",
		Example:     "  splitwise-pp-cli audit --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would audit synced expenses")
				return nil
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be >= 1"))
			}
			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "get-expenses")
			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}
			res := runAudit(expenses, limit)
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintf(tw, "LIKELY DUPLICATES (%d)\n", len(res.Duplicates))
			_, _ = fmt.Fprintln(tw, "DESCRIPTION\tCOST\tCURRENCY\tDATE\tGROUP\tCOUNT\tIDS")
			for _, row := range res.Duplicates {
				ids := make([]string, 0, len(row.ExpenseIDs))
				for _, id := range row.ExpenseIDs {
					ids = append(ids, fmt.Sprintf("%d", id))
				}
				_, _ = fmt.Fprintf(tw, "%s\t%.2f\t%s\t%s\t%d\t%d\t%s\n", row.Description, row.Cost, row.CurrencyCode, row.Date, row.GroupID, row.Count, strings.Join(ids, ","))
			}
			_, _ = fmt.Fprintln(tw)
			_, _ = fmt.Fprintf(tw, "COST OUTLIERS (%d)\n", len(res.Outliers))
			_, _ = fmt.Fprintln(tw, "DESCRIPTION\tCOST\tCURRENCY\tCATEGORY\tDATE\tCAT_MEAN\tID")
			for _, row := range res.Outliers {
				_, _ = fmt.Fprintf(tw, "%s\t%.2f\t%s\t%s\t%s\t%.2f\t%d\n", row.Description, row.Cost, row.CurrencyCode, row.Category, row.Date, row.CategoryMean, row.ExpenseID)
			}
			_, _ = fmt.Fprintf(tw, "\nScanned %d expenses.\n", res.ScannedExpenses)
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum findings to report per finding type")
	return cmd
}

func runAudit(expenses []Expense, limit int) auditResult {
	filtered := make([]Expense, 0, len(expenses))
	for _, e := range expenses {
		if e.Payment || expenseDeleted(e.DeletedAt) {
			continue
		}
		filtered = append(filtered, e)
	}
	if limit < 1 {
		limit = 1
	}

	duplicates := detectDuplicateClusters(filtered)
	outliers := detectCostOutliers(filtered)
	if len(duplicates) > limit {
		duplicates = duplicates[:limit]
	}
	if len(outliers) > limit {
		outliers = outliers[:limit]
	}
	return auditResult{
		Duplicates:      duplicates,
		Outliers:        outliers,
		ScannedExpenses: len(filtered),
	}
}

func auditNormalizeDescription(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func auditDateKey(date string) string {
	t := strings.TrimSpace(date)
	if len(t) >= 10 {
		return t[:10]
	}
	return t
}

func detectDuplicateClusters(expenses []Expense) []auditDuplicateCluster {
	type key struct {
		Description string
		Cost        string
		CurrencyCode string
		Date        string
		GroupID     int
	}
	clusters := make(map[key][]Expense)
	for _, e := range expenses {
		cost := fmt.Sprintf("%.2f", round2(parseAmount(e.Cost)))
		k := key{
			Description: auditNormalizeDescription(e.Description),
			Cost:        cost,
			CurrencyCode: strings.TrimSpace(e.CurrencyCode),
			Date:        auditDateKey(e.Date),
			GroupID:     e.GroupID,
		}
		clusters[k] = append(clusters[k], e)
	}
	out := make([]auditDuplicateCluster, 0)
	for k, items := range clusters {
		if len(items) < 2 {
			continue
		}
		ids := make([]int, 0, len(items))
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		sort.Ints(ids)
		out = append(out, auditDuplicateCluster{
			Description:  strings.TrimSpace(items[0].Description),
			Cost:         round2(parseAmount(items[0].Cost)),
			CurrencyCode: k.CurrencyCode,
			Date:         k.Date,
			GroupID:      k.GroupID,
			ExpenseIDs:   ids,
			Count:        len(items),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			if out[i].Description == out[j].Description {
				return out[i].Date < out[j].Date
			}
			return out[i].Description < out[j].Description
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func detectCostOutliers(expenses []Expense) []auditCostOutlier {
	byCategory := make(map[string][]Expense)
	for _, e := range expenses {
		category := strings.TrimSpace(e.Category.Name)
		if category == "" {
			category = "Uncategorized"
		}
		byCategory[category] = append(byCategory[category], e)
	}

	out := make([]auditCostOutlier, 0)
	for category, items := range byCategory {
		n := len(items)
		if n < 5 {
			continue
		}
		sum := 0.0
		values := make([]float64, 0, n)
		for _, e := range items {
			v := parseAmount(e.Cost)
			values = append(values, v)
			sum += v
		}
		mean := sum / float64(n)

		ss := 0.0
		for _, v := range values {
			d := v - mean
			ss += d * d
		}
		stddev := math.Sqrt(ss / float64(n-1))
		if stddev == 0 {
			continue
		}
		cutoff := mean + (3 * stddev)
		for idx, e := range items {
			v := values[idx]
			if v > cutoff {
				out = append(out, auditCostOutlier{
					ExpenseID:    e.ID,
					Description:  strings.TrimSpace(e.Description),
					Cost:         round2(v),
					CurrencyCode: strings.TrimSpace(e.CurrencyCode),
					Category:     category,
					Date:         auditDateKey(e.Date),
					CategoryMean: round2(mean),
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Cost == out[j].Cost {
			if out[i].Category == out[j].Category {
				return out[i].ExpenseID < out[j].ExpenseID
			}
			return out[i].Category < out[j].Category
		}
		return out[i].Cost > out[j].Cost
	})
	return out
}
