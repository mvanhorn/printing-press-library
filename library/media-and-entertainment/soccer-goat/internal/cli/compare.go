// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/report"
)

type playerComparison struct {
	A *report.PlayerReport `json:"a"`
	B *report.PlayerReport `json:"b"`
}

// pp:data-source live
func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare <p1> <p2>",
		Short:       "Side-by-side value, rating, potential, and stats for two players.",
		Example:     "  soccer-goat-pp-cli compare mbappe haaland\n  soccer-goat-pp-cli compare mbappe haaland --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compare players %q and %q\n", argAt(args, 0), argAt(args, 1))
				return nil
			}
			if len(args) != 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare requires exactly two player names"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			agg := report.NewAggregator(c)
			if ps, closePS := openPotentialStore(cmd); ps != nil {
				defer closePS()
				agg.WithPotentialStore(ps)
			}
			ctx := cmd.Context()
			a, err := agg.ResolvePlayer(ctx, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			b, err := agg.ResolvePlayer(ctx, args[1])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			comparison := playerComparison{A: a, B: b}
			if novelMachineOutput(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), comparison, flags)
			}
			return printPlayerComparison(cmd.OutOrStdout(), comparison)
		},
	}
	return cmd
}

func argAt(args []string, index int) string {
	if index >= len(args) {
		return ""
	}
	return args[index]
}

func printPlayerComparison(w io.Writer, comparison playerComparison) error {
	tw := newTabWriter(w)
	fmt.Fprintf(tw, "METRIC\t%s\t%s\n", comparison.A.Name, comparison.B.Name)
	rows := [][3]string{
		{"Market value", comparison.A.MarketValueLabel, comparison.B.MarketValueLabel},
		{"EA rating", ratingLabel(comparison.A.EAOverall), ratingLabel(comparison.B.EAOverall)},
		{"Potential", ratingLabel(comparison.A.Potential), ratingLabel(comparison.B.Potential)},
		{"PAC", ratingLabel(comparison.A.Pace), ratingLabel(comparison.B.Pace)},
		{"SHO", ratingLabel(comparison.A.Shooting), ratingLabel(comparison.B.Shooting)},
		{"PAS", ratingLabel(comparison.A.Passing), ratingLabel(comparison.B.Passing)},
		{"DRI", ratingLabel(comparison.A.Dribbling), ratingLabel(comparison.B.Dribbling)},
		{"DEF", ratingLabel(comparison.A.Defending), ratingLabel(comparison.B.Defending)},
		{"PHY", ratingLabel(comparison.A.Physical), ratingLabel(comparison.B.Physical)},
		{"Club", comparison.A.Club, comparison.B.Club},
		{"Age", fmt.Sprintf("%d", comparison.A.Age), fmt.Sprintf("%d", comparison.B.Age)},
	}
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", row[0], row[1], row[2])
	}
	return tw.Flush()
}
