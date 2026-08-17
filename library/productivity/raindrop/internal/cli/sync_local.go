// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/raindrop/internal/store"
	"github.com/spf13/cobra"
)

func newSyncStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "status", Short: "Show local mirror counts and checkpoints", Example: "  raindrop-pp-cli sync status --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			dbPath = defaultDBPath("raindrop-pp-cli")
		}
		if _, err := os.Stat(dbPath); err != nil {
			if os.IsNotExist(err) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"database": dbPath, "exists": false}, flags)
			}
			return err
		}
		db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		counts, err := db.Status()
		if err != nil {
			return err
		}
		states := map[string]any{}
		for resource := range counts {
			cursor, last, count, err := db.GetSyncState(resource)
			if err == nil {
				states[resource] = map[string]any{"cursor": cursor, "last_synced": last, "count": count}
			}
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"database": dbPath, "exists": true, "counts": counts, "sync": states}, flags)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newSyncDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := newNovelChangesCmd(flags)
	cmd.Use = "diff"
	cmd.Short = "Show changes recorded between sync snapshots"
	cmd.Example = "  raindrop-pp-cli sync diff --since 7d"
	return cmd
}
