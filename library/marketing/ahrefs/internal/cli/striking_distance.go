// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type strikingDistanceResult struct {
	Keyword           string  `json:"keyword"`
	Volume            int     `json:"volume"`
	KeywordDifficulty int     `json:"keyword_difficulty"`
	BestPosition      int     `json:"best_position"`
	BestPositionURL   string  `json:"best_position_url,omitempty"`
	SumTraffic        int     `json:"sum_traffic"`
	Opportunity       float64 `json:"opportunity"`
}

func newStrikingDistanceCmd(flags *rootFlags) *cobra.Command {
	var flagTarget string
	var flagCountry string
	var flagMinPosition int
	var flagMaxPosition int
	var flagMinVolume int
	var flagMaxDifficulty int
	var flagLimit int
	var flagMode string

	cmd := &cobra.Command{
		Use:         "striking-distance",
		Short:       "Find keywords ranking just off page-one wins",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  ahrefs-pp-cli striking-distance --target bestself.co --country us
  ahrefs-pp-cli striking-distance --target bestself.co --min-position 4 --max-position 15 --min-volume 200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagTarget == "" && !flags.dryRun {
				return fmt.Errorf("required flag %q not set", "target")
			}
			if flagMaxPosition < flagMinPosition {
				return fmt.Errorf("--max-position must be greater than or equal to --min-position")
			}
			if err := validateCompositeMode(cmd, flagMode); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			where := compositeWhere(
				compositeNumberWhere("best_position", "gte", flagMinPosition),
				compositeNumberWhere("best_position", "lte", flagMaxPosition),
				compositeNumberWhere("volume", "gte", flagMinVolume),
				difficultyWhere(flagMaxDifficulty),
			)
			params := organicKeywordsCompositeParams(flagTarget, flagCountry, flagMode, flagLimit, where)
			rows, prov, err := fetchCompositeRows[organicKeywordCompositeRow](cmd, c, flags, "/site-explorer/organic-keywords", params)
			if err != nil {
				return classifyAPIError(err)
			}
			if flags.dryRun {
				return nil
			}

			span := flagMaxPosition - flagMinPosition
			results := make([]strikingDistanceResult, 0, len(rows))
			for _, row := range rows {
				if row.Keyword == "" || row.BestPosition == nil {
					continue
				}
				if *row.BestPosition < flagMinPosition || *row.BestPosition > flagMaxPosition || row.Volume < flagMinVolume {
					continue
				}
				opportunity := float64(row.Volume)
				if span > 0 {
					opportunity = float64(row.Volume) * (1 - float64(*row.BestPosition-flagMinPosition)/float64(span))
				}
				results = append(results, strikingDistanceResult{
					Keyword:           row.Keyword,
					Volume:            row.Volume,
					KeywordDifficulty: row.KeywordDifficulty,
					BestPosition:      *row.BestPosition,
					BestPositionURL:   row.BestPositionURL,
					SumTraffic:        row.SumTraffic,
					Opportunity:       opportunity,
				})
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].Opportunity == results[j].Opportunity {
					return results[i].Volume > results[j].Volume
				}
				return results[i].Opportunity > results[j].Opportunity
			})
			results = limitCompositeRows(results, flagLimit)
			return printCompositeOutputWithCompact(cmd, results, compactStrikingDistanceResults(results), len(results), prov, flags)
		},
	}
	cmd.Flags().StringVar(&flagTarget, "target", "", "Your target domain or URL.")
	cmd.Flags().StringVar(&flagCountry, "country", "", "A two-letter country code (ISO 3166-1 alpha-2).")
	cmd.Flags().IntVar(&flagMinPosition, "min-position", 4, "Lowest ranking position to include.")
	cmd.Flags().IntVar(&flagMaxPosition, "max-position", 15, "Highest ranking position to include.")
	cmd.Flags().IntVar(&flagMinVolume, "min-volume", 100, "Minimum keyword search volume.")
	cmd.Flags().IntVar(&flagMaxDifficulty, "max-difficulty", 0, "Maximum keyword difficulty; 0 disables this filter.")
	cmd.Flags().IntVar(&flagLimit, "limit", 1000, "Maximum rows to request and return after sorting.")
	cmd.Flags().StringVar(&flagMode, "mode", "subdomains", "The scope of the search based on the target you entered. (one of: exact, prefix, domain, subdomains)")
	return cmd
}

func compactStrikingDistanceResults(rows []strikingDistanceResult) []map[string]any {
	compact := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		compact = append(compact, map[string]any{
			"keyword":       row.Keyword,
			"volume":        row.Volume,
			"best_position": row.BestPosition,
			"opportunity":   row.Opportunity,
		})
	}
	return compact
}
