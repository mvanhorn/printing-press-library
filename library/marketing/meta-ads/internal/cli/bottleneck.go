// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/internal/store"

	"github.com/spf13/cobra"
)

type bottleneckRow struct {
	AdsetID         string  `json:"adset_id"`
	AdsetName       string  `json:"adset_name"`
	CampaignID      string  `json:"campaign_id,omitempty"`
	Spend           float64 `json:"spend"`
	Roas            float64 `json:"roas"`
	EffectiveStatus string  `json:"effective_status,omitempty"`
	LearningStatus  string  `json:"learning_status,omitempty"`
	Why             string  `json:"why,omitempty"`
}

type bottleneckView struct {
	Account   string          `json:"account,omitempty"`
	Limit     int             `json:"limit"`
	Total     int             `json:"total"`     // matched adsets before --limit truncation
	Returned  int             `json:"returned"`  // adsets actually present in rows after truncation
	Truncated bool            `json:"truncated"` // true when total > limit so caller can raise --limit
	Rows      []bottleneckRow `json:"rows"`
	Note      string          `json:"note,omitempty"`
}

func newNovelBottleneckCmd(flags *rootFlags) *cobra.Command {
	var flagAccount string
	var flagLimit string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "bottleneck",
		Short: "Highest-spend adsets with worst ROAS, ranked with 'why' column",
		Long: `Surface the adsets that combine high spend with poor return-on-ad-spend.
Joins to learning_stage_info and effective_status to populate a 'why' hint
explaining why each adset is bottlenecked.

Requires synced adsets and insights data in the local store.`,
		Example: `  meta-ads-pp-cli bottleneck --account act_4327210487520472 --limit 10 --agent
  meta-ads-pp-cli bottleneck --json --select rows.adset_name,rows.spend,rows.roas`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank adsets by spend/ROAS from local store")
				return nil
			}
			limit := 10
			if flagLimit != "" {
				n, err := strconv.Atoi(flagLimit)
				if err != nil || n <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--limit must be a positive integer, got %q", flagLimit))
				}
				limit = n
			}

			if dbPath == "" {
				dbPath = defaultDBPath("meta-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Walk adsets. For each, aggregate insights spend and compute ROAS.
			query := `
				SELECT id, data FROM resources
				WHERE resource_type IN ('adsets', 'adaccounts_adsets', 'ad_accounts_adsets')`
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

			out := make([]bottleneckRow, 0)
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
					EffectiveStatus   string `json:"effective_status"`
					LearningStageInfo struct {
						Status string `json:"status"`
					} `json:"learning_stage_info"`
				}
				if err := json.Unmarshal(data, &adset); err != nil {
					continue
				}

				// Sum spend and purchase value across insights rows for this adset.
				// Propagate Scan errors — discarding them would zero spend/value
				// on context cancellation or SQLITE_BUSY, ranking the adset with
				// incorrect ROAS metadata.
				var spend, value float64
				if err := db.DB().QueryRowContext(cmd.Context(),
					`SELECT
						COALESCE(SUM(CAST(json_extract(data, '$.spend') AS REAL)), 0),
						COALESCE(SUM(CAST(json_extract(data, '$.purchase_roas[0].value') AS REAL)), 0)
					 FROM resources
					 WHERE resource_type IN ('insights', 'adsets_insights')
					   AND json_extract(data, '$.adset_id') = ?`,
					adset.ID).Scan(&spend, &value); err != nil {
					return fmt.Errorf("aggregating insights for adset %s: %w", adset.ID, err)
				}

				roas := 0.0
				if spend > 0 {
					roas = value / spend
				}

				why := ""
				if adset.LearningStageInfo.Status == "LEARNING" {
					why = "stuck-in-learning"
				} else if adset.EffectiveStatus != "" && adset.EffectiveStatus != "ACTIVE" {
					why = "not-actively-delivering: " + adset.EffectiveStatus
				} else if spend > 0 && roas < 1.0 {
					why = "negative-roas"
				}

				out = append(out, bottleneckRow{
					AdsetID:         adset.ID,
					AdsetName:       adset.Name,
					CampaignID:      adset.CampaignID,
					Spend:           spend,
					Roas:            roas,
					EffectiveStatus: adset.EffectiveStatus,
					LearningStatus:  adset.LearningStageInfo.Status,
					Why:             why,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating adset rows: %w", err)
			}

			// Sort: highest spend × (1 / max(roas, 0.01)) — worst-leverage first.
			sort.SliceStable(out, func(i, j int) bool {
				si := out[i].Spend / max64(out[i].Roas, 0.01)
				sj := out[j].Spend / max64(out[j].Roas, 0.01)
				return si > sj
			})
			totalMatched := len(out)
			truncated := false
			if len(out) > limit {
				out = out[:limit]
				truncated = true
			}

			view := bottleneckView{
				Account:   flagAccount,
				Limit:     limit,
				Total:     totalMatched,
				Returned:  len(out),
				Truncated: truncated,
				Rows:      out,
			}
			if totalMatched == 0 {
				view.Note = "no adsets in local store; run 'meta-ads-pp-cli sync --resources adsets' first"
			} else if truncated {
				view.Note = fmt.Sprintf("%d adsets matched; --limit truncated to top %d. Raise --limit to see more.", totalMatched, limit)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagAccount, "account", "", "Filter to a specific ad account (e.g., act_1234567890)")
	cmd.Flags().StringVar(&flagLimit, "limit", "10", "Max rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.meta-ads-pp-cli/data.db)")
	return cmd
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
