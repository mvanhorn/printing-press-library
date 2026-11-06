// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelAddCourtDaysCmd(flags *rootFlags) *cobra.Command {
	var back bool
	cmd := &cobra.Command{
		Use:   "add-court-days <start-date> <n>",
		Short: "Count N court days forward or backward from a date, skipping weekends and LASC holidays.",
		Long: "Use this command to count N court days forward or backward. Do NOT use it for calendar-day periods that roll forward to the next court day; use 'deadline' instead.\n\n" +
			"The start date is excluded (CCP 12). Pass --back to count backward, e.g. from a hearing date.\n" +
			"Not legal advice: confirm critical deadlines against the governing statute and the court.",
		Example: "  lacourt-holidays-pp-cli add-court-days 2027-03-01 16 --agent\n  lacourt-holidays-pp-cli add-court-days 2027-05-20 16 --back",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "start=2027-03-01;n=16",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "add-court-days")
			}
			if len(args) != 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("expected <start-date> <n>, got %d argument(s)", len(args)))
			}
			start, err := parseUserDate(args[0])
			if err != nil {
				return err
			}
			n, err := parseCount(args[1], back)
			if err != nil {
				return err
			}
			cal, err := newLASCCalendar(cmd, flags)
			if err != nil {
				return err
			}
			res, err := cal.AddCourtDays(start, n)
			if err != nil {
				return calendarErr(err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), res, flags, "description")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %+d court days = %s (%s)\n", res.Start, res.CourtDays, res.Result, res.ResultWeekday)
			for _, h := range res.SkippedHolidays {
				fmt.Fprintf(cmd.OutOrStdout(), "  skipped %s %s\n", h.Date, h.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&back, "back", false, "Count backward from the start date (e.g. court days before a hearing)")
	return cmd
}
