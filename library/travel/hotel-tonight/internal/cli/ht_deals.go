// `deals` — the rich, agent-friendly last-minute deal search. Wraps
// /v6/inventory with the --category tier filter, %-off sort/threshold, and a
// flattened deal view, and records a price snapshot so history/verdict/watch
// have data. Hand-authored (survives `generate --force`).
package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newDealsCmd(flags *rootFlags) *cobra.Command {
	var (
		lat, lng    string
		metro       int
		when        string
		checkIn     string
		checkOut    string
		rooms       int
		category    string
		sortKey     string
		minDiscount int
		limit       int
	)

	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Search last-minute hotel deals, filtered by tier and ranked by price or % off",
		Long: "Search HotelTonight's last-minute deals near a coordinate or market and return a " +
			"flattened, agent-friendly view (hotel, neighborhood, price, % off vs the 30-day high, tier). " +
			"Filter by hotel quality tier with --category, rank with --sort, and require a minimum discount " +
			"with --min-discount. Each search also records a price snapshot into the local store so the " +
			"history, verdict, and watch commands have data to work with.",
		Example: strings.Trim(`
  hotel-tonight-pp-cli deals --lat 37.7749 --lng -122.4194
  hotel-tonight-pp-cli deals --metro 1 --category luxe,hip --sort discount
  hotel-tonight-pp-cli deals --lat 30.3071 --lng -97.7354 --when weekend --min-discount 30 --agent
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
			ci, co := checkIn, checkOut
			if ci == "" && co == "" {
				ci, co = computeDates(when, timeNow())
			}
			inv, err := fetchAndRecord(cmd.Context(), cmd, c, rlat, rlng, ci, co, rooms)
			if err != nil {
				return err
			}
			deals := dealsFrom(inv, cats, minDiscount, sortKey)
			if limit > 0 && len(deals) > limit {
				deals = deals[:limit]
			}

			result := map[string]any{
				"market":   inv.PrimaryMarket.CityName,
				"check_in": inv.CurrentDay,
				"nights":   inv.NumNights,
				"count":    len(deals),
				"deals":    deals,
			}
			rows := make([][]string, 0, len(deals))
			for _, d := range deals {
				pct := ""
				if d.PctOff > 0 {
					pct = strconv.Itoa(d.PctOff) + "%"
				}
				rows = append(rows, []string{d.HotelName, d.Neighborhood, d.Category, money(d.Price), pct})
			}
			return emitJSONOrTable(cmd, flags, result,
				[]string{"Hotel", "Neighborhood", "Tier", "Price", "Off"}, rows)
		},
	}

	cmd.Flags().StringVar(&lat, "lat", "", "Latitude of the search center")
	cmd.Flags().StringVar(&lng, "lng", "", "Longitude of the search center")
	cmd.Flags().IntVar(&metro, "metro", 0, "Market id to search (see 'markets list'); alternative to --lat/--lng")
	cmd.Flags().StringVar(&when, "when", "tonight", "When to stay: tonight, tomorrow, or weekend")
	cmd.Flags().StringVar(&checkIn, "check-in", "", "Explicit check-in date YYYY-MM-DD (overrides --when)")
	cmd.Flags().StringVar(&checkOut, "check-out", "", "Explicit check-out date YYYY-MM-DD (overrides --when)")
	cmd.Flags().IntVar(&rooms, "rooms", 0, "Number of rooms")
	cmd.Flags().StringVar(&category, "category", "", categoryFlagHelp)
	cmd.Flags().StringVar(&sortKey, "sort", "price", "Sort order: price (cheapest first) or discount (biggest % off first)")
	cmd.Flags().IntVar(&minDiscount, "min-discount", 0, "Only show deals at least this % off the 30-day high")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of deals to return (0 = all)")
	return cmd
}
