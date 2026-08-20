// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/client"
	"github.com/spf13/cobra"
)

// mrrTrendPoint is one period's MRR level joined with its movement breakdown.
type mrrTrendPoint struct {
	PeriodStart string  `json:"period_start"`
	MRR         float64 `json:"mrr"`
	Movement    float64 `json:"net_movement"`
	Delta       float64 `json:"period_over_period_delta"`
}

type mrrTrendView struct {
	ProjectID  string          `json:"project_id"`
	Resolution string          `json:"resolution"`
	Currency   string          `json:"currency,omitempty"`
	Points     []mrrTrendPoint `json:"points"`
	Count      int             `json:"count"`
	Note       string          `json:"note,omitempty"`
}

func newNovelMrrTrendCmd(flags *rootFlags) *cobra.Command {
	var projectFlag string
	var period string
	var limit int
	cmd := &cobra.Command{
		Use:   "mrr-trend",
		Short: "MRR over time joined with mrr_movement, with period-over-period deltas",
		Long: `Fetches the live 'mrr' and 'mrr_movement' charts for a project and joins the
two series by period into one table: the MRR level, the net movement
(new/expansion/contraction/churn aggregated by the API), and the
period-over-period delta of the MRR level itself.

Use this command for MRR over time and its movement breakdown. Do NOT use it
for a single current-moment total and its run-over-run diff; use
'revenue-snapshot' instead.

Data source: live (chart data is served live by /charts/{chart_name}).`,
		Example: "  revenuecat-pp-cli mrr-trend --project proj1ab2c3d4 --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the mrr and mrr_movement charts and join them by period")
				return nil
			}
			projectID, err := resolveProjectID(projectFlag)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view, err := runMrrTrend(cmd.Context(), c, projectID, period, limit)
			if err != nil {
				return apiErr(err)
			}
			return emitMrrTrend(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "RevenueCat project id (or set REVENUECAT_PROJECT_ID)")
	cmd.Flags().StringVar(&period, "period", "", "Chart resolution passed through to the API (e.g. P1W, P1M); see 'charts get-options'")
	cmd.Flags().IntVar(&limit, "limit", 0, "Show only the most recent N periods (0 = all)")
	return cmd
}

func emitMrrTrend(cmd *cobra.Command, flags *rootFlags, view mrrTrendView) error {
	if len(view.Points) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Points))
		for _, p := range view.Points {
			items = append(items, map[string]any{
				"period_start": p.PeriodStart,
				"mrr":          fmt.Sprintf("%.2f", p.MRR),
				"net_movement": fmt.Sprintf("%+.2f", p.Movement),
				"delta":        fmt.Sprintf("%+.2f", p.Delta),
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nProject %s  resolution=%s  %d period(s)\n",
			view.ProjectID, view.Resolution, view.Count)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// runMrrTrend fetches mrr + mrr_movement and joins them by period start.
//
// TODO(verify): confirm chart `values` row layout (positional [timestamp_ms,
// value...]) and movement series ordering against live chart data.
func runMrrTrend(ctx context.Context, c *client.Client, projectID, period string, limit int) (mrrTrendView, error) {
	view := mrrTrendView{ProjectID: projectID, Points: []mrrTrendPoint{}}

	params := map[string]string{}
	if period != "" {
		params["resolution"] = period
	}

	mrrChart, err := fetchChart(ctx, c, projectID, "mrr", params)
	if err != nil {
		return view, fmt.Errorf("fetching mrr chart: %w", err)
	}
	moveChart, err := fetchChart(ctx, c, projectID, "mrr_movement", params)
	if err != nil {
		return view, fmt.Errorf("fetching mrr_movement chart: %w", err)
	}
	view.Resolution = mrrChart.Resolution
	view.Currency = mrrChart.YAxisCurrency
	points := joinMrrSeries(mrrChart, moveChart)
	// --limit trims to the most recent N periods (joinMrrSeries returns
	// chronological order, so the tail is the most recent).
	if limit > 0 && len(points) > limit {
		points = points[len(points)-limit:]
	}
	view.Points = points
	view.Count = len(view.Points)
	if view.Count == 0 {
		view.Note = "mrr chart returned no data points for this project"
	}
	return view, nil
}

// joinMrrSeries joins the mrr level series with the mrr_movement series by
// period start and computes the period-over-period delta of the level. Pure
// function over the two decoded charts so it is directly unit-testable.
func joinMrrSeries(mrrChart, moveChart chartData) []mrrTrendPoint {
	out := make([]mrrTrendPoint, 0)

	// Index movement by period start.
	moveByPeriod := map[string]float64{}
	for _, mp := range moveChart.points() {
		moveByPeriod[periodKey(mp.When)] = mp.firstSeriesValue()
	}

	mrrPoints := mrrChart.points()
	// Chronological order so the period-over-period delta is meaningful.
	sort.Slice(mrrPoints, func(i, j int) bool { return mrrPoints[i].When.Before(mrrPoints[j].When) })

	var prevMRR float64
	havePrev := false
	for _, mp := range mrrPoints {
		key := periodKey(mp.When)
		level := mp.firstSeriesValue()
		pt := mrrTrendPoint{
			PeriodStart: key,
			MRR:         level,
			Movement:    moveByPeriod[key],
		}
		if havePrev {
			pt.Delta = level - prevMRR
		}
		prevMRR = level
		havePrev = true
		out = append(out, pt)
	}
	return out
}

// fetchChart GETs /projects/{id}/charts/{chart_name} and decodes the response.
func fetchChart(ctx context.Context, c *client.Client, projectID, chartName string, params map[string]string) (chartData, error) {
	path := replacePathParam("/projects/{project_id}/charts/{chart_name}", "project_id", projectID)
	path = replacePathParam(path, "chart_name", chartName)
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return chartData{}, err
	}
	return parseChartData(raw), nil
}

// periodKey renders a period start as a stable YYYY-MM-DD key for joining two
// chart series. Empty time renders as "(unknown)" so degenerate rows still join
// consistently across both series.
func periodKey(t time.Time) string {
	if t.IsZero() {
		return "(unknown)"
	}
	return t.UTC().Format("2006-01-02")
}
