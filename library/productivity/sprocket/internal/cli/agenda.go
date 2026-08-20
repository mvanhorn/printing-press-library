// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelAgendaCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "agenda",
		Short: "Both kids' separate team schedules merged into one chronological agenda over an N-day window.",
		Long: "Show every event across all your players' teams for the next N days as one chronological agenda.\n\n" +
			"Use this command for the merged multi-child schedule over a date range. Do NOT use it for the " +
			"single next event ('next'), the fixed current week ('week'), or conflict detection alone ('conflicts').",
		Example:     "  sprocket-pp-cli agenda --days 14\n  sprocket-pp-cli agenda --days 30 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			if days < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			now := time.Now()
			events, err := fetchCalendar(ctx, c, now, now.AddDate(0, 0, days))
			if err != nil {
				return err
			}
			return renderEventList(cmd, flags, events, fmt.Sprintf("No events in the next %d days.", days))
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "number of days ahead to include")
	return cmd
}
