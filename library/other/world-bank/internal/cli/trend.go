// Hand-authored novel feature. Body is hand-written; survives regen via regen-merge.
// pp:data-source live
package cli

import (
	"fmt"
	"math"
	"sort"

	"github.com/spf13/cobra"
)

type wbTrendView struct {
	Country     string   `json:"country"`
	CountryCode string   `json:"country_code"`
	Indicator   string   `json:"indicator"`
	Points      int      `json:"points"`
	FirstDate   string   `json:"first_date"`
	FirstValue  *float64 `json:"first_value"`
	LatestDate  string   `json:"latest_date"`
	LatestValue *float64 `json:"latest_value"`
	MinValue    *float64 `json:"min_value"`
	MaxValue    *float64 `json:"max_value"`
	YoYPct      *float64 `json:"yoy_pct,omitempty"`
	CAGRPct     *float64 `json:"cagr_pct,omitempty"`
	TotalPctChg *float64 `json:"total_pct_change,omitempty"`
}

func newNovelTrendCmd(flags *rootFlags) *cobra.Command {
	var flagWindow int

	cmd := &cobra.Command{
		Use:         "trend <country> <indicator>",
		Short:       "CAGR, YoY change, and min/max/latest for one country+indicator series.",
		Long:        "Summarize one country's trajectory on one indicator: CAGR, year-over-year change, min/max, first/latest.\nUse this for a single series. Do NOT use it to compare countries; use 'compare'.",
		Example:     "  world-bank-pp-cli trend USA NY.GDP.MKTP.CD --window 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("country and indicator are required"))
			}
			country, indicator := args[0], args[1]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			extra := map[string]string{}
			if flagWindow > 0 {
				extra["mrv"] = fmt.Sprintf("%d", flagWindow)
			}
			obs, err := wbFetchObservations(ctx, c, country, indicator, extra, 10)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Keep non-null, sort by date ascending.
			pts := make([]wbObservation, 0, len(obs))
			for _, o := range obs {
				if o.Value != nil {
					pts = append(pts, o)
				}
			}
			if len(pts) == 0 {
				return usageErr(fmt.Errorf("no observations for %s / %s", country, indicator))
			}
			sort.Slice(pts, func(i, j int) bool { return pts[i].Date < pts[j].Date })

			first, latest := pts[0], pts[len(pts)-1]
			view := wbTrendView{
				Country: latest.Country.Value, CountryCode: latest.CountryISO3Code,
				Indicator: indicator, Points: len(pts),
				FirstDate: first.Date, FirstValue: first.Value,
				LatestDate: latest.Date, LatestValue: latest.Value,
			}
			minV, maxV := *pts[0].Value, *pts[0].Value
			for _, p := range pts {
				if *p.Value < minV {
					minV = *p.Value
				}
				if *p.Value > maxV {
					maxV = *p.Value
				}
			}
			view.MinValue, view.MaxValue = &minV, &maxV

			if len(pts) >= 2 && *first.Value != 0 {
				total := (*latest.Value - *first.Value) / *first.Value * 100
				view.TotalPctChg = &total
				// CAGR over the year span.
				var y0, y1 int
				// Best-effort year parse; unparsed values stay 0 and are caught by the y1 > y0 guard below.
				_, _ = fmt.Sscanf(first.Date, "%d", &y0)
				_, _ = fmt.Sscanf(latest.Date, "%d", &y1)
				if y1 > y0 && *first.Value > 0 && *latest.Value > 0 {
					n := float64(y1 - y0)
					cagr := (math.Pow(*latest.Value / *first.Value, 1/n) - 1) * 100
					view.CAGRPct = &cagr
				}
			}
			if len(pts) >= 2 {
				prev := pts[len(pts)-2]
				if prev.Value != nil && *prev.Value != 0 {
					yoy := (*latest.Value - *prev.Value) / *prev.Value * 100
					view.YoYPct = &yoy
				}
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().IntVar(&flagWindow, "window", 0, "Limit to the most recent N values (0 = full series)")
	return cmd
}
