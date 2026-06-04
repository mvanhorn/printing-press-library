// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
	"github.com/spf13/cobra"
)

func newTripsExportCmd(flags *rootFlags) *cobra.Command {
	var state, since, format string
	cmd := &cobra.Command{
		Use:         "export",
		Short:       "Export trips as CSV or JSON",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]booking.Trip, 0))
			}
			if format == "" {
				format = "csv"
			}
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("trips export: %w", err)
			}
			// secure.booking.com hosts authenticated trip data; www.booking.com returns 404.
			data, err := c.Get("https://secure.booking.com/mytrips.html", nil)
			if err != nil {
				return fmt.Errorf("trips export: %w", err)
			}
			parsed, err := booking.ParseTrips(data)
			if err != nil {
				return fmt.Errorf("trips export: %w", err)
			}
			trips := make([]booking.Trip, 0)
			if err := json.Unmarshal(parsed, &trips); err != nil {
				return fmt.Errorf("trips export: %w", err)
			}
			cutoff, _ := time.Parse(dateOnly, since)
			out := make([]booking.Trip, 0)
			for _, t := range trips {
				if state != "" && state != "all" && !strings.EqualFold(t.State, state) {
					continue
				}
				when, _ := time.Parse(dateOnly, firstNonEmptyString(t.BookedOn, t.Checkin))
				if !cutoff.IsZero() && !when.IsZero() && when.Before(cutoff) {
					continue
				}
				out = append(out, t)
			}
			if flags.asJSON || format == "json" {
				return flags.printJSON(cmd, out)
			}
			w := csv.NewWriter(cmd.OutOrStdout())
			_ = w.Write([]string{"confirmation_number", "property_name", "property_slug", "country", "city", "checkin", "checkout", "nights", "total_price", "currency", "booked_on"})
			for _, t := range out {
				_ = w.Write([]string{t.ConfirmationNumber, t.PropertyName, t.PropertySlug, t.Country, t.City, t.Checkin, t.Checkout, strconv.Itoa(t.Nights), strconv.FormatFloat(t.TotalPrice, 'f', -1, 64), t.Currency, t.BookedOn})
			}
			w.Flush()
			return w.Error()
		},
	}
	cmd.Flags().StringVar(&state, "state", "past", "Trip state: past, upcoming, cancelled, all")
	cmd.Flags().StringVar(&since, "since", "", "Earliest booked/check-in date YYYY-MM-DD")
	cmd.Flags().StringVar(&format, "format", "csv", "Output format: csv or json")
	return cmd
}
