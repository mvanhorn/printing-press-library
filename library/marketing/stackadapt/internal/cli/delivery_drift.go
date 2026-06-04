// Hand-authored novel command (no generated header): preserved across regen.
package cli

import (
	"fmt"
	"math"
	"sort"

	"github.com/spf13/cobra"
)

type driftRow struct {
	Campaign      string   `json:"campaign"`
	Name          string   `json:"name"`
	SpendCurrent  float64  `json:"spend_current"`
	SpendPrior    float64  `json:"spend_prior"`
	SpendDeltaPct *float64 `json:"spend_delta_pct"`
	CtrCurrent    float64  `json:"ctr_current"`
	CtrPrior      float64  `json:"ctr_prior"`
	CtrDeltaPct   *float64 `json:"ctr_delta_pct"`
	Flag          string   `json:"flag,omitempty"`
}

// pctDelta returns the percent change from prior to cur, or nil when prior is
// zero (a new or reactivated campaign with no prior-window baseline). A nil
// result renders as JSON null, distinguishing "no prior data" from a genuine
// 0% change so new launches aren't silently flattened.
func pctDelta(cur, prior float64) *float64 {
	if prior == 0 {
		return nil
	}
	v := math.Round((cur-prior)/prior*1000) / 10
	return &v
}

// ctrDriftKey is the sort key for delivery drift: the CTR delta, or +Inf when
// there's no prior baseline so those rows sort after the real drops.
func ctrDriftKey(r driftRow) float64 {
	if r.CtrDeltaPct == nil {
		return math.Inf(1)
	}
	return *r.CtrDeltaPct
}

func newNovelDeliveryDriftCmd(flags *rootFlags) *cobra.Command {
	var campaign, advertiser string
	var days int

	cmd := &cobra.Command{
		Use:   "delivery-drift",
		Short: "Compare CTR and spend for the last N days vs the prior N days",
		Long: "Pull delivery for the current window and the immediately preceding window of the same length, then " +
			"report per-campaign spend and CTR drift. Flags campaigns whose CTR dropped sharply. Scope with --campaign " +
			"or --advertiser. Read-only.",
		Example:     "  stackadapt-pp-cli delivery-drift --advertiser 123 --days 7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "delivery-drift", "would compare current vs prior delivery window")
			}
			filter := map[string]any{"archived": false}
			if campaign != "" {
				filter["ids"] = []string{campaign}
			}
			if advertiser != "" {
				filter["advertiserIds"] = []string{advertiser}
			}
			cf, ct, pf, pt := priorWindow(days)
			cur, err := fetchCampaignDelivery(cmd.Context(), flags, filter, "TOTAL", cf, ct)
			if err != nil {
				return err
			}
			prior, err := fetchCampaignDelivery(cmd.Context(), flags, filter, "TOTAL", pf, pt)
			if err != nil {
				return err
			}
			priorBy := make(map[string]deliveryRecord, len(prior))
			for _, p := range prior {
				priorBy[p.CampaignID] = p
			}

			rows := make([]driftRow, 0, len(cur))
			for _, c := range cur {
				if c.Metrics["cost"] <= 0 {
					continue
				}
				p := priorBy[c.CampaignID]
				row := driftRow{
					Campaign:      c.CampaignID,
					Name:          c.CampaignName,
					SpendCurrent:  c.Metrics["cost"],
					SpendPrior:    p.Metrics["cost"],
					SpendDeltaPct: pctDelta(c.Metrics["cost"], p.Metrics["cost"]),
					CtrCurrent:    c.Metrics["ctr"],
					CtrPrior:      p.Metrics["ctr"],
					CtrDeltaPct:   pctDelta(c.Metrics["ctr"], p.Metrics["ctr"]),
				}
				if row.CtrPrior > 0 && row.CtrDeltaPct != nil && *row.CtrDeltaPct <= -20 {
					row.Flag = "CTR down >20% vs prior window"
				}
				rows = append(rows, row)
			}
			// Largest CTR drop first; rows with no prior baseline (nil) sort last.
			sort.SliceStable(rows, func(i, j int) bool { return ctrDriftKey(rows[i]) < ctrDriftKey(rows[j]) })

			return emitView(cmd, flags, map[string]any{
				"current_window": fmt.Sprintf("%s..%s", cf, ct),
				"prior_window":   fmt.Sprintf("%s..%s", pf, pt),
				"count":          len(rows),
				"drift":          rows,
			})
		},
	}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Limit to one campaign ID")
	cmd.Flags().StringVar(&advertiser, "advertiser", "", "Limit to one advertiser ID")
	cmd.Flags().IntVar(&days, "days", 7, "Window length in days (compared against the prior window of the same length)")
	return cmd
}
