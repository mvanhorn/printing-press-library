// Hand-authored transcendence command: corridor. Best hotel per stop on a
// user-defined multi-city route, in one pass. Not generated.
package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type corridorStop struct {
	City    string     `json:"city"`
	Label   string     `json:"label"`
	Matches int        `json:"matches"`
	Top     []hotelOut `json:"top"`
	Note    string     `json:"note,omitempty"`
}

type corridorView struct {
	Source     string         `json:"source"`
	Disclaimer string         `json:"disclaimer"`
	Stops      []corridorStop `json:"stops"`
}

func newNovelCorridorCmd(flags *rootFlags) *cobra.Command {
	var cities, amenities string
	var minRating, maxPrice float64
	var top int

	cmd := &cobra.Command{
		Use:   "corridor",
		Short: "Find the best hotel in each stop of a multi-city route, with shared filters",
		Long: "Scan a user-defined corridor of cities in one pass and return the best hotels in each, " +
			"applying the same compound filters everywhere. Resolves each city via the local geohash " +
			"table. Use 'rank-country' for an exhaustive national scan. Data is scraped from " +
			"hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli corridor --cities "Chiang Mai,Lisbon,Tbilisi" --min-rating 7.5 --max-price 120
  hotelist-pp-cli corridor --cities "Bangkok,Bali,Medellin" --amenities coworking --top 3 --json`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cityList := splitCSV(cities)
			if len(cityList) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--cities is required, e.g. --cities \"Chiang Mai,Lisbon\""))
			}

			c, err := flags.politeClient()
			if err != nil {
				return err
			}
			db, err := openHotelStore(cmd.Context(), flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			var extra []apiFilter
			for _, a := range splitCSV(amenities) {
				if label := amenityLabel(a); label != "" {
					extra = append(extra, filterAmenity(label))
				}
			}
			if minRating > 0 {
				extra = append(extra, filterMinRating(minRating))
			}
			if maxPrice > 0 {
				extra = append(extra, filterMaxPrice(maxPrice))
			}
			bestValue := filterSort("best-value", "desc")

			view := corridorView{Source: hotelistSource, Disclaimer: hotelistDisclaimer}
			for _, city := range cityList {
				loc, err := resolveLocation(cmd.Context(), c, db, city)
				if err != nil {
					view.Stops = append(view.Stops, corridorStop{City: city, Note: err.Error()})
					continue
				}
				filters := append([]apiFilter{}, loc.Filters...)
				filters = append(filters, extra...)
				filters = append(filters, bestValue)
				hotels, err := adaptiveFetch(cmd.Context(), c, filters, "")
				if err != nil {
					view.Stops = append(view.Stops, corridorStop{City: city, Label: loc.Label, Note: "fetch error: " + err.Error()})
					continue
				}
				hotels = dedupeHotels(hotels)
				storeHotels(db, hotels)
				// The upstream endpoint ignores the amenities filter param, so
				// enforce --amenities locally at every stop.
				hotels = filterByAmenities(hotels, splitCSV(amenities))
				hotels = dropUnpriced(hotels) // value ranking requires a real price
				sortHotelsByValue(hotels)
				if top > 0 && len(hotels) > top {
					hotels = hotels[:top]
				}
				stop := corridorStop{City: city, Label: loc.Label, Matches: len(hotels), Note: loc.Note}
				for _, h := range hotels {
					stop.Top = append(stop.Top, toHotelOut(h))
				}
				view.Stops = append(view.Stops, stop)
			}
			return printCorridor(cmd.OutOrStdout(), flags, view)
		},
	}
	cmd.Flags().StringVar(&cities, "cities", "", "Comma-separated list of cities for the route")
	cmd.Flags().StringVar(&amenities, "amenities", "", "Comma-separated photo-verified amenities to require everywhere")
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Only hotels with a Hotelist Score at or above this (0-10)")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Only hotels at or below this nightly price")
	cmd.Flags().IntVar(&top, "top", 3, "Hotels to show per stop")
	return cmd
}

func printCorridor(out io.Writer, flags *rootFlags, view corridorView) error {
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, view, flags)
	}
	for _, stop := range view.Stops {
		label := stop.Label
		if label == "" {
			label = stop.City
		}
		fmt.Fprintf(out, "\n%s (%d matches)\n", label, stop.Matches)
		if stop.Note != "" {
			fmt.Fprintf(out, "  note: %s\n", stop.Note)
		}
		for i, h := range stop.Top {
			val := ""
			if h.ValuePer100USD > 0 {
				val = fmt.Sprintf("  value %.1f", h.ValuePer100USD)
			}
			fmt.Fprintf(out, "  %d. %-36s ⭐%.1f $%.0f%s\n", i+1, truncate(h.Name, 36), h.Rating, h.Price, val)
		}
	}
	fmt.Fprintf(out, "\n%s\n", view.Disclaimer)
	return nil
}
