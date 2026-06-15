// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// gaqlKeywordRow is one results[] entry from a keyword_view GAQL query.
type gaqlKeywordRow struct {
	AdGroupCriterion struct {
		Keyword struct {
			Text      string `json:"text"`
			MatchType string `json:"matchType"`
		} `json:"keyword"`
	} `json:"adGroupCriterion"`
	Metrics struct {
		CostMicros       flexInt `json:"costMicros"`
		Conversions      float64 `json:"conversions"`
		ConversionsValue float64 `json:"conversionsValue"`
	} `json:"metrics"`
}

// keywordTierResult buckets a keyword into scale / hold / cut by ROAS.
type keywordTierResult struct {
	Keyword     string  `json:"keyword"`
	MatchType   string  `json:"match_type"`
	Cost        float64 `json:"cost"`
	Conversions float64 `json:"conversions"`
	Revenue     float64 `json:"revenue"`
	ROAS        float64 `json:"roas"`
	CPA         float64 `json:"cpa,omitempty"`
	Tier        string  `json:"tier"`
}

func newKeywordRoiTiersCmd(flags *rootFlags) *cobra.Command {
	var flagCustomerID string
	var flagDays int
	var flagMinCost float64
	var flagScaleROAS float64
	var flagCutROAS float64
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "keyword-roi-tiers",
		Short:       "Bucket keywords into scale / hold / cut by return on ad spend",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  google-ads-pp-cli keyword-roi-tiers --customer-id 1234567890
  google-ads-pp-cli keyword-roi-tiers --customer-id 1234567890 --days 30 --scale-roas 4 --cut-roas 1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagCustomerID == "" {
				return fmt.Errorf("required flag %q not set", "customer-id")
			}
			if flagDays <= 0 {
				return fmt.Errorf("--days must be greater than 0")
			}
			if flagMinCost < 0 {
				return fmt.Errorf("--min-cost cannot be negative")
			}
			if flagCutROAS > flagScaleROAS {
				return fmt.Errorf("--cut-roas must be less than or equal to --scale-roas")
			}

			query := buildKeywordTiersQuery(flagDays, time.Now().UTC())

			// Dry-run: emit nothing (composite read-only contract).
			if flags.dryRun {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := fetchGAQLRows[gaqlKeywordRow](c, flagCustomerID, query)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			results := scoreKeywordTiers(rows, flagMinCost, flagScaleROAS, flagCutROAS, flagLimit)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers := []string{"KEYWORD", "TIER", "COST", "REVENUE", "ROAS"}
				tableRows := make([][]string, 0, len(results))
				for _, r := range results {
					tableRows = append(tableRows, []string{
						r.Keyword,
						r.Tier,
						fmt.Sprintf("%.2f", r.Cost),
						fmt.Sprintf("%.2f", r.Revenue),
						fmt.Sprintf("%.2f", r.ROAS),
					})
				}
				return flags.printTable(cmd, headers, tableRows)
			}

			payload := map[string]any{
				"customer_id": flagCustomerID,
				"window_days": flagDays,
				"scale_roas":  flagScaleROAS,
				"cut_roas":    flagCutROAS,
				"count":       len(results),
				"tier_counts": tierCounts(results),
				"results":     results,
			}
			return flags.printJSON(cmd, payload)
		},
	}

	cmd.Flags().StringVar(&flagCustomerID, "customer-id", "", "Google Ads customer ID (required).")
	cmd.Flags().IntVar(&flagDays, "days", 7, "Look-back window in days.")
	cmd.Flags().Float64Var(&flagMinCost, "min-cost", 1, "Ignore keywords below this spend (in account currency).")
	cmd.Flags().Float64Var(&flagScaleROAS, "scale-roas", 4, "ROAS at or above which a keyword is tiered 'scale'.")
	cmd.Flags().Float64Var(&flagCutROAS, "cut-roas", 1, "ROAS at or below which a keyword is tiered 'cut'.")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows to return after ranking.")
	return cmd
}

// buildKeywordTiersQuery builds the GAQL over keyword_view for an inclusive
// window of `days` ending at `now`, restricted to keywords with spend.
func buildKeywordTiersQuery(days int, now time.Time) string {
	end := now
	start := end.AddDate(0, 0, -(days - 1))
	return fmt.Sprintf(
		"SELECT ad_group_criterion.keyword.text, ad_group_criterion.keyword.match_type, "+
			"metrics.cost_micros, metrics.conversions, metrics.conversions_value "+
			"FROM keyword_view WHERE segments.date BETWEEN '%s' AND '%s' "+
			"AND metrics.cost_micros > 0 ORDER BY metrics.cost_micros DESC",
		start.Format("2006-01-02"), end.Format("2006-01-02"),
	)
}

// scoreKeywordTiers computes ROAS (revenue/cost) per keyword and buckets it:
// scale if ROAS >= scaleROAS, cut if ROAS <= cutROAS, otherwise hold. Keywords
// below minCost are ignored. Ranked by cost descending, capped at limit
// (limit <= 0 means no cap).
func scoreKeywordTiers(rows []gaqlKeywordRow, minCost, scaleROAS, cutROAS float64, limit int) []keywordTierResult {
	results := make([]keywordTierResult, 0, len(rows))
	for _, row := range rows {
		cost := float64(row.Metrics.CostMicros) / 1e6
		if cost < minCost {
			continue
		}
		revenue := row.Metrics.ConversionsValue
		roas := 0.0
		if cost > 0 {
			roas = revenue / cost
		}
		cpa := 0.0
		if row.Metrics.Conversions > 0 {
			cpa = cost / row.Metrics.Conversions
		}
		tier := "hold"
		if roas >= scaleROAS {
			tier = "scale"
		} else if roas <= cutROAS {
			tier = "cut"
		}
		results = append(results, keywordTierResult{
			Keyword:     row.AdGroupCriterion.Keyword.Text,
			MatchType:   row.AdGroupCriterion.Keyword.MatchType,
			Cost:        cost,
			Conversions: row.Metrics.Conversions,
			Revenue:     revenue,
			ROAS:        roas,
			CPA:         cpa,
			Tier:        tier,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Cost > results[j].Cost
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

// tierCounts tallies how many keywords fell into each tier.
func tierCounts(rows []keywordTierResult) map[string]int {
	counts := map[string]int{"scale": 0, "hold": 0, "cut": 0}
	for _, r := range rows {
		counts[r.Tier]++
	}
	return counts
}
