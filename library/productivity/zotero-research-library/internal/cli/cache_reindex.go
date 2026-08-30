// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: zero-network FTS rebuild.
// pp:data-source local

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/zotero-research-library/internal/store"
	"github.com/spf13/cobra"
)

func newNovelCacheReindexCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "reindex",
		Short:   "Rebuild the full-text search indexes from the cached base tables — zero network",
		Long:    "Rebuilds every FTS5 index (items, collections, resources) from the data already in the cache. The recovery path when 'ground' results look wrong or an interrupted write corrupted an index — no resync required.",
		Example: "  zotero-research-library-pp-cli cache reindex",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			dbPath := defaultDBPath("zotero-research-library-pp-cli")
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				return fmt.Errorf("local cache is empty — run 'zotero-research-library-pp-cli sync --full' first")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening cache for reindex: %w", err)
			}
			defer db.Close()

			rebuilt := []string{}
			for _, fts := range []string{"items_fts", "collections_fts", "resources_fts"} {
				if _, err := db.DB().ExecContext(ctx,
					fmt.Sprintf(`INSERT INTO %q(%q) VALUES('rebuild')`, fts, fts)); err != nil {
					return fmt.Errorf("rebuilding %s: %w", fts, err)
				}
				rebuilt = append(rebuilt, fts)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"rebuilt": rebuilt,
					"db_path": dbPath,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rebuilt %d FTS indexes: %v\n", len(rebuilt), rebuilt)
			return nil
		},
	}
	return cmd
}
