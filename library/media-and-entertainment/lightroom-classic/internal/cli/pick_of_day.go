// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: one image per day via the pick ladder.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelPickOfDayCmd(flags *rootFlags) *cobra.Command {
	var flagDate, flagSince, flagUntil string

	cmd := &cobra.Command{
		Use:   "pick-of-day",
		Short: "Get exactly one image per day — flag beats rating beats recency — with its file path resolved for publishing.",
		Long: "Applies a selection ladder (flagged pick, then highest rating, then most recently touched) and returns at most\n" +
			"one image per requested day, with the absolute file path resolved. 'photos' returns all matches; this picks one.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli pick-of-day --date 2026-07-12 --json
  lightroom-classic-pp-cli pick-of-day --since 2026-07-01 --until 2026-07-19 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--date=2026-07-12"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the day's pick from the local catalog")
				return nil
			}
			if flagDate == "" && (flagSince == "" || flagUntil == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--date, or both --since and --until, is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			if flagDate != "" {
				p, err := cat.PickOfDay(ctx, flagDate)
				if err != nil {
					return err
				}
				if p == nil {
					if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "no photos on %s; run streaks to see gap days\n", flagDate)
					return nil
				}
				return emitLrcat(cmd, flags, p, func(w io.Writer) { photoLine(w, *p) })
			}
			picks, err := cat.PickOfDayRange(ctx, flagSince, flagUntil)
			if err != nil {
				return err
			}
			return emitLrcat(cmd, flags, picks, func(w io.Writer) {
				for _, p := range picks {
					photoLine(w, p)
				}
				fmt.Fprintf(w, "%d picks (days without photos omitted)\n", len(picks))
			})
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Day to resolve (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Range start for one-pick-per-day mode")
	cmd.Flags().StringVar(&flagUntil, "until", "", "Range end for one-pick-per-day mode")
	return cmd
}
