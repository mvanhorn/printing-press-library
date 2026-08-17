// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"sort"

	"github.com/spf13/cobra"
)

func newNovelTagHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "health",
		Short:       "Discover case variants, near-duplicates, singleton tags and merge candidates.",
		Example:     "  raindrop-pp-cli tag health raindrop tag health --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate required flags here
			if dryRunOK(flags) {
				return nil
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
			counts := map[string]int{}
			variants := map[string]map[string]int{}
			for _, item := range items {
				for _, tag := range item.Tags {
					counts[tag]++
					key := normalizedTag(tag)
					if variants[key] == nil {
						variants[key] = map[string]int{}
					}
					variants[key][tag]++
				}
			}
			var collisions []map[string]any
			var singletons []string
			for tag, count := range counts {
				if count == 1 {
					singletons = append(singletons, tag)
				}
			}
			for key, names := range variants {
				if key == "" || len(names) < 2 {
					continue
				}
				display := sortedKeys(names)
				total := 0
				for _, count := range names {
					total += count
				}
				collisions = append(collisions, map[string]any{"normalized": key, "variants": display, "count": total})
			}
			sort.Strings(singletons)
			sort.Slice(collisions, func(i, j int) bool {
				return collisions[i]["normalized"].(string) < collisions[j]["normalized"].(string)
			})
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"tag_count": len(counts), "variant_groups": collisions, "singletons": singletons, "singleton_count": len(singletons)}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
