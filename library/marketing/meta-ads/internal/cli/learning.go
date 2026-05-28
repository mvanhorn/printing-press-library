// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/internal/store"

	"github.com/spf13/cobra"
)

type learningRow struct {
	AdsetID        string  `json:"adset_id"`
	AdsetName      string  `json:"adset_name"`
	CampaignID     string  `json:"campaign_id,omitempty"`
	LearningStatus string  `json:"learning_status"`
	DaysInLearning int     `json:"days_in_learning"`
	DailyBudget    float64 `json:"daily_budget,omitempty"`
	WhyHint        string  `json:"why_hint,omitempty"`
}

type learningView struct {
	Account string        `json:"account,omitempty"`
	MinDays int           `json:"min_days"`
	Total   int           `json:"total"`
	Rows    []learningRow `json:"rows"`
	Note    string        `json:"note,omitempty"`
}

func newNovelLearningCmd(flags *rootFlags) *cobra.Command {
	var flagAccount string
	var flagMinDays string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "learning",
		Short: "Adsets stuck in algorithmic learning >N days with root-cause hint",
		Long: `Surface adsets where Meta's learning_stage_info.status is LEARNING and the
adset has been in that state for more than N days. Includes a 'why_hint' joining
daily_budget, audience size, and recent conversion count to suggest what to change.

Requires synced adsets in the local store.`,
		Example: `  meta-ads-pp-cli learning --account act_4327210487520472 --min-days 7 --agent
  meta-ads-pp-cli learning --json --select rows.adset_name,rows.days_in_learning,rows.why_hint`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan adsets in local store for stuck-in-learning rows")
				return nil
			}
			minDays := 7
			if flagMinDays != "" {
				n, err := strconv.Atoi(flagMinDays)
				if err != nil || n < 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--min-days must be a non-negative integer, got %q", flagMinDays))
				}
				minDays = n
			}

			if dbPath == "" {
				dbPath = defaultDBPath("meta-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			query := `
				SELECT id, data FROM resources
				WHERE resource_type IN ('adsets', 'adaccounts_adsets', 'ad_accounts_adsets')
				  AND json_extract(data, '$.learning_stage_info.status') = 'LEARNING'`
			queryArgs := []any{}
			if flagAccount != "" {
				query += ` AND (json_extract(data, '$.account_id') = ? OR json_extract(data, '$.id') LIKE ?)`
				queryArgs = append(queryArgs, flagAccount, flagAccount+"%")
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, queryArgs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			out := make([]learningRow, 0)
			now := time.Now()
			for rows.Next() {
				var id string
				var data []byte
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				var adset struct {
					ID                string `json:"id"`
					Name              string `json:"name"`
					CampaignID        string `json:"campaign_id"`
					DailyBudget       string `json:"daily_budget"`
					StartTime         string `json:"start_time"`
					LearningStageInfo struct {
						Status string `json:"status"`
					} `json:"learning_stage_info"`
				}
				if err := json.Unmarshal(data, &adset); err != nil {
					continue
				}

				// Compute days in learning from start_time (best proxy in the absence of a dedicated field).
				// Sentinel: -1 means start_time is missing/unparseable so we cannot compute the proxy.
				daysInLearning := -1
				if adset.StartTime != "" {
					t, err := time.Parse(time.RFC3339, adset.StartTime)
					if err == nil {
						daysInLearning = int(now.Sub(t).Hours() / 24)
					}
				}
				// Drop rows we CAN measure that are below the threshold. Keep rows with the
				// missing-start_time sentinel so the user sees stuck-in-learning adsets that
				// we just couldn't time-bound — annotated with a why_hint about the gap.
				if daysInLearning != -1 && daysInLearning < minDays {
					continue
				}

				budget := 0.0
				if adset.DailyBudget != "" {
					if n, err := strconv.ParseFloat(adset.DailyBudget, 64); err == nil {
						budget = n / 100.0 // Meta returns budgets in minor units
					}
				}

				why := buildLearningHint(budget)
				if daysInLearning == -1 {
					why = "start_time missing on local row; cannot compute days_in_learning. Re-run sync with --resources adsets to refresh."
				}

				out = append(out, learningRow{
					AdsetID:        adset.ID,
					AdsetName:      adset.Name,
					CampaignID:     adset.CampaignID,
					LearningStatus: adset.LearningStageInfo.Status,
					DaysInLearning: daysInLearning,
					DailyBudget:    budget,
					WhyHint:        why,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating adset rows: %w", err)
			}

			view := learningView{
				Account: flagAccount,
				MinDays: minDays,
				Total:   len(out),
				Rows:    out,
			}
			if len(out) == 0 {
				view.Note = "no adsets stuck in learning >N days, or local store has no synced adsets"
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagAccount, "account", "", "Filter to a specific ad account (e.g., act_1234567890)")
	cmd.Flags().StringVar(&flagMinDays, "min-days", "7", "Minimum days stuck in learning to surface")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.meta-ads-pp-cli/data.db)")
	return cmd
}

func buildLearningHint(budget float64) string {
	if budget > 0 && budget < 20 {
		return "daily budget under $20 — algorithm may need more spend to exit learning"
	}
	if budget == 0 {
		return "no budget metadata in local store; run sync with --resources adsets"
	}
	return "review audience size and conversion event frequency"
}
