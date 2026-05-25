package cmd

import (
	"context"
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newTripCmd() *cobra.Command {
	var (
		tripID         string
		tripItemToken  string
		removeItemID   string
		travelersJSON  string
		contactJSON    string
	)
	c := &cobra.Command{
		Use:   "trip",
		Short: "Build a multi-product trip: add flights + hotels, set travelers, get a trip_id.",
		Long: `One command for the whole cart lifecycle. Combine flags freely:

  # Create a new trip with a flight
  jinko trip --trip-item-token <flight_token>

  # Add a hotel to the same trip
  jinko trip --trip-id <id> --trip-item-token <hotel_token>

  # Swap one flight item for another in a single call
  jinko trip --trip-id <id> --remove-item-id <item_id> --trip-item-token <new_token>

  # Set travelers + contact on an existing trip
  jinko trip --trip-id <id> --travelers '[{"first_name":"Jane",...}]' --contact '{"email":"...","phone":"..."}'

Returns the full trip with items, totals, and lifecycle state. The same trip_id
accepts both flight and hotel trip_item_tokens — this is Jinko's multi-product
cart differentiator.`,
		RunE: withClient(func(ctx context.Context, c *client.Client, f output.Format) (any, error) {
			body := map[string]any{}
			if tripID != "" {
				body["trip_id"] = tripID
			}
			if removeItemID != "" {
				if tripID == "" {
					return nil, &InputError{Message: "--remove-item-id requires --trip-id"}
				}
				body["remove_item"] = map[string]any{"item_id": removeItemID}
			}
			if tripItemToken != "" {
				body["add_item"] = map[string]any{"trip_item_token": tripItemToken}
			}
			if travelersJSON != "" {
				var travelers []any
				if err := json.Unmarshal([]byte(travelersJSON), &travelers); err != nil {
					return nil, &InputError{Message: "--travelers must be a valid JSON array: " + err.Error()}
				}
				upsert := map[string]any{"travelers": travelers}
				if contactJSON != "" {
					var contact any
					if err := json.Unmarshal([]byte(contactJSON), &contact); err != nil {
						return nil, &InputError{Message: "--contact must be a valid JSON object: " + err.Error()}
					}
					upsert["contact"] = contact
				}
				body["upsert_travelers"] = upsert
			} else if contactJSON != "" {
				return nil, &InputError{Message: "--contact requires --travelers"}
			}

			var resp any
			if err := c.Post(ctx, "/api/v1/shop/sync", body, &resp); err != nil {
				return nil, err
			}
			return resp, nil
		}),
	}
	c.Flags().StringVar(&tripID, "trip-id", "", "existing trip ID (omit to create new)")
	c.Flags().StringVar(&tripItemToken, "trip-item-token", "", "add a flight or hotel item (from flight-search / hotel-search)")
	c.Flags().StringVar(&removeItemID, "remove-item-id", "", "remove an existing item by item_id (requires --trip-id)")
	c.Flags().StringVar(&travelersJSON, "travelers", "", "travelers JSON array")
	c.Flags().StringVar(&contactJSON, "contact", "", "contact JSON {email, phone}")
	return c
}
