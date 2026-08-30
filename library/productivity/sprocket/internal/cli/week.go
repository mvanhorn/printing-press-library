// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newNovelWeekCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "week",
		Short: "Every game and practice for the current week across all your players' teams, in one view.",
		Long: "Show this week's full schedule (Monday through Sunday) merged across all your players' teams.\n\n" +
			"Use this command for the current calendar week. Do NOT use it for an arbitrary day range " +
			"(use 'agenda --days') or just the next item (use 'next').",
		Example:     "  sprocket-pp-cli week\n  sprocket-pp-cli week --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start, end := weekBounds(time.Now())
			events, err := fetchCalendar(ctx, c, start, end)
			if err != nil {
				return err
			}
			return renderEventList(cmd, flags, events, "No events this week.")
		},
	}
	return cmd
}
