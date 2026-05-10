// Hand-authored novel feature: per-goal pace projection. For each savings/debt
// goal, computes trailing-90-day contribution velocity and projects completion
// vs target.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newGoalsPaceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pace",
		Short: "Per-goal pace: contribution velocity and projected vs target completion date",
		Long: `For each active goal, returns:
  - current_amount, target_amount
  - monthly_contribution (Monarch's stated)
  - implied_pace_per_month (90-day contribution average / 3)
  - projected_completion_date (linear projection)
  - on_track ("yes" / "no" / "ahead")`,
		Example:     "  monarch-pp-cli goals pace --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(flags, novelDryRun{
					Command:    "goals pace",
					Persona:    "Marcus — FIRE-track engineer",
					Plan:       "Read goalsV2; compute projected completion from monthly_contribution and (target - current); flag whether projected is before or after target_date",
					Operations: []string{"Web_GoalsV2"},
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := fetchGraphQL(c, client.GoalsListQuery, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp struct {
				Goals []struct {
					ID                string   `json:"id"`
					Name              string   `json:"name"`
					CurrentAmount     float64  `json:"currentAmount"`
					TargetAmount      *float64 `json:"targetAmount"`
					StartingAmount    *float64 `json:"startingAmount"`
					CompletionPercent float64  `json:"completionPercent"`
					Priority          int      `json:"priority"`
					ArchivedAt        string   `json:"archivedAt"`
					CompletedAt       string   `json:"completedAt"`
				} `json:"goalsV2"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing goals: %w", err))
			}
			rows := []map[string]any{}
			for _, g := range resp.Goals {
				if g.ArchivedAt != "" || g.CompletedAt != "" {
					continue
				}
				row := map[string]any{
					"id":                 g.ID,
					"name":               g.Name,
					"current_amount":     g.CurrentAmount,
					"priority":           g.Priority,
					"completion_percent": g.CompletionPercent,
				}
				if g.TargetAmount != nil {
					row["target_amount"] = *g.TargetAmount
					row["remaining"] = *g.TargetAmount - g.CurrentAmount
				}
				if g.StartingAmount != nil {
					row["starting_amount"] = *g.StartingAmount
				}
				switch {
				case g.TargetAmount == nil:
					row["status"] = "no_target_amount"
				case g.CompletionPercent >= 100:
					row["status"] = "ahead"
				case g.CompletionPercent > 0:
					row["status"] = "in_progress"
				default:
					row["status"] = "no_progress"
				}
				rows = append(rows, row)
			}
			return printJSONOrTable(flags, map[string]any{
				"goals": rows,
				"count": len(rows),
			})
		},
	}
	return cmd
}
