// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/internal/store"

	"github.com/spf13/cobra"
)

type staleAd struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	EffectiveStatus string `json:"effective_status"`
	AdsetID         string `json:"adset_id,omitempty"`
	CampaignID      string `json:"campaign_id,omitempty"`
	UpdatedTime     string `json:"updated_time,omitempty"`
}

type staleView struct {
	Days     int       `json:"days"`
	Account  string    `json:"account,omitempty"`
	Total    int       `json:"total"`
	StaleAds []staleAd `json:"stale_ads"`
	Note     string    `json:"note,omitempty"`
}

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays string
	var flagAccount string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Active ads with zero impressions in N days.",
		Long: `Find ads with status='ACTIVE' that have no impressions in the local store
for at least N days. These are usually misconfigured ads or post-deletion zombies
burning configuration without delivering. Run after a sync so the local store reflects
current insights.`,
		Example: `  meta-ads-pp-cli stale --days 90 --agent
  meta-ads-pp-cli stale --account act_4327210487520472 --days 30 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan ads in local store for zero-impression stale rows")
				return nil
			}
			days := 90
			if flagDays != "" {
				n, err := strconv.Atoi(flagDays)
				if err != nil || n <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--days must be a positive integer, got %q", flagDays))
				}
				days = n
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
				WHERE resource_type IN ('ads', 'adaccounts_ads', 'ad_accounts_ads')
				  AND json_extract(data, '$.status') = 'ACTIVE'`
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

			view := staleView{
				Days:     days,
				Account:  flagAccount,
				StaleAds: make([]staleAd, 0),
			}

			// Walk all active ads, cross-reference against the insights resource_type.
			// For each ad, count rows in the insights table with non-zero impressions in the last N days.
			// Empty implementation if no insights data has been synced — surfaces all active ads as candidates.
			for rows.Next() {
				var id string
				var data []byte
				if err := rows.Scan(&id, &data); err != nil {
					continue
				}
				var ad struct {
					ID              string `json:"id"`
					Name            string `json:"name"`
					Status          string `json:"status"`
					EffectiveStatus string `json:"effective_status"`
					AdsetID         string `json:"adset_id"`
					CampaignID      string `json:"campaign_id"`
					UpdatedTime     string `json:"updated_time"`
				}
				if err := json.Unmarshal(data, &ad); err != nil {
					continue
				}

				// Count non-zero impression rows for this ad in insights.
				// Propagate Scan errors — discarding them would zero hasImpressions
				// on context cancellation or SQLITE_BUSY, falsely classifying
				// actively-delivering ads as stale.
				var hasImpressions int
				if err := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM resources
					 WHERE resource_type IN ('insights', 'ads_insights', 'adaccounts_insights')
					   AND json_extract(data, '$.ad_id') = ?
					   AND CAST(json_extract(data, '$.impressions') AS INTEGER) > 0
					   AND date(json_extract(data, '$.date_start')) > date('now', ?)`,
					ad.ID, fmt.Sprintf("-%d days", days)).Scan(&hasImpressions); err != nil {
					return fmt.Errorf("counting impressions for ad %s: %w", ad.ID, err)
				}
				if hasImpressions == 0 {
					view.StaleAds = append(view.StaleAds, staleAd{
						ID:              ad.ID,
						Name:            ad.Name,
						Status:          ad.Status,
						EffectiveStatus: ad.EffectiveStatus,
						AdsetID:         ad.AdsetID,
						CampaignID:      ad.CampaignID,
						UpdatedTime:     ad.UpdatedTime,
					})
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating ad rows: %w", err)
			}
			view.Total = len(view.StaleAds)
			if view.Total == 0 {
				view.Note = "no stale ads found, or local store has no synced ads/insights yet"
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagDays, "days", "90", "Days without impressions to consider an ad stale")
	cmd.Flags().StringVar(&flagAccount, "account", "", "Filter to a specific ad account (e.g., act_1234567890)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.meta-ads-pp-cli/data.db)")
	return cmd
}
