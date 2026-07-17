// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Novel feature: head-to-head career comparison of two riders.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source computed
func newNovelH2hCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "h2h <riderA> <riderB>",
		Short: "Compare two riders' career stats side by side.",
		Long: "Resolves two current-season riders by name and lays their career statistics\n" +
			"(titles, wins, podiums, poles) alongside each other. Quote multi-word names.",
		Example: strings.Trim(`
  motogp-pp-cli h2h "Marc Marquez" "Francesco Bagnaia" --agent
  motogp-pp-cli h2h marquez bagnaia`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("need two riders, e.g. h2h \"Marc Marquez\" \"Francesco Bagnaia\""))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// With exactly two args, treat each as one rider; with more, the
			// user passed unquoted multi-word names — require quoting.
			if len(args) != 2 {
				return usageErr(fmt.Errorf("expected exactly two riders; quote multi-word names, e.g. h2h \"Marc Marquez\" \"Francesco Bagnaia\""))
			}

			riderA, err := resolveRider(ctx, c, flags, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			riderB, err := resolveRider(ctx, c, flags, args[1])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			statsA, err := riderStats(ctx, c, flags, riderA.LegacyID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			statsB, err := riderStats(ctx, c, flags, riderB.LegacyID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			type side struct {
				Rider   string         `json:"rider"`
				Number  int            `json:"number"`
				Class   string         `json:"class"`
				Country string         `json:"country"`
				Stats   map[string]any `json:"stats"`
			}
			out := struct {
				A side `json:"a"`
				B side `json:"b"`
			}{
				A: side{riderA.fullName(), riderA.Step.Number, riderA.Step.Category.Name, riderA.Country.Name, statsA},
				B: side{riderB.fullName(), riderB.Step.Number, riderB.Step.Category.Name, riderB.Country.Name, statsB},
			}

			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			metrics := []struct{ label, key string }{
				{"World titles", "world_championship_wins"},
				{"GP victories", "grand_prix_victories"},
				{"Podiums", "podiums"},
				{"Poles", "poles"},
				{"Sprint victories", "sprint_victories"},
				{"Fastest laps", "race_fastest_laps"},
			}
			rows := make([][]string, 0, len(metrics))
			for _, m := range metrics {
				rows = append(rows, []string{m.label, statNum(statsA, m.key), statNum(statsB, m.key)})
			}
			return flags.printTable(cmd, []string{"STAT", out.A.Rider, out.B.Rider}, rows)
		},
	}
	return cmd
}
