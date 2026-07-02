// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: month-over-month drift in a program's valuation.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

// pp:data-source local
func newNovelValuationsDriftCmd(flags *rootFlags) *cobra.Command {
	var program string
	var months int

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Show how a program's cents-per-point value changed month over month",
		Long: strings.TrimSpace(`
Show the trend in a program's valuation across the months mirrored in your local
store. History accumulates each time you sync (or run a valuations command), so
run this over time to spot devaluations. Requires --program.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli valuations drift --program "Marriott Bonvoy"
  thepointsguy-pp-cli valuations drift --program "Chase Ultimate Rewards" --months 6 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show valuation drift")
				return nil
			}
			if program == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--program is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Opportunistically capture the current month so a first run has a
			// data point, then read the full local history.
			if flags.dataSource != "local" {
				c := &tpgClientCtx{client: newTPGClient(flags), ctx: ctx}
				_, _, _ = currentValuations(cmd, flags, c)
			}
			st, err := openTPGStore()
			if err != nil {
				return err
			}
			defer st.Close()
			raws, err := st.List(rtValuations, 100000)
			if err != nil {
				return err
			}

			target := normProgram(program)
			var series []tpg.Valuation
			var canonical string
			for _, r := range raws {
				var v tpg.Valuation
				if json.Unmarshal(r, &v) != nil {
					continue
				}
				key := normProgram(v.Program)
				if key == target || strings.Contains(key, target) || strings.Contains(target, key) {
					series = append(series, v)
					canonical = v.Program
				}
			}
			if len(series) == 0 {
				if flags.asJSON || flags.agent {
					return emitJSON(cmd, flags, map[string]any{"error": "no valuation history for program", "program": program})
				}
				return notFoundErr(fmt.Errorf("no valuation history for %q; run 'thepointsguy-pp-cli sync --resources valuations' now and again next month", program))
			}
			sort.Slice(series, func(i, j int) bool {
				return monthOrder(series[i].Month).Before(monthOrder(series[j].Month))
			})
			if months > 0 && len(series) > months {
				series = series[len(series)-months:]
			}

			type point struct {
				Month         string  `json:"month"`
				CentsPerPoint float64 `json:"cents_per_point"`
				DeltaFromPrev float64 `json:"delta_from_prev"`
			}
			points := make([]point, 0, len(series))
			var prev float64
			for i, v := range series {
				p := point{Month: v.Month, CentsPerPoint: v.CentsPerPoint}
				if i > 0 {
					p.DeltaFromPrev = round2(v.CentsPerPoint - prev)
				}
				prev = v.CentsPerPoint
				points = append(points, p)
			}
			view := struct {
				Program string  `json:"program"`
				Points  []point `json:"history"`
				Note    string  `json:"note,omitempty"`
			}{Program: canonical, Points: points}
			if len(points) == 1 {
				view.Note = "only one month is mirrored so far; sync again in future months to build a trend"
			}

			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, view)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "MONTH\tCENTS/POINT\tΔ\n")
			for _, p := range points {
				delta := ""
				if p.DeltaFromPrev != 0 {
					delta = fmt.Sprintf("%+.2f", p.DeltaFromPrev)
				}
				fmt.Fprintf(w, "%s\t%.2f\t%s\n", p.Month, p.CentsPerPoint, delta)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "\n%s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&program, "program", "", "Loyalty program name (fuzzy), e.g. \"Marriott Bonvoy\"")
	cmd.Flags().IntVar(&months, "months", 0, "Limit to the most recent N months (0 = all mirrored)")
	return cmd
}

// monthOrder parses a "January 2006" month string for chronological sorting.
func monthOrder(month string) time.Time {
	t, err := time.Parse("January 2006", strings.TrimSpace(month))
	if err != nil {
		return time.Time{}
	}
	return t
}
