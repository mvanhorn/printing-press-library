// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-written: flags recent days whose energy production
// falls well below this site's own trailing historical average, computed
// entirely from one /site/{siteId}/energy call (the API has no baseline
// comparison feature of its own).
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/solaredge/internal/cliutil"
	"github.com/spf13/cobra"
)

// underperformanceThreshold flags a day whose energy falls below this
// fraction of the site's own trailing baseline mean.
const underperformanceThreshold = 0.7

type underperformanceDay struct {
	Date              string  `json:"date"`
	EnergyWh          float64 `json:"energy_wh"`
	PercentOfBaseline float64 `json:"percent_of_baseline"`
}

type underperformanceView struct {
	SiteID            string                `json:"site_id"`
	Since             string                `json:"since"`
	BaselineMeanWh    float64               `json:"baseline_mean_wh"`
	BaselineDaysUsed  int                   `json:"baseline_days_used"`
	RecentDaysChecked int                   `json:"recent_days_checked"`
	Flagged           []underperformanceDay `json:"flagged"`
	Note              string                `json:"note,omitempty"`
}

func newNovelSiteUnderperformanceCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "underperformance <siteId>",
		Short: "Flag days where production fell below this site's own historical average for that time of year.",
		Long: "Use this command to flag days that are statistically low vs this site's own history. " +
			"Do NOT use it for short-term deltas; use 'site changes' instead.",
		Example:     "  solaredge-pp-cli site underperformance 1223050 --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("siteId is required"))
			}
			siteID := args[0]

			since := flagSince
			if since == "" {
				since = "30d"
			}
			sinceDur, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			recentDays := int(sinceDur.Hours() / 24)
			if recentDays < 1 {
				recentDays = 1
			}
			if recentDays > solarEdgeMaxUnderperformanceSinceDays {
				recentDays = solarEdgeMaxUnderperformanceSinceDays
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			startDate := now.AddDate(0, 0, -solarEdgeEnergyHistoryCapDays).Format("2006-01-02")
			endDate := now.Format("2006-01-02")

			energyRaw, err := c.Get(ctx, replacePathParam("/site/{siteId}/energy", "siteId", siteID), map[string]string{
				"startDate": startDate,
				"endDate":   endDate,
				"timeUnit":  "DAY",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recordSolarEdgeCalls(ctx, siteID, 1)

			view, err := buildUnderperformanceView(siteID, since, recentDays, energyRaw)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "How far back to flag underperforming days, e.g. 7d, 30d, 90d")
	return cmd
}

func buildUnderperformanceView(siteID, since string, recentDays int, energyRaw json.RawMessage) (underperformanceView, error) {
	view := underperformanceView{SiteID: siteID, Since: since, Flagged: []underperformanceDay{}}

	points, err := parseEnergySeriesValues(energyRaw)
	if err != nil {
		return view, err
	}
	if points == nil {
		view.Note = "no energy values returned for this site"
		return view, nil
	}

	if recentDays >= len(points) {
		view.Note = "not enough history to separate a baseline period from the recent window"
		return view, nil
	}

	baseline := points[:len(points)-recentDays]
	recent := points[len(points)-recentDays:]

	sum, count := sumEnergyPoints(baseline)
	if count < solarEdgeMinBaselineDays {
		view.Note = fmt.Sprintf("only %d days of baseline history available; flags may be unreliable", count)
	}
	view.BaselineDaysUsed = count
	view.RecentDaysChecked = len(recent)
	if count == 0 {
		view.Note = "no baseline history available to compare against"
		return view, nil
	}
	view.BaselineMeanWh = sum / float64(count)

	for _, p := range recent {
		if p.Value == nil {
			continue
		}
		pct := (*p.Value / view.BaselineMeanWh) * 100
		if *p.Value < view.BaselineMeanWh*underperformanceThreshold {
			view.Flagged = append(view.Flagged, underperformanceDay{
				Date:              p.Date,
				EnergyWh:          *p.Value,
				PercentOfBaseline: pct,
			})
		}
	}

	return view, nil
}
