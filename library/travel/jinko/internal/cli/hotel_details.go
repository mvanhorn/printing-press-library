package cli

import (
	"context"
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newHotelDetailsCmd() *cobra.Command {
	var hotelID, checkin, checkout string
	c := &cobra.Command{
		Use:   "hotel-details",
		Short: "Fetch rich metadata for a hotel — gallery, facilities, policies, room details.",
		RunE: withClient(func(ctx context.Context, c *client.Client, f output.Format) (any, error) {
			if hotelID == "" {
				return nil, &InputError{Message: "--hotel-id is required"}
			}
			path := "/api/v1/hotels/" + url.PathEscape(hotelID) + "/details"
			q := url.Values{}
			if checkin != "" {
				q.Set("checkin", checkin)
			}
			if checkout != "" {
				q.Set("checkout", checkout)
			}
			if encoded := q.Encode(); encoded != "" {
				path += "?" + encoded
			}
			var resp any
			if err := c.Get(ctx, path, &resp); err != nil {
				return nil, err
			}
			return resp, nil
		}),
	}
	c.Flags().StringVar(&hotelID, "hotel-id", "", "hotel ID (from a prior hotel-search result)")
	c.Flags().StringVar(&checkin, "checkin", "", "check-in date (YYYY-MM-DD)")
	c.Flags().StringVar(&checkout, "checkout", "", "check-out date (YYYY-MM-DD)")
	return c
}
