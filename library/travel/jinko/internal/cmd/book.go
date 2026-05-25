package cmd

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newBookCmd() *cobra.Command {
	var tripID string
	c := &cobra.Command{
		Use:   "book",
		Short: "Schedule checkout for a trip — returns a Stripe payment URL + ancillaries.",
		Long: `Schedule checkout for an entire multi-product trip in one shot.

The response includes:
  - checkout_url : Jinko-hosted Stripe page; user pays for the full cart there
  - items        : per-item summary with item_id (use for select-ancillaries)
  - ancillaries  : bags / seats / meals offered per item (optional to preselect)
  - total        : grand total across all flights + hotels

Quote is automatic — handled inside book; you don't call a separate quote step.`,
		RunE: withClient(func(ctx context.Context, c *client.Client, f output.Format) (any, error) {
			if tripID == "" {
				return nil, &InputError{Message: "--trip-id is required"}
			}
			var resp any
			if err := c.Post(ctx, "/api/v1/shop/sync/checkout", map[string]any{"trip_id": tripID}, &resp); err != nil {
				return nil, err
			}
			return resp, nil
		}),
	}
	c.Flags().StringVar(&tripID, "trip-id", "", "trip ID (from trip command)")
	return c
}
