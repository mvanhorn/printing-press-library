package cli

import "github.com/spf13/cobra"

// pp:novel-static-reference
func newCompareCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout string
	var guests int
	cmd := &cobra.Command{
		Use:         "compare <listing-url>",
		Short:       "Compare platform fees with the cheapest direct booking option",
		Example:     "  airbnb-pp-cli compare https://www.airbnb.com/rooms/37124493 --checkin 2026-07-10 --checkout 2026-07-14 --guests 4",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			target := stripURLArg(args[0])
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"listing": dryRunCheapest(target).Listing,
					"savings": map[string]any{"absolute": nil, "percent": nil},
				}, flags)
			}
			ch, err := computeCheapest(cmd.Context(), target, cheapestParams{Checkin: checkin, Checkout: checkout, Guests: guests})
			if err != nil {
				return classifyAPIError(err)
			}
			platformTotal, platformFees := firstPlatformTotals(ch)
			directTotal := cheapestDirectTotal(ch)
			out := map[string]any{
				"listing":        ch.Listing,
				"platform_total": nullableFloat(platformTotal),
				"platform_fees":  platformFees,
				"direct_total":   nullableFloat(directTotal),
				"direct_fees":    map[string]float64{},
				"savings":        savings(platformTotal, directTotal),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Arrival date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Departure date YYYY-MM-DD")
	cmd.Flags().IntVar(&guests, "guests", 1, "Guest count")
	return cmd
}

func firstPlatformTotals(ch *cheapestOutput) (float64, any) {
	for _, opt := range ch.Options {
		m, ok := opt.(map[string]any)
		if !ok || m["source"] == "direct" {
			continue
		}
		return valueAsFloat(m["total"]), m["fees"]
	}
	return 0, map[string]float64{}
}

func cheapestDirectTotal(ch *cheapestOutput) float64 {
	for _, opt := range ch.Options {
		m, ok := opt.(map[string]any)
		if !ok || m["source"] != "direct" {
			continue
		}
		cands, _ := m["candidates"].([]directCandidate)
		var best float64
		for _, c := range cands {
			if c.Total != nil && (best == 0 || *c.Total < best) {
				best = *c.Total
			}
		}
		return best
	}
	return 0
}

func savings(platform, direct float64) map[string]any {
	out := map[string]any{"absolute": nil, "percent": nil}
	if platform > 0 && direct > 0 {
		abs := platform - direct
		out["absolute"] = abs
		out["percent"] = abs / platform * 100
	}
	return out
}
