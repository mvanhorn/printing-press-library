// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelRelatedCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var dbPath string
	var bookmarkID string

	cmd := &cobra.Command{
		Use:         "related",
		Args:        cobra.MaximumNArgs(1),
		Short:       "Find explainable related bookmarks offline using text, tags and domains.",
		Example:     "  raindrop-pp-cli related --limit 10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "related bookmarks")
			}
			db, _, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			items, err := loadLocalBookmarks(db)
			if err != nil {
				return err
			}
			var targetID = bookmarkID
			if targetID == "" {
				if len(args) == 1 {
					targetID = args[0]
				} else if len(items) > 0 {
					targetID = items[0].ID
				}
			}
			var target *localBookmark
			for i := range items {
				if items[i].ID == targetID {
					target = &items[i]
					break
				}
			}
			if target == nil {
				if targetID == "" {
					return fmt.Errorf("local mirror is empty; run 'raindrop-pp-cli sync' first")
				}
				return fmt.Errorf("bookmark %s not found in local mirror", targetID)
			}
			type scored struct {
				Bookmark localBookmark `json:"bookmark"`
				Score    float64       `json:"score"`
			}
			var matches []scored
			for _, item := range items {
				if item.ID == target.ID {
					continue
				}
				if score := overlapScore(*target, item); score > 0 {
					matches = append(matches, scored{item, score})
				}
			}
			sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
			if flagLimit > 0 && len(matches) > flagLimit {
				matches = matches[:flagLimit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"bookmark_id": target.ID, "matches": matches, "count": len(matches)}, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum related bookmarks")
	cmd.Flags().StringVar(&bookmarkID, "bookmark-id", "", "Bookmark ID; defaults to the first bookmark in the local mirror")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
