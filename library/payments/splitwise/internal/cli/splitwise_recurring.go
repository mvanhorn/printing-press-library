package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newRecurringCmd(flags *rootFlags) *cobra.Command {
	limit := 20
	minOccurrences := 2

	cmd := &cobra.Command{
		Use:         "recurring",
		Short:       "Surface repeating charges (rent, utilities, subscriptions) from synced history",
		Example:     "  splitwise-pp-cli recurring --agent\n  splitwise-pp-cli recurring --min-occurrences 3 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would analyze recurring expenses")
				return nil
			}
			if minOccurrences < 2 {
				return usageErr(fmt.Errorf("--min-occurrences must be >= 2"))
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
			hintIfStale(cmd, db, "get-expenses", flags.maxAge)

			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}

			type grouped struct {
				expenses   []Expense
				labels     map[string]int
				currencies map[string]int
			}
			clusters := make(map[string]*grouped)
			scanned := 0
			for _, e := range expenses {
				if e.Payment || e.DeletedAt != nil {
					continue
				}
				scanned++
				key := normalizeRecurringKey(e.Description)
				if key == "" {
					key = "(untitled)"
				}
				if clusters[key] == nil {
					clusters[key] = &grouped{expenses: make([]Expense, 0), labels: make(map[string]int), currencies: make(map[string]int)}
				}
				clusters[key].expenses = append(clusters[key].expenses, e)
				label := strings.TrimSpace(e.Description)
				if label == "" {
					label = key
				}
				clusters[key].labels[label]++
				clusters[key].currencies[strings.TrimSpace(e.CurrencyCode)]++
			}

			type recurringItem struct {
				Description string  `json:"description"`
				Occurrences int     `json:"occurrences"`
				AvgCost     float64 `json:"avg_cost"`
				Currency    string  `json:"currency_code"`
				CadenceDays int     `json:"cadence_days"`
				LastDate    string  `json:"last_date"`
				Overdue     bool    `json:"overdue"`
				lastTime    time.Time
			}
			items := make([]recurringItem, 0)
			now := time.Now().UTC()

			for key, g := range clusters {
				if len(g.expenses) < minOccurrences {
					continue
				}
				total := 0.0
				for _, e := range g.expenses {
					total += parseAmount(e.Cost)
				}
				avg := 0.0
				if len(g.expenses) > 0 {
					avg = round2(total / float64(len(g.expenses)))
				}
				label := mostCommonString(g.labels, key)
				currency := mostCommonString(g.currencies, "")

				dates := make([]time.Time, 0)
				for _, e := range g.expenses {
					if t, ok := parseFlexibleDate(e.Date); ok {
						dates = append(dates, t)
					}
				}
				sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

				cadence := 0
				lastDate := ""
				lastTime := time.Time{}
				if len(dates) > 0 {
					lastTime = dates[len(dates)-1]
					lastDate = lastTime.Format("2006-01-02")
				}
				if len(dates) >= 2 {
					totalGap := 0.0
					for i := 1; i < len(dates); i++ {
						totalGap += dates[i].Sub(dates[i-1]).Hours() / 24
					}
					cadence = int(math.Round(totalGap / float64(len(dates)-1)))
					if cadence < 0 {
						cadence = 0
					}
				}
				overdue := false
				if cadence > 0 && !lastTime.IsZero() {
					daysSince := now.Sub(lastTime).Hours() / 24
					overdue = daysSince > (1.5 * float64(cadence))
				}

				items = append(items, recurringItem{
					Description: label,
					Occurrences: len(g.expenses),
					AvgCost:     avg,
					Currency:    currency,
					CadenceDays: cadence,
					LastDate:    lastDate,
					Overdue:     overdue,
					lastTime:    lastTime,
				})
			}

			sort.Slice(items, func(i, j int) bool {
				if items[i].Occurrences == items[j].Occurrences {
					return items[i].lastTime.After(items[j].lastTime)
				}
				return items[i].Occurrences > items[j].Occurrences
			})
			if len(items) > limit {
				items = items[:limit]
			}

			type viewItem struct {
				Description string  `json:"description"`
				Occurrences int     `json:"occurrences"`
				AvgCost     float64 `json:"avg_cost"`
				Currency    string  `json:"currency_code"`
				CadenceDays int     `json:"cadence_days"`
				LastDate    string  `json:"last_date"`
				Overdue     bool    `json:"overdue"`
			}
			outItems := make([]viewItem, 0, len(items))
			for _, it := range items {
				outItems = append(outItems, viewItem{
					Description: it.Description,
					Occurrences: it.Occurrences,
					AvgCost:     it.AvgCost,
					Currency:    it.Currency,
					CadenceDays: it.CadenceDays,
					LastDate:    it.LastDate,
					Overdue:     it.Overdue,
				})
			}
			view := struct {
				Items           []viewItem `json:"items"`
				ScannedExpenses int        `json:"scanned_expenses"`
			}{Items: outItems, ScannedExpenses: scanned}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, view)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "DESCRIPTION\tOCCURRENCES\tAVG\tCADENCE\tLAST\tOVERDUE")
			for _, row := range outItems {
				_, _ = fmt.Fprintf(tw, "%s\t%d\t%.2f %s\t%d\t%s\t%t\n", row.Description, row.Occurrences, row.AvgCost, row.Currency, row.CadenceDays, row.LastDate, row.Overdue)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum recurring groups to return")
	cmd.Flags().IntVar(&minOccurrences, "min-occurrences", 2, "Minimum occurrences to treat as recurring")
	return cmd
}

func normalizeRecurringKey(s string) string {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(s)))
	for len(tokens) > 0 {
		tail := strings.Trim(tokens[len(tokens)-1], ",./-_")
		if tail == "" || isAllDigits(tail) || isMonthToken(tail) || isDateLikeToken(tail) {
			tokens = tokens[:len(tokens)-1]
			continue
		}
		break
	}
	return strings.Join(tokens, " ")
}

func isMonthToken(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "jan", "january", "feb", "february", "mar", "march", "apr", "april", "may", "jun", "june", "jul", "july", "aug", "august", "sep", "sept", "september", "oct", "october", "nov", "november", "dec", "december":
		return true
	default:
		return false
	}
}

func isDateLikeToken(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	hasDigit := false
	for _, r := range t {
		if r >= '0' && r <= '9' {
			hasDigit = true
			continue
		}
		if r == '-' || r == '/' || r == '.' {
			continue
		}
		return false
	}
	return hasDigit
}

func parseFlexibleDate(s string) (time.Time, bool) {
	input := strings.TrimSpace(s)
	if input == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02T15:04:05Z07:00", "01/02/2006"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, input); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func mostCommonString(m map[string]int, fallback string) string {
	best := fallback
	bestCount := -1
	for k, c := range m {
		if c > bestCount {
			best = k
			bestCount = c
		}
	}
	return best
}
