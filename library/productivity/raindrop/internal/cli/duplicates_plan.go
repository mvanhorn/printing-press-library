// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelDuplicatesPlanCmd(flags *rootFlags) *cobra.Command {
	var flagCanonical string
	var dbPath string

	cmd := &cobra.Command{
		Use:     "plan",
		Short:   "Choose canonical bookmarks while preserving tags, notes and highlights.",
		Example: "  raindrop-pp-cli duplicates plan --canonical richest --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "duplicates plan")
			}
			if flagCanonical != "richest" && flagCanonical != "newest" && flagCanonical != "oldest" {
				return fmt.Errorf("--canonical must be richest, newest, or oldest")
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
			groups := map[string][]localBookmark{}
			for _, item := range items {
				key := canonicalBookmarkURL(item.Link)
				if key != "" {
					groups[key] = append(groups[key], item)
				}
			}
			var plans []map[string]any
			for _, key := range sortedKeys(groups) {
				group := groups[key]
				if len(group) < 2 {
					continue
				}
				sort.SliceStable(group, func(i, j int) bool {
					switch flagCanonical {
					case "newest":
						return group[i].Created.After(group[j].Created)
					case "oldest":
						return group[i].Created.Before(group[j].Created)
					default:
						return bookmarkRichness(group[i]) > bookmarkRichness(group[j])
					}
				})
				remove := make([]string, 0, len(group)-1)
				for _, duplicate := range group[1:] {
					remove = append(remove, duplicate.ID)
				}
				plans = append(plans, map[string]any{
					"canonical_url": key,
					"keep":          group[0].ID,
					"remove":        remove,
					"merge_tags":    mergedTags(group),
					"merge_note":    mergedNote(group),
					"highlights":    mergedHighlights(group),
					"richness":      bookmarkRichness(group[0]),
				})
			}
			payload, _ := json.Marshal(plans)
			res, err := db.DB().ExecContext(cmd.Context(), `INSERT INTO cleanup_plans(kind,payload) VALUES('duplicates',?)`, string(payload))
			if err != nil {
				return err
			}
			planID, _ := res.LastInsertId()
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan_id": planID, "canonical_policy": flagCanonical, "groups": plans, "duplicate_groups": len(plans)}, flags)
		},
	}
	cmd.Flags().StringVar(&flagCanonical, "canonical", "richest", "Canonical selection: richest, newest, or oldest")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func mergedNote(items []localBookmark) string {
	seen := map[string]struct{}{}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		note := strings.TrimSpace(item.Note)
		if note == "" {
			continue
		}
		if _, ok := seen[note]; ok {
			continue
		}
		seen[note] = struct{}{}
		parts = append(parts, note)
	}
	return strings.Join(parts, "\n\n")
}

func mergedHighlights(items []localBookmark) []map[string]any {
	seen := map[string]struct{}{}
	result := make([]map[string]any, 0)
	for _, item := range items {
		for _, highlight := range item.Highlights {
			clean := highlightMutationBody(highlight)
			if valueString(clean["text"]) == "" {
				continue
			}
			key := highlightSignature(clean)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, clean)
		}
	}
	return result
}

func mergedTags(items []localBookmark) []string {
	seen := map[string]string{}
	for _, item := range items {
		for _, tag := range item.Tags {
			key := normalizedTag(tag)
			if key != "" {
				if _, ok := seen[key]; !ok {
					seen[key] = tag
				}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for _, tag := range seen {
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}
