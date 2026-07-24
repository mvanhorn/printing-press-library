// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newNovelMoviePlanCmd(flags *rootFlags) *cobra.Command {
	var flagZipCode string
	var flagDate string
	var flagAfter string
	var flagBefore string

	cmd := &cobra.Command{
		Use:         "movie-plan",
		Short:       "Rank practical nearby screenings inside a date and time window and return purchase links.",
		Example:     "  fandango-pp-cli movie-plan --zip-code 10001 --date 2026-07-25 --after 18:00 --before 22:00 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank official Fandango showtimes")
				return nil
			}
			if flagZipCode == "" {
				return usageErr(fmt.Errorf("--zip-code is required"))
			}
			if flagDate == "" {
				flagDate = time.Now().Format("2006-01-02")
			}
			start, err := fandangoDateTime(flagDate, defaultString(flagAfter, "00:00"))
			if err != nil {
				return usageErr(fmt.Errorf("--date/--after must use YYYY-MM-DD and HH:MM"))
			}
			end, err := fandangoDateTime(flagDate, defaultString(flagBefore, "23:59"))
			if err != nil || end.Before(start) {
				return usageErr(fmt.Errorf("--before must use HH:MM and not precede --after"))
			}
			rows, err := fetchFandangoShowtimes(cmd, flags, map[string]string{
				"ZipCode": flagZipCode, "Radius": "25", "StartDisplayDate": flagDate,
				"EndDisplayDate": flagDate, "Limit": strconv.Itoa(100),
			})
			if err != nil {
				return err
			}
			rows = filterFandangoWindow(rows, start, end)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"zip_code": flagZipCode, "date": flagDate, "count": len(rows), "options": rows,
				"purchase": "Links open Fandango checkout; this CLI does not select seats or complete payment.",
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagZipCode, "zip-code", "", "Postal code for nearby showtimes")
	cmd.Flags().StringVar(&flagDate, "date", "", "Showtime date (YYYY-MM-DD; default today)")
	cmd.Flags().StringVar(&flagAfter, "after", "", "Earliest local start time (HH:MM)")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Latest local start time (HH:MM)")
	return cmd
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
