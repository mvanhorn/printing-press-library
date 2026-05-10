// Hand-authored novel feature: per-category burn-rate analysis. Tells you
// on day 18 that groceries will overrun by $120 with time still to course-correct.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newBudgetsBurnCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn",
		Short: "Per-category budget burn rate with month-end projection",
		Long: `For each active budget, computes:
  spent_per_day = actual / day_of_period
  allocation_per_day = planned / days_in_period
  projected_month_end = spent + spent_per_day * days_remaining
Flags categories where projected_month_end > planned (overshoot).`,
		Example:     "  monarch-pp-cli budgets burn --json --select category,projected_overshoot,days_remaining",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			now := time.Now().UTC()
			start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			daysIn := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour).Day()
			dayOf := now.Day()
			daysLeft := daysIn - dayOf
			startDate := start.Format("2006-01-02")
			endDate := time.Date(now.Year(), now.Month(), daysIn, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command:    "budgets burn",
					Persona:    "Priya — Sunday-night CFO",
					Plan:       fmt.Sprintf("Read budgets for window %s..%s, project to month-end (day %d of %d)", startDate, endDate, dayOf, daysIn),
					Operations: []string{"Common_GetBudgetStatus"},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"startDate": startDate, "endDate": endDate}
			data, err := fetchGraphQL(c, client.BudgetsStatusQuery, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				BudgetData struct {
					MonthlyAmountsByCategory []struct {
						Category struct {
							ID, Name, Icon string
						} `json:"category"`
						MonthlyAmounts []struct {
							Month                 string  `json:"month"`
							PlannedCashFlowAmount float64 `json:"plannedCashFlowAmount"`
							ActualAmount          float64 `json:"actualAmount"`
							RemainingAmount       float64 `json:"remainingAmount"`
						} `json:"monthlyAmounts"`
					} `json:"monthlyAmountsByCategory"`
				} `json:"budgetData"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing budgets: %w", err))
			}
			rows := []map[string]any{}
			for _, mc := range resp.BudgetData.MonthlyAmountsByCategory {
				if len(mc.MonthlyAmounts) == 0 {
					continue
				}
				m := mc.MonthlyAmounts[0]
				if m.PlannedCashFlowAmount == 0 {
					continue
				}
				planned := m.PlannedCashFlowAmount
				actual := -m.ActualAmount // expenses are negative; flip for burn-pace math
				if actual < 0 {
					actual = 0
				}
				perDay := 0.0
				if dayOf > 0 {
					perDay = actual / float64(dayOf)
				}
				projected := actual + perDay*float64(daysLeft)
				overshoot := 0.0
				if projected > planned {
					overshoot = projected - planned
				}
				row := map[string]any{
					"category":            mc.Category.Name,
					"category_id":         mc.Category.ID,
					"planned":             planned,
					"actual":              actual,
					"day_of_period":       dayOf,
					"days_in_period":      daysIn,
					"days_remaining":      daysLeft,
					"per_day_pace":        perDay,
					"projected_month_end": projected,
					"projected_overshoot": overshoot,
					"on_track":            projected <= planned,
				}
				rows = append(rows, row)
			}
			// Sort by projected_overshoot desc
			sort.Slice(rows, func(i, j int) bool {
				return rows[i]["projected_overshoot"].(float64) > rows[j]["projected_overshoot"].(float64)
			})
			return printJSONOrTable(flags, map[string]any{
				"window":          map[string]string{"start": startDate, "end": endDate},
				"day_of_period":   dayOf,
				"days_in_period":  daysIn,
				"days_remaining":  daysLeft,
				"categories":      rows,
				"overshoot_count": countOvershoots(rows),
			})
		},
	}
	return cmd
}

func countOvershoots(rows []map[string]any) int {
	n := 0
	for _, r := range rows {
		if v, ok := r["projected_overshoot"].(float64); ok && v > 0 {
			n++
		}
	}
	return n
}
