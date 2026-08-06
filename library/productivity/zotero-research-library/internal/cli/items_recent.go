// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: recently-added triage from the local cache.
// pp:data-source local

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelItemsRecentCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "What landed in the library recently, sorted by dateAdded from the local cache",
		Long:        "Lists items added within the last --days days (default 7), newest first, from the local cache. Attachments and notes are excluded so the list reads as papers, not files.",
		Example:     "  zotero-research-library-pp-cli items recent --days 7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagDays <= 0 {
				flagDays = 7
			}
			if flagLimit <= 0 {
				flagLimit = 50
			}
			ctx := cmd.Context()
			db, err := requireLocalCache(ctx, "zotero-research-library-pp-cli")
			if err != nil {
				return err
			}
			defer db.Close()

			// dateAdded is RFC3339 UTC, so a lexicographic >= on the cutoff
			// is a correct time comparison.
			cutoff := time.Now().UTC().AddDate(0, 0, -flagDays).Format(time.RFC3339)
			// The envelope-nested API leaves dedicated columns empty, so
			// read fields straight from the stored JSON. dateAdded is
			// RFC3339 UTC — lexicographic compare is a time compare.
			rows, err := db.DB().QueryContext(ctx,
				`SELECT "data" FROM "items"
				 WHERE json_extract("data", '$.data.dateAdded') >= ?
				   AND json_extract("data", '$.data.itemType') NOT IN ('attachment', 'note')
				 ORDER BY json_extract("data", '$.data.dateAdded') DESC LIMIT ?`, cutoff, flagLimit)
			if err != nil {
				return fmt.Errorf("querying recent items: %w", err)
			}
			defer rows.Close()

			type recentItem struct {
				Key       string `json:"key"`
				Title     string `json:"title"`
				Creators  string `json:"creators,omitempty"`
				ItemType  string `json:"item_type"`
				DateAdded string `json:"date_added"`
				DOI       string `json:"doi,omitempty"`
			}
			var items []recentItem
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					return err
				}
				d, err := decodeItemData(raw)
				if err != nil || d.Key == "" {
					continue
				}
				items = append(items, recentItem{
					Key:       d.Key,
					Title:     d.Title,
					Creators:  creatorSummary(d.Creators),
					ItemType:  d.ItemType,
					DateAdded: d.DateAdded,
					DOI:       d.DOI,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"days":  flagDays,
					"count": len(items),
					"items": items,
				}, flags)
			}
			out := cmd.OutOrStdout()
			if len(items) == 0 {
				fmt.Fprintf(out, "Nothing added in the last %d days (per the local cache — run 'sync' if it may be stale).\n", flagDays)
				return nil
			}
			fmt.Fprintf(out, "Added in the last %d days:\n", flagDays)
			for _, it := range items {
				day := it.DateAdded
				if len(day) >= 10 {
					day = day[:10]
				}
				fmt.Fprintf(out, "  %s  %s", day, it.Title)
				if it.Creators != "" {
					fmt.Fprintf(out, " — %s", it.Creators)
				}
				fmt.Fprintf(out, "  (key=%s)\n", it.Key)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 7, "Look-back window in days")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum items to list")
	return cmd
}
