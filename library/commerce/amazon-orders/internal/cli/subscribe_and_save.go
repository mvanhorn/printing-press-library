package cli

import (
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
	"github.com/spf13/cobra"
)

// SubscribeSaveItem is one inferred recurring purchase (a de-facto subscription).
type SubscribeSaveItem struct {
	ASIN            string `json:"asin,omitempty"`
	Title           string `json:"title"`
	Occurrences     int    `json:"occurrences"`
	FirstOrdered    string `json:"firstOrdered,omitempty"`
	LastOrdered     string `json:"lastOrdered,omitempty"`
	AvgIntervalDays int    `json:"avgIntervalDays,omitempty"`
	NextExpected    string `json:"nextExpected,omitempty"`
	Regular         bool   `json:"regular"`
}

// newSubscribeAndSaveCmd detects items bought on a regular cadence directly from
// the live order-history listing — no local store / sync required.
func newSubscribeAndSaveCmd(flags *rootFlags) *cobra.Command {
	var minOccurrences int
	var window string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "subscribe-and-save",
		Short: "Recurring purchases inferred from order history (de-facto subscriptions).",
		Long: `Groups order history by ASIN and measures the spacing between repeat
purchases to surface items you buy on a regular cadence — candidates for
actual Subscribe & Save, and de-facto subscriptions you may not realize you
have. Computed live from the order-history listing; no sync required.

Widen --window to scan more history: cadences spanning multiple years need a
wider window than the default current-year scan.`,
		Example:     "  amazon-orders-pp-cli subscribe-and-save --min-occurrences 3 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			orders, err := fetchOrderListPages(cmd.Context(), c, window, maxPages)
			if err != nil {
				return err
			}
			items := detectRecurringPurchases(orders, minOccurrences)
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().IntVar(&minOccurrences, "min-occurrences", 3, "Minimum repeat purchases of the same ASIN to flag as recurring.")
	cmd.Flags().StringVar(&window, "window", defaultYearWindow(), "Time window to scan: year-YYYY, last30days, months-3, months-6, archived.")
	cmd.Flags().IntVar(&maxPages, "max-pages", 12, "Maximum order-history pages to walk (10 orders/page).")
	return cmd
}

// detectRecurringPurchases groups orders by ASIN and reports those purchased at
// least minOccurrences times, with cadence statistics. An item is flagged
// Regular when its purchase intervals are consistent (low coefficient of
// variation), distinguishing a true cadence from sporadic re-buys.
func detectRecurringPurchases(orders []parser.OrderSummary, minOccurrences int) []SubscribeSaveItem {
	if minOccurrences < 2 {
		minOccurrences = 2
	}
	type group struct {
		title string
		dates []time.Time
	}
	groups := map[string]*group{}
	for _, o := range orders {
		d := parser.ParseDate(o.PlacedDate)
		if d.IsZero() {
			continue // can't place it on a timeline (e.g. unparsed digital-order date)
		}
		for i, asin := range o.ASINs {
			if asin == "" {
				continue
			}
			g, ok := groups[asin]
			if !ok {
				g = &group{}
				groups[asin] = g
			}
			if g.title == "" && i < len(o.ItemTitles) {
				g.title = o.ItemTitles[i]
			}
			g.dates = append(g.dates, d)
		}
	}

	out := []SubscribeSaveItem{}
	for asin, g := range groups {
		if len(g.dates) < minOccurrences {
			continue
		}
		sort.Slice(g.dates, func(i, j int) bool { return g.dates[i].Before(g.dates[j]) })

		intervals := make([]float64, 0, len(g.dates)-1)
		for i := 1; i < len(g.dates); i++ {
			intervals = append(intervals, g.dates[i].Sub(g.dates[i-1]).Hours()/24)
		}
		mean := meanFloat(intervals)

		title := g.title
		if title == "" {
			title = "(no title)"
		}
		last := g.dates[len(g.dates)-1]
		item := SubscribeSaveItem{
			ASIN:            asin,
			Title:           title,
			Occurrences:     len(g.dates),
			FirstOrdered:    g.dates[0].Format("2006-01-02"),
			LastOrdered:     last.Format("2006-01-02"),
			AvgIntervalDays: int(math.Round(mean)),
			Regular:         len(intervals) >= 2 && coefficientOfVariation(intervals, mean) <= 0.4,
		}
		if mean > 0 {
			item.NextExpected = last.AddDate(0, 0, int(math.Round(mean))).Format("2006-01-02")
		}
		out = append(out, item)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Regular != out[j].Regular {
			return out[i].Regular // regular cadences first
		}
		if out[i].Occurrences != out[j].Occurrences {
			return out[i].Occurrences > out[j].Occurrences
		}
		return out[i].AvgIntervalDays < out[j].AvgIntervalDays
	})
	return out
}

func meanFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// coefficientOfVariation is stddev/mean — a unitless measure of how regular the
// intervals are. Lower means a more consistent cadence.
func coefficientOfVariation(xs []float64, mean float64) float64 {
	if len(xs) == 0 || mean == 0 {
		return math.Inf(1)
	}
	var ss float64
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	return math.Sqrt(ss/float64(len(xs))) / mean
}
