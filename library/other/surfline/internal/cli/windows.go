// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Scans the forecast horizon and emits only the
// contiguous daylight blocks where surf, top-swell and wind optimalScore all
// clear the bar — "when exactly is it good", which no single endpoint answers.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/cliutil"

	"github.com/spf13/cobra"
)

type windowBlock struct {
	Start    string  `json:"start"`
	End      string  `json:"end"`
	StartTS  int64   `json:"start_ts"`
	EndTS    int64   `json:"end_ts"`
	Hours    float64 `json:"hours"`
	SurfMin  float64 `json:"surf_min"`
	SurfMax  float64 `json:"surf_max"`
	WindType string  `json:"wind_type,omitempty"`
}

type windowsView struct {
	Spot      string        `json:"spot"`
	SpotID    string        `json:"spot_id"`
	Threshold int           `json:"min_optimal_score"`
	Windows   []windowBlock `json:"windows"`
	Note      string        `json:"note,omitempty"`
}

func newNovelWindowsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var primo bool

	cmd := &cobra.Command{
		Use:   "windows <spotId>",
		Short: "Emit only the contiguous time blocks where wave, wind and swell optimalScore are all good, daylight-only.",
		Long: "Walks the forecast horizon and returns the contiguous daylight blocks worth surfing.\n\n" +
			"Use this command to find the good time blocks at one spot today or this week. " +
			"It already drops after-dark windows, so there is no separate daylight command.",
		Example: strings.Trim(`
  surfline-pp-cli windows 5842041f4e65fad6a7708807 --days 3
  surfline-pp-cli windows 5842041f4e65fad6a7708807 --primo --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan the forecast for good daylight windows")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a spotId argument is required"))
			}
			spotID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if cliutil.IsDogfoodEnv() && days > 2 {
				days = 2
			}
			threshold := 1
			if primo {
				threshold = 2
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			waves, err := fetchWave(ctx, c, spotID, days, 1)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(waves) == 0 {
				return notFoundErr(fmt.Errorf("no wave forecast returned for spot %q", spotID))
			}
			winds, _ := fetchWind(ctx, c, spotID, days, 1)
			sun, _ := fetchSunlight(ctx, c, spotID, days)
			name := fetchSpotName(ctx, c, spotID)
			windIdx := windByTimestamp(winds)

			view := windowsView{Spot: name, SpotID: spotID, Threshold: threshold, Windows: []windowBlock{}}
			// Each forecast point represents one interval of conditions, so a
			// window's duration is the span between its first and last good
			// point PLUS one interval (the last point's own coverage).
			stepSecs := int64(3600)
			if len(waves) >= 2 {
				if d := waves[1].Timestamp - waves[0].Timestamp; d > 0 {
					stepSecs = d
				}
			}
			var cur *windowBlock
			flush := func() {
				if cur != nil {
					cur.Hours = float64(cur.EndTS-cur.StartTS+stepSecs) / 3600.0
					view.Windows = append(view.Windows, *cur)
					cur = nil
				}
			}
			for _, wv := range waves {
				good := wv.Surf.OptimalScore >= threshold && isDaylight(wv.Timestamp, sun)
				if sw, ok := wv.topSwell(); ok {
					good = good && sw.OptimalScore >= threshold
				}
				wd, hasWind := windIdx[wv.Timestamp]
				if hasWind {
					good = good && wd.OptimalScore >= threshold
				}
				if !good {
					flush()
					continue
				}
				if cur == nil {
					cur = &windowBlock{
						Start:   localTime(wv.Timestamp, wv.UTCOffset, "Mon 15:04"),
						StartTS: wv.Timestamp,
						SurfMin: wv.Surf.Min,
						SurfMax: wv.Surf.Max,
					}
					if hasWind {
						cur.WindType = wd.DirectionType
					}
				}
				cur.End = localTime(wv.Timestamp, wv.UTCOffset, "Mon 15:04")
				cur.EndTS = wv.Timestamp
				if wv.Surf.Min < cur.SurfMin {
					cur.SurfMin = wv.Surf.Min
				}
				if wv.Surf.Max > cur.SurfMax {
					cur.SurfMax = wv.Surf.Max
				}
			}
			flush()

			if len(view.Windows) == 0 {
				label := "good"
				if primo {
					label = "primo"
				}
				view.Note = fmt.Sprintf("no %s daylight windows in the next %d day(s); try --days to widen or drop --primo", label, days)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", bold(name), spotID)
			if view.Note != "" {
				fmt.Fprintln(out, view.Note)
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "WINDOW\tHOURS\tSURF\tWIND")
			for _, w := range view.Windows {
				wind := w.WindType
				if wind == "" {
					wind = "-"
				}
				fmt.Fprintf(tw, "%s → %s\t%.0f\t%.0f-%.0fft\t%s\n", w.Start, w.End, w.Hours, w.SurfMin, w.SurfMax, wind)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&days, "days", 3, "Forecast horizon in days to scan")
	cmd.Flags().BoolVar(&primo, "primo", false, "Only blocks where every optimalScore is at the top (2)")
	return cmd
}
