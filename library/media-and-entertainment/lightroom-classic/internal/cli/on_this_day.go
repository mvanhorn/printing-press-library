// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-year calendar-date lookup.
package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelOnThisDayCmd(flags *rootFlags) *cobra.Command {
	var flagMonth, flagDay int

	cmd := &cobra.Command{
		Use:   "on-this-day",
		Short: "Everything shot on this calendar date across all years, grouped by year.",
		Long: "Calendar-position retrieval: photos captured on a fixed month/day in every year, newest year first,\n" +
			"with each year's best image resolved via the pick ladder. 'photos' date filters are range-based; this is calendar-based.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli on-this-day --json
  lightroom-classic-pp-cli on-this-day --month 7 --day 19 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--month=7;--day=12"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would look up this calendar date across all years")
				return nil
			}
			now := time.Now()
			if flagMonth == 0 {
				flagMonth = int(now.Month())
			}
			if flagDay == 0 {
				flagDay = now.Day()
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			years, err := cat.OnThisDay(ctx, flagMonth, flagDay)
			if err != nil {
				return err
			}
			return emitLrcat(cmd, flags, years, func(w io.Writer) {
				if len(years) == 0 {
					fmt.Fprintf(w, "nothing shot on %02d-%02d in any year\n", flagMonth, flagDay)
					return
				}
				fmt.Fprintf(w, "on %02d-%02d across %d years:\n", flagMonth, flagDay, len(years))
				for _, y := range years {
					fmt.Fprintf(w, "%s  %d photos\n", y.Year, y.Photos)
					if y.Best != nil {
						photoLine(w, *y.Best)
					}
				}
			})
		},
	}
	cmd.Flags().IntVar(&flagMonth, "month", 0, "Calendar month 1-12 (default: current month)")
	cmd.Flags().IntVar(&flagDay, "day", 0, "Calendar day 1-31 (default: today)")
	return cmd
}
