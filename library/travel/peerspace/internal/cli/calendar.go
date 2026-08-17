// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Calendar availability (from listing click-around HAR 2026-07-16).

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newCalendarCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "calendar",
		Short:       "Booking calendar availability for a space/listing",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCalendarAvailabilityStartCmd(flags))
	cmd.AddCommand(newCalendarAvailabilityEndCmd(flags))
	cmd.AddCommand(newCalendarAvailabilityMonthCmd(flags))
	return cmd
}

func newCalendarAvailabilityMonthCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSpaceID string
		flagYear    int
		flagMonth   int
	)
	cmd := &cobra.Command{
		Use:   "availability-month",
		Short: "Month-level availability for a space (GET .../month).",
		Long: `Fetch which days in a month have availability for a space.

Path (message-host HAR): GET /v1/calendar/bookings/availability/space/{space_id}/month?month=&year=`,
		Example: `  peerspace-pp-cli calendar availability-month --space-id 68d458dba45ae0878156d4b6 --year 2026 --month 9`,
		Annotations: map[string]string{
			"pp:endpoint": "calendar.availability.month",
			"pp:method":   "GET",
			"pp:path":     "/v1/calendar/bookings/availability/space/{space_id}/month",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagSpaceID) == "" {
				return fmt.Errorf("--space-id is required")
			}
			if flagYear < 2000 || flagMonth < 1 || flagMonth > 12 {
				return fmt.Errorf("invalid --year/--month")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/v1/calendar/bookings/availability/space/%s/month", strings.TrimSpace(flagSpaceID))
			params := map[string]string{
				"year":  strconv.Itoa(flagYear),
				"month": strconv.Itoa(flagMonth),
			}
			data, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "calendar", false, path, params, listingAuthHeaders(c), "", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&flagSpaceID, "space-id", "", "Space id (listing parentSpaceId)")
	cmd.Flags().IntVar(&flagYear, "year", 0, "Year")
	cmd.Flags().IntVar(&flagMonth, "month", 0, "Month 1-12")
	_ = cmd.MarkFlagRequired("space-id")
	_ = cmd.MarkFlagRequired("year")
	_ = cmd.MarkFlagRequired("month")
	return cmd
}

func newCalendarAvailabilityStartCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSpaceID   string
		flagListingID string
		flagYear      int
		flagMonth     int
		flagDay       int
	)
	cmd := &cobra.Command{
		Use:   "availability-start",
		Short: "List available start times for a space on a given day (GET .../day/start).",
		Long: `Fetch start-slot availability for a listing/space on one calendar day.

Path (HAR): GET /v1/calendar/bookings/availability/space/{space_id}/day/start
Query: listing_id, year, month, day, is_min_duration_test_group=true

space_id is listing.parentSpaceId from venues get / hydrate.`,
		Example: `  peerspace-pp-cli calendar availability-start --space-id 68d458dba45ae0878156d4b6 --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 9 --day 15`,
		Annotations: map[string]string{
			"pp:endpoint": "calendar.availability.start",
			"pp:method":   "GET",
			"pp:path":     "/v1/calendar/bookings/availability/space/{space_id}/day/start",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCalendarFlags(flagSpaceID, flagListingID, flagYear, flagMonth, flagDay); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/v1/calendar/bookings/availability/space/%s/day/start", strings.TrimSpace(flagSpaceID))
			params := map[string]string{
				"listing_id":                 strings.TrimSpace(flagListingID),
				"year":                       strconv.Itoa(flagYear),
				"month":                      strconv.Itoa(flagMonth),
				"day":                        strconv.Itoa(flagDay),
				"is_min_duration_test_group": "true",
			}
			data, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "calendar", false, path, params, listingAuthHeaders(c), "", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	// Inline flags (not a shared helper) so verify-skill can see them via grep.
	cmd.Flags().StringVar(&flagSpaceID, "space-id", "", "Space id (listing parentSpaceId)")
	cmd.Flags().StringVar(&flagListingID, "listing-id", "", "Listing id")
	cmd.Flags().IntVar(&flagYear, "year", 0, "Year (e.g. 2026)")
	cmd.Flags().IntVar(&flagMonth, "month", 0, "Month 1-12")
	cmd.Flags().IntVar(&flagDay, "day", 0, "Day of month")
	_ = cmd.MarkFlagRequired("space-id")
	_ = cmd.MarkFlagRequired("listing-id")
	_ = cmd.MarkFlagRequired("year")
	_ = cmd.MarkFlagRequired("month")
	_ = cmd.MarkFlagRequired("day")
	return cmd
}

func newCalendarAvailabilityEndCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSpaceID   string
		flagListingID string
		flagYear      int
		flagMonth     int
		flagDay       int
		flagTimeIndex int
	)
	cmd := &cobra.Command{
		Use:   "availability-end",
		Short: "List available end times after a chosen start time_index (GET .../day/end).",
		Long: `Fetch end-slot availability for a listing/space after a start time_index.

Path (HAR): GET /v1/calendar/bookings/availability/space/{space_id}/day/end
Query: listing_id, year, month, day, time_index, is_min_duration_test_group=true`,
		Example: `  peerspace-pp-cli calendar availability-end --space-id 68d458dba45ae0878156d4b6 --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 9 --day 15 --time-index 22`,
		Annotations: map[string]string{
			"pp:endpoint": "calendar.availability.end",
			"pp:method":   "GET",
			"pp:path":     "/v1/calendar/bookings/availability/space/{space_id}/day/end",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := requireCalendarFlags(flagSpaceID, flagListingID, flagYear, flagMonth, flagDay); err != nil {
				return err
			}
			if flagTimeIndex < 0 {
				return fmt.Errorf("--time-index is required (from availability-start slots)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/v1/calendar/bookings/availability/space/%s/day/end", strings.TrimSpace(flagSpaceID))
			params := map[string]string{
				"listing_id":                 strings.TrimSpace(flagListingID),
				"year":                       strconv.Itoa(flagYear),
				"month":                      strconv.Itoa(flagMonth),
				"day":                        strconv.Itoa(flagDay),
				"time_index":                 strconv.Itoa(flagTimeIndex),
				"is_min_duration_test_group": "true",
			}
			data, _, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "calendar", false, path, params, listingAuthHeaders(c), "", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	// Inline flags (not a shared helper) so verify-skill can see them via grep.
	cmd.Flags().StringVar(&flagSpaceID, "space-id", "", "Space id (listing parentSpaceId)")
	cmd.Flags().StringVar(&flagListingID, "listing-id", "", "Listing id")
	cmd.Flags().IntVar(&flagYear, "year", 0, "Year (e.g. 2026)")
	cmd.Flags().IntVar(&flagMonth, "month", 0, "Month 1-12")
	cmd.Flags().IntVar(&flagDay, "day", 0, "Day of month")
	cmd.Flags().IntVar(&flagTimeIndex, "time-index", -1, "Start slot time_index from availability-start (required)")
	_ = cmd.MarkFlagRequired("space-id")
	_ = cmd.MarkFlagRequired("listing-id")
	_ = cmd.MarkFlagRequired("year")
	_ = cmd.MarkFlagRequired("month")
	_ = cmd.MarkFlagRequired("day")
	_ = cmd.MarkFlagRequired("time-index")
	return cmd
}

func requireCalendarFlags(spaceID, listingID string, year, month, day int) error {
	if strings.TrimSpace(spaceID) == "" {
		return fmt.Errorf("--space-id is required")
	}
	if strings.TrimSpace(listingID) == "" {
		return fmt.Errorf("--listing-id is required")
	}
	if year < 2000 || month < 1 || month > 12 || day < 1 || day > 31 {
		return fmt.Errorf("invalid --year/--month/--day")
	}
	return nil
}
