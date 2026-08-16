// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless) agent-native cancel: given just a reservation id,
// resolve the full cancellation payload from upcoming-bookings and POST it — so
// callers never hand-assemble the /common-booking/cancel body. Preview by
// default; --confirm performs the real (refunding) cancellation.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/client"
	"github.com/spf13/cobra"
)

// upcomingBooking is the subset of a /common-booking/upcoming-bookings record
// needed to build a cancellation. Field casing matches the API (note the
// capitalized IsBookingApprovalOn and reservable.KubeId).
type upcomingBooking struct {
	UUID                string `json:"uuid"`
	StartsAt            string `json:"startsAt"`
	EndsAt              string `json:"endsAt"`
	BookingSourceType   int    `json:"bookingSourceType"`
	IsBookingApprovalOn bool   `json:"IsBookingApprovalOn"`
	CreditOrder         struct {
		Price json.Number `json:"price"`
	} `json:"creditOrder"`
	Reservable struct {
		Name     string `json:"name"`
		UUID     string `json:"uuid"`   // reservableId (== inventoryUuid / WeWorkSpaceID)
		KubeID   string `json:"KubeId"` // spaceId (kubeSpaceId)
		Location struct {
			UUID        string `json:"uuid"` // locationId
			Name        string `json:"name"`
			AccountType int    `json:"accountType"` // bookingLocationType
		} `json:"location"`
	} `json:"reservable"`
	Order struct {
		GrandTotal struct {
			Amount   float64 `json:"amount"`
			Currency string  `json:"currency"`
		} `json:"grandTotal"`
	} `json:"order"`
}

type upcomingBookingsResp struct {
	WeWorkBookings []upcomingBooking `json:"WeWorkBookings"`
}

// fetchUpcomingBookings returns the caller's upcoming bookings.
func fetchUpcomingBookings(ctx context.Context, c *client.Client) ([]upcomingBooking, error) {
	raw, err := c.Get(ctx, "/common-booking/upcoming-bookings", nil)
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	var resp upcomingBookingsResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing bookings: %w", err)
	}
	return resp.WeWorkBookings, nil
}

// buildCancelPayload maps a booking record to the verified /common-booking/cancel
// body. bookingId/reservationId/bookingExternaluuid are all the reservation id.
func buildCancelPayload(b upcomingBooking, note string) map[string]any {
	credits := 0.0
	if b.CreditOrder.Price != "" {
		credits, _ = b.CreditOrder.Price.Float64()
	}
	return map[string]any{
		"bookingId":           b.UUID,
		"reservationId":       b.UUID,
		"bookingExternaluuid": b.UUID,
		"bookingType":         strconv.Itoa(b.BookingSourceType),
		"bookingLocationType": strconv.Itoa(b.Reservable.Location.AccountType),
		"locationId":          b.Reservable.Location.UUID,
		"spaceId":             b.Reservable.KubeID,
		"reservableId":        b.Reservable.UUID,
		"startTime":           b.StartsAt,
		"endTime":             b.EndsAt,
		"creditsUsed":         credits,
		"isBookingApprovalOn": b.IsBookingApprovalOn,
		"cancellationNote":    note,
		"mailParams":          map[string]any{},
	}
}

func newCancelCmd(flags *rootFlags) *cobra.Command {
	var flagReservationID, flagNote string
	var flagConfirm bool
	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel a desk booking by reservation id (headless; --confirm to cancel)",
		Long: "Cancels a booking given only its reservation id — the full cancellation payload is\n" +
			"resolved automatically from your upcoming bookings (see `bookings`). Prints a preview\n" +
			"by default; pass --confirm to perform the REAL cancellation (refunds your card; full\n" +
			"refund when cancelled outside the location's same-day window).",
		Example: strings.Trim(`
  wework-pp-cli cancel --reservation-id 11189637
  wework-pp-cli cancel --reservation-id 11189637 --confirm`, "\n"),
		Annotations: map[string]string{}, // mutating: not read-only
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cancel booking")
			}
			resID := strings.TrimSpace(flagReservationID)
			if resID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--reservation-id is required (list them with `bookings`)"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			bookings, err := fetchUpcomingBookings(ctx, c)
			if err != nil {
				return err
			}
			var found *upcomingBooking
			ids := make([]string, 0, len(bookings))
			for i := range bookings {
				ids = append(ids, bookings[i].UUID)
				if bookings[i].UUID == resID {
					found = &bookings[i]
				}
			}
			if found == nil {
				if len(ids) == 0 {
					return fmt.Errorf("no upcoming bookings found; nothing to cancel")
				}
				return fmt.Errorf("no upcoming booking with reservation id %q (have: %s)", resID, strings.Join(ids, ", "))
			}

			note := strings.TrimSpace(flagNote)
			if note == "" {
				note = "Cancelled via wework-pp-cli"
			}
			payload := buildCancelPayload(*found, note)
			w := cmd.OutOrStdout()

			if !flagConfirm {
				out := map[string]any{
					"preview": true, "reservationId": found.UUID, "location": found.Reservable.Location.Name,
					"desk": found.Reservable.Name, "start": found.StartsAt, "end": found.EndsAt,
					"refund": found.Order.GrandTotal.Amount, "currency": found.Order.GrandTotal.Currency,
					"payload": payload,
				}
				if flags.asJSON {
					return printJSONFiltered(w, out, flags)
				}
				fmt.Fprintf(w, "Preview: cancel reservation %s — %s (%s), %s to %s\n",
					found.UUID, found.Reservable.Location.Name, found.Reservable.Name, found.StartsAt, found.EndsAt)
				fmt.Fprintf(w, "  Expected refund: $%.0f to your saved card.\n", found.Order.GrandTotal.Amount)
				fmt.Fprintf(w, "\nThis is a preview. Re-run with --confirm to cancel for real.\n")
				return nil
			}

			raw, _, err := c.Post(ctx, "/common-booking/cancel", payload)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// The API returns bare `true` on success; an object body may carry errors.
			ok := true
			var boolResp bool
			if json.Unmarshal(raw, &boolResp) == nil {
				ok = boolResp
			}
			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{
					"cancelled": ok, "reservationId": found.UUID, "location": found.Reservable.Location.Name,
					"refund": found.Order.GrandTotal.Amount,
				}, flags)
			}
			if !ok {
				return fmt.Errorf("cancellation did not succeed for reservation %s: %s", found.UUID, string(raw))
			}
			fmt.Fprintf(w, "Cancelled reservation %s (%s). Refund of $%.0f issued to your saved card.\n",
				found.UUID, found.Reservable.Location.Name, found.Order.GrandTotal.Amount)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagReservationID, "reservation-id", "", "Reservation id to cancel (from `bookings`) (required)")
	cmd.Flags().StringVar(&flagNote, "note", "", "Cancellation note (default: \"Cancelled via wework-pp-cli\")")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Perform the REAL cancellation (refunds your card). Without it, preview only.")
	return cmd
}
