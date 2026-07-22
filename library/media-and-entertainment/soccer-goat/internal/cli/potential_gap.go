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

const noPotentialDataNote = "no potential data available (sofifa/fifacm behind Cloudflare; set SOCCER_GOAT_FIFACM_COOKIE)"

type potentialGap struct {
	Name      string `json:"name"`
	EAOverall int    `json:"eaOverall"`
	Potential int    `json:"potential"`
	Gap       int    `json:"gap"`
}

type potentialGapResult struct {
	Team string         `json:"team"`
	Gaps []potentialGap `json:"gaps"`
	Note string         `json:"note"`
}

// pp:data-source live
func newNovelPotentialGapCmd(flags *rootFlags) *cobra.Command {
	var flagTeam string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "potential-gap",
		Short:       "Rank players by headroom (potential minus current rating).",
		Example:     "  soccer-goat-pp-cli potential-gap --team benfica --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would rank potential gaps for team %q (limit %d)\n", flagTeam, flagLimit)
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
			result := rankPotentialGaps(team, flagLimit)
			if novelMachineOutput(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return printPotentialGaps(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&flagTeam, "team", "", "Club name to analyze")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum players to show")
	return cmd
}

func rankPotentialGaps(team *report.TeamReport, limit int) potentialGapResult {
	result := potentialGapResult{Team: team.ClubName, Gaps: make([]potentialGap, 0)}
	hasPotential := false
	for _, player := range team.Players {
		if player.Potential > 0 {
			hasPotential = true
		}
		if player.Potential > 0 && player.EAOverall > 0 {
			result.Gaps = append(result.Gaps, potentialGap{
				Name: player.Name, EAOverall: player.EAOverall, Potential: player.Potential,
				Gap: player.Potential - player.EAOverall,
			})
		}
	}
	if !hasPotential {
		result.Note = noPotentialDataNote
	}
	sort.SliceStable(result.Gaps, func(i, j int) bool {
		if result.Gaps[i].Gap == result.Gaps[j].Gap {
			return result.Gaps[i].Name < result.Gaps[j].Name
		}
		return result.Gaps[i].Gap > result.Gaps[j].Gap
	})
	if len(result.Gaps) > limit {
		result.Gaps = result.Gaps[:limit]
	}
	return result
}

func printPotentialGaps(w io.Writer, result potentialGapResult) error {
	fmt.Fprintf(w, "%s — potential gaps\n", result.Team)
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tRATING\tPOTENTIAL\tGAP")
	for _, entry := range result.Gaps {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%+d\n", entry.Name, entry.EAOverall, entry.Potential, entry.Gap)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if result.Note != "" {
		fmt.Fprintln(w, result.Note)
	}
	return nil
}
