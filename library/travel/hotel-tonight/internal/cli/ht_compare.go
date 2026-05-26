// `compare-neighborhoods` — group a metro's live deals by neighborhood and
// rank the neighborhoods by median price or best % off. The flat /v6/inventory
// feed returns no neighborhood rollup, so this group-by is a local
// computation. Hand-authored (survives `generate --force`).
package cli

import (
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// htNeighborhood is the rolled-up view of one neighborhood's deals.
type htNeighborhood struct {
	Neighborhood  string  `json:"neighborhood"`
	DealCount     int     `json:"deal_count"`
	MinPrice      float64 `json:"min_price"`
	MedianPrice   float64 `json:"median_price"`
	BestPctOff    int     `json:"best_pct_off"`
	CheapestHotel string  `json:"cheapest_hotel"`
}

func newCompareNeighborhoodsCmd(flags *rootFlags) *cobra.Command {
	var (
		lat, lng string
		metro    int
		when     string
		rooms    int
		category string
		sortKey  string
	)

	cmd := &cobra.Command{
		Use:   "compare-neighborhoods",
		Short: "Group a metro's deals by neighborhood and rank by median price or best % off",
		Long: "Pull the current deals for a market or coordinate and roll them up by neighborhood, showing the " +
			"deal count, cheapest and median price, and best % off per neighborhood. Answers \"which area of " +
			"town has the best value tonight\" — a comparison the app, which shows one pin at a time, can't make.",
		Example: strings.Trim(`
  hotel-tonight-pp-cli compare-neighborhoods --metro 1
  hotel-tonight-pp-cli compare-neighborhoods --lat 30.3071 --lng -97.7354 --sort discount --agent
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
			ci, co := computeDates(when, timeNow())
			inv, err := fetchAndRecord(cmd.Context(), cmd, c, rlat, rlng, ci, co, rooms)
			if err != nil {
				return err
			}

			groups := groupByNeighborhood(dealsFrom(inv, cats, 0, "price"))
			sortNeighborhoods(groups, sortKey)

			result := map[string]any{
				"market":        inv.PrimaryMarket.CityName,
				"check_in":      inv.CurrentDay,
				"neighborhoods": groups,
				"count":         len(groups),
			}
			rows := make([][]string, 0, len(groups))
			for _, g := range groups {
				rows = append(rows, []string{
					g.Neighborhood, strconv.Itoa(g.DealCount),
					money(g.MinPrice), money(g.MedianPrice), strconv.Itoa(g.BestPctOff) + "%",
				})
			}
			return emitJSONOrTable(cmd, flags, result,
				[]string{"Neighborhood", "Deals", "From", "Median", "Best Off"}, rows)
		},
	}

	cmd.Flags().StringVar(&lat, "lat", "", "Latitude of the search center")
	cmd.Flags().StringVar(&lng, "lng", "", "Longitude of the search center")
	cmd.Flags().IntVar(&metro, "metro", 0, "Market id (see 'markets list'); alternative to --lat/--lng")
	cmd.Flags().StringVar(&when, "when", "tonight", "When to stay: tonight, tomorrow, or weekend")
	cmd.Flags().IntVar(&rooms, "rooms", 0, "Number of rooms")
	cmd.Flags().StringVar(&category, "category", "", categoryFlagHelp)
	cmd.Flags().StringVar(&sortKey, "sort", "price", "Rank neighborhoods by: price (lowest median first) or discount (best % off first)")
	return cmd
}

// groupByNeighborhood rolls deals up by neighborhood. Deals with no
// neighborhood are bucketed under "(unknown)".
func groupByNeighborhood(deals []htDeal) []htNeighborhood {
	type acc struct {
		prices   []float64
		bestOff  int
		cheapest string
		cheapAt  float64
	}
	byHood := map[string]*acc{}
	for _, d := range deals {
		hood := d.Neighborhood
		if hood == "" {
			hood = "(unknown)"
		}
		a := byHood[hood]
		if a == nil {
			a = &acc{cheapAt: -1}
			byHood[hood] = a
		}
		if d.Price > 0 {
			a.prices = append(a.prices, d.Price)
			if a.cheapAt < 0 || d.Price < a.cheapAt {
				a.cheapAt = d.Price
				a.cheapest = d.HotelName
			}
			// Keep BestPctOff on the same priced-deal set as DealCount so the
			// two metrics never disagree (e.g. DealCount 0 with BestPctOff > 0).
			if d.PctOff > a.bestOff {
				a.bestOff = d.PctOff
			}
		}
	}
	out := make([]htNeighborhood, 0, len(byHood))
	for hood, a := range byHood {
		sort.Float64s(a.prices)
		n := htNeighborhood{
			Neighborhood:  hood,
			DealCount:     len(a.prices),
			BestPctOff:    a.bestOff,
			CheapestHotel: a.cheapest,
		}
		if len(a.prices) > 0 {
			n.MinPrice = a.prices[0]
			n.MedianPrice = percentile(a.prices, 0.50)
		}
		out = append(out, n)
	}
	return out
}

// sortNeighborhoods ranks groups by best % off descending when discount sort
// is requested, else by lowest median price.
func sortNeighborhoods(groups []htNeighborhood, sortKey string) {
	if isDiscountSort(sortKey) {
		sort.SliceStable(groups, func(i, j int) bool { return groups[i].BestPctOff > groups[j].BestPctOff })
		return
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].MedianPrice < groups[j].MedianPrice })
}
