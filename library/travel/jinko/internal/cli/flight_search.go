package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newFlightSearchCmd() *cobra.Command {
	var (
		from, to, date, ret           string
		originType, destinationType   string
		passengers                    int
		cabin                         string
		directOnly                    bool
		maxPrice                      float64
		includeCarriers, excludeCarrs []string
		offerToken                    string
	)
	c := &cobra.Command{
		Use:   "flight-search",
		Short: "Live flight search or price-check a specific offer. Returns trip_item_token for booking.",
		Long: `Two modes:
  Search mode  — pass --from, --to, --date (+ optional filters) for live prices.
  Price-check  — pass --offer-token (from a prior find-flight result) to refresh
                 a single offer with live availability and a bookable trip_item_token.

Use the returned trip_item_token with "jinko trip --trip-item-token ..." to add
the flight to a multi-product trip (flight + hotel) before "jinko book".`,
		RunE: withClient(func(ctx context.Context, c *client.Client, f output.Format) (any, error) {
			if offerToken != "" {
				body := map[string]any{
					"offer_token": offerToken,
					"passengers":  map[string]any{"adults": passengers},
				}
				var resp any
				if err := c.Post(ctx, "/api/v1/flights/shop", body, &resp); err != nil {
					return nil, err
				}
				return resp, nil
			}

			if from == "" || to == "" || date == "" {
				return nil, &InputError{
					Message: "search mode requires --from, --to, and --date (or pass --offer-token for price-check)",
				}
			}
			body := map[string]any{
				"origin":           from,
				"origin_type":      originType,
				"destination":      to,
				"destination_type": destinationType,
				"departure_date":   date,
				"trip_type":        tripType(ret),
				"cabin_class":      cabin,
				"direct_only":      directOnly,
				"passengers":       map[string]any{"adults": passengers},
			}
			if ret != "" {
				body["return_date"] = ret
			}
			if maxPrice > 0 {
				body["max_price"] = maxPrice
			}
			if len(includeCarriers) > 0 {
				body["include_carriers"] = includeCarriers
			}
			if len(excludeCarrs) > 0 {
				body["exclude_carriers"] = excludeCarrs
			}

			var resp any
			if err := c.Post(ctx, "/api/v1/flights/shop", body, &resp); err != nil {
				return nil, err
			}
			return resp, nil
		}),
	}
	c.Flags().StringVar(&from, "from", "", "origin IATA code (city or airport)")
	c.Flags().StringVar(&originType, "origin-type", "city", "origin code type: city or airport")
	c.Flags().StringVar(&to, "to", "", "destination IATA code")
	c.Flags().StringVar(&destinationType, "destination-type", "city", "destination code type: city or airport")
	c.Flags().StringVar(&date, "date", "", "departure date (YYYY-MM-DD)")
	c.Flags().StringVar(&ret, "return", "", "return date (YYYY-MM-DD)")
	c.Flags().IntVar(&passengers, "passengers", 1, "number of adult passengers")
	c.Flags().StringVar(&cabin, "cabin", "economy", "cabin class")
	c.Flags().BoolVar(&directOnly, "direct-only", false, "only direct flights")
	c.Flags().Float64Var(&maxPrice, "max-price", 0, "maximum price")
	c.Flags().StringSliceVar(&includeCarriers, "include-carriers", nil, "include only these IATA 2-letter codes")
	c.Flags().StringSliceVar(&excludeCarrs, "exclude-carriers", nil, "exclude these IATA 2-letter codes")
	c.Flags().StringVar(&offerToken, "offer-token", "", "price-check a specific offer (from find-flight result)")
	return c
}
