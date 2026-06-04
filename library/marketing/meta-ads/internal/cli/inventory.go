// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads/internal/store"

	"github.com/spf13/cobra"
)

type inventoryExample struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type inventoryGroup struct {
	Key      string             `json:"key"`
	Count    int                `json:"count"`
	Examples []inventoryExample `json:"examples,omitempty"`
}

type inventoryView struct {
	Account       string           `json:"account,omitempty"`
	GroupBy       string           `json:"group_by"`
	TotalAds      int              `json:"total_ads"`
	Groups        []inventoryGroup `json:"groups"`
	MismatchedAds int              `json:"mismatched_ads"`
	Note          string           `json:"note,omitempty"`
}

func newNovelInventoryCmd(flags *rootFlags) *cobra.Command {
	var flagAccount string
	var flagBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Group every ad in an account by effective_status",
		Long: `Group every ad in an account by effective_status. Surfaces ads where
configured status='ACTIVE' but effective_status='WITH_ISSUES' or 'DISAPPROVED' —
silent failures the Meta UI buries behind per-ad clicks.

Requires 'meta-ads-pp-cli sync' to have populated the local store with ads first.`,
		Example: `  meta-ads-pp-cli inventory --account act_4327210487520472 --by effective_status --agent
  meta-ads-pp-cli inventory --by effective_status --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would group ads in local store by effective_status")
				return nil
			}
			if flagBy == "" {
				flagBy = "effective_status"
			}
			if flagBy != "effective_status" && flagBy != "status" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by must be 'effective_status' or 'status', got %q", flagBy))
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
				WHERE resource_type IN ('ads', 'adaccounts_ads', 'ad_accounts_ads')`
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

			groups := make(map[string]*inventoryGroup)
			total := 0
			mismatched := 0
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
				}
				if err := json.Unmarshal(data, &ad); err != nil {
					continue
				}
				total++
				key := ad.EffectiveStatus
				if flagBy == "status" {
					key = ad.Status
				}
				if key == "" {
					key = "UNKNOWN"
				}
				g, ok := groups[key]
				if !ok {
					g = &inventoryGroup{Key: key}
					groups[key] = g
				}
				g.Count++
				if ad.Status == "ACTIVE" && ad.EffectiveStatus != "" && ad.EffectiveStatus != "ACTIVE" {
					mismatched++
				}
				if len(g.Examples) < 3 {
					g.Examples = append(g.Examples, inventoryExample{
						ID: ad.ID, Name: ad.Name, Status: ad.Status,
					})
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating ad rows: %w", err)
			}

			view := inventoryView{
				Account:       flagAccount,
				GroupBy:       flagBy,
				TotalAds:      total,
				Groups:        make([]inventoryGroup, 0, len(groups)),
				MismatchedAds: mismatched,
			}
			for _, g := range groups {
				view.Groups = append(view.Groups, *g)
			}
			// Deterministic order so successive runs produce diff-stable output.
			sort.Slice(view.Groups, func(i, j int) bool {
				return view.Groups[i].Key < view.Groups[j].Key
			})
			if total == 0 {
				view.Note = "no ads found in local store; run 'meta-ads-pp-cli sync --path-context adAccountId=act_<id> --resources ads' first"
			} else if mismatched > 0 {
				view.Note = fmt.Sprintf("%d ads have configured status='ACTIVE' but a different effective_status — review these first", mismatched)
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagAccount, "account", "", "Filter to a specific ad account (e.g., act_1234567890)")
	cmd.Flags().StringVar(&flagBy, "by", "effective_status", "Group key: effective_status or status")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.meta-ads-pp-cli/data.db)")
	return cmd
}
