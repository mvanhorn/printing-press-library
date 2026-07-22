// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// calendar: render a month of reservation state for one resort. Hand-authored.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type calendarDayView struct {
	Date   string `json:"date"`
	Status string `json:"status"`
}

type calendarView struct {
	ResortID int               `json:"resort_id"`
	Month    string            `json:"month"`
	Days     []calendarDayView `json:"days"`
}

func classifyAvailabilityDate(passes []passAvailability, date string) string {
	hasAccess := false
	for _, p := range passes {
		if !p.HasAccess {
			continue
		}
		hasAccess = true
		if p.ReservationsAvailable <= 0 {
			continue
		}
		if p.MaxReservationDate != "" && date > p.MaxReservationDate {
			continue
		}
		full := false
		for _, d := range p.UnavailableDates {
			full = full || d == date
		}
		for _, d := range p.BlackoutDates {
			full = full || d == date
		}
		for _, d := range p.ClosedDates {
			full = full || d == date
		}
		if !full {
			return "open"
		}
	}
	if !hasAccess {
		return "no_access"
	}
	for _, p := range passes {
		if !p.HasAccess {
			continue
		}
		for _, d := range p.BlackoutDates {
			if d == date {
				return "blackout"
			}
		}
	}
	for _, p := range passes {
		if !p.HasAccess {
			continue
		}
		for _, d := range p.ClosedDates {
			if d == date {
				return "closed"
			}
		}
	}
	for _, p := range passes {
		if !p.HasAccess {
			continue
		}
		for _, d := range p.UnavailableDates {
			if d == date {
				return "full"
			}
		}
	}
	return "unavailable"
}

func newNovelCalendarCmd(flags *rootFlags) *cobra.Command {
	var flagMonth string

	cmd := &cobra.Command{
		Use:         "calendar <resort_id> --month YYYY-MM",
		Short:       "Month grid of open vs full vs blackout vs closed days for a reservable resort.",
		Example:     "  ikon-pp-cli calendar 14 --month 2026-01 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would check reservation calendar for resort %v in %s\n", args, flagMonth)
				return nil
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("resort_id is required"))
			}
			resortID, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid resort_id %q", args[0]))
			}
			if flagMonth == "" {
				return usageErr(fmt.Errorf("--month is required"))
			}
			from, to, err := monthBounds(flagMonth)
			if err != nil {
				return usageErr(err)
			}
			dates, err := enumerateDates(from, to)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			availability, err := fetchAvailability(ctx, c, resortID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := calendarView{ResortID: resortID, Month: flagMonth}
			for _, d := range dates {
				view.Days = append(view.Days, calendarDayView{Date: d, Status: classifyAvailabilityDate(availability, d)})
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagMonth, "month", "", "Month to render (YYYY-MM)")
	return cmd
}
