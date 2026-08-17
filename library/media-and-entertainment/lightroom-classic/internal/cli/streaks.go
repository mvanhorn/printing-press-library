// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: daily-shooting streak and gap report.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelStreaksCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagUntil string
	var flagDays bool

	cmd := &cobra.Command{
		Use:   "streaks",
		Short: "See your current and longest daily-shooting streaks and every missed day in a range.",
		Long: "Reports day-level shooting coverage: current streak, longest streak, and every gap day.\n" +
			"current_streak is always anchored to today, even when --until is historical.\n" +
			"Use 'project' for progress against a fixed-length target; use 'on-this-day' for cross-year same-date lookup.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli streaks --since 2026-01-01 --json
  lightroom-classic-pp-cli streaks --since 2026-07-01 --json --select gaps
  lightroom-classic-pp-cli streaks --since 2026-01-01 --days`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--since=2026-01-01"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute streaks from the local catalog")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			rep, err := cat.Streaks(ctx, flagSince, flagUntil, flagDays)
			if err != nil {
				return err
			}
			return emitLrcat(cmd, flags, rep, func(w io.Writer) {
				fmt.Fprintf(w, "%s → %s: shot on %d of %d days\n", rep.Since, rep.Until, rep.DaysWithShots, rep.TotalDays)
				fmt.Fprintf(w, "current streak: %d days   longest streak: %d days\n", rep.CurrentStreak, rep.LongestStreak)
				if len(rep.Gaps) > 0 {
					show := rep.Gaps
					if len(show) > 15 {
						fmt.Fprintf(w, "gaps (%d, last 15 shown): %s\n", len(rep.Gaps), strings.Join(show[len(show)-15:], ", "))
					} else {
						fmt.Fprintf(w, "gaps (%d): %s\n", len(show), strings.Join(show, ", "))
					}
				} else {
					fmt.Fprintln(w, "no gaps — every day covered")
				}
			})
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Range start YYYY-MM-DD (default: 365 days before --until)")
	cmd.Flags().StringVar(&flagUntil, "until", "", "Range end YYYY-MM-DD (default: today)")
	cmd.Flags().BoolVar(&flagDays, "days", false, "Include per-day photo and pick counts")
	return cmd
}
