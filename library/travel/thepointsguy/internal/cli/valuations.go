// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: The Points Guy monthly points valuations, as structured data.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

func newNovelValuationsCmd(flags *rootFlags) *cobra.Command {
	var program string
	var vtype string
	var limit int

	cmd := &cobra.Command{
		Use:   "valuations",
		Short: "Look up The Points Guy's monthly points-and-miles valuations (cents per point)",
		Long: strings.TrimSpace(`
Look up The Points Guy's current cents-per-point valuations for airline, hotel,
and transferable-currency programs. Use --program to match one program or --type
to filter a category. Use the 'drift' subcommand to see month-over-month change.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli valuations
  thepointsguy-pp-cli valuations --type transferable
  thepointsguy-pp-cli valuations --program "Chase Ultimate Rewards" --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch The Points Guy valuations")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := &tpgClientCtx{client: newTPGClient(flags), ctx: ctx}
			byProg, month, err := currentValuations(cmd, flags, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var rows []tpg.Valuation
			if program != "" {
				v, cands, ok := resolveValuation(byProg, program)
				if !ok {
					if flags.asJSON || flags.agent {
						_ = emitJSON(cmd, flags, map[string]any{"error": "program not found", "program": program, "candidates": cands})
					}
					return notFoundErr(fmt.Errorf("no valuation for %q; did you mean one of: %s", program, strings.Join(cands, ", ")))
				}
				rows = []tpg.Valuation{v}
			} else {
				for _, v := range byProg {
					if vtype != "" && !strings.EqualFold(v.Type, vtype) {
						continue
					}
					rows = append(rows, v)
				}
				sort.Slice(rows, func(i, j int) bool {
					if rows[i].Type != rows[j].Type {
						return rows[i].Type < rows[j].Type
					}
					return rows[i].Program < rows[j].Program
				})
				if limit > 0 && len(rows) > limit {
					rows = rows[:limit]
				}
			}

			view := struct {
				Month      string          `json:"month"`
				Count      int             `json:"count"`
				Valuations []tpg.Valuation `json:"valuations"`
			}{Month: month, Count: len(rows), Valuations: rows}

			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, view)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "PROGRAM\tTYPE\tCENTS/POINT\n")
			for _, v := range rows {
				fmt.Fprintf(w, "%s\t%s\t%.2f\n", v.Program, v.Type, v.CentsPerPoint)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			if month != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nThe Points Guy valuations, %s\n", month)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&program, "program", "", "Match a single program by name (fuzzy), e.g. \"Chase Ultimate Rewards\"")
	cmd.Flags().StringVar(&vtype, "type", "", "Filter by category: airline | hotel | transferable | other")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum programs to return (0 = all)")
	cmd.AddCommand(newNovelValuationsDriftCmd(flags))
	return cmd
}
