// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/report"
)

// pp:data-source live
func newNovelTeamCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "team <club>",
		Short:       "Type a club name and get the full squad with each player's market value and rating, plus squad totals.",
		Example:     "  soccer-goat-pp-cli team benfica\n  soccer-goat-pp-cli team benfica --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			club := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would resolve team %q\n", club)
				return nil
			}
			if club == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("club name is required"))
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
			team, err := agg.ResolveTeam(ctx, club)
			if err != nil {
				return err
			}
			if novelMachineOutput(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), team, flags)
			}
			return printTeamReport(cmd.OutOrStdout(), team)
		},
	}
	return cmd
}

func printTeamReport(w io.Writer, team *report.TeamReport) error {
	players := append([]report.PlayerReport(nil), team.Players...)
	sort.SliceStable(players, func(i, j int) bool {
		if players[i].MarketValue == players[j].MarketValue {
			return players[i].Name < players[j].Name
		}
		return players[i].MarketValue > players[j].MarketValue
	})

	fmt.Fprintf(w, "%s — squad value %s (%d players)\n", team.ClubName, team.SquadValueLabel, len(players))
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tVALUE\tPOS\tRATING\tPOTENTIAL")
	for _, player := range players {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", player.Name, player.MarketValueLabel, player.Position,
			tableRatingLabel(player.EAOverall), tableRatingLabel(player.Potential))
	}
	return tw.Flush()
}
