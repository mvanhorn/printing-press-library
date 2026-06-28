// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

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
	ExpectedDelta   float64 `json:"expected_cpa_delta,omitempty"`
	ChangeDirection string  `json:"change_direction"` // "increase", "decrease", "hold"
	Confidence      string  `json:"confidence"`       // "high", "medium", "low"
}

func newNovelOptimizeSuggestCmd(flags *rootFlags) *cobra.Command {
	var flagMetric string
	var flagTarget float64
	var flagCampaignID string
	var flagApply bool

	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Get CPA/ROAS-driven bid adjustment suggestions with revenue impact forecast before applying.",
		Long: `Analyze keyword performance and suggest bid adjustments to hit a CPA or ROAS target.
Each suggestion includes the current vs suggested bid, expected metric delta, and a confidence
rating based on statistical volume (taps/installs). Use --apply to apply suggestions immediately.`,
		Example: `  apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50
  apple-search-ads-pp-cli optimize suggest --metric cpa --target 2.50 --campaign-id 12345 --apply --dry-run`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagMetric != "cpa" && flagMetric != "roas" && flagMetric != "taps" {
				return fmt.Errorf("--metric must be one of: cpa, roas, taps (got %q)", flagMetric)
			}
			if flagTarget <= 0 {
				return fmt.Errorf("--target must be a positive number (got %f)", flagTarget)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{"limit": "100"}
			var allSuggestions []bidSuggestion

			if flagCampaignID != "" {
				data, err := c.Get(cmd.Context(), "/campaigns/"+flagCampaignID+"/adgroups/targetingkeywords", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				sug, err := buildBidSuggestions(data, flagCampaignID, flagMetric, flagTarget)
				if err != nil {
					return err
				}
				allSuggestions = sug
			} else {
				campaignsData, err := c.Get(cmd.Context(), "/campaigns", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				campaignIDs := extractCampaignIDs(campaignsData)
				if len(campaignIDs) == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "No campaigns found")
					return printJSONFiltered(cmd.OutOrStdout(), []bidSuggestion{}, flags)
				}
				for _, cid := range campaignIDs {
					kwData, err := c.Get(cmd.Context(), "/campaigns/"+cid+"/adgroups/targetingkeywords", params)
					if err != nil {
						continue
					}
					sug, err := buildBidSuggestions(kwData, cid, flagMetric, flagTarget)
					if err != nil {
						continue
					}
					allSuggestions = append(allSuggestions, sug...)
				}
			}

			if flagApply && len(allSuggestions) > 0 {
				if err := applyBidSuggestionsWithClient(cmd.Context(), c, allSuggestions, cmd.ErrOrStderr()); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: some bids could not be applied: %v\n", err)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), allSuggestions, flags)
		},
	}

	cmd.Flags().StringVar(&flagMetric, "metric", "cpa", "Optimization metric: cpa, roas, or taps")
	cmd.Flags().Float64Var(&flagTarget, "target", 0, "Target metric value (e.g. 2.50 for $2.50 CPA)")
	cmd.Flags().StringVar(&flagCampaignID, "campaign-id", "", "Limit suggestions to a specific campaign")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Apply the suggested bids (skipped in --dry-run)")
	_ = cmd.MarkFlagRequired("target")
	return cmd
}

// buildBidSuggestions analyzes keyword data and computes bid suggestions for the given metric/target.
func buildBidSuggestions(data json.RawMessage, campaignID, metric string, target float64) ([]bidSuggestion, error) {
	keywords := extractKeywordsFromPayload(data)
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
			if kw.spend <= 0 {
				continue
			}
			currentMetric = kw.revenue / kw.spend
		case "taps":
			currentMetric = float64(kw.taps)
		}

		var suggestedBid float64
		var direction string
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
			if suggestedBid < 0.01 {
				suggestedBid = 0.01
			}
			delta = suggestedBid - currentBid
		case "taps":
			if float64(kw.taps) < target {
				suggestedBid = math.Round(currentBid*1.2*100) / 100
			} else {
				suggestedBid = currentBid
			}
		}

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
		}
		if metric == "cpa" {
			s.CurrentCPA = currentMetric
			s.ExpectedDelta = math.Round(delta*100) / 100
		} else if metric == "roas" {
			s.CurrentROAS = currentMetric
		}
		suggestions = append(suggestions, s)
	}
	return suggestions, nil
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

func extractKeywordsFromPayload(data json.RawMessage) []keywordPerf {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil
	}
	var items []json.RawMessage
	for _, key := range []string{"data", "items", "keywords", "targetingKeywords"} {
		if raw, ok := top[key]; ok {
			if err := json.Unmarshal(raw, &items); err == nil {
				break
			}
		}
	}
	if items == nil {
		if err := json.Unmarshal(data, &items); err != nil {
			return nil
		}
	}

	var result []keywordPerf
	for _, item := range items {
		var m map[string]interface{}
		if err := json.Unmarshal(item, &m); err != nil {
			continue
		}
		kw := keywordPerf{
			id:        optimizeStringField(m, "id", "keywordId"),
			text:      optimizeStringField(m, "text", "keyword"),
			matchType: optimizeStringField(m, "matchType"),
			adGroupID: optimizeStringField(m, "adGroupId"),
			bid:       optimizeFloatField(m, "bidAmount", "bid", "amount"),
			taps:      optimizeInt64Field(m, "taps", "tapCount"),
			installs:  optimizeInt64Field(m, "installs", "installCount"),
			spend:     optimizeFloatField(m, "localSpend", "spend"),
		}
		result = append(result, kw)
	}
	return result
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
func applyBidSuggestionsWithClient(ctx context.Context, c *client.Client, suggestions []bidSuggestion, stderr interface{ Write([]byte) (int, error) }) error {
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
				"currency": "USD",
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
