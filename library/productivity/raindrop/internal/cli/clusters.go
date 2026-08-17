// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
package cli

import (
	"sort"

	"github.com/spf13/cobra"
)

func newClustersCmd(flags *rootFlags) *cobra.Command {
	var minSize int
	var dbPath string
	cmd := &cobra.Command{Use: "clusters", Short: "Discover explainable bookmark clusters from shared tags", Example: "  raindrop-pp-cli clusters --min-size 3 --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		bookmarks, err := loadLocalBookmarks(db)
		if err != nil {
			return err
		}
		byTag := map[string][]localBookmark{}
		for _, bookmark := range bookmarks {
			seen := map[string]bool{}
			for _, raw := range bookmark.Tags {
				tag := normalizedTag(raw)
				if tag != "" && !seen[tag] {
					byTag[tag] = append(byTag[tag], bookmark)
					seen[tag] = true
				}
			}
		}
		type cluster struct {
			Tag       string          `json:"tag"`
			Size      int             `json:"size"`
			Bookmarks []localBookmark `json:"bookmarks"`
		}
		var result []cluster
		for tag, items := range byTag {
			if len(items) >= minSize {
				result = append(result, cluster{tag, len(items), items})
			}
		}
		sort.Slice(result, func(i, j int) bool {
			if result[i].Size == result[j].Size {
				return result[i].Tag < result[j].Tag
			}
			return result[i].Size > result[j].Size
		})
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"count": len(result), "clusters": result}, flags)
	}}
	cmd.Flags().IntVar(&minSize, "min-size", 3, "Minimum bookmarks sharing a tag")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
