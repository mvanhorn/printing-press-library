// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Novel feature: rider career timeline merging profile + career stats.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelCareerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "career <rider>",
		Short: "Merge a rider's profile with career stats into one timeline.",
		Long: "Resolves a current-season rider by name and combines their profile (number, team,\n" +
			"class, nationality) with career statistics (wins, podiums, poles, titles).",
		Example: strings.Trim(`
  motogp-pp-cli career "Marc Marquez" --agent
  motogp-pp-cli career bagnaia`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("need a rider name, e.g. career \"Marc Marquez\""))
			}
			query := strings.Join(args, " ")

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			rider, err := resolveRider(ctx, c, flags, query)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			stats, err := riderStats(ctx, c, flags, rider.LegacyID)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			out := struct {
				Rider   string         `json:"rider"`
				Number  int            `json:"number"`
				Class   string         `json:"class"`
				Country string         `json:"country"`
				Stats   map[string]any `json:"stats"`
			}{
				Rider:   rider.fullName(),
				Number:  rider.Step.Number,
				Class:   rider.Step.Category.Name,
				Country: rider.Country.Name,
				Stats:   stats,
			}

			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  #%d  %s  (%s)\n", out.Rider, out.Number, out.Class, out.Country)
			rows := [][]string{
				{"World titles", statNum(stats, "world_championship_wins")},
				{"GP victories", statNum(stats, "grand_prix_victories")},
				{"Podiums", statNum(stats, "podiums")},
				{"Poles", statNum(stats, "poles")},
				{"Sprint victories", statNum(stats, "sprint_victories")},
				{"Fastest laps", statNum(stats, "race_fastest_laps")},
			}
			return flags.printTable(cmd, []string{"STAT", "VALUE"}, rows)
		},
	}
	return cmd
}

func statNum(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return "-"
	}
	switch n := v.(type) {
	case float64:
		return fmt.Sprintf("%d", int(n))
	case string:
		if n == "" {
			return "-"
		}
		return n
	default:
		return fmt.Sprintf("%v", v)
	}
}
