package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificCostCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Local credit-cost ledger and forecast (the Magnific API has no /v1/me/credits endpoint)",
		Long: `cost reads the local magnific_tasks ledger to answer questions
the Magnific API does not: how many credits did you spend on which model
this month, and how many would a planned batch cost. Magnific's web
dashboard shows the live balance; this ledger shows your history.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newMagnificCostLedgerCmd(flags))
	cmd.AddCommand(newMagnificCostForecastCmd(flags))
	return cmd
}

func newMagnificCostLedgerCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var groupBy string
	cmd := &cobra.Command{
		Use:         "ledger",
		Short:       "Aggregate credit spend over your local task history",
		Example:     "  magnific-pp-cli cost ledger --since 30d --group-by model --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			dur, err := parseDurationFlag(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("--since %q: %w", sinceStr, err))
			}
			cutoff := time.Now().Add(-dur).UTC().Format(time.RFC3339)

			validGroup := map[string]string{
				"model":  "model",
				"day":    "date(created_at)",
				"status": "status",
				"tag":    "tag",
			}
			groupExpr, ok := validGroup[groupBy]
			if !ok {
				return usageErr(fmt.Errorf("--group-by %q: must be one of model, day, status, tag", groupBy))
			}

			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}

			q := fmt.Sprintf(`
				SELECT COALESCE(%s,''), COUNT(*), COALESCE(SUM(credit_cost), 0)
				FROM magnific_tasks
				WHERE created_at >= ?
				GROUP BY %s
				ORDER BY SUM(credit_cost) DESC`, groupExpr, groupExpr)
			rows, err := db.DB().QueryContext(ctx, q, cutoff)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type row struct {
				Key     string  `json:"key"`
				Tasks   int     `json:"tasks"`
				Credits float64 `json:"credits"`
			}
			var out []row
			var totalCredits float64
			var totalTasks int
			for rows.Next() {
				var r row
				var k sql.NullString
				var c sql.NullInt64
				var s sql.NullFloat64
				if err := rows.Scan(&k, &c, &s); err != nil {
					continue
				}
				r.Key = k.String
				r.Tasks = int(c.Int64)
				r.Credits = s.Float64
				totalCredits += r.Credits
				totalTasks += r.Tasks
				out = append(out, r)
			}

			result := map[string]any{
				"group_by":      groupBy,
				"since":         sinceStr,
				"total_tasks":   totalTasks,
				"total_credits": totalCredits,
				"rows":          out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "30d", "Aggregate over this duration (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&groupBy, "group-by", "model", "Grouping: model, day, status, tag")
	return cmd
}

func newMagnificCostForecastCmd(flags *rootFlags) *cobra.Command {
	var model string
	var count int
	cmd := &cobra.Command{
		Use:   "forecast",
		Short: "Estimate credit cost for a planned batch against the curated model registry",
		Long: `forecast multiplies the curated per-model credit cost by your intended
run count. The number is a planning estimate — Magnific's actual billing
varies by resolution/duration and adjusts periodically. Check the live
balance on the web dashboard before submitting a large batch.`,
		Example:     "  magnific-pp-cli cost forecast --model kling-v2-6-pro --count 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			model = strings.TrimSpace(model)
			if model == "" {
				return usageErr(fmt.Errorf("--model is required (try `magnific-pp-cli models list`)"))
			}
			if count <= 0 {
				return usageErr(fmt.Errorf("--count must be a positive integer"))
			}
			m := lookupModel(model)
			if m == nil {
				return notFoundErr(fmt.Errorf("model %q not in the curated registry (run `magnific-pp-cli models list`)", model))
			}
			total := float64(count) * m.CreditCost
			out := map[string]any{
				"model":             m.Slug,
				"family":            m.Family,
				"capability":        m.Capability,
				"unit_credit_cost":  m.CreditCost,
				"count":             count,
				"estimated_credits": total,
				"caveat":            "Curated estimate. Magnific bills by resolution/duration; check the web dashboard for live balance.",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&model, "model", "", "Model slug (required)")
	cmd.Flags().IntVar(&count, "count", 1, "Number of runs to forecast")
	return cmd
}
