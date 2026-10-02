package cli

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/driveplaza/internal/driveplaza"
	"github.com/spf13/cobra"
)

// pp:data-source computed
func dpHandoff(flags *rootFlags) *cobra.Command {
	var purpose string
	cmd := &cobra.Command{Use: "handoff", Short: "Print observed toll, roadwork, ETC-lane and live-map handoff URLs without fetching them", Long: "Print source destinations observed during browser discovery. This command performs no navigation or current-status request. Use the operator pages to review current construction and ETC-lane restrictions.", Example: "  driveplaza-pp-cli handoff --agent\n  driveplaza-pp-cli handoff --purpose east_construction --agent", Annotations: dpAnnotations("computed", "")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "computed", false, func(ctx context.Context, c *driveplaza.Client) (any, error) {
			items := []driveplaza.Handoff{}
			for _, h := range driveplaza.Handoffs() {
				if purpose == "" || h.Purpose == purpose {
					items = append(items, h)
				}
			}
			if purpose != "" && len(items) == 0 {
				return nil, fmt.Errorf("--purpose is unknown; run handoff without a filter for supported purposes")
			}
			return map[string]any{"meta": map[string]any{"source": "computed", "observed_on": "2026-10-02", "retrieved_at": nil, "upstream_requests": 0, "status": "handoff_only"}, "results": items}, nil
		})
	}
	cmd.Flags().StringVar(&purpose, "purpose", "", "Exact handoff purpose from the unfiltered list")
	return cmd
}

// pp:data-source computed
func dpConditions(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "conditions", Short: "Explain supported vehicle classes, JST time increments, priorities and conditional toll/ETC assumptions", Example: "  driveplaza-pp-cli conditions --agent", Annotations: dpAnnotations("computed", "")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return dpRun(cmd, args, flags, "computed", false, func(ctx context.Context, c *driveplaza.Client) (any, error) {
			return map[string]any{"meta": map[string]any{"source": "computed", "observed_on": "2026-10-02", "upstream_requests": 0, "retrieved_at": nil, "source_urls": []string{driveplaza.English + "/dp/SearchTopEN"}}, "results": map[string]any{"timezone": "Asia/Tokyo", "vehicle_classes": map[string]string{"light": "0", "standard": "1", "medium": "2", "large": "3", "extra-large": "4"}, "priorities": map[string]string{"distance": "1", "time": "2", "toll": "3"}, "time_kinds": map[string]string{"departure": "1", "arrival": "2"}, "minute_increments": []int{0, 10, 20, 30, 40, 50}, "payment_columns": []string{"standard", "etc", "etc2"}, "currency": "JPY", "distance_unit": "km", "time_unit": "minutes", "conditional_etc": "Quoted by source using requested conditions; actual eligibility, device/card, timing and driving can change final toll.", "maximum_waypoints": 5}}, nil
		})
	}
	return cmd
}
