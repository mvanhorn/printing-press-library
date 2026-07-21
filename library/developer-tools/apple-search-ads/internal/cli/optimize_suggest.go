// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-search-ads/internal/client"
	"github.com/spf13/cobra"
)

// bidSuggestion is a single keyword bid recommendation.
type bidSuggestion struct {
	KeywordID       string  `json:"keyword_id"`
	KeywordText     string  `json:"keyword_text"`
	MatchType       string  `json:"match_type"`
	AdGroupID       string  `json:"ad_group_id"`
	CampaignID      string  `json:"campaign_id"`
	CurrentBid      float64 `json:"current_bid"`
	CurrentCPA      float64 `json:"current_cpa,omitempty"`
	CurrentROAS     float64 `json:"current_roas,omitempty"`
	CurrentTaps     int64   `json:"current_taps"`
	CurrentInstalls int64   `json:"current_installs"`
	SuggestedBid    float64 `json:"suggested_bid"`
	ExpectedDelta   float64 `json:"expected_delta,omitempty"`
	ChangeDirection string  `json:"change_direction"` // "increase", "decrease", "hold"
	Confidence      string  `json:"confidence"`       // "high", "medium", "low"
}

func newNovelOptimizeSuggestCmd(flags *rootFlags) *cobra.Command {
	var flagMetric string
	var flagTarget float64
	var flagCampaignID string
	var flagCurrency string
	var flagLookbackDays int
	var flagApply bool

	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Get CPA/ROAS-driven bid adjustment suggestions with revenue impact forecast before applying.",
		Long: `Analyze keyword performance and suggest bid adjustments to hit a CPA or ROAS target.
Each suggestion includes the current vs suggested bid, expected metric delta, and a confidence
rating based on statistical volume (taps/installs). Use --apply to apply suggestions immediately.

Performance data is sourced from the keyword reporting API over the --lookback-days window.
ROAS mode requires revenue data; keywords with no revenue in the lookback period are skipped.`,
		Example: `  apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50
  apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50 --campaign-id 12345 --apply --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagMetric != "cpa" && flagMetric != "roas" && flagMetric != "taps" {
				return fmt.Errorf("--metric must be one of: cpa, roas, taps (got %q)", flagMetric)
			}
			if flagTarget <= 0 {
				return fmt.Errorf("--target must be a positive number (got %f)", flagTarget)
			}
			if flagLookbackDays < 1 || flagLookbackDays > 90 {
				return fmt.Errorf("--lookback-days must be between 1 and 90 (got %d)", flagLookbackDays)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			end := time.Now().UTC()
			start := end.AddDate(0, 0, -flagLookbackDays)
			reportBody := map[string]any{
				"startTime":   start.Format("2006-01-02"),
				"endTime":     end.Format("2006-01-02"),
				"granularity": "DAILY",
				"selector": map[string]any{
					"orderBy":    []map[string]string{{"field": "localSpend", "sortOrder": "DESCENDING"}},
					"pagination": map[string]int{"offset": 0, "limit": 1000},
				},
				"returnRowTotals":            false,
				"returnRecordsWithNoMetrics": false,
			}

			var allSuggestions []bidSuggestion
			var fetchFailed, fetchAttempted int

			fetchSuggestions := func(cid string) {
				fetchAttempted++
				data, _, err := c.Post(cmd.Context(), "/reports/campaigns/"+cid+"/keywords", reportBody)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: campaign %s: failed to fetch keyword report: %v\n", cid, err)
					fetchFailed++
					return
				}
				keywords := extractKeywordsFromReportPayload(data)
				if len(keywords) >= 1000 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: campaign %s hit the 1000-keyword report limit; results may be truncated\n", cid)
				}
				sug, _ := buildBidSuggestions(keywords, cid, flagMetric, flagTarget)
				allSuggestions = append(allSuggestions, sug...)
			}

			if flagCampaignID != "" {
				fetchSuggestions(flagCampaignID)
			} else {
				campaignsData, err := c.Get(cmd.Context(), "/campaigns", map[string]string{"limit": "1000"})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				campaignIDs := extractCampaignIDs(campaignsData)
				if len(campaignIDs) == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "No campaigns found")
					return printJSONFiltered(cmd.OutOrStdout(), []bidSuggestion{}, flags)
				}
				for _, cid := range campaignIDs {
					fetchSuggestions(cid)
				}
			}

			if fetchAttempted > 0 && fetchFailed == fetchAttempted {
				return fmt.Errorf("%d of %d campaign keyword report(s) failed; check stderr for details", fetchFailed, fetchAttempted)
			}

			if flagApply && !dryRunOK(flags) && len(allSuggestions) > 0 {
				if err := applyBidSuggestionsWithClient(cmd.Context(), c, allSuggestions, flagCurrency, cmd.ErrOrStderr()); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: some bids could not be applied: %v\n", err)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), allSuggestions, flags)
		},
	}

	cmd.Flags().StringVar(&flagMetric, "metric", "cpa", "Optimization metric: cpa, roas, or taps")
	cmd.Flags().Float64Var(&flagTarget, "target", 0, "Target metric value (e.g. 2.50 for $2.50 CPA)")
	cmd.Flags().StringVar(&flagCampaignID, "campaign-id", "", "Limit suggestions to a specific campaign")
	cmd.Flags().StringVar(&flagCurrency, "currency", "USD", "Currency code for bid amounts (e.g. USD, EUR, GBP, AUD)")
	cmd.Flags().IntVar(&flagLookbackDays, "lookback-days", 14, "Days of keyword performance history to analyze (1-90)")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Apply the suggested bids (skipped in --dry-run)")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

// buildBidSuggestions analyzes keyword performance data and computes bid suggestions.
func buildBidSuggestions(keywords []keywordPerf, campaignID, metric string, target float64) ([]bidSuggestion, error) {
	var suggestions []bidSuggestion
	for _, kw := range keywords {
		currentBid := kw.bid
		if currentBid <= 0 {
			continue
		}
		var currentMetric float64
		switch metric {
		case "cpa":
			if kw.installs <= 0 {
				continue
			}
			currentMetric = kw.spend / float64(kw.installs)
		case "roas":
			if kw.spend <= 0 || kw.revenue == 0 {
				continue // no spend data or no revenue data; ROAS cannot be computed
			}
			currentMetric = kw.revenue / kw.spend
		case "taps":
			if kw.taps <= 0 {
				continue
			}
			currentMetric = float64(kw.taps)
		}

		var suggestedBid float64
		var delta float64
		switch metric {
		case "cpa":
			ratio := target / currentMetric
			suggestedBid = math.Round(currentBid*ratio*100) / 100
			if suggestedBid > currentBid*2 {
				suggestedBid = currentBid * 2
			}
			if suggestedBid < 0.01 {
				suggestedBid = 0.01
			}
			delta = (suggestedBid/currentBid)*currentMetric - currentMetric
		case "roas":
			ratio := target / currentMetric
			suggestedBid = math.Round(currentBid*ratio*100) / 100
			if suggestedBid > currentBid*2 {
				suggestedBid = currentBid * 2
			}
			if suggestedBid < 0.01 {
				suggestedBid = 0.01
			}
			delta = suggestedBid - currentBid
		case "taps":
			if float64(kw.taps) < target {
				// ponytail: proportional scale capped at 2x; bid/taps relationship is non-linear
				// but uncapped ratios cause runaway bids when taps << target
				ratio := math.Min(target/float64(kw.taps), 2.0)
				suggestedBid = math.Round(currentBid*ratio*100) / 100
			} else {
				suggestedBid = currentBid
			}
			delta = suggestedBid - currentBid
		}

		var direction string
		switch {
		case suggestedBid > currentBid:
			direction = "increase"
		case suggestedBid < currentBid:
			direction = "decrease"
		default:
			direction = "hold"
		}

		confidence := "low"
		if kw.taps >= 100 && kw.installs >= 10 {
			confidence = "high"
		} else if kw.taps >= 30 && kw.installs >= 3 {
			confidence = "medium"
		}

		s := bidSuggestion{
			KeywordID:       kw.id,
			KeywordText:     kw.text,
			MatchType:       kw.matchType,
			AdGroupID:       kw.adGroupID,
			CampaignID:      campaignID,
			CurrentBid:      currentBid,
			CurrentTaps:     kw.taps,
			CurrentInstalls: kw.installs,
			SuggestedBid:    suggestedBid,
			ChangeDirection: direction,
			Confidence:      confidence,
			ExpectedDelta:   math.Round(delta*100) / 100,
		}
		if metric == "cpa" {
			s.CurrentCPA = currentMetric
		} else if metric == "roas" {
			s.CurrentROAS = currentMetric
		}
		suggestions = append(suggestions, s)
	}
	return suggestions, nil
}

// extractKeywordsFromReportPayload parses the keyword reporting API response shape:
// {"data": {"reportingDataResponse": {"row": [{"metadata": {...}, "granularity": [...]}]}}}
func extractKeywordsFromReportPayload(data json.RawMessage) []keywordPerf {
	payload := data
	for _, key := range []string{"data", "reportingDataResponse"} {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(payload, &nested); err == nil {
			if v, ok := nested[key]; ok {
				payload = v
			}
		}
	}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(payload, &wrapper); err != nil {
		return nil
	}
	rowsRaw, ok := wrapper["row"]
	if !ok {
		return nil
	}
	var apiRows []map[string]json.RawMessage
	if err := json.Unmarshal(rowsRaw, &apiRows); err != nil {
		return nil
	}

	var result []keywordPerf
	for _, apiRow := range apiRows {
		var meta map[string]interface{}
		if err := json.Unmarshal(apiRow["metadata"], &meta); err != nil {
			continue
		}
		kw := keywordPerf{
			id:        optimizeStringField(meta, "keywordId"),
			text:      optimizeStringField(meta, "keyword", "keywordText"),
			matchType: optimizeStringField(meta, "matchType"),
			adGroupID: optimizeStringField(meta, "adGroupId"),
			bid:       optimizeFloatField(meta, "bidAmount"),
		}
		if kw.id == "" {
			continue
		}
		for _, gr := range analyticsExtractGranularityRows(apiRow["granularity"]) {
			kw.taps += analyticsParseInt(gr["taps"])
			kw.installs += analyticsParseInt(gr["installs"])
			kw.spend += analyticsParseFloat(gr["localSpend"])
			kw.revenue += analyticsParseFloat(gr["revenue"])
		}
		result = append(result, kw)
	}
	return result
}

// keywordPerf holds parsed keyword performance data.
type keywordPerf struct {
	id        string
	text      string
	matchType string
	adGroupID string
	bid       float64
	taps      int64
	installs  int64
	spend     float64
	revenue   float64
}


func optimizeStringField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			if f, ok := v.(float64); ok {
				return strconv.FormatInt(int64(f), 10)
			}
		}
	}
	return ""
}

func optimizeInt64Field(m map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch val := v.(type) {
			case float64:
				return int64(val)
			case string:
				n, _ := strconv.ParseInt(val, 10, 64)
				return n
			}
		}
	}
	return 0
}

func optimizeFloatField(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case string:
				f, _ := strconv.ParseFloat(val, 64)
				return f
			case map[string]interface{}:
				if amt, ok := val["amount"].(string); ok {
					f, _ := strconv.ParseFloat(amt, 64)
					return f
				}
				if amt, ok := val["amount"].(float64); ok {
					return amt
				}
			}
		}
	}
	return 0
}

func extractCampaignIDs(data json.RawMessage) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil
	}
	var items []json.RawMessage
	for _, key := range []string{"data", "campaigns"} {
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
		id := optimizeStringField(m, "id", "campaignId")
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// applyBidSuggestionsWithClient sends bid updates to the API for each suggestion.
// Apple Search Ads uses PUT /campaigns/{cid}/adgroups/{agid}/targetingkeywords/{kid}
// with a body containing the updated bidAmount.
func applyBidSuggestionsWithClient(ctx context.Context, c *client.Client, suggestions []bidSuggestion, currency string, stderr interface{ Write([]byte) (int, error) }) error {
	if currency == "" {
		currency = "USD"
	}
	var applied int
	for _, s := range suggestions {
		if s.ChangeDirection == "hold" {
			continue
		}
		if s.CampaignID == "" || s.AdGroupID == "" || s.KeywordID == "" {
			continue
		}
		path := fmt.Sprintf("/campaigns/%s/adgroups/%s/targetingkeywords/%s",
			s.CampaignID, s.AdGroupID, s.KeywordID)
		body := map[string]any{
			"bidAmount": map[string]any{
				"amount":   strconv.FormatFloat(s.SuggestedBid, 'f', 2, 64),
				"currency": currency,
			},
		}
		if _, _, err := c.Put(ctx, path, body); err != nil {
			fmt.Fprintf(stderr, "warning: could not update keyword %s: %v\n", s.KeywordID, err)
		} else {
			applied++
		}
	}
	fmt.Fprintf(stderr, "applied %d bid updates\n", applied)
	return nil
}
