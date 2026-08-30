// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: find orphans across campaigns, ad groups, ads, and custom audiences.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. This command is local-only.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
	"github.com/spf13/cobra"
)

// orphanFinding is one orphan discovered in the store, grouped by kind.
type orphanFinding struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

const (
	orphanCampaignNoAdGroups = "campaign_without_ad_groups"
	orphanAdGroupNoAds       = "ad_group_without_ads"
	orphanAdNoAdGroup        = "ad_without_ad_group"
	orphanAudienceNoBid      = "audience_without_bid_multiplier"
)

// orphanData bundles the local-state inputs for classification so the pure
// logic can be unit-tested without a database.
type orphanData struct {
	campaigns       map[string]string // id -> name
	adGroups        map[string]string // id -> name
	adGroupCampaign map[string]string // ad_group_id -> campaign_id
	ads             map[string]string // id -> name
	adAdGroup       map[string]string // ad_id -> ad_group_id
	audiences       map[string]string // id -> name
	referencedAuds  map[string]bool   // audience ids referenced by any bid multiplier
}

// computeOrphans classifies all four orphan kinds from the bundled data.
func computeOrphans(d orphanData) []orphanFinding {
	adGroupNames := d.adGroups
	findings := make([]orphanFinding, 0)

	// campaigns with no ad groups
	hasAdGroup := map[string]bool{}
	for _, cid := range d.adGroupCampaign {
		hasAdGroup[cid] = true
	}
	campIDs := make([]string, 0, len(d.campaigns))
	for id := range d.campaigns {
		campIDs = append(campIDs, id)
	}
	sort.Strings(campIDs)
	for _, id := range campIDs {
		if !hasAdGroup[id] {
			findings = append(findings, orphanFinding{Kind: orphanCampaignNoAdGroups, ID: id, Name: d.campaigns[id]})
		}
	}

	// ad groups with no ads
	hasAd := map[string]bool{}
	for _, agid := range d.adAdGroup {
		hasAd[agid] = true
	}
	agIDs := make([]string, 0, len(d.adGroups))
	for id := range d.adGroups {
		agIDs = append(agIDs, id)
	}
	sort.Strings(agIDs)
	for _, id := range agIDs {
		if !hasAd[id] {
			findings = append(findings, orphanFinding{Kind: orphanAdGroupNoAds, ID: id, Name: d.adGroups[id]})
		}
	}

	// ads not referenced by any ad group
	adIDs := make([]string, 0, len(d.ads))
	for id := range d.ads {
		adIDs = append(adIDs, id)
	}
	sort.Strings(adIDs)
	for _, id := range adIDs {
		ag, ok := d.adAdGroup[id]
		if !ok || ag == "" {
			findings = append(findings, orphanFinding{Kind: orphanAdNoAdGroup, ID: id, Name: d.ads[id]})
			continue
		}
		if _, valid := adGroupNames[ag]; !valid {
			findings = append(findings, orphanFinding{Kind: orphanAdNoAdGroup, ID: id, Name: d.ads[id]})
		}
	}

	// custom audiences referenced by no bid multiplier
	aud := make([]string, 0, len(d.audiences))
	for id := range d.audiences {
		aud = append(aud, id)
	}
	sort.Strings(aud)
	for _, id := range aud {
		if !d.referencedAuds[id] {
			findings = append(findings, orphanFinding{Kind: orphanAudienceNoBid, ID: id, Name: d.audiences[id]})
		}
	}
	return findings
}

func newNovelOrphansCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orphans",
		Short: "Find ad groups with no ads, campaigns with no delivery, and audiences nothing references.",
		Long: `Find orphaned entities in the local store, grouped by kind:
campaigns with no ad groups, ad groups with no ads, ads not referenced by any
ad group, and custom audiences no bid multiplier references.`,
		Example: strings.Trim(`
  openai-ads-pp-cli orphans --agent
  openai-ads-pp-cli orphans --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "orphans")
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
					return printJSONFiltered(cmd.OutOrStdout(), make([]orphanFinding, 0), flags)
				}
				return nil
			}
			defer db.Close()

			data, err := loadOrphanData(db)
			if err != nil {
				return err
			}
			findings := computeOrphans(data)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), findings, flags)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no orphans found")
				return nil
			}
			maps, merr := rowsToMaps(findings)
			if merr != nil {
				return merr
			}
			return printAutoTable(cmd.OutOrStdout(), maps)
		},
	}
	return cmd
}

func loadOrphanData(db *store.Store) (orphanData, error) {
	var d orphanData
	d.campaigns = map[string]string{}
	d.adGroups = map[string]string{}
	d.adGroupCampaign = map[string]string{}
	d.ads = map[string]string{}
	d.adAdGroup = map[string]string{}
	d.audiences = map[string]string{}
	d.referencedAuds = map[string]bool{}

	rows, err := db.Query(`SELECT id, name FROM campaigns`)
	if err != nil {
		return d, fmt.Errorf("querying campaigns: %w", err)
	}
	for rows.Next() {
		var id, name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			_ = rows.Close()
			return d, err
		}
		d.campaigns[id.String] = name.String
	}
	if err := closeRows(rows); err != nil {
		return d, err
	}

	rows, err = db.Query(`SELECT id, name, campaign_id FROM ad_groups`)
	if err != nil {
		return d, fmt.Errorf("querying ad_groups: %w", err)
	}
	for rows.Next() {
		var id, name, cid sql.NullString
		if err := rows.Scan(&id, &name, &cid); err != nil {
			_ = rows.Close()
			return d, err
		}
		d.adGroups[id.String] = name.String
		d.adGroupCampaign[id.String] = cid.String
	}
	if err := closeRows(rows); err != nil {
		return d, err
	}

	rows, err = db.Query(`SELECT id, name, ad_group_id FROM ads`)
	if err != nil {
		return d, fmt.Errorf("querying ads: %w", err)
	}
	for rows.Next() {
		var id, name, agid sql.NullString
		if err := rows.Scan(&id, &name, &agid); err != nil {
			_ = rows.Close()
			return d, err
		}
		d.ads[id.String] = name.String
		d.adAdGroup[id.String] = agid.String
	}
	if err := closeRows(rows); err != nil {
		return d, err
	}

	rows, err = db.Query(`SELECT id, name FROM custom_audiences`)
	if err != nil {
		return d, fmt.Errorf("querying custom_audiences: %w", err)
	}
	for rows.Next() {
		var id, name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			_ = rows.Close()
			return d, err
		}
		d.audiences[id.String] = name.String
	}
	if err := closeRows(rows); err != nil {
		return d, err
	}

	// collect bid-multiplier referenced audiences from ad group bidding configs
	rows, err = db.Query(`SELECT data FROM ad_groups`)
	if err != nil {
		return d, fmt.Errorf("querying ad_groups data: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			return d, err
		}
		if !data.Valid || data.String == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data.String), &obj); err != nil {
			continue
		}
		referenceBidMultipliers(obj, d.referencedAuds)
	}
	if err := rows.Err(); err != nil {
		return d, err
	}
	return d, nil
}

// referenceBidMultipliers adds every audience id referenced by an ad group's
// bidding_config.custom_audience_bid_multipliers to the given set.
func referenceBidMultipliers(obj map[string]any, out map[string]bool) {
	bc, ok := obj["bidding_config"]
	if !ok {
		return
	}
	bcm, ok := bc.(map[string]any)
	if !ok {
		return
	}
	mult, ok := bcm["custom_audience_bid_multipliers"]
	if !ok {
		return
	}
	list, ok := mult.([]any)
	if !ok {
		return
	}
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := m["custom_audience_id"].(string); ok && id != "" {
			out[id] = true
		}
	}
}
