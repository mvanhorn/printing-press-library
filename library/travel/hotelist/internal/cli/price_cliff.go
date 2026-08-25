// Hand-authored transcendence command: price-cliff. Finds the price point where
// rating-per-extra-dollar collapses — the cheapest hotel that's still good.
// Not generated.
package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type priceBin struct {
	Low        float64 `json:"low"`
	High       float64 `json:"high"`
	Hotels     int     `json:"hotels"`
	MeanRating float64 `json:"mean_rating"`
}

type priceCliffView struct {
	Source      string     `json:"source"`
	Disclaimer  string     `json:"disclaimer"`
	Location    string     `json:"location"`
	BinSize     float64    `json:"bin_size"`
	Bins        []priceBin `json:"bins"`
	CliffPrice  float64    `json:"cliff_price,omitempty"`
	Recommended []hotelOut `json:"recommended,omitempty"`
	Note        string     `json:"note,omitempty"`
}

func newNovelPriceCliffCmd(flags *rootFlags) *cobra.Command {
	var minRating, binSize float64

	cmd := &cobra.Command{
		Use:   "price-cliff <city>",
		Short: "Find the price point in a city where rating-per-extra-dollar collapses",
		Long: "Bin a city's hotels by price, compute the mean Hotelist rating per bin, and find the price " +
			"point where paying more stops buying meaningfully better hotels — the cheapest level that's " +
			"still legitimately good. The site's histogram is visual-only; this extracts the breakpoint. " +
			"Data is scraped from hotelist.com (community/AI-rated, not an official API).",
		Example: trimExample(`
  hotelist-pp-cli price-cliff bangkok
  hotelist-pp-cli price-cliff lisbon --min-rating 7 --bin-size 20 --json`),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<city>=bangkok"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a city is required"))
			}
			if binSize <= 0 {
				binSize = 25
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

			loc, err := resolveLocation(cmd.Context(), c, db, args[0])
			if err != nil {
				return err
			}
			var extra []apiFilter
			if minRating > 0 {
				extra = append(extra, filterMinRating(minRating))
			}
			filters := append([]apiFilter{}, loc.Filters...)
			filters = append(filters, extra...)
			hotels, err := adaptiveFetch(cmd.Context(), c, filters, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			hotels = dedupeHotels(hotels)
			storeHotels(db, hotels)

			view := priceCliffView{Source: hotelistSource, Disclaimer: hotelistDisclaimer, Location: loc.Label, BinSize: binSize}
			priced := make([]hlHotel, 0, len(hotels))
			for _, h := range hotels {
				if h.Price > 0 {
					priced = append(priced, h)
				}
			}
			if len(priced) < 4 {
				view.Note = "not enough priced hotels to compute a price cliff (need at least 4)"
				return printPriceCliff(cmd.OutOrStdout(), flags, view)
			}

			// Bin by price.
			binMap := map[int][]float64{}
			for _, h := range priced {
				idx := int(h.Price / binSize)
				binMap[idx] = append(binMap[idx], h.Rating)
			}
			idxs := make([]int, 0, len(binMap))
			for k := range binMap {
				idxs = append(idxs, k)
			}
			sort.Ints(idxs)
			for _, k := range idxs {
				view.Bins = append(view.Bins, priceBin{
					Low:        float64(k) * binSize,
					High:       float64(k+1) * binSize,
					Hotels:     len(binMap[k]),
					MeanRating: round2(meanF(binMap[k])),
				})
			}

			// Cliff: the first bin (ascending) whose mean rating reaches within
			// 0.3 of the best bin's mean rating. That's the cheapest "still good"
			// tier — paying beyond it buys little extra quality. Only bins with a
			// meaningful sample (>=3 hotels) define "best" so a single luxury
			// outlier in a sparse high-price bin can't push the cliff upward.
			const minBinForBest = 3
			best := 0.0
			for _, b := range view.Bins {
				if b.Hotels >= minBinForBest && b.MeanRating > best {
					best = b.MeanRating
				}
			}
			if best == 0 { // all bins sparse; fall back to overall best
				for _, b := range view.Bins {
					if b.MeanRating > best {
						best = b.MeanRating
					}
				}
			}
			for _, b := range view.Bins {
				if b.Hotels >= minBinForBest && b.MeanRating >= best-0.3 {
					view.CliffPrice = b.High
					break
				}
			}

			// Recommend the best-value hotels at or below the cliff price.
			var below []hlHotel
			for _, h := range priced {
				if view.CliffPrice == 0 || h.Price <= view.CliffPrice {
					below = append(below, h)
				}
			}
			sortHotelsByValue(below)
			if len(below) > 5 {
				below = below[:5]
			}
			for _, h := range below {
				view.Recommended = append(view.Recommended, toHotelOut(h))
			}
			// No bin had a meaningful sample, so no cliff was identified and the
			// recommendations are just the overall best-value hotels. Say so, so
			// the list isn't mislabelled as cliff-constrained.
			if view.CliffPrice == 0 && len(view.Recommended) > 0 {
				view.Note = "data too sparse to identify a price cliff; showing overall best value"
			}
			return printPriceCliff(cmd.OutOrStdout(), flags, view)
		},
	}
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Only consider hotels at or above this rating")
	cmd.Flags().Float64Var(&binSize, "bin-size", 25, "Price bin width (base currency)")
	return cmd
}

func printPriceCliff(out io.Writer, flags *rootFlags, view priceCliffView) error {
	if !wantsHumanTable(out, flags) {
		return printJSONFiltered(out, view, flags)
	}
	fmt.Fprintf(out, "Price cliff — %s (bins of $%.0f)\n", view.Location, view.BinSize)
	fmt.Fprintln(out, strings.Repeat("-", 48))
	// No bins means there was nothing to chart (too few priced hotels); the note
	// is the whole story. With bins present the note is a qualifier printed below.
	if len(view.Bins) == 0 {
		if view.Note != "" {
			fmt.Fprintf(out, "%s\n", view.Note)
		}
		fmt.Fprintf(out, "%s\n", view.Disclaimer)
		return nil
	}
	for _, b := range view.Bins {
		bar := strings.Repeat("█", int(b.MeanRating))
		fmt.Fprintf(out, "  $%4.0f-%-4.0f  ⭐%.1f  (%d)  %s\n", b.Low, b.High, b.MeanRating, b.Hotels, bar)
	}
	if view.CliffPrice > 0 {
		fmt.Fprintf(out, "\nCliff: ~$%.0f/night — the cheapest tier that's still close to the best.\n", view.CliffPrice)
	} else if view.Note != "" {
		fmt.Fprintf(out, "\n%s\n", view.Note)
	}
	if len(view.Recommended) > 0 {
		if view.CliffPrice > 0 {
			fmt.Fprintln(out, "\nBest value at or below the cliff:")
		} else {
			fmt.Fprintln(out, "\nBest overall value:")
		}
		for i, h := range view.Recommended {
			fmt.Fprintf(out, "  %d. %-34s ⭐%.1f $%.0f\n", i+1, truncate(h.Name, 34), h.Rating, h.Price)
		}
	}
	fmt.Fprintf(out, "\n%s\n", view.Disclaimer)
	return nil
}
