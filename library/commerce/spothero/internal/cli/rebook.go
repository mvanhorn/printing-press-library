// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
)

func newNovelRebookCmd(flags *rootFlags) *cobra.Command {
	var in bookInputs
	cmd := &cobra.Command{
		Use:   "rebook [reservation-id]",
		Short: "Re-quote a past reservation's garage for a new window and compare with what you paid.",
		Long: "Use this command to re-quote one past reservation's garage for one new window. With no id it uses your most recent reservation that has already started. It never places anything.\n" +
			"Do NOT use this command for a recurring series; use 'commute' instead.",
		Example:     "  spothero-pp-cli rebook --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --agent",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--starts=2027-01-15T09:00;--ends=2027-01-15T12:00"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rebook")
			}
			if in.starts == "" || in.ends == "" {
				return usageErr(fmt.Errorf("--starts and --ends are required"))
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			var past consumer.Reservation
			if len(args) == 1 {
				if past, err = c.GetReservation(args[0]); err != nil {
					return mapConsumerErr(err)
				}
			} else {
				// "Book my last garage again" is the common ask, and it should not
				// require looking up an id first.
				latest, err := c.ListReservationsPage(1)
				if err != nil {
					return mapConsumerErr(err)
				}
				found, ok := latestPastReservation(latest, time.Now())
				if !ok {
					return flags.printJSON(cmd, map[string]any{"quote": nil, "message": "No past reservations on the latest page to rebook. Pass a reservation id."})
				}
				past = found
			}
			if past.FacilityID == 0 {
				return apiErr(fmt.Errorf("reservation %s has no facility id", past.ID))
			}
			in.facilityID = past.FacilityID
			preview, err := in.quote(c)
			if err != nil {
				return err
			}
			preview.Message = "Quote only, nothing placed."
			return flags.printJSON(cmd, map[string]any{
				"quote": preview, "last_paid_cents": past.PriceCents, "difference_cents": preview.PriceCents - past.PriceCents,
				"next": fmt.Sprintf("spothero-pp-cli book preview --facility-id %d --starts %s --ends %s", past.FacilityID, in.starts, in.ends),
			})
		},
	}
	cmd.Flags().StringVar(&in.starts, "starts", "", "New parking start (local or RFC3339)")
	cmd.Flags().StringVar(&in.ends, "ends", "", "New parking end (local or RFC3339)")
	cmd.Flags().StringVar(&in.citySlug, "city-slug", "", "Optional. Time zone fallback for naive datetimes when the facility publishes none")
	return cmd
}

// latestPastReservation picks the first row that has started. The list runs newest
// start first and includes future bookings, so taking row zero would compare the new
// quote against a trip that has not happened yet.
func latestPastReservation(list []consumer.Reservation, now time.Time) (consumer.Reservation, bool) {
	for _, r := range list {
		if start, err := time.Parse(time.RFC3339, r.Starts); err == nil && start.Before(now) {
			return r, true
		}
	}
	return consumer.Reservation{}, false
}
