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

type wonderkid struct {
	Name             string `json:"name"`
	Age              int    `json:"age"`
	MarketValue      int64  `json:"marketValue"`
	MarketValueLabel string `json:"marketValueLabel"`
	EAOverall        int    `json:"eaOverall"`
	Potential        int    `json:"potential"`
}

type wonderkidsResult struct {
	Team       string                         `json:"team"`
	MaxAge     int                            `json:"max_age"`
	OrderedBy  string                         `json:"ordered_by"`
	Wonderkids []wonderkid                    `json:"wonderkids"`
	Sources    map[string]report.SourceStatus `json:"sources"`
}

// pp:data-source live
func newNovelWonderkidsCmd(flags *rootFlags) *cobra.Command {
	var flagTeam string
	var flagMaxAge int
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "wonderkids",
		Short:       "Find young players with high potential and rising market value.",
		Example:     "  soccer-goat-pp-cli wonderkids --team benfica --max-age 21 --limit 15",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would rank wonderkids for team %q (max age %d, limit %d)\n", flagTeam, flagMaxAge, flagLimit)
				return nil
			}
			if strings.TrimSpace(flagTeam) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--team is required"))
			}
			if flagMaxAge < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--max-age must be non-negative"))
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
			result := rankWonderkids(team, flagMaxAge, flagLimit)
			if novelMachineOutput(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return printWonderkids(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&flagTeam, "team", "", "Club name to analyze")
	cmd.Flags().IntVar(&flagMaxAge, "max-age", 21, "Maximum player age")
	cmd.Flags().IntVar(&flagLimit, "limit", 15, "Maximum players to show")
	return cmd
}

func rankWonderkids(team *report.TeamReport, maxAge, limit int) wonderkidsResult {
	result := wonderkidsResult{Team: team.ClubName, MaxAge: maxAge, Wonderkids: make([]wonderkid, 0), Sources: team.Sources}
	for _, player := range team.Players {
		if player.Age <= 0 || player.Age > maxAge {
			continue
		}
		result.Wonderkids = append(result.Wonderkids, wonderkid{
			Name: player.Name, Age: player.Age, MarketValue: player.MarketValue,
			MarketValueLabel: player.MarketValueLabel, EAOverall: player.EAOverall, Potential: player.Potential,
		})
	}
	// Record the actual ranking basis. "high potential" ordering only holds
	// when the potential source returned data; otherwise the sort below falls
	// back to market value, and callers must be told so the ordering isn't
	// misread as potential-based.
	result.OrderedBy = "market_value"
	for _, wk := range result.Wonderkids {
		if wk.Potential > 0 {
			result.OrderedBy = "potential"
			break
		}
	}
	sort.SliceStable(result.Wonderkids, func(i, j int) bool {
		left, right := result.Wonderkids[i], result.Wonderkids[j]
		if (left.Potential > 0) != (right.Potential > 0) {
			return left.Potential > 0
		}
		if left.Potential != right.Potential {
			return left.Potential > right.Potential
		}
		if left.MarketValue != right.MarketValue {
			return left.MarketValue > right.MarketValue
		}
		return left.Name < right.Name
	})
	if len(result.Wonderkids) > limit {
		result.Wonderkids = result.Wonderkids[:limit]
	}
	return result
}

func printWonderkids(w io.Writer, result wonderkidsResult) error {
	fmt.Fprintf(w, "%s — wonderkids age %d or younger\n", result.Team, result.MaxAge)
	if result.OrderedBy != "potential" {
		potentialDetail := "potential data unavailable"
		if status, ok := result.Sources["potential"]; ok && strings.TrimSpace(status.Detail) != "" {
			potentialDetail = status.Detail
		}
		fmt.Fprintf(w, "Note: ranked by market value (%s); potential-based ranking unavailable\n", potentialDetail)
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "NAME\tAGE\tVALUE\tRATING\tPOTENTIAL")
	for _, player := range result.Wonderkids {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", player.Name, player.Age, player.MarketValueLabel,
			tableRatingLabel(player.EAOverall), tableRatingLabel(player.Potential))
	}
	return tw.Flush()
}
