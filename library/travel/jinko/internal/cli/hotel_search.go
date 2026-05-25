package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newHotelSearchCmd() *cobra.Command {
	var (
		query, city, country     string
		lat, lng                 float64
		radius                   float64
		placeID                  string
		hotelIDs                 []string
		checkin, checkout        string
		adults                   int
		children                 []int
		rooms                    int
		currency, nationality    string
		minRating                float64
		stars                    int
		minReviews, maxResults   int
	)
	c := &cobra.Command{
		Use:   "hotel-search",
		Short: "Search live hotel inventory and rates. Returns trip_item_token per bookable room.",
		Long: `Live hotel search across Jinko's supplier network. Returns a list of hotels
with bookable rates — each rate includes a trip_item_token you can pass to
"jinko trip --trip-item-token" to add it to a multi-product trip alongside flights.

Provide ONE destination signal:
  --query <text>        free-text destination (city, region, POI)
  --city <name>         city name
  --lat + --lng         geo-radius search (use --radius to widen)
  --place-id <id>       Google Places ID
  --hotel-ids <ids>     re-shop specific hotel IDs`,
		RunE: withClient(func(ctx context.Context, c *client.Client, f output.Format) (any, error) {
			if checkin == "" || checkout == "" {
				return nil, &InputError{Message: "--checkin and --checkout are required (YYYY-MM-DD)"}
			}
			hasDest := query != "" || city != "" || lat != 0 || placeID != "" || len(hotelIDs) > 0
			if !hasDest {
				return nil, &InputError{
					Message: "provide a destination: --query, --city, --lat+--lng, --place-id, or --hotel-ids",
				}
			}

			body := map[string]any{
				"checkin":  checkin,
				"checkout": checkout,
				"adults":   adults,
			}
			if query != "" {
				body["query"] = query
			}
			if city != "" {
				body["city_name"] = city
			}
			if country != "" {
				body["country_code"] = country
			}
			if lat != 0 {
				body["latitude"] = lat
			}
			if lng != 0 {
				body["longitude"] = lng
			}
			if radius != 0 {
				body["radius_km"] = radius
			}
			if placeID != "" {
				body["place_id"] = placeID
			}
			if len(hotelIDs) > 0 {
				body["hotel_ids"] = hotelIDs
			}
			if len(children) > 0 {
				body["children"] = children
			}
			if rooms > 0 {
				body["rooms"] = rooms
			}
			if currency != "" {
				body["currency"] = currency
			}
			if nationality != "" {
				body["guest_nationality"] = nationality
			}
			if minRating > 0 {
				body["min_rating"] = minRating
			}
			if stars > 0 {
				body["star_rating"] = stars
			}
			if minReviews > 0 {
				body["min_reviews"] = minReviews
			}
			if maxResults > 0 {
				body["max_results"] = maxResults
			}

			var resp any
			if err := c.Post(ctx, "/api/v1/hotels/shop", body, &resp); err != nil {
				return nil, err
			}
			return resp, nil
		}),
	}
	c.Flags().StringVar(&query, "query", "", "free-text destination")
	c.Flags().StringVar(&city, "city", "", "city name")
	c.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 country code")
	c.Flags().Float64Var(&lat, "lat", 0, "latitude (geo-radius search)")
	c.Flags().Float64Var(&lng, "lng", 0, "longitude (geo-radius search)")
	c.Flags().Float64Var(&radius, "radius", 0, "search radius in km (default server-side: 5)")
	c.Flags().StringVar(&placeID, "place-id", "", "Google Places ID")
	c.Flags().StringSliceVar(&hotelIDs, "hotel-ids", nil, "re-shop specific hotel IDs (repeat or comma-separate)")
	c.Flags().StringVar(&checkin, "checkin", "", "check-in date (YYYY-MM-DD)")
	c.Flags().StringVar(&checkout, "checkout", "", "check-out date (YYYY-MM-DD)")
	c.Flags().IntVar(&adults, "adults", 2, "number of adult guests")
	c.Flags().IntSliceVar(&children, "children", nil, "children ages (e.g. --children 5,7)")
	c.Flags().IntVar(&rooms, "rooms", 0, "number of rooms")
	c.Flags().StringVar(&currency, "currency", "", "ISO 4217 currency code (default USD)")
	c.Flags().StringVar(&nationality, "nationality", "", "guest nationality (ISO 3166-1 alpha-2)")
	c.Flags().Float64Var(&minRating, "min-rating", 0, "minimum guest review rating (0-10)")
	c.Flags().IntVar(&stars, "stars", 0, "minimum star rating (1-5)")
	c.Flags().IntVar(&minReviews, "min-reviews", 0, "minimum review count")
	c.Flags().IntVar(&maxResults, "max-results", 0, "maximum hotels to return (default 50)")
	return c
}
