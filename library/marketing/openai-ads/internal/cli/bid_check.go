// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: flag ad groups whose max bid is irrational against the parent campaign budget.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. This command is local-only.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
	"github.com/spf13/cobra"
)

// bidCheckRow is one flagged ad group.
type bidCheckRow struct {
	AdGroupID           string  `json:"ad_group_id"`
	Name                string  `json:"name"`
	Status              string  `json:"status"`
	CampaignID          string  `json:"campaign_id"`
	DailyBudget         string  `json:"daily_budget"` // rendered money
	MaxBid              string  `json:"max_bid"`      // rendered money
	ImpliedClicksPerDay float64 `json:"implied_clicks_per_day"`
	MinClicks           int     `json:"min_clicks"`
}

// bidImpliedClicksPerDay is the number of clicks a day's budget could buy at
// the maximum bid. Returns 0 when the max bid is missing/zero (the creative
// cannot run at all, which is always a flag below min-clicks).
func bidImpliedClicksPerDay(dailyBudgetMicros, maxBidMicros int64) float64 {
	if maxBidMicros <= 0 {
		return 0
	}
	return float64(dailyBudgetMicros) / float64(maxBidMicros)
}

// bidIsFlagged reports whether an ad group's implied daily clicks clear the
// minimum threshold.
func bidIsFlagged(implied float64, minClicks int) bool {
	return implied < float64(minClicks)
}

func newNovelBidCheckCmd(flags *rootFlags) *cobra.Command {
	var flagMinClicks int

	cmd := &cobra.Command{
		Use:   "bid-check",
		Short: "Flag ad groups whose maximum bid is irrational against the parent campaign budget.",
		Long: `Join each ad group's max bid to its parent campaign's daily budget and flag
any ad group that could not generate at least --min-clicks clicks per day.
implied_clicks_per_day = daily_budget_micros / max_bid_micros.`,
		Example: strings.Trim(`
  openai-ads-pp-cli bid-check --agent
  openai-ads-pp-cli bid-check --min-clicks 20
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "bid-check")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := openStoreForRead(ctx, "openai-ads-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: run 'openai-ads-pp-cli sync' first to populate the local database.")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]bidCheckRow, 0), flags)
				}
				return nil
			}
			defer db.Close()

			minClicks := flagMinClicks
			if minClicks <= 0 {
				minClicks = 10 // default per brief
			}
			rows, err := loadBidCheckRows(db, minClicks)
			if err != nil {
				return err
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			maps, merr := rowsToMaps(rows)
			if merr != nil {
				return merr
			}
			return printAutoTable(cmd.OutOrStdout(), maps)
		},
	}
	cmd.Flags().IntVar(&flagMinClicks, "min-clicks", 10, "Minimum implied clicks per day before an ad group is flagged.")
	return cmd
}

// loadBidCheckRows joins ad groups to their parent campaign and returns only
// the flagged rows.
func loadBidCheckRows(db *store.Store, minClicks int) ([]bidCheckRow, error) {
	rows, err := db.Query(`SELECT ag.id, ag.name, ag.status, ag.campaign_id,
		json_extract(ag.data, '$.bidding_config.max_bid_micros') AS max_bid,
		COALESCE(
			json_extract(c.data, '$.budget.daily_spend_limit_micros'),
			json_extract(c.data, '$.budget.lifetime_spend_limit_micros')
		) AS budget
		FROM ad_groups ag
		LEFT JOIN campaigns c ON c.id = ag.campaign_id`)
	if err != nil {
		return nil, fmt.Errorf("querying bid check: %w", err)
	}
	defer rows.Close()

	currency := accountCurrency(db)
	result := make([]bidCheckRow, 0)
	for rows.Next() {
		var (
			id, name, status, campaignID sql.NullString
			maxBid, budget               sql.NullInt64
		)
		if err := rows.Scan(&id, &name, &status, &campaignID, &maxBid, &budget); err != nil {
			return nil, fmt.Errorf("scanning bid check row: %w", err)
		}
		var budgetMicros int64
		if budget.Valid {
			budgetMicros = budget.Int64
		}
		var maxBidMicros int64
		if maxBid.Valid {
			maxBidMicros = maxBid.Int64
		}
		implied := bidImpliedClicksPerDay(budgetMicros, maxBidMicros)
		if !bidIsFlagged(implied, minClicks) {
			continue
		}
		result = append(result, bidCheckRow{
			AdGroupID:           id.String,
			Name:                name.String,
			Status:              status.String,
			CampaignID:          campaignID.String,
			DailyBudget:         renderNullableMicros(budget, currency),
			MaxBid:              renderNullableMicros(maxBid, currency),
			ImpliedClicksPerDay: round2(implied),
			MinClicks:           minClicks,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_ = result
	return result, nil
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
