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

type forecastEntry struct {
	Description    string  `json:"description"`
	Group          string  `json:"group"`
	LastDate       string  `json:"last_date"`
	ExpectedDate   string  `json:"expected_date"`
	ExpectedAmount float64 `json:"expected_amount"`
	CadenceDays    int     `json:"cadence_days"`
	Occurrences    int     `json:"occurrences"`
	Overdue        bool    `json:"overdue"`
}

func looksLikeSettlement(desc string) bool {
	d := strings.ToLower(strings.TrimSpace(desc))
	switch d {
	case "settle all balances", "settle up", "payment":
		return true
	}
	return strings.HasPrefix(d, "paid via ")
}

// computeForecast projects upcoming obligations. groupNames maps group_id -> display name
// (with 0 -> "Non-group"). now is the reference time. windowDays is the forecast window.
// limit caps the returned slice (apply after sorting). Returns entries sorted by
// expected_date ascending.
func computeForecast(expenses []Expense, groupNames map[int]string, now time.Time, windowDays, limit int) []forecastEntry {
	type cluster struct {
		dates      []time.Time
		costs      []float64
		labels     map[string]int
		groupCount map[int]int
	}

	clusters := make(map[string]*cluster)
	for _, e := range expenses {
		if e.Payment || expenseDeleted(e.DeletedAt) || looksLikeSettlement(e.Description) {
			continue
		}
		key := normalizeRecurringKey(e.Description)
		if key == "" {
			continue
		}
		if clusters[key] == nil {
			clusters[key] = &cluster{
				dates:      make([]time.Time, 0),
				costs:      make([]float64, 0),
				labels:     make(map[string]int),
				groupCount: make(map[int]int),
			}
		}
		c := clusters[key]
		if t, ok := parseFlexibleDate(e.Date); ok {
			c.dates = append(c.dates, t)
			c.costs = append(c.costs, parseAmount(e.Cost))
			label := strings.TrimSpace(e.Description)
			if label == "" {
				label = key
			}
			c.labels[label]++
			c.groupCount[e.GroupID]++
		}
	}

	result := make([]forecastEntry, 0)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	windowEnd := today.AddDate(0, 0, windowDays)
	for key, c := range clusters {
		if len(c.dates) < 3 {
			continue
		}
		sort.Slice(c.dates, func(i, j int) bool { return c.dates[i].Before(c.dates[j]) })

		gaps := make([]float64, 0, len(c.dates)-1)
		totalGap := 0.0
		for i := 1; i < len(c.dates); i++ {
			gap := c.dates[i].Sub(c.dates[i-1]).Hours() / 24
			gaps = append(gaps, gap)
			totalGap += gap
		}
		if len(gaps) == 0 {
			continue
		}
		cadence := int(math.Round(totalGap / float64(len(gaps))))
		if cadence < 2 || cadence > 400 {
			continue
		}
		// Invariant: len(c.dates) >= 3 implies len(gaps) >= 2.
		minGap := gaps[0]
		maxGap := gaps[0]
		for _, gap := range gaps[1:] {
			if gap < minGap {
				minGap = gap
			}
			if gap > maxGap {
				maxGap = gap
			}
		}
		if minGap <= 0 {
			continue
		}
		if maxGap > 3.0*minGap {
			continue
		}

		lastDate := c.dates[len(c.dates)-1]
		expectedDate := lastDate.AddDate(0, 0, cadence)
		overdue := expectedDate.Before(today)
		if overdue {
			// Staleness cap: once a recurring series has missed several
			// consecutive cycles it has clearly stopped (e.g. a charge shared
			// with a since-departed roommate), so stop surfacing it as
			// "overdue" forever.
			const maxMissedCycles = 3
			if today.After(expectedDate.AddDate(0, 0, maxMissedCycles*cadence)) {
				continue
			}
		} else if expectedDate.After(windowEnd) {
			continue
		}

		totalCost := 0.0
		for _, cost := range c.costs {
			totalCost += cost
		}
		expectedAmount := 0.0
		if len(c.costs) > 0 {
			expectedAmount = round2(totalCost / float64(len(c.costs)))
		}

		groupID := 0
		bestCount := -1
		for id, count := range c.groupCount {
			if count > bestCount || (count == bestCount && id < groupID) {
				groupID = id
				bestCount = count
			}
		}
		groupName := ""
		if groupID == 0 {
			groupName = groupNames[0]
		} else if name, ok := groupNames[groupID]; ok && strings.TrimSpace(name) != "" {
			groupName = strings.TrimSpace(name)
		} else {
			groupName = fmt.Sprintf("Group %d", groupID)
		}

		result = append(result, forecastEntry{
			Description:    mostCommonString(c.labels, key),
			Group:          groupName,
			LastDate:       lastDate.Format("2006-01-02"),
			ExpectedDate:   expectedDate.Format("2006-01-02"),
			ExpectedAmount: expectedAmount,
			CadenceDays:    cadence,
			Occurrences:    len(c.dates),
			Overdue:        overdue,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].ExpectedDate == result[j].ExpectedDate {
			return result[i].Description < result[j].Description
		}
		return result[i].ExpectedDate < result[j].ExpectedDate
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

// pp:data-source local
func newForecastCmd(flags *rootFlags) *cobra.Command {
	days := 35
	limit := 50

	cmd := &cobra.Command{
		Use:         "forecast",
		Short:       "Project upcoming shared obligations from recurring spending patterns",
		Example:     "  splitwise-pp-cli forecast --agent\n  splitwise-pp-cli forecast --days 60 --limit 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would forecast upcoming obligations")
				return nil
			}
			if days < 1 {
				return usageErr(fmt.Errorf("--days must be >= 1"))
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
			groups, err := loadGroups(db)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			groupNames := make(map[int]string)
			groupNames[0] = "Non-group"
			for _, g := range groups {
				groupNames[g.ID] = strings.TrimSpace(g.Name)
			}

			entries := computeForecast(expenses, groupNames, now, days, limit)
			if entries == nil {
				entries = make([]forecastEntry, 0)
			}
			view := struct {
				AsOf       string          `json:"as_of"`
				WindowDays int             `json:"window_days"`
				Upcoming   []forecastEntry `json:"upcoming"`
			}{
				AsOf:       now.Format("2006-01-02"),
				WindowDays: days,
				Upcoming:   entries,
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, view)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "As of %s, forecast window %d days:\n", view.AsOf, days)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "DESCRIPTION\tGROUP\tEXPECTED\tAMOUNT\tCADENCE\tLAST\tOCCURRENCES\tOVERDUE")
			for _, row := range view.Upcoming {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%.2f\t%d\t%s\t%d\t%t\n", row.Description, row.Group, row.ExpectedDate, row.ExpectedAmount, row.CadenceDays, row.LastDate, row.Occurrences, row.Overdue)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 35, "Forecast window in days")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum upcoming entries to return")
	return cmd
}
