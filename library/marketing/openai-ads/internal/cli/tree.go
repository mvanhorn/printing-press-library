// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: campaign > ad group > ad hierarchy from the local store.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. This command is local-only.

package cli

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
	"github.com/spf13/cobra"
)

// treeCampaign is the campaign level of the rendered hierarchy.
type treeCampaign struct {
	Type     string        `json:"type"`
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Budget   string        `json:"budget"` // rendered money
	AdGroups []treeAdGroup `json:"ad_groups"`
}

// treeAdGroup is the ad group level of the rendered hierarchy.
type treeAdGroup struct {
	Type       string   `json:"type"`
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	CampaignID string   `json:"campaign_id"`
	MaxBid     string   `json:"max_bid"` // rendered money
	Ads        []treeAd `json:"ads"`
}

// treeAd is the ad level of the rendered hierarchy.
type treeAd struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	AdGroupID    string `json:"ad_group_id"`
	ReviewStatus string `json:"review_status"`
}

func newNovelTreeCmd(flags *rootFlags) *cobra.Command {
	var flagCampaignID string
	var flagStatus string

	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Render the whole campaign, ad group, and ad hierarchy with status, budget, and review state.",
		Long:  "Render the local campaign > ad group > ad hierarchy with status, budget (campaign), max bid (ad group) and review status (ad). Budgets and bids are rendered in the account currency, not raw micros.",
		Example: strings.Trim(`
  openai-ads-pp-cli tree --agent
  openai-ads-pp-cli tree --campaign-id cmpn_9x2VfkRgcZ
  openai-ads-pp-cli tree --status ACTIVE
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tree")
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
					return printJSONFiltered(cmd.OutOrStdout(), make([]treeCampaign, 0), flags)
				}
				return nil
			}
			defer db.Close()
			currency := accountCurrency(db)

			campaigns, err := loadTreeCampaigns(db, flagCampaignID, flagStatus, currency)
			if err != nil {
				return err
			}
			if err := attachTreeAdGroups(db, campaigns, currency); err != nil {
				return err
			}
			if err := attachTreeAds(db, campaigns); err != nil {
				return err
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), campaigns, flags)
			}
			return renderTreeHuman(cmd.OutOrStdout(), campaigns)
		},
	}
	cmd.Flags().StringVar(&flagCampaignID, "campaign-id", "", "Only show the subtree rooted at this campaign id.")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Only show campaigns with this status (e.g. ACTIVE, PAUSED).")
	return cmd
}

// loadTreeCampaigns reads campaign rows (optionally filtered) as the tree root.
func loadTreeCampaigns(db *store.Store, campaignID, status, currency string) ([]treeCampaign, error) {
	query := `SELECT id, name, status,
		COALESCE(json_extract(data, '$.budget.daily_spend_limit_micros'), json_extract(data, '$.budget.lifetime_spend_limit_micros')) AS budget_micros
		FROM campaigns`
	var clauses []string
	var args []any
	if campaignID != "" {
		clauses = append(clauses, "id = ?")
		args = append(args, campaignID)
	}
	if status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, status)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY name"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying campaigns: %w", err)
	}
	defer rows.Close()

	result := make([]treeCampaign, 0)
	for rows.Next() {
		var (
			id, name, statusVar sql.NullString
			budget              sql.NullInt64
		)
		if err := rows.Scan(&id, &name, &statusVar, &budget); err != nil {
			return nil, fmt.Errorf("scanning campaign: %w", err)
		}
		result = append(result, treeCampaign{
			Type:     "campaign",
			ID:       id.String,
			Name:     name.String,
			Status:   statusVar.String,
			Budget:   renderNullableMicros(budget, currency),
			AdGroups: make([]treeAdGroup, 0),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// attachTreeAdGroups loads ad groups and nests them under their campaign.
func attachTreeAdGroups(db *store.Store, campaigns []treeCampaign, currency string) error {
	campaignIdx := make(map[string]int, len(campaigns))
	for i := range campaigns {
		campaignIdx[campaigns[i].ID] = i
	}
	rows, err := db.Query(`SELECT id, name, status, campaign_id,
		json_extract(data, '$.bidding_config.max_bid_micros') AS bid_micros
		FROM ad_groups ORDER BY name`)
	if err != nil {
		return fmt.Errorf("querying ad_groups: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id, name, statusVar, campaignID sql.NullString
			bid                             sql.NullInt64
		)
		if err := rows.Scan(&id, &name, &statusVar, &campaignID, &bid); err != nil {
			return fmt.Errorf("scanning ad_group: %w", err)
		}
		ci, ok := campaignIdx[campaignID.String]
		if !ok {
			continue // orphan or filtered-out campaign
		}
		campaigns[ci].AdGroups = append(campaigns[ci].AdGroups, treeAdGroup{
			Type:       "ad_group",
			ID:         id.String,
			Name:       name.String,
			Status:     statusVar.String,
			CampaignID: campaignID.String,
			MaxBid:     renderNullableMicros(bid, currency),
			Ads:        make([]treeAd, 0),
		})
	}
	return rows.Err()
}

// attachTreeAds loads ads and nests them under their ad group.
func attachTreeAds(db *store.Store, campaigns []treeCampaign) error {
	rows, err := db.Query(`SELECT id, name, status, ad_group_id, review_status FROM ads ORDER BY name`)
	if err != nil {
		return fmt.Errorf("querying ads: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id, name, statusVar, adGroupID, review sql.NullString
		)
		if err := rows.Scan(&id, &name, &statusVar, &adGroupID, &review); err != nil {
			return fmt.Errorf("scanning ad: %w", err)
		}
		for ci := range campaigns {
			for ai := range campaigns[ci].AdGroups {
				if campaigns[ci].AdGroups[ai].ID == adGroupID.String {
					campaigns[ci].AdGroups[ai].Ads = append(campaigns[ci].AdGroups[ai].Ads, treeAd{
						Type:         "ad",
						ID:           id.String,
						Name:         name.String,
						Status:       statusVar.String,
						AdGroupID:    adGroupID.String,
						ReviewStatus: review.String,
					})
				}
			}
		}
	}
	return rows.Err()
}

// renderTreeHuman prints an indented hierarchy line per node.
func renderTreeHuman(w io.Writer, campaigns []treeCampaign) error {
	for _, cam := range campaigns {
		fmt.Fprintf(w, "%s  %-28s  %-10s  budget %s\n", cam.ID, cam.Name, dash(cam.Status), dash(cam.Budget))
		for _, ag := range cam.AdGroups {
			fmt.Fprintf(w, "  %s  %-26s  %-10s  bid %s\n", ag.ID, ag.Name, dash(ag.Status), dash(ag.MaxBid))
			for _, ad := range ag.Ads {
				fmt.Fprintf(w, "    %s  %-24s  %-10s  review %s\n", ad.ID, ad.Name, dash(ad.Status), dash(ad.ReviewStatus))
			}
		}
	}
	return nil
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
