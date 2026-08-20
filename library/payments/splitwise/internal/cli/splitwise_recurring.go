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

const (
	// recurringMinCadenceDays is the smallest mean gap that counts as recurring.
	// Below it, repeats are same-period bursts (several taxis on one trip), not
	// a recurring charge.
	recurringMinCadenceDays = 2
	// recurringMaxGapSpread bounds how irregular the intervals may be: the
	// largest gap may be at most this multiple of the smallest. Catches
	// descriptions that recur across unrelated trips at wildly varying spacing.
	recurringMaxGapSpread = 3.0
	// recurringMaxCadenceDays caps how infrequent a recurring charge may be. A
	// recurring charge (rent, utilities, subscriptions) repeats at most about
	// annually; wider mean spacing means the repeats are coincidental, not a
	// recurring obligation, even when they happen to be evenly spaced. The cap
	// sits above a calendar year so an annual charge with late renewals (mean
	// just over 365 days) is not dropped by rounding/billing drift.
	recurringMaxCadenceDays = 400
)

// isSettlementDescription reports whether a description is an auto-generated
// settle-up label. Splitwise stores some of these as non-payment expenses
// (payment=false), so the e.Payment filter alone misses them — they are
// settlements, not recurring charges.
func isSettlementDescription(desc string) bool {
	d := strings.ToLower(strings.TrimSpace(desc))
	switch d {
	case "settle all balances", "settle up", "payment":
		return true
	}
	return strings.HasPrefix(d, "paid via ")
}

// recurringCadence returns the mean inter-expense gap (rounded days) and whether
// the gaps form a regular-enough cadence to count as a recurring charge. It is
// regular when the mean cadence is at least recurringMinCadenceDays and, with
// two or more gaps, the largest gap is at most recurringMaxGapSpread times the
// smallest. Fewer than two gaps (one or two occurrences) cannot establish
// regularity, so the spread check is skipped and only the cadence floor applies.
func recurringCadence(gaps []float64) (cadence int, regular bool) {
	if len(gaps) == 0 {
		return 0, false
	}
	total := 0.0
	for _, gp := range gaps {
		total += gp
	}
	cadence = int(math.Round(total / float64(len(gaps))))
	if cadence < 0 {
		cadence = 0
	}
	if cadence < recurringMinCadenceDays || cadence > recurringMaxCadenceDays {
		return cadence, false
	}
	if len(gaps) >= 2 {
		minGap, maxGap := gaps[0], gaps[0]
		for _, gp := range gaps {
			if gp < minGap {
				minGap = gp
			}
			if gp > maxGap {
				maxGap = gp
			}
		}
		if minGap <= 0 || maxGap > recurringMaxGapSpread*minGap {
			return cadence, false
		}
	}
	return cadence, true
}

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
				if e.Payment || expenseDeleted(e.DeletedAt) || isSettlementDescription(e.Description) {
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
				gaps := make([]float64, 0, len(dates))
				for i := 1; i < len(dates); i++ {
					gaps = append(gaps, dates[i].Sub(dates[i-1]).Hours()/24)
				}
				// Regularity gate: a recurring charge repeats at a roughly steady
				// interval. Drop clusters that are merely the same description seen
				// repeatedly — same-period bursts (tiny cadence, e.g. several taxis
				// in one trip) and irregular spreads (a description that recurs
				// across unrelated trips years apart). Without this, ANY description
				// seen >= min-occurrences times was reported as "recurring" with a
				// meaningless mean-gap cadence.
				var regular bool
				cadence, regular = recurringCadence(gaps)
				if !regular {
					continue
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
				return flags.emitStructured(cmd, view)
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
	cmd.Flags().IntVar(&minOccurrences, "min-occurrences", 3, "Minimum occurrences to treat as recurring (>= 3 lets regularity be assessed)")
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
