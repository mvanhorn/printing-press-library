// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelDeadlineCmd(flags *rootFlags) *cobra.Command {
	var back bool
	cmd := &cobra.Command{
		Use:   "deadline <start-date> <n>",
		Short: "Add N calendar days and roll a weekend or holiday end date to the next court day.",
		Long: "Use this command for calendar-day periods whose last day rolls to the next court day. Do NOT use it to count court days; use 'add-court-days' instead.\n\n" +
			"Adds N calendar days to the start date. If the end date is a weekend or LASC holiday it moves to the next court day (CCP 12a pattern);\n" +
			"with --back it counts backward and moves to the previous court day. Not legal advice: confirm critical deadlines.",
		Example: "  lacourt-holidays-pp-cli deadline 2027-03-01 30 --agent\n  lacourt-holidays-pp-cli deadline 2027-06-15 20 --back",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "start=2027-03-01;n=30",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "deadline")
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
			res, err := cal.CalendarDeadline(start, n, back)
			if err != nil {
				return calendarErr(err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), res, flags, "reason")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %+d calendar days = %s\n", res.Start, res.CalendarDays, res.RawEnd)
			if res.Rolled {
				for _, s := range res.RolledPast {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s is not a court day (%s)\n", s.Date, s.Reason)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deadline: %s (%s)\n", res.Deadline, res.Weekday)
			return nil
		},
	}
	cmd.Flags().BoolVar(&back, "back", false, "Count backward and roll to the previous court day")
	return cmd
}
