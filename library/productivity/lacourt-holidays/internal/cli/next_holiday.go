// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/lacourt-holidays/internal/courtdays"
)

type nextHolidayView struct {
	courtdays.Holiday
	DaysUntil int `json:"days_until"`
}

func newNovelNextHolidayCmd(flags *rootFlags) *cobra.Command {
	var count int
	cmd := &cobra.Command{
		Use:   "next-holiday [from-date]",
		Short: "Show the next LASC court holiday and how many days away it is.",
		Long: "Shows the next Superior Court of Los Angeles County judicial holiday on or after today (or the given date), rolling into later published years.\n" +
			"Always returns a JSON array (one element by default); use --count to list several upcoming holidays.",
		Example: "  lacourt-holidays-pp-cli next-holiday --agent\n  lacourt-holidays-pp-cli next-holiday 2027-06-01 --count 3",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "next-holiday")
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("expected at most one date, got %d", len(args)))
			}
			if count < 1 {
				return usageErr(fmt.Errorf("--count must be at least 1"))
			}
			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}
			from, err := parseUserDate(arg)
			if err != nil {
				return err
			}
			cal, err := newLASCCalendar(cmd, flags)
			if err != nil {
				return err
			}
			hs, err := cal.NextHolidays(from, count)
			if err != nil {
				return calendarErr(err)
			}
			views := make([]nextHolidayView, 0, len(hs))
			for _, h := range hs {
				views = append(views, nextHolidayView{Holiday: h, DaysUntil: int(h.Time().Sub(from).Hours() / 24)})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), views, flags, "description")
			}
			if len(views) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No upcoming holidays in the years the court has published.")
				return nil
			}
			for _, v := range views {
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) %s, in %d days\n", v.Date, v.Day, v.Description, v.DaysUntil)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&count, "count", 1, "Number of upcoming holidays to list")
	return cmd
}
