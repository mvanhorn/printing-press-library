// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

func newDiffCmd(flags *rootFlags) *cobra.Command {
	var region string
	var since string
	var changeType string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Show what changed in a region since your last sync",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Compare current listings to detect new listings and updates since a given time.
Reads from the local synced store — run 'redfin-pp-cli sync' first.

Sync a region first:
  redfin-pp-cli sync --param region_id=<zip> --param region_type=2 --param al=1 --param num_homes=500`,
		Example: `  # See everything new or changed in zip 94102 in the last 24h
  redfin-pp-cli diff --region 94102

  # Only new listings from the last 7 days
  redfin-pp-cli diff --region 94102 --since 7d --type new_listing

  # JSON output for agent use
  redfin-pp-cli diff --region 94102 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if region == "" {
				return usageErr(fmt.Errorf("--region is required (zip code)"))
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'redfin-pp-cli sync' first.", err)
			}
			defer db.Close()

			var sinceThreshold time.Time
			if since != "" {
				t, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since: %w", err))
				}
				sinceThreshold = t
			} else {
				sinceThreshold = time.Now().Add(-24 * time.Hour)
			}
			sinceStr := sinceThreshold.UTC().Format("2006-01-02T15:04:05Z")

			query := `
				SELECT data, updated_at, synced_at
				FROM resources
				WHERE resource_type = 'listings'
				  AND json_extract(data, '$.address.zip') = ?
				  AND (updated_at >= ? OR synced_at >= ?)
				ORDER BY updated_at DESC
				LIMIT ?`

			rows, err := db.DB().QueryContext(cmd.Context(), query, region, sinceStr, sinceStr, limit)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("no data found — run 'redfin-pp-cli sync --param region_id=%s --param region_type=2 --param al=1 --param num_homes=500' first", region)
				}
				return fmt.Errorf("querying diff: %w", err)
			}
			defer rows.Close()

			type DiffEntry struct {
				ListingID  any    `json:"listing_id,omitempty"`
				Address    string `json:"address,omitempty"`
				Price      any    `json:"price,omitempty"`
				DOM        any    `json:"dom,omitempty"`
				Status     string `json:"status"`
				ChangeType string `json:"change_type"`
				UpdatedAt  string `json:"updated_at"`
			}

			var results []DiffEntry
			for rows.Next() {
				var dataJSON []byte
				var updatedAt, syncedAt string
				if err := rows.Scan(&dataJSON, &updatedAt, &syncedAt); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal(dataJSON, &obj); err != nil {
					continue
				}

				ct := diffChangeType(updatedAt, syncedAt)
				if changeType != "" && ct != changeType {
					continue
				}

				entry := DiffEntry{
					ListingID:  obj["listingId"],
					ChangeType: ct,
					UpdatedAt:  updatedAt,
					Status:     redfinStatusLabel(obj["status"]),
				}
				if addr, ok := obj["address"].(map[string]any); ok {
					entry.Address = fmt.Sprintf("%v, %v, %v %v",
						addr["street"], addr["city"], addr["state"], addr["zip"])
				}
				if p, ok := obj["price"].(map[string]any); ok {
					entry.Price = p["value"]
				}
				if d, ok := obj["dom"].(map[string]any); ok {
					entry.DOM = d["value"]
				} else {
					entry.DOM = obj["dom"]
				}
				results = append(results, entry)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			if len(results) == 0 {
				if flags.asJSON {
					fmt.Fprintln(os.Stdout, "[]")
				} else {
					fmt.Fprintf(os.Stdout, "No changes found in %s since %s.\n", region, sinceThreshold.Format("2006-01-02 15:04"))
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&region, "region", "", "Zip code to diff (required)")
	cmd.Flags().StringVar(&since, "since", "", "How far back to look (e.g. 24h, 7d). Default: 24h")
	cmd.Flags().StringVar(&changeType, "type", "", "Filter: new_listing or updated")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max listings to scan")

	return cmd
}

func diffChangeType(updatedAt, syncedAt string) string {
	if updatedAt == "" || syncedAt == "" || updatedAt == syncedAt {
		return "new_listing"
	}
	u, err1 := time.Parse("2006-01-02T15:04:05Z", updatedAt)
	s, err2 := time.Parse("2006-01-02T15:04:05Z", syncedAt)
	if err1 != nil || err2 != nil {
		return "updated"
	}
	diff := u.Sub(s)
	if diff < 0 {
		diff = -diff
	}
	if diff < 10*time.Second {
		return "new_listing"
	}
	return "updated"
}

func redfinStatusLabel(v any) string {
	code, ok := v.(float64)
	if !ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	switch int(code) {
	case 1:
		return "Active"
	case 2:
		return "Contingent"
	case 3:
		return "Pending"
	case 4:
		return "Sold"
	case 5:
		return "Off Market"
	default:
		return fmt.Sprintf("Status(%d)", int(code))
	}
}
