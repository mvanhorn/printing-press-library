// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelIsCourtDayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "is-court-day [date]",
		Short: "Answer yes/no for whether a date is an LASC court day, with the reason.",
		Long: "Checks a date against weekends and the Superior Court of Los Angeles County's published judicial holidays.\n" +
			"With no date, checks today. Dates in years the court has not published exit 3 instead of guessing.",
		Example: "  lacourt-holidays-pp-cli is-court-day 2027-11-26 --agent\n  lacourt-holidays-pp-cli is-court-day 11/26/2027\n  lacourt-holidays-pp-cli is-court-day",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "date=2027-11-26",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "is-court-day")
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("expected at most one date, got %d", len(args)))
			}
			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}
			day, err := parseUserDate(arg)
			if err != nil {
				return err
			}
			cal, err := newLASCCalendar(cmd, flags)
			if err != nil {
				return err
			}
			st, err := cal.Check(day)
			if err != nil {
				return calendarErr(err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), st, flags)
			}
			answer := "yes"
			if !st.CourtDay {
				answer = "no"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s, %s\n", st.Date, st.Weekday, answer, st.Reason)
			return nil
		},
	}
	return cmd
}
