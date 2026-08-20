// `datescan` — compare a location's deals across tonight, tomorrow, and the
// weekend in one ranked view. Fans out a real inventory call per date and
// summarizes each, so a failed date is reported rather than silently dropped.
// Hand-authored (survives `generate --force`).
package cli

import (
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// htDateOption summarizes the deals available for one candidate date.
type htDateOption struct {
	When          string  `json:"when"`
	CheckIn       string  `json:"check_in"`
	CheckOut      string  `json:"check_out"`
	DealCount     int     `json:"deal_count"`
	MinPrice      float64 `json:"min_price"`
	BestPctOff    int     `json:"best_pct_off"`
	CheapestHotel string  `json:"cheapest_hotel"`
	Error         string  `json:"error,omitempty"`
}

func newDatescanCmd(flags *rootFlags) *cobra.Command {
	var (
		lat, lng string
		metro    int
		rooms    int
		category string
	)

	cmd := &cobra.Command{
		Use:   "datescan",
		Short: "Compare a location's deals across tonight, tomorrow, and the weekend",
		Long: "Run the deal search for tonight, tomorrow, and the upcoming weekend over the same location and " +
			"line the results up side by side, ranked cheapest-night first. The app only shows one date at a " +
			"time; this answers \"which night is cheapest to stay here\" in a single call. Records a snapshot per date.",
		Example: strings.Trim(`
  hotel-tonight-pp-cli datescan --lat 30.3071 --lng -97.7354
  hotel-tonight-pp-cli datescan --metro 1 --category luxe --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cats, err := parseCategories(category)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rlat, rlng, _, err := resolveGeo(c, metro, lat, lng)
			if err != nil {
				return err
			}

			var options []htDateOption
			for _, when := range []string{"tonight", "tomorrow", "weekend"} {
				ci, co := computeDates(when, timeNow())
				opt := htDateOption{When: when, CheckIn: ci, CheckOut: co}
				inv, ferr := fetchAndRecord(cmd.Context(), cmd, c, rlat, rlng, ci, co, rooms)
				if ferr != nil {
					opt.Error = ferr.Error()
					options = append(options, opt)
					continue
				}
				if opt.CheckIn == "" {
					opt.CheckIn = inv.CurrentDay
				}
				summarizeDate(&opt, dealsFrom(inv, cats, 0, "price"))
				options = append(options, opt)
			}

			// Rank successful dates cheapest-first; failed/empty dates sink.
			sort.SliceStable(options, func(i, j int) bool {
				ai, aj := options[i].MinPrice, options[j].MinPrice
				if ai <= 0 {
					return false
				}
				if aj <= 0 {
					return true
				}
				return ai < aj
			})

			result := map[string]any{"options": options}
			rows := make([][]string, 0, len(options))
			for _, o := range options {
				note := o.CheapestHotel
				if o.Error != "" {
					note = "error: " + o.Error
				}
				rows = append(rows, []string{o.When, o.CheckIn, strconv.Itoa(o.DealCount), money(o.MinPrice), strconv.Itoa(o.BestPctOff) + "%", note})
			}
			return emitJSONOrTable(cmd, flags, result,
				[]string{"When", "Check-in", "Deals", "From", "Best Off", "Cheapest"}, rows)
		},
	}

	cmd.Flags().StringVar(&lat, "lat", "", "Latitude of the search center")
	cmd.Flags().StringVar(&lng, "lng", "", "Longitude of the search center")
	cmd.Flags().IntVar(&metro, "metro", 0, "Market id (see 'markets list'); alternative to --lat/--lng")
	cmd.Flags().IntVar(&rooms, "rooms", 0, "Number of rooms")
	cmd.Flags().StringVar(&category, "category", "", categoryFlagHelp)
	return cmd
}

// summarizeDate fills the per-date rollup fields from a date's deal list.
func summarizeDate(opt *htDateOption, deals []htDeal) {
	opt.DealCount = len(deals)
	for _, d := range deals {
		if d.Price > 0 && (opt.MinPrice == 0 || d.Price < opt.MinPrice) {
			opt.MinPrice = d.Price
			opt.CheapestHotel = d.HotelName
		}
		if d.PctOff > opt.BestPctOff {
			opt.BestPctOff = d.PctOff
		}
	}
}
