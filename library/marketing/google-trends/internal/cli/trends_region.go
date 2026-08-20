// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/gtrends"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-trends/internal/store"
)

// patchWidgetResolution best-effort patches a GEO_MAP widget's own request
// object with a caller-requested geo resolution. The live "resolution"
// mechanism is not independently verified against this exact endpoint, so
// this only overrides the key when the widget's request already carries a
// "resolution" field (which explore() has been observed to populate with a
// default like "COUNTRY") — if the field is absent, the request is passed
// through unchanged rather than guessing at an unsupported shape.
func patchWidgetResolution(request json.RawMessage, resolution string) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(request, &obj); err != nil {
		return request
	}
	if _, ok := obj["resolution"]; !ok {
		return request
	}
	obj["resolution"] = resolution
	patched, err := json.Marshal(obj)
	if err != nil {
		return request
	}
	return patched
}

// pp:data-source live
func newTrendsRegionCmd(flags *rootFlags) *cobra.Command {
	var flagResolution string
	var flagGeo string
	var flagTimeframe string
	var flagCategory int

	cmd := &cobra.Command{
		Use:         "region <keyword>",
		Short:       "Interest-by-region for a keyword.",
		Example:     "  google-trends-pp-cli trends region coffee --resolution REGION --geo US --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("keyword argument is required"))
			}
			keyword := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			explore, err := gtrends.Explore(ctx, c, []string{keyword}, []string{flagGeo}, flagTimeframe, flagCategory, "")
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

			// Google's comparedgeo response always covers every geo in scope,
			// even for a keyword with no real search volume — remaining rows are
			// relative-scaled noise (e.g. a nonsense keyword can still "peak" at
			// 100 in some region purely from scaling artifacts, which would be
			// misleading to present as a real regional signal). The TIMESERIES
			// widget's interest-over-time is empty specifically when a keyword
			// has no real data, so it's the reliable signal for "no data" here,
			// not the region values themselves.
			if tsWidget, ok := gtrends.FindWidget(explore.Widgets, "TIMESERIES"); ok {
				points, err := gtrends.InterestOverTime(ctx, c, tsWidget)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if len(points) == 0 {
					return notFoundErr(fmt.Errorf("no interest data for %q; nothing to show by region", keyword))
				}
			}

			regions, err := gtrends.InterestByRegion(ctx, c, widget)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			syncedAt := time.Now().UTC().Format(time.RFC3339)
			out := make([]gtRegionInterestRecord, 0, len(regions))

			db, dbErr := store.OpenWithContext(ctx, defaultDBPath("google-trends-pp-cli"))
			if dbErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open local store to cache region data: %v\n", dbErr)
			} else {
				defer db.Close()
			}

			for _, r := range regions {
				value := 0
				if len(r.Values) > 0 {
					value = r.Values[0]
				}
				row := gtRegionInterestRecord{Keyword: keyword, GeoCode: r.GeoCode, GeoName: r.GeoName, Timeframe: flagTimeframe, Value: value, SyncedAt: syncedAt}
				out = append(out, row)
				if db != nil {
					body, _ := json.Marshal(row)
					if err := db.Upsert("gt_region_interest", sha256ID(keyword, r.GeoCode, flagTimeframe), body); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to cache region interest for %q: %v\n", r.GeoCode, err)
					}
				}
			}

			return printLiveResult(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&flagResolution, "resolution", "", "Geo resolution: COUNTRY, REGION, DMA, or CITY (best-effort)")
	cmd.Flags().StringVar(&flagGeo, "geo", "", "Geo code to scope the comparison (e.g. US); default: worldwide")
	cmd.Flags().StringVar(&flagTimeframe, "timeframe", "today 12-m", "Time range, e.g. 'today 12-m'")
	cmd.Flags().IntVar(&flagCategory, "category", 0, "Google Trends category ID")
	return cmd
}
