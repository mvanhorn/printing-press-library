// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type qcDeliveryRank struct {
	Platform  string `json:"platform"`
	ETA       string `json:"eta"`
	Minutes   int    `json:"minutes,omitempty"`
	Open      any    `json:"open,omitempty"`
	StoreID   string `json:"store_id,omitempty"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

var qcETANumber = regexp.MustCompile(`\d+`)

func newNovelDeliveryFastestCmd(flags *rootFlags) *cobra.Command {
	var location, dbPath string
	cmd := &cobra.Command{Use: "fastest", Short: "Rank currently available delivery options while preserving closed or unparseable platforms.", Example: "  quickcommerce-pp-cli delivery fastest --location 12.9021,77.6639 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "delivery fastest")
		}
		if location == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--location is required as latitude,longitude"))
		}
		_, _, canonical, err := parseQCLocation(location)
		if err != nil {
			return usageErr(err)
		}
		path := qcDBPath(dbPath)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return qcMissingMirror(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, path, "delivery")
		}
		if err != nil {
			return apiErr(err)
		}
		db, err := openQCCreatable(cmd.Context(), path)
		if err != nil {
			return err
		}
		defer db.Close()
		rows, err := db.DB().QueryContext(cmd.Context(), `SELECT platform, eta, open, store_id FROM quickcommerce_observations WHERE resource='delivery' AND location=? ORDER BY captured_at DESC`, canonical)
		if err != nil {
			return apiErr(fmt.Errorf("querying delivery observations: %w", err))
		}
		defer rows.Close()
		latest := map[string]qcDeliveryRank{}
		for rows.Next() {
			var platform, eta, storeID string
			var open sql.NullBool
			if err := rows.Scan(&platform, &eta, &open, &storeID); err != nil {
				return apiErr(err)
			}
			if _, ok := latest[platform]; ok {
				continue
			}
			rank := qcDeliveryRank{Platform: platform, ETA: eta, StoreID: storeID, Available: open.Valid && open.Bool}
			if !rank.Available {
				rank.Reason = "closed or unavailable"
			}
			if match := qcETANumber.FindString(eta); match != "" {
				rank.Minutes, _ = strconv.Atoi(match)
			} else {
				rank.Available = false
				rank.Reason = "ETA is not numeric"
			}
			latest[platform] = rank
		}
		if err := rows.Err(); err != nil {
			return apiErr(err)
		}
		all := make([]qcDeliveryRank, 0, len(latest))
		for _, r := range latest {
			all = append(all, r)
		}
		sort.SliceStable(all, func(i, j int) bool {
			if all[i].Available != all[j].Available {
				return all[i].Available
			}
			if all[i].Minutes == 0 {
				return false
			}
			if all[j].Minutes == 0 {
				return true
			}
			return all[i].Minutes < all[j].Minutes
		})
		view := map[string]any{"location": canonical, "results": all, "ranked_count": countAvailable(all)}
		if len(all) == 0 {
			view["note"] = "no local ETA observations found; run delivery compare and pipe its JSON into mirror ingest"
		}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		}
		if len(all) == 0 {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "No local delivery observations found.")
			return err
		}
		return qcPrint(cmd.OutOrStdout(), flags, view, deliveryTable(all))
	}}
	cmd.Flags().StringVar(&location, "location", "", "Delivery coordinates as latitude,longitude")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path")
	return cmd
}
func countAvailable(rows []qcDeliveryRank) int {
	n := 0
	for _, r := range rows {
		if r.Available && r.Minutes > 0 {
			n++
		}
	}
	return n
}
func deliveryTable(rows []qcDeliveryRank) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{"platform": r.Platform, "eta": r.ETA, "available": r.Available, "reason": strings.TrimSpace(r.Reason)})
	}
	return out
}
