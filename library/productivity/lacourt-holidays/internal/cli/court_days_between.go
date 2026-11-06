// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelCourtDaysBetweenCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "court-days-between <from-date> <to-date>",
		Short: "Count court days between two dates and list the holidays in the span.",
		Long: "Use this command to count court days between two known dates. Do NOT use it to find the date N court days away; use 'add-court-days' instead.\n\n" +
			"Counts court days after the from date through the to date (from excluded, to included), matching add-court-days.",
		Example: "  lacourt-holidays-pp-cli court-days-between 2027-03-01 2027-04-15 --agent\n  lacourt-holidays-pp-cli court-days-between today 2027-01-15",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "from=2027-03-01;to=2027-04-15",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "court-days-between")
			}
			if len(args) != 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("expected <from-date> <to-date>, got %d argument(s)", len(args)))
			}
			from, err := parseUserDate(args[0])
			if err != nil {
				return err
			}
			to, err := parseUserDate(args[1])
			if err != nil {
				return err
			}
			cal, err := newLASCCalendar(cmd, flags)
			if err != nil {
				return err
			}
			res, err := cal.CourtDaysBetween(from, to)
			if err != nil {
				return calendarErr(err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), res, flags, "description")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d court days from %s to %s (%s)\n", res.CourtDays, res.From, res.To, res.Counting)
			for _, h := range res.Holidays {
				fmt.Fprintf(cmd.OutOrStdout(), "  holiday %s %s\n", h.Date, h.Description)
			}
			return nil
		},
	}
	return cmd
}
