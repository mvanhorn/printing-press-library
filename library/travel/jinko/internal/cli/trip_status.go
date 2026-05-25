package cli

import (
	"context"
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newTripStatusCmd() *cobra.Command {
	var tripID string
	c := &cobra.Command{
		Use:   "trip-status",
		Short: "Get full status of a trip — cart, quote, fulfillment, PNRs.",
		Long: `Poll a trip's lifecycle state.

After book, the trip moves through:
  pending → paid → fulfilling → fulfilled (PNRs issued)
                              ↘ failed    (booking rejected; refund issued)

Useful for end-to-end automations — TravelFusion flights may take up to 72h
to confirm; polling trip-status is how an agent knows when to send a confirmation
email or trigger downstream workflows.`,
		RunE: withClient(func(ctx context.Context, c *client.Client, f output.Format) (any, error) {
			if tripID == "" {
				return nil, &InputError{Message: "--trip-id is required"}
			}
			path := "/api/v1/shop/sync/" + url.PathEscape(tripID)
			var resp any
			if err := c.Get(ctx, path, &resp); err != nil {
				return nil, err
			}
			return resp, nil
		}),
	}
	c.Flags().StringVar(&tripID, "trip-id", "", "trip ID")
	return c
}
