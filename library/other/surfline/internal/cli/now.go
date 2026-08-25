// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Joins wave+swells, wind, rating and tides on the
// shared unix timestamp into one hour-by-hour paddle/no-paddle readout for a
// single spot — a view no single Surfline endpoint returns.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/cliutil"

	"github.com/spf13/cobra"
)

type nowRow struct {
	Time        string  `json:"time"`
	Timestamp   int64   `json:"timestamp"`
	SurfMin     float64 `json:"surf_min"`
	SurfMax     float64 `json:"surf_max"`
	Surf        string  `json:"surf"`
	SwellHeight float64 `json:"swell_ft,omitempty"`
	SwellPeriod float64 `json:"swell_period_s,omitempty"`
	SwellDir    float64 `json:"swell_dir_deg,omitempty"`
	WindSpeed   float64 `json:"wind_kts"`
	WindDir     float64 `json:"wind_dir_deg"`
	WindType    string  `json:"wind_type,omitempty"`
	Rating      string  `json:"rating,omitempty"`
}

type tideView struct {
	Time   string  `json:"time"`
	Type   string  `json:"type"`
	Height float64 `json:"height"`
}

type nowView struct {
	Spot          string     `json:"spot"`
	SpotID        string     `json:"spot_id"`
	Hours         []nowRow   `json:"hours"`
	NextTides     []tideView `json:"next_tides"`
	FetchFailures []string   `json:"fetch_failures"`
}

func newNovelNowCmd(flags *rootFlags) *cobra.Command {
	var hours int
	var days int

	cmd := &cobra.Command{
		Use:   "now <spotId>",
		Short: "One spot's next few hours as a paddle/no-paddle line readout: swell, wind, tide and rating joined per hour.",
		Long: "Joins wave, swells, wind, rating and tides on the shared timestamp into a single readout for one spot.\n\n" +
			"Use this command for a single spot's next few hours as a paddle/no-paddle line readout. " +
			"Do NOT use it to compare spots; use 'rank' for that.",
		Example: strings.Trim(`
  surfline-pp-cli now 5842041f4e65fad6a7708807
  surfline-pp-cli now 5842041f4e65fad6a7708807 --hours 6 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch wave+wind+rating+tides for the spot")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a spotId argument is required (get one from `surfline search`)"))
			}
			spotID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if cliutil.IsDogfoodEnv() && days > 1 {
				days = 1
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var (
				waves    []wavePoint
				winds    []windPoint
				ratings  []ratingPoint
				tides    []tidePoint
				name     string
				waveErr  error
				failures []string
				mu       sync.Mutex
				wg       sync.WaitGroup
			)
			record := func(label string, e error) {
				if e == nil {
					return
				}
				mu.Lock()
				failures = append(failures, fmt.Sprintf("%s: %v", label, e))
				mu.Unlock()
			}
			wg.Add(5)
			go func() { defer wg.Done(); waves, waveErr = fetchWave(ctx, c, spotID, days, 1) }()
			go func() { defer wg.Done(); w, e := fetchWind(ctx, c, spotID, days, 1); record("wind", e); winds = w }()
			go func() { defer wg.Done(); r, e := fetchRating(ctx, c, spotID, days); record("rating", e); ratings = r }()
			go func() { defer wg.Done(); t, e := fetchTides(ctx, c, spotID, days); record("tides", e); tides = t }()
			go func() { defer wg.Done(); name = fetchSpotName(ctx, c, spotID) }()
			wg.Wait()

			if waveErr != nil {
				return classifyAPIError(waveErr, flags)
			}
			if len(waves) == 0 {
				return notFoundErr(fmt.Errorf("no wave forecast returned for spot %q", spotID))
			}

			windIdx := windByTimestamp(winds)
			ratingIdx := ratingByTimestamp(ratings)
			from := waves[0].Timestamp
			cutoff := from + int64(hours)*3600

			view := nowView{Spot: name, SpotID: spotID, Hours: []nowRow{}, NextTides: []tideView{}, FetchFailures: failures}
			for _, wv := range waves {
				if wv.Timestamp > cutoff {
					break
				}
				row := nowRow{
					Time:      localTime(wv.Timestamp, wv.UTCOffset, "Mon 15:04"),
					Timestamp: wv.Timestamp,
					SurfMin:   wv.Surf.Min,
					SurfMax:   wv.Surf.Max,
					Surf:      wv.Surf.HumanRelation,
				}
				if sw, ok := wv.topSwell(); ok {
					row.SwellHeight = sw.Height
					row.SwellPeriod = sw.Period
					row.SwellDir = sw.Direction
				}
				if wd, ok := windIdx[wv.Timestamp]; ok {
					row.WindSpeed = wd.Speed
					row.WindDir = wd.Direction
					row.WindType = wd.DirectionType
				}
				if rt, ok := ratingIdx[wv.Timestamp]; ok {
					row.Rating = rt.Rating.Key
				}
				view.Hours = append(view.Hours, row)
			}
			for _, t := range nextTides(tides, from, 4) {
				view.NextTides = append(view.NextTides, tideView{
					Time:   localTime(t.Timestamp, t.UTCOffset, "Mon 15:04"),
					Type:   t.Type,
					Height: t.Height,
				})
			}

			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of 3 secondary fetches failed (%s); showing wave data with gaps\n",
					len(failures), strings.Join(failures, "; "))
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", bold(name), spotID)
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "TIME\tSURF\tSWELL\tWIND\tRATING")
			for _, r := range view.Hours {
				swell := "-"
				if r.SwellHeight > 0 || r.SwellPeriod > 0 {
					swell = fmt.Sprintf("%.1fft@%.0fs %.0f°", r.SwellHeight, r.SwellPeriod, r.SwellDir)
				}
				wind := fmt.Sprintf("%.0fkt %.0f°", r.WindSpeed, r.WindDir)
				if r.WindType != "" {
					wind += " " + r.WindType
				}
				surf := r.Surf
				if surf == "" {
					surf = fmt.Sprintf("%.0f-%.0fft", r.SurfMin, r.SurfMax)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Time, surf, swell, wind, r.Rating)
			}
			_ = tw.Flush()
			if len(view.NextTides) > 0 {
				parts := make([]string, 0, len(view.NextTides))
				for _, t := range view.NextTides {
					parts = append(parts, fmt.Sprintf("%s %s %.1fft", t.Time, t.Type, t.Height))
				}
				fmt.Fprintf(out, "Next tides: %s\n", strings.Join(parts, "  "))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&hours, "hours", 12, "How many hours ahead to show")
	cmd.Flags().IntVar(&days, "days", 2, "Forecast days to fetch (bounds the hourly window)")
	return cmd
}
