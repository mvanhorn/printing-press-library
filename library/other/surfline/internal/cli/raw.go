// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Emits the underlying numeric forecast fields
// (surf min/max/optimalScore/humanRelation, swell components, wind
// directionType/gust) as a pipe-friendly table or JSON, with no rating
// editorializing — for people who trust the numbers over the star.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/cliutil"

	"github.com/spf13/cobra"
)

type rawSwell struct {
	Height       float64 `json:"height"`
	Period       float64 `json:"period"`
	Direction    float64 `json:"direction"`
	OptimalScore int     `json:"optimal_score"`
}

type rawRow struct {
	Time          string     `json:"time"`
	Timestamp     int64      `json:"timestamp"`
	SurfMin       float64    `json:"surf_min"`
	SurfMax       float64    `json:"surf_max"`
	SurfOptimal   int        `json:"surf_optimal_score"`
	HumanRelation string     `json:"human_relation,omitempty"`
	Swells        []rawSwell `json:"swells"`
	WindSpeed     float64    `json:"wind_speed"`
	WindGust      float64    `json:"wind_gust"`
	WindDir       float64    `json:"wind_direction"`
	WindType      string     `json:"wind_direction_type,omitempty"`
	WindOptimal   int        `json:"wind_optimal_score"`
}

type rawView struct {
	Spot   string   `json:"spot"`
	SpotID string   `json:"spot_id"`
	Points []rawRow `json:"points"`
}

func newNovelRawCmd(flags *rootFlags) *cobra.Command {
	var days int
	var interval int

	cmd := &cobra.Command{
		Use:   "raw <spotId>",
		Short: "Pipe-friendly table/JSON of min/max/optimalScore/humanRelation plus swell components and wind directionType/gust.",
		Long: "Dumps the underlying numeric forecast fields with no rating editorializing.\n\n" +
			"Use this command when you want unfiltered numeric fields to pipe elsewhere. " +
			"For a ranked/judged view use 'rank' or the rating in 'now'.",
		Example: strings.Trim(`
  surfline-pp-cli raw 5842041f4e65fad6a7708807
  surfline-pp-cli raw 5842041f4e65fad6a7708807 --agent --select points.swells.period`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch raw wave+wind fields for the spot")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a spotId argument is required"))
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

			waves, err := fetchWave(ctx, c, spotID, days, interval)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(waves) == 0 {
				return notFoundErr(fmt.Errorf("no wave forecast returned for spot %q", spotID))
			}
			winds, _ := fetchWind(ctx, c, spotID, days, interval)
			name := fetchSpotName(ctx, c, spotID)
			windIdx := windByTimestamp(winds)

			view := rawView{Spot: name, SpotID: spotID, Points: []rawRow{}}
			for _, wv := range waves {
				row := rawRow{
					Time:          localTime(wv.Timestamp, wv.UTCOffset, "2006-01-02 15:04"),
					Timestamp:     wv.Timestamp,
					SurfMin:       wv.Surf.Min,
					SurfMax:       wv.Surf.Max,
					SurfOptimal:   wv.Surf.OptimalScore,
					HumanRelation: wv.Surf.HumanRelation,
					Swells:        []rawSwell{},
				}
				for _, s := range wv.Swells {
					if s.Height == 0 && s.Period == 0 {
						continue
					}
					row.Swells = append(row.Swells, rawSwell{
						Height:       s.Height,
						Period:       s.Period,
						Direction:    s.Direction,
						OptimalScore: s.OptimalScore,
					})
				}
				if wd, ok := windIdx[wv.Timestamp]; ok {
					row.WindSpeed = wd.Speed
					row.WindGust = wd.Gust
					row.WindDir = wd.Direction
					row.WindType = wd.DirectionType
					row.WindOptimal = wd.OptimalScore
				}
				view.Points = append(view.Points, row)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  (%s)\n", bold(name), spotID)
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "TIME\tSURF_MIN\tSURF_MAX\tS_OPT\tSWELL_HT\tSWELL_PER\tSWELL_DIR\tWIND\tGUST\tW_TYPE\tW_OPT")
			for _, r := range view.Points {
				var sh, sp, sd float64
				if len(r.Swells) > 0 {
					sh, sp, sd = r.Swells[0].Height, r.Swells[0].Period, r.Swells[0].Direction
				}
				fmt.Fprintf(tw, "%s\t%.1f\t%.1f\t%d\t%.1f\t%.0f\t%.0f\t%.0f\t%.0f\t%s\t%d\n",
					r.Time, r.SurfMin, r.SurfMax, r.SurfOptimal, sh, sp, sd, r.WindSpeed, r.WindGust, r.WindType, r.WindOptimal)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&days, "days", 1, "Forecast horizon in days")
	cmd.Flags().IntVar(&interval, "interval", 0, "Hours between forecast points")
	return cmd
}
