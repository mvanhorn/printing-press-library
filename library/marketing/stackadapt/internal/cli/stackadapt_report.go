// Hand-authored absorbed reporting command (no generated header): preserved
// across regen. Exposes raw campaign delivery metrics over a window.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "report", Short: "StackAdapt delivery reporting", Annotations: map[string]string{"mcp:read-only": "true"}}
	cmd.RunE = parentNoSubcommandRunE(flags)

	var advertiser, campaign string
	var days int
	cd := &cobra.Command{
		Use:   "campaign-delivery",
		Short: "Per-campaign delivery metrics (spend, CTR, conversions, ROAS inputs) over a window",
		Long: "Pull per-campaign delivery metrics from StackAdapt over the last N days: cost, CTR, conversions, " +
			"conversion revenue, and more. Scope with --advertiser or --campaign. Read-only.",
		Example:     "  stackadapt-pp-cli report campaign-delivery --days 30 --agent --select records.campaign_name,records.metrics.cost",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "report campaign-delivery", "would pull campaign delivery metrics")
			}
			filter := map[string]any{"archived": false}
			if advertiser != "" {
				filter["advertiserIds"] = []string{advertiser}
			}
			if campaign != "" {
				filter["ids"] = []string{campaign}
			}
			from, to := dateRange(days)
			recs, err := fetchCampaignDelivery(cmd.Context(), flags, filter, "TOTAL", from, to)
			if err != nil {
				return err
			}
			return emitView(cmd, flags, map[string]any{
				"window":  fmt.Sprintf("%s..%s", from, to),
				"count":   len(recs),
				"records": recs,
			})
		},
	}
	cd.Flags().StringVar(&advertiser, "advertiser", "", "Limit to one advertiser ID")
	cd.Flags().StringVar(&campaign, "campaign", "", "Limit to one campaign ID")
	cd.Flags().IntVar(&days, "days", 30, "Days of delivery history")
	cmd.AddCommand(cd)
	return cmd
}
