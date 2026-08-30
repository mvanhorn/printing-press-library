// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelResearchDiffCmd(flags *rootFlags) *cobra.Command {
	var beforeID, afterID string
	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Compare saved runs for URL changes, rank movement, metadata edits, and content-hash drift.",
		Example:     "  keenable-pp-cli research diff --before latest --after latest --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "research diff")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := openResearchStore(ctx)
			if err != nil {
				return fmt.Errorf("opening local research store: %w", err)
			}
			defer s.Close()
			before, err := loadResearchSnapshot(s, beforeID)
			if err != nil {
				return err
			}
			after, err := loadResearchSnapshot(s, afterID)
			if err != nil {
				return err
			}
			beforeResults, err := loadSnapshotResults(s, before.ID)
			if err != nil {
				return err
			}
			afterResults, err := loadSnapshotResults(s, after.ID)
			if err != nil {
				return err
			}
			beforeMap := map[string]researchResult{}
			afterMap := map[string]researchResult{}
			for _, item := range beforeResults {
				beforeMap[item.URL] = item
			}
			for _, item := range afterResults {
				afterMap[item.URL] = item
			}
			added, removed := make([]researchResult, 0), make([]researchResult, 0)
			rankChanges := make([]map[string]any, 0)
			for rawURL, item := range afterMap {
				old, ok := beforeMap[rawURL]
				if !ok {
					added = append(added, item)
					continue
				}
				if old.Rank != item.Rank || old.Title != item.Title || old.Description != item.Description {
					rankChanges = append(rankChanges, map[string]any{"url": rawURL, "before_rank": old.Rank, "after_rank": item.Rank, "title_changed": old.Title != item.Title, "description_changed": old.Description != item.Description})
				}
			}
			for rawURL, item := range beforeMap {
				if _, ok := afterMap[rawURL]; !ok {
					removed = append(removed, item)
				}
			}
			beforePages, _ := loadSnapshotPages(s, before.ID)
			afterPages, _ := loadSnapshotPages(s, after.ID)
			beforeHashes, afterHashes := map[string]string{}, map[string]string{}
			for _, p := range beforePages {
				beforeHashes[p.URL] = p.ContentHash
			}
			for _, p := range afterPages {
				afterHashes[p.URL] = p.ContentHash
			}
			contentChanges := make([]map[string]any, 0)
			for rawURL, oldHash := range beforeHashes {
				if newHash, ok := afterHashes[rawURL]; ok && oldHash != newHash {
					contentChanges = append(contentChanges, map[string]any{"url": rawURL, "before_hash": oldHash, "after_hash": newHash})
				}
			}
			view := map[string]any{"before": before.ID, "after": after.ID, "added": added, "removed": removed, "rank_or_metadata_changes": rankChanges, "content_changes": contentChanges}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s → %s: +%d added, -%d removed, %d rank/metadata changes, %d content changes\n", before.ID, after.ID, len(added), len(removed), len(rankChanges), len(contentChanges))
			return nil
		},
	}
	cmd.Flags().StringVar(&beforeID, "before", "", "Earlier snapshot ID")
	cmd.Flags().StringVar(&afterID, "after", "latest", "Later snapshot ID or latest")
	return cmd
}
