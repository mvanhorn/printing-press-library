// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"
)

type cheapestDay struct {
	CheckIn string  `json:"checkin"`
	Weekday string  `json:"weekday"`
	Price   float64 `json:"price"`
	// Agoda's price-trend endpoint answers with one city-level aggregate per
	// date, so this counts trend observations folded into the date, not
	// properties priced. The old "properties_sampled" name read as though the
	// cheapest-date ranking rested on a single hotel.
	Observations int    `json:"observations"`
	TrendType    string `json:"trend_type,omitempty"`
}

type cheapestResult struct {
	Destination   string        `json:"destination"`
	CityID        int           `json:"city_id"`
	WindowStart   string        `json:"window_start"`
	WindowEnd     string        `json:"window_end"`
	Nights        int           `json:"nights"`
	Currency      string        `json:"currency"`
	DaysCovered   int           `json:"days_covered"`
	CheapestDate  string        `json:"cheapest_date,omitempty"`
	CheapestPrice float64       `json:"cheapest_price,omitempty"`
	MedianPrice   float64       `json:"median_price,omitempty"`
	SavingsPct    float64       `json:"savings_vs_median_pct,omitempty"`
	Results       []cheapestDay `json:"results"`
	Note          string        `json:"note,omitempty"`
}

func newNovelPricesCheapestCmd(flags *rootFlags) *cobra.Command {
	var cityID int
	var window string
	var nights int
	var occupancy int
	var currency string
	var limit int

	cmd := &cobra.Command{
		Use:   "cheapest [destination]",
		Short: "Find the cheapest check-in dates across a flexible window",
		Long: `Sweep a date window and return the cheapest check-in dates for a destination.

This uses Agoda's own price-trend operation, which returns prices across the
whole window in a single request. A tool built on ordinary search would have to
issue one search per candidate date, so this answers a flexible-date question in
one round trip instead of dozens.

Prices here are Agoda's trend figures for the destination, intended for choosing
dates. Once a date is chosen, price it properly with 'hotels search', which
reports the true all-in cost per property.`,
		Example: "  agoda-pp-cli prices cheapest Tokyo --window 2026-10-01..2026-11-30 --nights 3 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "destination=Tokyo;--nights=2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "prices cheapest")
			}
			dest := ""
			if len(args) > 0 {
				dest = args[0]
			}
			if dest == "" && cityID <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a destination argument or --city-id is required"))
			}
			start, end, err := parseWindow(window)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if nights <= 0 {
				nights = 1
			}
			if occupancy <= 0 {
				occupancy = defaultAdults
			}
			cur := strings.ToUpper(strings.TrimSpace(currency))
			if cur == "" {
				cur = defaultCurrency
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newAgodaClient(flags)
			d, err := resolveCity(ctx, c, dest, cityID)
			if err != nil {
				return err
			}
			points, err := c.PriceTrend(ctx, d.CityID, start, end, nights, occupancy, cur, hasAgodaSession())
			if err != nil {
				return err
			}

			out := cheapestResult{
				Destination: displayDestination(d, dest),
				CityID:      d.CityID,
				WindowStart: start.Format("2006-01-02"),
				WindowEnd:   end.Format("2006-01-02"),
				Nights:      nights,
				Currency:    cur,
				Results:     make([]cheapestDay, 0),
			}
			days := collapseByDate(points)
			out.DaysCovered = len(days)
			if len(days) == 0 {
				out.Note = "Agoda returned no price-trend data for this window. It answers with an " +
					"empty result rather than an error when it soft-throttles, so retry once before " +
					"concluding the window is genuinely unpriced; otherwise try a nearer window or another destination"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No price-trend data for %s between %s and %s.\n%s\n",
					out.Destination, out.WindowStart, out.WindowEnd, out.Note)
				return nil
			}

			sort.SliceStable(days, func(i, j int) bool { return days[i].Price < days[j].Price })
			out.CheapestDate = days[0].CheckIn
			out.CheapestPrice = days[0].Price
			out.MedianPrice = medianOfDays(days)
			if out.MedianPrice > 0 {
				out.SavingsPct = round2Pct((out.MedianPrice - out.CheapestPrice) / out.MedianPrice * 100)
			}
			if limit > 0 && len(days) > limit {
				days = days[:limit]
			}
			out.Results = days

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAgodaJSON(cmd.OutOrStdout(), out, flags, "live")
			}
			return renderCheapest(cmd, out)
		},
	}
	cmd.Flags().IntVar(&cityID, "city-id", 0, "Agoda numeric city id; skips destination lookup")
	cmd.Flags().StringVar(&window, "window", "",
		"Date window as YYYY-MM-DD..YYYY-MM-DD (default: the next 60 days)")
	cmd.Flags().IntVar(&nights, "nights", 1, "Length of stay to price for each candidate date")
	cmd.Flags().IntVar(&occupancy, "adults", defaultAdults, "Number of adult guests")
	cmd.Flags().StringVar(&currency, "currency", defaultCurrency, "ISO currency code, e.g. USD, EUR, JPY")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum dates to return, cheapest first")
	return cmd
}

// parseWindow accepts "START..END" and defaults to the next 60 days, which is
// roughly the span Agoda's own trend operation covers.
func parseWindow(w string) (time.Time, time.Time, error) {
	w = strings.TrimSpace(w)
	if w == "" {
		now := time.Now()
		return now, now.AddDate(0, 0, 60), nil
	}
	parts := strings.SplitN(w, "..", 2)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("--window must be YYYY-MM-DD..YYYY-MM-DD, got %q", w)
	}
	start, err := time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--window start must be YYYY-MM-DD, got %q", parts[0])
	}
	end, err := time.Parse("2006-01-02", strings.TrimSpace(parts[1]))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--window end must be YYYY-MM-DD, got %q", parts[1])
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("--window end (%s) must be after start (%s)", parts[1], parts[0])
	}
	return start, end, nil
}

// collapseByDate reduces per-property observations to one cheapest price per
// date, which is the question a flexible-date traveler is actually asking.
func collapseByDate(points []agoda.TrendPoint) []cheapestDay {
	type acc struct {
		min   float64
		count int
		trend string
	}
	byDate := map[string]*acc{}
	for _, p := range points {
		if p.CheckIn == "" || p.Price <= 0 {
			continue
		}
		a, ok := byDate[p.CheckIn]
		if !ok {
			byDate[p.CheckIn] = &acc{min: p.Price, count: 1, trend: p.TrendType}
			continue
		}
		a.count++
		if p.Price < a.min {
			a.min = p.Price
			a.trend = p.TrendType
		}
	}
	out := make([]cheapestDay, 0, len(byDate))
	for date, a := range byDate {
		wd := ""
		if t, err := time.Parse("2006-01-02", date); err == nil {
			wd = t.Format("Mon")
		}
		out = append(out, cheapestDay{
			CheckIn:      date,
			Weekday:      wd,
			Price:        a.min,
			Observations: a.count,
			TrendType:    a.trend,
		})
	}
	return out
}

func medianOfDays(days []cheapestDay) float64 {
	if len(days) == 0 {
		return 0
	}
	vals := make([]float64, 0, len(days))
	for _, d := range days {
		vals = append(vals, d.Price)
	}
	sort.Float64s(vals)
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return (vals[mid-1] + vals[mid]) / 2
}

func renderCheapest(cmd *cobra.Command, res cheapestResult) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s (city %d) - %s to %s, %d night(s), prices in %s\n\n",
		res.Destination, res.CityID, res.WindowStart, res.WindowEnd, res.Nights, res.Currency)
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CHECK-IN\tDAY\tFROM")
	for _, d := range res.Results {
		fmt.Fprintf(w, "%s\t%s\t%.2f\n", d.CheckIn, d.Weekday, d.Price)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\nCheapest: %s at %.2f (%.1f%% below the window median of %.2f), across %d dates.\n",
		res.CheapestDate, res.CheapestPrice, res.SavingsPct, res.MedianPrice, res.DaysCovered)
	fmt.Fprintln(out, "Price a specific date properly with: agoda-pp-cli hotels search <destination> --checkin <date>")
	if res.Note != "" {
		fmt.Fprintf(out, "%s\n", res.Note)
	}
	return nil
}
