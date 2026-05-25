package cmd

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newFindFlightCmd() *cobra.Command {
	var (
		from, to, date, ret, cabin, sort string
		directOnly                       bool
		maxPrice                         float64
		limit                            int
	)
	c := &cobra.Command{
		Use:   "find-flight",
		Short: "Cached flight search by route + date. Returns offer_tokens for live price-check.",
		Long: `Cached flight search. Returns offer_tokens you can pass to flight-search
for a live (bookable) price + trip_item_token.

For end-to-end booking, prefer flight-search directly — it returns live
prices and a bookable trip_item_token in one step.`,
		RunE: withClient(func(ctx context.Context, c *client.Client, f output.Format) (any, error) {
			if from == "" || to == "" || date == "" {
				return nil, &InputError{Message: "--from, --to, and --date are required"}
			}
			filters := map[string]any{
				"locations": map[string]any{
					"origins":      []string{from},
					"destinations": []string{to},
				},
				"dates": map[string]any{
					"departure_dates": []string{date},
				},
				"trip_type":   tripType(ret),
				"cabin_class": cabin,
				"direct_only": directOnly,
			}
			if ret != "" {
				filters["dates"].(map[string]any)["return_dates"] = []string{ret}
			}

			body := map[string]any{
				"passengers": map[string]any{"adt": 1},
				"filters":    filters,
				"sort":       sort,
				"limit":      limit,
			}
			if maxPrice > 0 {
				body["price_constraints"] = map[string]any{"max_total": maxPrice}
			}

			var resp any
			if err := c.Post(ctx, "/api/v1/flights/search", body, &resp); err != nil {
				return nil, err
			}
			return resp, nil
		}),
	}
	c.Flags().StringVar(&from, "from", "", "origin airport code (e.g. PAR)")
	c.Flags().StringVar(&to, "to", "", "destination airport code (e.g. NYC)")
	c.Flags().StringVar(&date, "date", "", "departure date (YYYY-MM-DD)")
	c.Flags().StringVar(&ret, "return", "", "return date for round-trip (YYYY-MM-DD)")
	c.Flags().StringVar(&cabin, "cabin", "economy", "cabin class: economy, premium_economy, business, first")
	c.Flags().StringVar(&sort, "sort", "lowest", "sort: lowest, recommendation")
	c.Flags().BoolVar(&directOnly, "direct-only", false, "only show direct flights")
	c.Flags().Float64Var(&maxPrice, "max-price", 0, "maximum total price filter")
	c.Flags().IntVar(&limit, "limit", 10, "max results")
	return c
}

func tripType(returnDate string) string {
	if returnDate == "" {
		return "oneway"
	}
	return "roundtrip"
}
