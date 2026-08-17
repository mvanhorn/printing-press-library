// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/report"
)

type ratingDivergence struct {
	Name                string  `json:"name"`
	MarketValue         int64   `json:"marketValue"`
	MarketValueLabel    string  `json:"marketValueLabel"`
	EAOverall           int     `json:"eaOverall"`
	ValueRankPercentile float64 `json:"valueRankPercentile"`
	Divergence          float64 `json:"divergence"`
	Tag                 string  `json:"tag"`
}

type overUnderRatedResult struct {
	Team           string             `json:"team"`
	MarketHyped    []ratingDivergence `json:"market_hyped"`
	MarketBargains []ratingDivergence `json:"market_bargains"`
	Skipped        int                `json:"skipped"`
}

// pp:data-source live
func newNovelOverUnderRatedCmd(flags *rootFlags) *cobra.Command {
	var flagTeam string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "over-under-rated",
		Short:       "Flag players whose transfer-market value is far above or below their EA game rating.",
		Example:     "  soccer-goat-pp-cli over-under-rated --team benfica --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would rank over- and under-rated players for team %q (limit %d)\n", flagTeam, flagLimit)
				return nil
			}
			if strings.TrimSpace(flagTeam) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--team is required"))
			}
			if flagLimit < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be non-negative"))
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
			team, err := agg.ResolveTeam(ctx, strings.TrimSpace(flagTeam))
			if err != nil {
				return err
			}
			result := rankRatingDivergence(team, flagLimit)
			if novelMachineOutput(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return printRatingDivergence(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&flagTeam, "team", "", "Club name to analyze")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum players to show in each direction")
	return cmd
}

func rankRatingDivergence(team *report.TeamReport, limit int) overUnderRatedResult {
	result := overUnderRatedResult{
		Team:           team.ClubName,
		MarketHyped:    make([]ratingDivergence, 0),
		MarketBargains: make([]ratingDivergence, 0),
	}
	players := make([]report.PlayerReport, 0, len(team.Players))
	for _, player := range team.Players {
		if player.MarketValue <= 0 || player.EAOverall <= 0 {
			result.Skipped++
			continue
		}
		players = append(players, player)
	}
	sort.SliceStable(players, func(i, j int) bool {
		if players[i].MarketValue == players[j].MarketValue {
			return players[i].Name < players[j].Name
		}
		return players[i].MarketValue < players[j].MarketValue
	})

	entries := make([]ratingDivergence, 0, len(players))
	for start := 0; start < len(players); {
		end := start + 1
		for end < len(players) && players[end].MarketValue == players[start].MarketValue {
			end++
		}
		percentile := 99.0
		if len(players) > 1 {
			averageRank := float64(start+end-1) / 2
			percentile = 99 * averageRank / float64(len(players)-1)
		}
		for index := start; index < end; index++ {
			player := players[index]
			divergence := percentile - float64(player.EAOverall)
			tag := ""
			if divergence > 0 {
				tag = "market-hyped"
			} else if divergence < 0 {
				tag = "market-bargain"
			}
			entries = append(entries, ratingDivergence{
				Name:                player.Name,
				MarketValue:         player.MarketValue,
				MarketValueLabel:    player.MarketValueLabel,
				EAOverall:           player.EAOverall,
				ValueRankPercentile: percentile,
				Divergence:          divergence,
				Tag:                 tag,
			})
		}
		start = end
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := math.Abs(entries[i].Divergence), math.Abs(entries[j].Divergence)
		if left == right {
			return entries[i].Name < entries[j].Name
		}
		return left > right
	})
	for _, entry := range entries {
		if entry.Divergence > 0 && len(result.MarketHyped) < limit {
			result.MarketHyped = append(result.MarketHyped, entry)
		} else if entry.Divergence < 0 && len(result.MarketBargains) < limit {
			result.MarketBargains = append(result.MarketBargains, entry)
		}
	}
	return result
}

func printRatingDivergence(w io.Writer, result overUnderRatedResult) error {
	fmt.Fprintf(w, "%s — market/game rating divergence\n", result.Team)
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tVALUE\tRATING\tDIVERGENCE\tTAG")
	for _, entries := range [][]ratingDivergence{result.MarketHyped, result.MarketBargains} {
		for _, entry := range entries {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%+.1f\t%s\n", entry.Name, entry.MarketValueLabel, entry.EAOverall, entry.Divergence, entry.Tag)
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(w, "Skipped (missing value or rating): %d\n", result.Skipped)
	return nil
}
