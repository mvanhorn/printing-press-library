package cmd

import (
	"context"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newFindDestinationCmd() *cobra.Command {
	var (
		from       []string
		date, ret  string
		cabin      string
		sort       string
		directOnly bool
		maxPrice   float64
		limit      int
	)
	c := &cobra.Command{
		Use:   "find-destination",
		Short: "Discover destinations from one or more origins (cheapest-from search).",
		RunE: withClient(func(ctx context.Context, c *client.Client, f output.Format) (any, error) {
			if len(from) == 0 {
				return nil, &InputError{Message: "--from is required (one or more IATA codes)"}
			}

			filters := map[string]any{
				"locations": map[string]any{"origins": from},
				"trip_type": tripType(ret),
			}
			if date != "" {
				dates := map[string]any{"departure_dates": []string{date}}
				if ret != "" {
					dates["return_dates"] = []string{ret}
				}
				filters["dates"] = dates
			} else {
				// BFF requires dates — default to next 12 months when not provided.
				today := time.Now().UTC().Truncate(24 * time.Hour)
				end := today.AddDate(1, 0, 0)
				filters["dates"] = map[string]any{
					"departure_date_ranges": []map[string]string{
						{"start": today.Format("2006-01-02"), "end": end.Format("2006-01-02")},
					},
				}
			}

			body := map[string]any{
				"passengers":              map[string]any{"adt": 1},
				"filters":                 filters,
				"flights_per_destination": 1,
				"sort":                    sort,
				"limit":                   limit,
			}
			if maxPrice > 0 {
				body["price_constraints"] = map[string]any{"max_total": maxPrice}
			}
			if directOnly {
				filters["direct_only"] = true
			}
			if cabin != "" {
				filters["cabin_class"] = cabin
			}

			var resp any
			if err := c.Post(ctx, "/api/v1/flights/destination-search", body, &resp); err != nil {
				return nil, err
			}
			return resp, nil
		}),
	}
	c.Flags().StringSliceVar(&from, "from", nil, "origin airport code(s) — repeat or comma-separate (e.g. PAR,CDG)")
	c.Flags().StringVar(&date, "date", "", "departure date (YYYY-MM-DD); omit to scan next 12 months")
	c.Flags().StringVar(&ret, "return", "", "return date (YYYY-MM-DD)")
	c.Flags().StringVar(&cabin, "cabin", "economy", "cabin class")
	c.Flags().StringVar(&sort, "sort", "lowest", "sort: lowest, recommendation")
	c.Flags().BoolVar(&directOnly, "direct-only", false, "only direct flights")
	c.Flags().Float64Var(&maxPrice, "max-price", 0, "maximum total price")
	c.Flags().IntVar(&limit, "limit", 20, "max destinations")
	return c
}
