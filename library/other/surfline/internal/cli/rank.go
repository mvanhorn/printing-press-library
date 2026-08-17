// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Fans out forecast fetches across several spots,
// scores each on a transparent blend of rating and optimalScore, and sorts them
// best-first — the multi-spot comparison Surfline only offers on the web.
//
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/cliutil"

	"github.com/spf13/cobra"
)

type rankEntry struct {
	Rank      int     `json:"rank"`
	Spot      string  `json:"spot"`
	SpotID    string  `json:"spot_id"`
	Score     float64 `json:"score"`
	Rating    string  `json:"rating"`
	SurfMin   float64 `json:"surf_min"`
	SurfMax   float64 `json:"surf_max"`
	WindKts   float64 `json:"wind_kts"`
	WindType  string  `json:"wind_type,omitempty"`
	SwellSecs float64 `json:"swell_period_s,omitempty"`
}

type rankView struct {
	Spots         []rankEntry `json:"spots"`
	Scored        int         `json:"scored"`
	FetchFailures []string    `json:"fetch_failures"`
}

// scoreSpot fetches a spot's near-term forecast and computes a composite score.
// Returns the entry, or an error tagged with the spotId on any fetch failure.
func scoreSpot(ctx context.Context, c *client.Client, spotID string, days, points int) (rankEntry, error) {
	waves, err := fetchWave(ctx, c, spotID, days, 1)
	if err != nil {
		return rankEntry{}, fmt.Errorf("%s: wave: %w", spotID, err)
	}
	if len(waves) == 0 {
		return rankEntry{}, fmt.Errorf("%s: no wave forecast", spotID)
	}
	winds, _ := fetchWind(ctx, c, spotID, days, 1)
	ratings, _ := fetchRating(ctx, c, spotID, days)
	name := fetchSpotName(ctx, c, spotID)

	windIdx := windByTimestamp(winds)
	ratingIdx := ratingByTimestamp(ratings)

	var (
		sumOptimal float64
		sumRating  float64
		n          int
	)
	entry := rankEntry{Spot: name, SpotID: spotID}
	for i, wv := range waves {
		if i >= points {
			break
		}
		opt := float64(wv.Surf.OptimalScore)
		if sw, ok := wv.topSwell(); ok {
			opt += float64(sw.OptimalScore)
			if i == 0 {
				entry.SwellSecs = sw.Period
			}
		}
		if wd, ok := windIdx[wv.Timestamp]; ok {
			opt += float64(wd.OptimalScore)
			if i == 0 {
				entry.WindKts = wd.Speed
				entry.WindType = wd.DirectionType
			}
		}
		if rt, ok := ratingIdx[wv.Timestamp]; ok {
			sumRating += ratingValue(rt)
			if i == 0 {
				entry.Rating = rt.Rating.Key
			}
		}
		if i == 0 {
			entry.SurfMin = wv.Surf.Min
			entry.SurfMax = wv.Surf.Max
		}
		sumOptimal += opt
		n++
	}
	if n == 0 {
		return rankEntry{}, fmt.Errorf("%s: no scorable forecast points", spotID)
	}
	// Composite: rating weighted x2, plus mean optimalScore across surf/swell/wind.
	entry.Score = (sumRating/float64(n))*2 + sumOptimal/float64(n)
	if entry.Rating == "" {
		entry.Rating = "-"
	}
	return entry, nil
}

func newNovelRankCmd(flags *rootFlags) *cobra.Command {
	var days int
	var points int

	cmd := &cobra.Command{
		Use:   "rank <spotId> [spotId...]",
		Short: "Score and sort several spots best-first on a transparent sum of wave, wind and swell optimalScore.",
		Long: "Fetches each spot's near-term forecast and ranks them best-first.\n\n" +
			"Use this command to compare a set of spots and pick today's best. " +
			"For one spot's hour-by-hour detail use 'now'.",
		Example: strings.Trim(`
  surfline-pp-cli rank 5842041f4e65fad6a7708807 5842041f4e65fad6a7708cfd
  surfline-pp-cli rank 5842041f4e65fad6a7708807 5842041f4e65fad6a7708cfd --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and rank the given spots")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one spotId argument is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if cliutil.IsDogfoodEnv() {
				days = 1
				if points > 4 {
					points = 4
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type res struct {
				entry rankEntry
				err   error
			}
			results := make([]res, len(args))
			var wg sync.WaitGroup
			for i, spotID := range args {
				wg.Add(1)
				go func(i int, spotID string) {
					defer wg.Done()
					e, err := scoreSpot(ctx, c, spotID, days, points)
					results[i] = res{entry: e, err: err}
				}(i, spotID)
			}
			wg.Wait()

			view := rankView{Spots: []rankEntry{}, FetchFailures: []string{}}
			for _, r := range results {
				if r.err != nil {
					view.FetchFailures = append(view.FetchFailures, r.err.Error())
					continue
				}
				view.Spots = append(view.Spots, r.entry)
			}
			sort.SliceStable(view.Spots, func(i, j int) bool { return view.Spots[i].Score > view.Spots[j].Score })
			for i := range view.Spots {
				view.Spots[i].Rank = i + 1
			}
			view.Scored = len(view.Spots)

			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d spots failed to fetch; ranked over the remaining %d\n",
					len(view.FetchFailures), len(args), view.Scored)
			}
			if view.Scored == 0 {
				return apiErr(fmt.Errorf("no spots could be ranked: %s", strings.Join(view.FetchFailures, "; ")))
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "#\tSPOT\tSCORE\tSURF\tRATING\tWIND")
			for _, e := range view.Spots {
				wind := fmt.Sprintf("%.0fkt", e.WindKts)
				if e.WindType != "" {
					wind += " " + e.WindType
				}
				fmt.Fprintf(tw, "%d\t%s\t%.1f\t%.0f-%.0fft\t%s\t%s\n",
					e.Rank, truncate(e.Spot, 28), e.Score, e.SurfMin, e.SurfMax, e.Rating, wind)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&days, "days", 1, "Forecast days to consider")
	cmd.Flags().IntVar(&points, "points", 8, "Forecast points per spot to average into the score")
	return cmd
}
