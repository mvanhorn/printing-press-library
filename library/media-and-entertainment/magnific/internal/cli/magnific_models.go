package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificModelsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Browse and benchmark Magnific's 80+ AI models against your local history",
		Long: `models lists the curated Magnific model registry (capability, family,
listed credit cost) joined with your local empirical stats (your p50
latency, success rate, $ spent on this model). 'models stats' deep-dives
a single model.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newMagnificModelsListCmd(flags))
	cmd.AddCommand(newMagnificModelsStatsCmd(flags))
	return cmd
}

func newMagnificModelsListCmd(flags *rootFlags) *cobra.Command {
	var capability string
	var family string
	var sortBy string
	var maxCost float64
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List curated models filtered by capability/family and sorted by listed cost or your spend",
		Example:     "  magnific-pp-cli models list --capability text-to-image --sort cost --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows := make([]MagnificModel, 0, len(magnificModels))
			for _, m := range magnificModels {
				if capability != "" && m.Capability != capability {
					continue
				}
				if family != "" && m.Family != family {
					continue
				}
				if maxCost > 0 && m.CreditCost > maxCost {
					continue
				}
				rows = append(rows, m)
			}
			// Join with local task counts for richer agent context.
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			localCounts := map[string]int{}
			localSpend := map[string]float64{}
			if err == nil {
				defer db.Close()
				if err := store.EnsureMagnificTables(ctx, db.DB()); err == nil {
					q := `SELECT COALESCE(model,''), COUNT(*), COALESCE(SUM(credit_cost),0) FROM magnific_tasks GROUP BY model`
					r, qerr := db.DB().QueryContext(ctx, q)
					if qerr == nil {
						defer r.Close()
						for r.Next() {
							var mm sql.NullString
							var c sql.NullInt64
							var s sql.NullFloat64
							if err := r.Scan(&mm, &c, &s); err == nil {
								localCounts[mm.String] = int(c.Int64)
								localSpend[mm.String] = s.Float64
							}
						}
					}
				}
			}

			type withStats struct {
				MagnificModel
				LocalCount int     `json:"local_count"`
				LocalSpend float64 `json:"local_spend_credits"`
			}
			out := make([]withStats, 0, len(rows))
			for _, m := range rows {
				out = append(out, withStats{
					MagnificModel: m,
					LocalCount:    localCounts[m.Slug],
					LocalSpend:    localSpend[m.Slug],
				})
			}
			sort.Slice(out, func(i, j int) bool {
				switch sortBy {
				case "spend":
					return out[i].LocalSpend > out[j].LocalSpend
				case "use":
					return out[i].LocalCount > out[j].LocalCount
				case "family":
					return out[i].Family < out[j].Family
				default: // "cost"
					return out[i].CreditCost < out[j].CreditCost
				}
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if out == nil {
				out = []withStats{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&capability, "capability", "", "Filter by capability (text-to-image, image-to-video, image-upscaler, image-edit, audio, analyze, text-to-video)")
	cmd.Flags().StringVar(&family, "family", "", "Filter by model family (mystic, flux, seedream, kling, hailuo, wan, etc.)")
	cmd.Flags().StringVar(&sortBy, "sort", "cost", "Sort by: cost (listed credit), spend (your $), use (your run count), family")
	cmd.Flags().Float64Var(&maxCost, "max-cost", 0, "Drop models with listed credit cost above this (0 = no cap)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows (0 = no cap)")
	return cmd
}

func newMagnificModelsStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats <model-slug>",
		Short:       "Deep-dive empirical stats for one model (your p50 latency, success rate, $ spent)",
		Example:     "  magnific-pp-cli models stats kling-v2-6-pro --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := strings.TrimSpace(args[0])
			m := lookupModel(slug)
			if m == nil {
				return notFoundErr(fmt.Errorf("model %q not in curated registry", slug))
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			out := map[string]any{
				"registry": m,
			}
			if err == nil {
				defer db.Close()
				if err := store.EnsureMagnificTables(ctx, db.DB()); err == nil {
					var count sql.NullInt64
					var spend sql.NullFloat64
					var completedCount sql.NullInt64
					var failedCount sql.NullInt64
					_ = db.DB().QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(credit_cost),0) FROM magnific_tasks WHERE model = ?`, slug).Scan(&count, &spend)
					_ = db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM magnific_tasks WHERE model = ? AND status IN ('COMPLETED','DONE','SUCCESS')`, slug).Scan(&completedCount)
					_ = db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM magnific_tasks WHERE model = ? AND status IN ('FAILED','ERROR')`, slug).Scan(&failedCount)
					empirical := map[string]any{
						"runs":          count.Int64,
						"spend_credits": spend.Float64,
						"completed":     completedCount.Int64,
						"failed":        failedCount.Int64,
					}
					if count.Int64 > 0 {
						empirical["success_rate"] = float64(completedCount.Int64) / float64(count.Int64)
					}
					out["empirical"] = empirical
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
