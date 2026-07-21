// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"
)

// spendForecast is the forecast for a single campaign.
type spendForecast struct {
	CampaignID              string  `json:"campaign_id"`
	CampaignName            string  `json:"campaign_name"`
	Status                  string  `json:"status"`
	AvgDailySpend           float64 `json:"avg_daily_spend"`
	ProjectedSpend          float64 `json:"projected_spend"`
	DailyBudgetCap          float64 `json:"daily_budget_cap"`
	LifetimeBudgetCap       float64 `json:"lifetime_budget_cap"`
	ProjectedUtilizationPct float64 `json:"projected_utilization_pct"`
	Alert                   bool    `json:"alert"`
	AlertReason             string  `json:"alert_reason,omitempty"`
}

func newNovelCampaignsForecastSpendCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagAlertThreshold float64

	cmd := &cobra.Command{
		Use:   "forecast-spend",
		Short: "Predict daily/monthly spend and flag campaigns on track to overshoot budget caps.",
		Long: `Fetch campaigns with their budgets, pull the last 7 days of daily spend from the
reporting API, then project spend forward over the specified window. Campaigns
projected to exceed --alert-threshold percent of their budget cap are flagged.

Use --agent to get a structured JSON list for automation.`,
		Example: `  apple-search-ads-pp-cli campaigns forecast-spend --days 30 --alert-threshold 90
  apple-search-ads-pp-cli campaigns forecast-spend --days 7 --alert-threshold 80 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagDays < 1 || flagDays > 365 {
				return fmt.Errorf("--days must be between 1 and 365 (got %d)", flagDays)
			}
			if flagAlertThreshold < 0 || flagAlertThreshold > 200 {
				return fmt.Errorf("--alert-threshold must be between 0 and 200 (got %f)", flagAlertThreshold)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch all campaigns.
			campaignsData, err := c.Get(cmd.Context(), "/campaigns", map[string]string{"limit": "200"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			campaigns := extractCampaignsForForecast(campaignsData)
			if len(campaigns) == 200 {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: campaign list hit the 200-campaign limit; results may be truncated")
			}
			if len(campaigns) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No campaigns found")
				return printJSONFiltered(cmd.OutOrStdout(), []spendForecast{}, flags)
			}

			// Pull last 7 days of reporting to compute average daily spend.
			lookback := 7
			end := time.Now().UTC()
			start := end.AddDate(0, 0, -lookback)
			reqBody := map[string]any{
				"startTime":   start.Format("2006-01-02"),
				"endTime":     end.Format("2006-01-02"),
				"granularity": "DAILY",
				"selector": map[string]any{
					"orderBy":    []map[string]string{{"field": "localSpend", "sortOrder": "DESCENDING"}},
					"pagination": map[string]int{"offset": 0, "limit": 1000},
				},
				"returnRowTotals":            true,
				"returnRecordsWithNoMetrics": false,
			}

			reportData, _, err := c.Post(cmd.Context(), "/reports/campaigns", reqBody)
			spendByCampaign := map[string]float64{}
			if err == nil {
				spendByCampaign = extractCampaignSpendMap(reportData, lookback)
			}
			// If reporting fails, we fall back to zero avg (forecast will show 0 spend).

			var forecasts []spendForecast
			for _, camp := range campaigns {
				avgDailySpend := spendByCampaign[camp.id]
				projectedSpend := math.Round(avgDailySpend*float64(flagDays)*100) / 100

				var budgetCap float64
				var alertReason string
				var utilizationPct float64

				if camp.dailyBudget > 0 {
					budgetCap = camp.dailyBudget * float64(flagDays)
					if budgetCap > 0 {
						utilizationPct = math.Round(projectedSpend/budgetCap*100*100) / 100
					}
					if utilizationPct >= flagAlertThreshold {
						alertReason = fmt.Sprintf("projected %.0f%% of %d-day daily budget (cap: $%.2f)", utilizationPct, flagDays, budgetCap)
					}
				} else if camp.lifetimeBudget > 0 {
					budgetCap = camp.lifetimeBudget
					if budgetCap > 0 {
						utilizationPct = math.Round(projectedSpend/budgetCap*100*100) / 100
					}
					if utilizationPct >= flagAlertThreshold {
						alertReason = fmt.Sprintf("projected %.0f%% of lifetime budget (cap: $%.2f)", utilizationPct, budgetCap)
					}
				}

				f := spendForecast{
					CampaignID:              camp.id,
					CampaignName:            camp.name,
					Status:                  camp.status,
					AvgDailySpend:           math.Round(avgDailySpend*100) / 100,
					ProjectedSpend:          projectedSpend,
					DailyBudgetCap:          camp.dailyBudget,
					LifetimeBudgetCap:       camp.lifetimeBudget,
					ProjectedUtilizationPct: utilizationPct,
					Alert:                   alertReason != "",
					AlertReason:             alertReason,
				}
				forecasts = append(forecasts, f)
			}

			return printJSONFiltered(cmd.OutOrStdout(), forecasts, flags)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 30, "Forecast window in days")
	cmd.Flags().Float64Var(&flagAlertThreshold, "alert-threshold", 90, "Alert when projected spend exceeds this % of budget cap")
	return cmd
}

// campaignBudget holds campaign-level budget data.
type campaignBudget struct {
	id             string
	name           string
	status         string
	dailyBudget    float64
	lifetimeBudget float64
}

func extractCampaignsForForecast(data json.RawMessage) []campaignBudget {
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
	var result []campaignBudget
	for _, item := range items {
		var m map[string]interface{}
		if err := json.Unmarshal(item, &m); err != nil {
			continue
		}
		id := forecastStringField(m, "id", "campaignId")
		if id == "" {
			continue
		}
		c := campaignBudget{
			id:     id,
			name:   forecastStringField(m, "name"),
			status: forecastStringField(m, "status"),
		}
		// Try dailyBudget and budgetAmount fields.
		c.dailyBudget = forecastFloatField(m, "dailyBudget", "dailyBudgetAmount")
		c.lifetimeBudget = forecastFloatField(m, "totalBudget", "lifetimeBudget", "budgetAmount")
		result = append(result, c)
	}
	return result
}

// extractCampaignSpendMap parses the reporting API response and returns
// a map of campaignId → average daily spend over the lookback period.
func extractCampaignSpendMap(data json.RawMessage, lookbackDays int) map[string]float64 {
	result := map[string]float64{}
	// Unwrap nested data structures.
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
		return result
	}
	rowsRaw, ok := wrapper["row"]
	if !ok {
		return result
	}
	var apiRows []map[string]json.RawMessage
	if err := json.Unmarshal(rowsRaw, &apiRows); err != nil {
		return result
	}
	for _, apiRow := range apiRows {
		meta := analyticsExtractStringMap(apiRow["metadata"])
		campaignID := meta["campaignId"]
		if campaignID == "" {
			continue
		}
		// Sum spend from granularity rows.
		granRows := analyticsExtractGranularityRows(apiRow["granularity"])
		var totalSpend float64
		for _, gr := range granRows {
			totalSpend += analyticsParseFloat(gr["localSpend"])
		}
		if lookbackDays > 0 {
			result[campaignID] = totalSpend / float64(lookbackDays)
		}
	}
	return result
}

func forecastStringField(m map[string]interface{}, keys ...string) string {
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

func forecastFloatField(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case string:
				var f float64
				fmt.Sscanf(val, "%f", &f) // #nosec G104 -- zero-value fallback is correct for unparseable currency fields
				return f
			case map[string]interface{}:
				if amt, ok := val["amount"].(string); ok {
					var f float64
					fmt.Sscanf(amt, "%f", &f) // #nosec G104 -- zero-value fallback is correct for unparseable currency fields
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
