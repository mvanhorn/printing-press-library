// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newNovelRevisitCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:     "revisit",
		Short:   "Resurface useful forgotten bookmarks without repeating recent suggestions.",
		Example: "  raindrop-pp-cli revisit --older-than 180d --limit 20 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "revisit")
			}
			age, err := parseAge(flagOlderThan, 180*24*time.Hour)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-age)
			db, _, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			items, err := loadLocalBookmarks(db)
			if err != nil {
				return err
			}
			type candidate struct {
				Bookmark localBookmark `json:"bookmark"`
				Score    int           `json:"score"`
			}
			var candidates []candidate
			for _, item := range items {
				basis := item.LastUpdate
				if basis.IsZero() {
					basis = item.Created
				}
				if basis.IsZero() || basis.After(cutoff) {
					continue
				}
				var shown string
				if err := db.DB().QueryRowContext(cmd.Context(), `SELECT COALESCE(last_shown_at,'') FROM reading_state WHERE bookmark_id=?`, item.ID).Scan(&shown); err == nil {
					if last := valueTime(shown); !last.IsZero() && last.After(time.Now().Add(-30*24*time.Hour)) {
						continue
					}
				}
				score := bookmarkRichness(item) + int(time.Since(basis).Hours()/(24*90))
				candidates = append(candidates, candidate{item, score})
			}
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
			if flagLimit > 0 && len(candidates) > flagLimit {
				candidates = candidates[:flagLimit]
			}
			now := time.Now().UTC().Format(time.RFC3339)
			for _, item := range candidates {
				_, _ = db.DB().ExecContext(cmd.Context(), `INSERT INTO reading_state(bookmark_id,status,last_shown_at) VALUES(?,'queued',?) ON CONFLICT(bookmark_id) DO UPDATE SET status='queued',last_shown_at=excluded.last_shown_at`, item.Bookmark.ID, now)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"older_than": flagOlderThan, "items": candidates, "count": len(candidates)}, flags)
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "180d", "Minimum age (for example 90d or 52w)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum bookmarks")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
