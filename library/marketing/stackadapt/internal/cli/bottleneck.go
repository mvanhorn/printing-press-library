// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type bottleneckRow struct {
	Campaign          string  `json:"campaign"`
	Name              string  `json:"name"`
	Spend             float64 `json:"spend"`
	Roas              float64 `json:"roas"`
	ConversionRevenue float64 `json:"conversion_revenue"`
	Conversions       float64 `json:"conversions"`
	Ecpa              float64 `json:"ecpa"`
	Reason            string  `json:"reason"`
}

func bottleneckReason(r bottleneckRow) string {
	switch {
	case r.Conversions == 0:
		return "spend with zero conversions"
	case r.Roas >= 0 && r.Roas < 1:
		return "returning less revenue than it spends (ROAS < 1)"
	case r.Roas >= 0 && r.Roas < 2:
		return "low ROAS for the spend level"
	default:
		return "ROAS >= 2; healthy return, listed for spend context only"
	}
}

func newNovelBottleneckCmd(flags *rootFlags) *cobra.Command {
	var advertiser string
	var days, limit int

	cmd := &cobra.Command{
		Use:   "bottleneck",
		Short: "Rank the highest-spend campaigns by worst ROAS, with a reason",
		Long: "Pull campaign delivery over a window and rank the spending campaigns by worst return on ad spend " +
			"(conversionRevenue / cost), so you can see where budget is being wasted. Scope with --advertiser. Read-only.",
		Example:     "  stackadapt-pp-cli bottleneck --advertiser 123 --days 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "bottleneck", "would rank campaigns by worst ROAS")
			}
			filter := map[string]any{"archived": false}
			if advertiser != "" {
				filter["advertiserIds"] = []string{advertiser}
			}
			from, to := dateRange(days)
			recs, err := fetchCampaignDelivery(cmd.Context(), flags, filter, "TOTAL", from, to)
			if err != nil {
				return err
			}
			rows := make([]bottleneckRow, 0, len(recs))
			for _, r := range recs {
				if r.Metrics["cost"] <= 0 {
					continue // only rank campaigns that actually spent
				}
				row := bottleneckRow{
					Campaign:          r.CampaignID,
					Name:              r.CampaignName,
					Spend:             r.Metrics["cost"],
					Roas:              roas(r.Metrics),
					ConversionRevenue: r.Metrics["conversionRevenue"],
					Conversions:       r.Metrics["conversions"],
					Ecpa:              r.Metrics["ecpa"],
				}
				row.Reason = bottleneckReason(row)
				rows = append(rows, row)
			}
			// Worst ROAS first (zero-conversion campaigns have roas -1 → surface first).
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Roas < rows[j].Roas })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return emitView(cmd, flags, map[string]any{
				"window":      fmt.Sprintf("%s..%s", from, to),
				"advertiser":  advertiser,
				"count":       len(rows),
				"bottlenecks": rows,
			})
		},
	}
	cmd.Flags().StringVar(&advertiser, "advertiser", "", "Limit to one advertiser ID")
	cmd.Flags().IntVar(&days, "days", 30, "Days of delivery history to evaluate")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max campaigns to return")
	return cmd
}
