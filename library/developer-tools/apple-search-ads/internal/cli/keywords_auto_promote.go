// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// promoteResult is the result of an auto-promote operation.
type promoteResult struct {
	Promoted int               `json:"promoted"`
	Skipped  int               `json:"skipped"`
	Keywords []promotedKeyword `json:"keywords"`
}

// promotedKeyword is a search term that was (or would be) promoted to a keyword.
type promotedKeyword struct {
	SearchTerm string `json:"search_term"`
	MatchType  string `json:"match_type"`
	Taps       int64  `json:"taps"`
	Installs   int64  `json:"installs"`
	Action     string `json:"action"` // "promoted" or "dry-run"
	AdGroupID  string `json:"ad_group_id"`
	Reason     string `json:"reason"`
}

func newNovelKeywordsAutoPromoteCmd(flags *rootFlags) *cobra.Command {
	var flagCampaignID string
	var flagMinInstalls int
	var flagMinTaps int
	var flagMatchType string
	var flagBidAmount float64
	var flagCurrency string

	cmd := &cobra.Command{
		Use:   "auto-promote",
		Short: "Batch analyze high-converting search terms and promote them to keywords with smart match-type routing.",
		Long: `Fetch search terms from a campaign's ad groups, filter for those meeting minimum
install and tap thresholds, and promote them to targeting keywords. Smart match-type
routing: taps > 50 → EXACT, taps > 10 → BROAD, else skipped.

Use --dry-run to preview what would be promoted without making API changes.`,
		Example: `  apple-search-ads-pp-cli keywords auto-promote --campaign-id 12345 --min-installs 3 --dry-run
  apple-search-ads-pp-cli keywords auto-promote --campaign-id 12345 --min-installs 5 --match-type exact`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagCampaignID == "" {
				return fmt.Errorf("--campaign-id is required")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch ad groups for this campaign.
			adGroupsData, err := c.Get(cmd.Context(), "/campaigns/"+flagCampaignID+"/adgroups", map[string]string{"limit": "100"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			adGroupIDs := extractAdGroupIDs(adGroupsData)
			if len(adGroupIDs) >= 100 {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: ad group list hit the 100-ad-group limit; search terms in ad groups beyond 100 may be silently skipped")
			}
			if len(adGroupIDs) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No ad groups found for campaign", flagCampaignID)
				return printJSONFiltered(cmd.OutOrStdout(), promoteResult{}, flags)
			}

			var result promoteResult
			isDryRun := flags != nil && flags.dryRun

			endTime := time.Now().UTC()
			startTime := endTime.AddDate(0, 0, -90)
			for _, agID := range adGroupIDs {
				// Fetch search terms report for this ad group.
				reqBody := map[string]any{
					"startTime":   startTime.Format("2006-01-02"),
					"endTime":     endTime.Format("2006-01-02"),
					"granularity": "DAILY",
					"selector": map[string]any{
						"orderBy": []map[string]string{
							{"field": "installs", "sortOrder": "DESCENDING"},
						},
						"pagination": map[string]int{"offset": 0, "limit": 1000},
					},
					"groupBy":                    []string{"searchTerm"},
					"returnRowTotals":            false,
					"returnRecordsWithNoMetrics": false,
				}
				stData, _, err := c.Post(cmd.Context(), "/reports/campaigns/"+flagCampaignID+"/adgroups/"+agID+"/searchterms", reqBody)
				if err != nil {
					// Try the search terms endpoint directly.
					stData, err = c.Get(cmd.Context(), "/campaigns/"+flagCampaignID+"/adgroups/"+agID+"/searchterms", map[string]string{"limit": "500"})
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: ad group %s: failed to fetch search terms (report and fallback endpoints both failed): %v\n", agID, err)
						continue
					}
				}

				terms := extractSearchTerms(stData)
				for _, term := range terms {
					if term.installs < int64(flagMinInstalls) {
						result.Skipped++
						continue
					}
					if term.taps < int64(flagMinTaps) {
						result.Skipped++
						continue
					}

					// Smart match-type routing.
					matchType := flagMatchType
					if matchType == "" {
						if term.taps > 50 {
							matchType = "EXACT"
						} else if term.taps > 10 {
							matchType = "BROAD"
						} else {
							result.Skipped++
							continue
						}
					}

					action := "dry-run"
					if !isDryRun {
						// Create keyword from search term.
						kwBody := map[string]any{
							"adGroupId": agID,
							"text":      term.text,
							"matchType": matchType,
							"status":    "ENABLED",
							"bidAmount": map[string]any{
								"amount":   strconv.FormatFloat(flagBidAmount, 'f', 2, 64),
								"currency": flagCurrency,
							},
						}
						_, _, createErr := c.Post(cmd.Context(), "/campaigns/"+flagCampaignID+"/adgroups/"+agID+"/targetingkeywords", kwBody)
						if createErr != nil {
							result.Skipped++
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not promote %q: %v\n", term.text, createErr)
							continue
						}
						action = "promoted"
						result.Promoted++
					} else {
						result.Promoted++
					}

					result.Keywords = append(result.Keywords, promotedKeyword{
						SearchTerm: term.text,
						MatchType:  matchType,
						Taps:       term.taps,
						Installs:   term.installs,
						Action:     action,
						AdGroupID:  agID,
						Reason:     fmt.Sprintf("taps=%d installs=%d → %s", term.taps, term.installs, matchType),
					})
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&flagCampaignID, "campaign-id", "", "Campaign ID to fetch search terms from (required)")
	cmd.Flags().IntVar(&flagMinInstalls, "min-installs", 3, "Minimum installs for a search term to qualify for promotion")
	cmd.Flags().IntVar(&flagMinTaps, "min-taps", 10, "Minimum taps for a search term to qualify for promotion")
	cmd.Flags().StringVar(&flagMatchType, "match-type", "", "Force match type: EXACT, BROAD, or AUTO (default: smart routing by tap volume)")
	cmd.Flags().Float64Var(&flagBidAmount, "bid-amount", 0.50, "Initial bid amount for promoted keywords")
	cmd.Flags().StringVar(&flagCurrency, "currency", "USD", "Currency code for bid amounts (e.g. USD, EUR, GBP, AUD)")
	_ = cmd.MarkFlagRequired("campaign-id")
	return cmd
}

// searchTermPerf holds parsed search term performance data.
type searchTermPerf struct {
	text     string
	taps     int64
	installs int64
}

func extractSearchTerms(data json.RawMessage) []searchTermPerf {
	// Unwrap {"data": {"reportingDataResponse": ...}} envelope.
	payload := data
	for _, key := range []string{"data", "reportingDataResponse"} {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(payload, &nested); err == nil {
			if v, ok := nested[key]; ok {
				payload = v
			}
		}
	}

	// Reporting API shape: {"row": [{"metadata": {...}, "granularity": [...]}]}
	// Taps/installs are strings nested inside granularity sub-arrays.
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(payload, &wrapper); err == nil {
		if rowsRaw, ok := wrapper["row"]; ok {
			var apiRows []map[string]json.RawMessage
			if err := json.Unmarshal(rowsRaw, &apiRows); err == nil {
				var result []searchTermPerf
				for _, apiRow := range apiRows {
					var meta map[string]interface{}
					if err := json.Unmarshal(apiRow["metadata"], &meta); err != nil {
						continue
					}
					text := autoPromoteStringField(meta, "searchTerm", "keywordText", "text")
					if text == "" {
						continue
					}
					var taps, installs int64
					for _, gr := range analyticsExtractGranularityRows(apiRow["granularity"]) {
						taps += analyticsParseInt(gr["taps"])
						installs += analyticsParseInt(gr["installs"])
					}
					result = append(result, searchTermPerf{text: text, taps: taps, installs: installs})
				}
				return result
			}
		}
	}

	// Fallback: flat array shape (direct GET /adgroups/{id}/searchterms response).
	var items []map[string]interface{}
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil
	}
	var result []searchTermPerf
	for _, item := range items {
		text := autoPromoteStringField(item, "searchTerm", "text", "keyword")
		if text == "" {
			if meta, ok := item["metadata"].(map[string]interface{}); ok {
				text = autoPromoteStringField(meta, "searchTerm", "text", "keyword")
			}
		}
		if text == "" {
			continue
		}
		taps := autoPromoteInt64Field(item, "taps", "tapCount")
		installs := autoPromoteInt64Field(item, "installs", "installCount")
		result = append(result, searchTermPerf{text: text, taps: taps, installs: installs})
	}
	return result
}

func extractAdGroupIDs(data json.RawMessage) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil
	}
	var items []json.RawMessage
	for _, key := range []string{"data", "adGroups"} {
		if raw, ok := top[key]; ok {
			if err := json.Unmarshal(raw, &items); err == nil {
				break
			}
		}
	}
	var ids []string
	for _, item := range items {
		var m map[string]interface{}
		if err := json.Unmarshal(item, &m); err != nil {
			continue
		}
		id := autoPromoteStringField(m, "id", "adGroupId")
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func autoPromoteStringField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			if f, ok := v.(float64); ok {
				return fmt.Sprintf("%.0f", f)
			}
		}
	}
	return ""
}

func autoPromoteInt64Field(m map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch val := v.(type) {
			case float64:
				return int64(val)
			case string:
				var n int64
				fmt.Sscanf(val, "%d", &n) // #nosec G104 -- zero-value fallback is correct for unparseable API integer fields
				return n
			}
		}
	}
	return 0
}
