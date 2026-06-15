// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

const organicKeywordsCompositeSelect = "keyword,volume,keyword_difficulty,best_position,best_position_url,sum_traffic,cpc"

type keywordGapResult struct {
	Keyword                string `json:"keyword"`
	Volume                 int    `json:"volume"`
	KeywordDifficulty      int    `json:"keyword_difficulty"`
	YourPosition           *int   `json:"your_position,omitempty"`
	BestCompetitor         string `json:"best_competitor"`
	BestCompetitorPosition int    `json:"best_competitor_position"`
	BestPositionURL        string `json:"best_position_url,omitempty"`
	SumTraffic             int    `json:"sum_traffic"`
	CPC                    int    `json:"cpc,omitempty"`
}

func newKeywordGapCmd(flags *rootFlags) *cobra.Command {
	var flagTarget string
	var flagCompetitors []string
	var flagCountry string
	var flagMinVolume int
	var flagMaxDifficulty int
	var flagCompetitorMaxPosition int
	var flagYourMinPosition int
	var flagLimit int
	var flagTargetLimit int
	var flagMode string

	cmd := &cobra.Command{
		Use:         "keyword-gap",
		Short:       "Find competitor keywords you do not rank for, or rank worse for",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  ahrefs-pp-cli keyword-gap --target bestself.co --competitor intelligentchange.com --country us
  ahrefs-pp-cli keyword-gap --target bestself.co --competitor intelligentchange.com --competitor papier.com --min-volume 100 --max-difficulty 40`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagTarget == "" && !flags.dryRun {
				return fmt.Errorf("required flag %q not set", "target")
			}
			if len(flagCompetitors) == 0 && !flags.dryRun {
				return fmt.Errorf("required flag %q not set", "competitor")
			}
			if err := validateCompositeMode(cmd, flagMode); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			targetWhere := compositeWhere(
				compositeNumberWhere("volume", "gte", flagMinVolume),
				difficultyWhere(flagMaxDifficulty),
			)
			// Build "your rankings" from a separate, larger lookup so a
			// keyword you rank for that falls outside the first --limit
			// rows is not mistaken for a gap. --target-limit controls this
			// independently of the competitor fetch and output cap.
			targetParams := organicKeywordsCompositeParams(flagTarget, flagCountry, flagMode, flagTargetLimit, targetWhere)
			yourRows, targetProv, err := fetchCompositeRows[organicKeywordCompositeRow](cmd, c, flags, "/site-explorer/organic-keywords", targetParams)
			if err != nil {
				return classifyAPIError(err)
			}

			yourPositions := map[string]*int{}
			for _, row := range yourRows {
				if row.Keyword == "" {
					continue
				}
				yourPositions[row.Keyword] = row.BestPosition
			}

			bestByKeyword := map[string]keywordGapResult{}
			provs := []DataProvenance{targetProv}
			competitorWhere := compositeWhere(
				compositeNumberWhere("best_position", "lte", flagCompetitorMaxPosition),
				compositeNumberWhere("volume", "gte", flagMinVolume),
				difficultyWhere(flagMaxDifficulty),
			)
			for _, competitor := range flagCompetitors {
				params := organicKeywordsCompositeParams(competitor, flagCountry, flagMode, flagLimit, competitorWhere)
				rows, prov, err := fetchCompositeRows[organicKeywordCompositeRow](cmd, c, flags, "/site-explorer/organic-keywords", params)
				if err != nil {
					return classifyAPIError(err)
				}
				provs = append(provs, prov)
				for _, row := range rows {
					if row.Keyword == "" || row.BestPosition == nil || *row.BestPosition > flagCompetitorMaxPosition {
						continue
					}
					if row.Volume < flagMinVolume || exceedsDifficulty(row.KeywordDifficulty, flagMaxDifficulty) {
						continue
					}
					if yourPos := yourPositions[row.Keyword]; yourPos != nil && *yourPos <= flagYourMinPosition {
						continue
					}
					current, ok := bestByKeyword[row.Keyword]
					if ok && current.BestCompetitorPosition <= *row.BestPosition {
						continue
					}
					bestByKeyword[row.Keyword] = keywordGapResult{
						Keyword:                row.Keyword,
						Volume:                 row.Volume,
						KeywordDifficulty:      row.KeywordDifficulty,
						YourPosition:           yourPositions[row.Keyword],
						BestCompetitor:         competitor,
						BestCompetitorPosition: *row.BestPosition,
						BestPositionURL:        row.BestPositionURL,
						SumTraffic:             row.SumTraffic,
						CPC:                    row.CPC,
					}
				}
			}
			if flags.dryRun {
				return nil
			}
			results := make([]keywordGapResult, 0, len(bestByKeyword))
			for _, row := range bestByKeyword {
				results = append(results, row)
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].Volume == results[j].Volume {
					return results[i].BestCompetitorPosition < results[j].BestCompetitorPosition
				}
				return results[i].Volume > results[j].Volume
			})
			results = limitCompositeRows(results, flagLimit)
			return printCompositeOutputWithCompact(cmd, results, compactKeywordGapResults(results), len(results), mergeCompositeProvenance(provs...), flags)
		},
	}
	cmd.Flags().StringVar(&flagTarget, "target", "", "Your target domain or URL.")
	cmd.Flags().StringArrayVar(&flagCompetitors, "competitor", nil, "Competitor domain or URL. Repeat for multiple competitors.")
	cmd.Flags().StringVar(&flagCountry, "country", "", "A two-letter country code (ISO 3166-1 alpha-2).")
	cmd.Flags().IntVar(&flagMinVolume, "min-volume", 0, "Minimum keyword search volume.")
	cmd.Flags().IntVar(&flagMaxDifficulty, "max-difficulty", 0, "Maximum keyword difficulty; 0 disables this filter.")
	cmd.Flags().IntVar(&flagCompetitorMaxPosition, "competitor-max-position", 10, "Maximum competitor ranking position to count as a gap.")
	cmd.Flags().IntVar(&flagYourMinPosition, "your-min-position", 11, "Your position must be absent or worse than this value.")
	cmd.Flags().IntVar(&flagLimit, "limit", 1000, "Maximum rows to request per competitor and return after sorting.")
	cmd.Flags().IntVar(&flagTargetLimit, "target-limit", 10000, "Rows to fetch for your own rankings when detecting gaps; higher improves accuracy but costs more credits. 0 fetches the API maximum.")
	cmd.Flags().StringVar(&flagMode, "mode", "subdomains", "The scope of the search based on the target you entered. (one of: exact, prefix, domain, subdomains)")
	return cmd
}

func organicKeywordsCompositeParams(target, country, mode string, limit int, where string) map[string]string {
	params := map[string]string{
		"select":      organicKeywordsCompositeSelect,
		"target":      target,
		"mode":        mode,
		"date":        todayUTCDate(),
		"volume_mode": "monthly",
	}
	if limit != 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	if country != "" {
		params["country"] = country
	}
	if where != "" {
		params["where"] = where
	}
	return params
}

func difficultyWhere(maxDifficulty int) string {
	if maxDifficulty <= 0 {
		return ""
	}
	return compositeNumberWhere("keyword_difficulty", "lte", maxDifficulty)
}

func exceedsDifficulty(keywordDifficulty int, maxDifficulty int) bool {
	return maxDifficulty > 0 && keywordDifficulty > maxDifficulty
}

func compactKeywordGapResults(rows []keywordGapResult) []map[string]any {
	compact := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		compact = append(compact, map[string]any{
			"keyword":                  row.Keyword,
			"volume":                   row.Volume,
			"your_position":            row.YourPosition,
			"best_competitor":          row.BestCompetitor,
			"best_competitor_position": row.BestCompetitorPosition,
		})
	}
	return compact
}
