// pp:data-source local
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/store"
	"github.com/spf13/cobra"
)

type statsView struct {
	EventID           string   `json:"event_id"`
	Points            int      `json:"points"`
	Low               *float64 `json:"low,omitempty"`
	High              *float64 `json:"high,omitempty"`
	Median            *float64 `json:"median,omitempty"`
	Current           *float64 `json:"current,omitempty"`
	CurrentPercentile *float64 `json:"current_percentile,omitempty"`
	Volatility        *float64 `json:"volatility,omitempty"`
	BestWeekday       string   `json:"best_weekday,omitempty"`
	BestWeekdayAvg    *float64 `json:"best_weekday_avg,omitempty"`
}

func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "stats <event-id>",
		Short:       "Historical low/high, median, current percentile, volatility, and the weekday the floor is typically lowest",
		Long:        "Use `stats` for one event's price distribution and best day to buy. To compare multiple events use `compare`; for the whole-watchlist snapshot use `board`.",
		Example:     "  ticketdata-pp-cli stats 22323960 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "22323960"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && commandNFlag(cmd) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read local price history for one event")
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("event id is required"))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("stats accepts one event id"))
			}
			if !isNumericEventID(args[0]) {
				return usageErr(fmt.Errorf("event id must be numeric, e.g. 22323960"))
			}
			if !cmd.Flags().Changed("db") {
				dbPath = defaultDBPath("ticketdata-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			points, err := db.PricePoints(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			view := buildStatsView(args[0], points)
			if view.Points == 0 {
				hintIfUnsynced(cmd, db, "")
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printStatsTable(cmd, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("ticketdata-pp-cli"), "SQLite database file path")
	return cmd
}

func buildStatsView(eventID string, points []store.TDPricePoint) statsView {
	prices := make([]float64, 0, len(points))
	weekdayTotals := make(map[time.Weekday]float64)
	weekdayCounts := make(map[time.Weekday]int)
	for _, pt := range points {
		if pt.GetInPrice == 0 {
			continue
		}
		prices = append(prices, pt.GetInPrice)
		if ts, ok := parseTicketdataTime(pt.InsertedAt); ok {
			weekdayTotals[ts.Weekday()] += pt.GetInPrice
			weekdayCounts[ts.Weekday()]++
		}
	}
	view := statsView{EventID: eventID, Points: len(prices)}
	if len(prices) == 0 {
		return view
	}
	sorted := append([]float64(nil), prices...)
	sort.Float64s(sorted)
	low, high, current := sorted[0], sorted[len(sorted)-1], prices[len(prices)-1]
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	lte, sum := 0, 0.0
	for _, price := range prices {
		sum += price
		if price <= current {
			lte++
		}
	}
	currentPercentile := float64(lte) / float64(len(prices)) * 100
	mean := sum / float64(len(prices))
	variance := 0.0
	for _, price := range prices {
		variance += math.Pow(price-mean, 2)
	}
	volatility := math.Sqrt(variance / float64(len(prices)))
	view.Low, view.High, view.Median = &low, &high, &median
	view.Current, view.Volatility = &current, &volatility
	// A percentile from a single point is trivially 100 and carries no signal,
	// so only report it with >=2 points (matching `board`'s guard).
	if len(prices) >= 2 {
		view.CurrentPercentile = &currentPercentile
	}
	setBestWeekday(&view, weekdayTotals, weekdayCounts)
	return view
}

func parseTicketdataTime(raw string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02T15:04:05", raw); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func setBestWeekday(view *statsView, totals map[time.Weekday]float64, counts map[time.Weekday]int) {
	var bestAvg float64
	for day := time.Sunday; day <= time.Saturday; day++ {
		if counts[day] == 0 {
			continue
		}
		avg := totals[day] / float64(counts[day])
		if view.BestWeekday == "" || avg < bestAvg {
			bestAvg = avg
			dayAvg := avg
			view.BestWeekday = day.String()
			view.BestWeekdayAvg = &dayAvg
		}
	}
}

func printStatsTable(cmd *cobra.Command, view statsView) error {
	if view.Points == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "no price history found for %s\n", view.EventID)
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "EVENT\tPOINTS\tLOW\tHIGH\tMEDIAN\tCURRENT\tPERCENTILE\tVOLATILITY\tBEST WEEKDAY\tAVG")
	fmt.Fprintf(tw, "%s\t%d\t%.2f\t%.2f\t%.2f\t%.2f\t%.1f\t%.2f\t%s\t%.2f\n",
		view.EventID, view.Points, *view.Low, *view.High, *view.Median, *view.Current,
		valueOrZero(view.CurrentPercentile), *view.Volatility, view.BestWeekday, valueOrZero(view.BestWeekdayAvg))
	return tw.Flush()
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
