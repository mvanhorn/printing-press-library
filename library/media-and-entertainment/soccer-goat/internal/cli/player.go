// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/report"
)

// pp:data-source live
func newNovelPlayerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "player <name>",
		Short:       "Type any player name and get market value, EA FC rating, potential, and key stats in one report.",
		Example:     "  soccer-goat-pp-cli player andreas schjelderup\n  soccer-goat-pp-cli player schjelderup --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			name := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would resolve player %q\n", name)
				return nil
			}
			if name == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("player name is required"))
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
			player, err := agg.ResolvePlayer(ctx, name)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if novelMachineOutput(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), player, flags)
			}
			return printPlayerReport(cmd.OutOrStdout(), player)
		},
	}
	return cmd
}

func novelMachineOutput(w io.Writer, flags *rootFlags) bool {
	return wantsMachineOutput(flags) || (!isTerminal(w) && !flags.csv && !flags.quiet && !flags.plain)
}

func printPlayerReport(w io.Writer, player *report.PlayerReport) error {
	fmt.Fprintf(w, "%s — %s (%s, age %d, %s)\n", player.Name, player.Club, player.Position, player.Age, player.Nationality)
	fmt.Fprintf(w, "Market value: %s\n", player.MarketValueLabel)
	fmt.Fprintf(w, "EA FC rating: %s   Potential: %s   (source: %s)\n",
		ratingLabel(player.EAOverall), ratingLabel(player.Potential), availableLabel(player.PotentialSource))
	fmt.Fprintf(w, "PAC %d SHO %d PAS %d DRI %d DEF %d PHY %d\n",
		player.Pace, player.Shooting, player.Passing, player.Dribbling, player.Defending, player.Physical)
	printESPNBlock(w, player)
	fmt.Fprintf(w, "Sources: transfermarkt %s, ea-fc %s, potential %s, espn %s\n",
		sourceStatusLabel(player.Sources["transfermarkt"]),
		sourceStatusLabel(player.Sources["ea-fc"]),
		sourceStatusLabel(player.Sources["potential"]),
		sourceStatusLabel(player.Sources["espn"]))
	return nil
}

// printESPNBlock renders the ESPN season line, per-competition splits, and
// recent-match form when enrichment populated them. Silent when ESPN is absent.
func printESPNBlock(w io.Writer, player *report.PlayerReport) {
	e := player.ESPN
	if e == nil {
		return
	}
	if e.Stats != nil {
		s := e.Stats
		fmt.Fprintf(w, "ESPN season: %dG %dA, %d shots (%d on target), %d starts, %d YC/%d RC\n",
			s.Goals, s.Assists, s.Shots, s.ShotsOnTarget, s.Starts, s.YellowCards, s.RedCards)
	}
	for _, split := range e.Splits {
		fmt.Fprintf(w, "  %s: %dG %dA, %d shots\n",
			split.DisplayName, split.Stats.Goals, split.Stats.Assists, split.Stats.Shots)
	}
	if len(e.RecentGames) > 0 {
		parts := make([]string, 0, len(e.RecentGames))
		for _, g := range e.RecentGames {
			atVs := g.AtVs
			if atVs == "" {
				atVs = "vs"
			}
			parts = append(parts, fmt.Sprintf("%s %s %s %s (%dG %dA)", atVs, g.Opponent, g.Result, g.Score, g.Goals, g.Assists))
		}
		fmt.Fprintf(w, "  Last %d: %s\n", len(parts), strings.Join(parts, "; "))
	}
}

func ratingLabel(value int) string {
	if value == 0 {
		return "unavailable"
	}
	return fmt.Sprintf("%d", value)
}

func tableRatingLabel(value int) string {
	if value == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", value)
}

func availableLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unavailable"
	}
	return value
}

func sourceStatusLabel(status report.SourceStatus) string {
	if status.OK {
		return "OK"
	}
	return availableLabel(status.Detail)
}
