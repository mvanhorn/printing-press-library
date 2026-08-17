// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/gtrends"
)

type geoGapRow struct {
	GeoCode       string `json:"geo_code"`
	GeoName       string `json:"geo_name"`
	KeywordAValue int    `json:"keyword_a_value"`
	KeywordBValue int    `json:"keyword_b_value"`
	Delta         int    `json:"delta"`
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// pp:data-source live
func newNovelTrendsGeoGapCmd(flags *rootFlags) *cobra.Command {
	var flagResolution string
	var flagLimit int
	var flagGeo string
	var flagTimeframe string
	var flagCategory int

	cmd := &cobra.Command{
		Use:         "geo-gap <keywordA> <keywordB>",
		Short:       "Ranks the regions where two keywords' interest diverges most",
		Example:     "  google-trends-pp-cli trends geo-gap nike adidas --resolution REGION --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("two keyword arguments are required: <keywordA> <keywordB>"))
			}
			keywordA, keywordB := args[0], args[1]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			explore, err := gtrends.Explore(ctx, c, []string{keywordA, keywordB}, []string{flagGeo}, flagTimeframe, flagCategory, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			widget, ok := gtrends.FindWidget(explore.Widgets, "GEO_MAP")
			if !ok {
				return apiErr(fmt.Errorf("explore response did not include a GEO_MAP widget"))
			}
			if flagResolution != "" {
				widget.Request = patchWidgetResolution(widget.Request, flagResolution)
			}

			regions, err := gtrends.InterestByRegion(ctx, c, widget)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			out := make([]geoGapRow, 0, len(regions))
			for _, r := range regions {
				a, b := 0, 0
				if len(r.Values) > 0 {
					a = r.Values[0]
				}
				if len(r.Values) > 1 {
					b = r.Values[1]
				}
				out = append(out, geoGapRow{GeoCode: r.GeoCode, GeoName: r.GeoName, KeywordAValue: a, KeywordBValue: b, Delta: a - b})
			}
			sort.Slice(out, func(i, j int) bool { return absInt(out[i].Delta) > absInt(out[j].Delta) })
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}

			return printLiveResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagResolution, "resolution", "", "Geo resolution: COUNTRY, REGION, DMA, or CITY (best-effort — patches the widget's own request if present)")
	cmd.Flags().IntVar(&flagLimit, "limit", 15, "Maximum number of regions to return")
	cmd.Flags().StringVar(&flagGeo, "geo", "", "Restrict comparison to this geo (default: worldwide)")
	cmd.Flags().StringVar(&flagTimeframe, "timeframe", "today 12-m", "Time range, e.g. 'today 12-m'")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Google Trends category ID")
	return cmd
}
