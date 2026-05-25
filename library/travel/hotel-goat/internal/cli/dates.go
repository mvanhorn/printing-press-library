// Copyright 2026 kothari-nikunj. Licensed under Apache-2.0. See LICENSE.

// dates <location> --from YYYY-MM-DD --to YYYY-MM-DD [--nights N]
// Sweeps every valid (checkin, checkout) pair across the date window and
// returns each pair's cheapest hotel. Counterpart to flight-goat's `dates`
// command — the date-flex traveler's workflow without a calendar UI.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hotel-goat/internal/parser"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDatesCmd(flags *rootFlags) *cobra.Command {
	var fromStr, toStr string
	var nights int
	var maxPairs int
	var opts hotelSearchOpts
	var hotelClassCSV string
	var topPerPair int

	cmd := &cobra.Command{
		Use:   "dates <location>",
		Short: "Find the cheapest hotel per (checkin, checkout) pair across a date window",
		Long: `Find the cheapest hotel per (checkin, checkout) pair across a date window.

For each valid date pair in [--from, --to] with stay length --nights,
runs a hotels query and returns the cheapest hotel for that pair. Helpful
for date-flex travelers who want to see "which week is cheapest".

Cost: one Google Hotels request per pair (rate-limited via the shared
parser limiter). Cap with --max-pairs to keep runtime bounded.`,
		Example: strings.Trim(`
  hotel-goat-pp-cli dates "Lisbon" --from 2026-08-01 --to 2026-08-14 --nights 3
  hotel-goat-pp-cli dates "Paris" --from 2026-07-01 --to 2026-07-31 --nights 2 --max-pairs 10 --max-price 250
  hotel-goat-pp-cli dates "Tokyo" --from 2026-09-01 --to 2026-09-15 --nights 4 --min-rating 4.0 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				location := ""
				if len(args) > 0 {
					location = args[0]
				}
				url := fmt.Sprintf("https://www.google.com/travel/search?q=hotels+in+%s&checkin=<each>&checkout=<each+%dd>",
					jsonStringEscape(location), nights)
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"meta":      map[string]any{"dry_run": true, "url_template": url},
					"plan":      "sweep (checkin, checkout) pairs across window",
					"max_pairs": maxPairs,
				}, flags)
			}
			if len(args) < 1 {
				return cmd.Help()
			}
			location := args[0]

			if fromStr == "" || toStr == "" {
				return fmt.Errorf("--from and --to are required (YYYY-MM-DD)")
			}
			from, err := time.Parse("2006-01-02", fromStr)
			if err != nil {
				return fmt.Errorf("invalid --from %q: %w", fromStr, err)
			}
			to, err := time.Parse("2006-01-02", toStr)
			if err != nil {
				return fmt.Errorf("invalid --to %q: %w", toStr, err)
			}
			if !to.After(from) {
				return fmt.Errorf("--to must be after --from")
			}
			if nights < 1 {
				return fmt.Errorf("--nights must be >= 1")
			}

			// Parse hotel-class CSV into the shared opts struct
			if hotelClassCSV != "" {
				for _, s := range strings.Split(hotelClassCSV, ",") {
					var c int
					if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &c); err == nil && c >= 1 && c <= 5 {
						opts.HotelClass = append(opts.HotelClass, c)
					}
				}
			}

			pairs := buildPairs(from, to, nights, maxPairs)
			if len(pairs) == 0 {
				return fmt.Errorf("no valid date pairs in window [%s..%s] with --nights=%d", fromStr, toStr, nights)
			}

			type pairResult struct {
				Checkin  string         `json:"checkin"`
				Checkout string         `json:"checkout"`
				Cheapest *parser.Hotel  `json:"cheapest,omitempty"`
				Top      []parser.Hotel `json:"top,omitempty"`
				Count    int            `json:"count"`
				Error    string         `json:"error,omitempty"`
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()

			results := make([]pairResult, 0, len(pairs))
			for _, p := range pairs {
				hotels, _, err := fetchAndParseHotels(ctx, location, p.checkin, p.checkout, opts)
				pr := pairResult{Checkin: p.checkin, Checkout: p.checkout, Count: len(hotels)}
				if err != nil {
					pr.Error = err.Error()
				} else {
					sort.SliceStable(hotels, func(i, j int) bool {
						pi, pj := hotels[i].PricePerNight, hotels[j].PricePerNight
						// Zero-priced hotels go to the end. The previous
						// comparator `pi < pj && pi > 0` violated strict
						// weak ordering — both directions returned false
						// when pi > 0 and pj == 0, so a zero-priced hotel
						// could land at top with sort.SliceStable.
						if pi <= 0 {
							return false
						}
						if pj <= 0 {
							return true
						}
						return pi < pj
					})
					if len(hotels) > 0 {
						h := hotels[0]
						pr.Cheapest = &h
					}
					n := topPerPair
					if n > len(hotels) {
						n = len(hotels)
					}
					pr.Top = hotels[:n]
				}
				results = append(results, pr)
			}

			// Sort pairs by cheapest (cheapest first; pairs with no result go last)
			sort.SliceStable(results, func(i, j int) bool {
				ai, aj := math_MaxFloat, math_MaxFloat
				if results[i].Cheapest != nil && results[i].Cheapest.PricePerNight > 0 {
					ai = results[i].Cheapest.PricePerNight
				}
				if results[j].Cheapest != nil && results[j].Cheapest.PricePerNight > 0 {
					aj = results[j].Cheapest.PricePerNight
				}
				return ai < aj
			})

			out := map[string]any{
				"meta": map[string]any{
					"location":       location,
					"from":           fromStr,
					"to":             toStr,
					"nights":         nights,
					"pairs":          len(pairs),
					"max_pairs":      maxPairs,
					"fetched_at":     time.Now().UTC().Format(time.RFC3339),
					"parser_version": parser.ParserVersion,
				},
				"results": results,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&fromStr, "from", "", "Start of date window (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&toStr, "to", "", "End of date window (YYYY-MM-DD, required)")
	cmd.Flags().IntVar(&nights, "nights", 1, "Length of each stay in nights")
	cmd.Flags().IntVar(&maxPairs, "max-pairs", 14, "Maximum number of (checkin, checkout) pairs to query (default 14, ~2 weeks)")
	cmd.Flags().IntVar(&topPerPair, "top", 1, "How many top hotels to include per pair")
	cmd.Flags().StringVar(&opts.Currency, "currency", "", "ISO 4217 currency code (USD, EUR, ...)")
	cmd.Flags().Float64Var(&opts.MinRating, "min-rating", 0, "Filter to hotels with rating >= this (0-5)")
	cmd.Flags().Float64Var(&opts.MaxPrice, "max-price", 0, "Filter to hotels with price/night <= this")
	cmd.Flags().Float64Var(&opts.MinPrice, "min-price", 0, "Filter to hotels with price/night >= this")
	cmd.Flags().StringVar(&hotelClassCSV, "hotel-class", "", "Filter to specific star ratings (CSV: 3,4,5)")
	cmd.Flags().StringSliceVar(&opts.Brand, "brand", nil, "Filter by brand prefix (CSV: Hyatt,Marriott,Hilton)")
	cmd.Flags().StringSliceVar(&opts.Amenities, "amenities", nil, "Filter by amenities (CSV)")
	cmd.Flags().IntVar(&opts.Limit, "limit", 5, "Max hotels to consider per pair (cheapest taken)")

	return cmd
}

const math_MaxFloat = 1.7976931348623157e+308

type checkinPair struct {
	checkin  string
	checkout string
}

// buildPairs generates (checkin, checkout) pairs starting on each day in
// [from, to-nights] inclusive, capped at maxPairs. checkin and checkout
// are stored as YYYY-MM-DD strings ready for the Google Hotels query.
func buildPairs(from, to time.Time, nights, maxPairs int) []checkinPair {
	out := make([]checkinPair, 0)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		ci := d
		co := d.AddDate(0, 0, nights)
		if co.After(to) {
			break
		}
		out = append(out, checkinPair{
			checkin:  ci.Format("2006-01-02"),
			checkout: co.Format("2006-01-02"),
		})
		if maxPairs > 0 && len(out) >= maxPairs {
			break
		}
	}
	return out
}

func jsonStringEscape(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}
