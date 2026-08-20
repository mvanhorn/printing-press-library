// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newNovelChangesCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "changes",
		Short:       "Show field-level bookmark changes between sync snapshots.",
		Example:     "  raindrop-pp-cli changes --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			age, err := parseAge(flagSince, 7*24*time.Hour)
			if err != nil {
				return err
			}
			db, _, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT id, bookmark_id, old_data, new_data, changed_at FROM raindrop_history WHERE changed_at >= ? ORDER BY changed_at DESC`, time.Now().UTC().Add(-age).Format("2006-01-02 15:04:05"))
			if err != nil {
				return fmt.Errorf("querying history: %w", err)
			}
			defer rows.Close()
			var result []map[string]any
			for rows.Next() {
				var id int64
				var bookmarkID, oldRaw, newRaw, changed string
				if err := rows.Scan(&id, &bookmarkID, &oldRaw, &newRaw, &changed); err != nil {
					return err
				}
				var oldObj, newObj map[string]any
				_ = json.Unmarshal([]byte(oldRaw), &oldObj)
				_ = json.Unmarshal([]byte(newRaw), &newObj)
				fields := changedFields(oldObj, newObj)
				result = append(result, map[string]any{"id": id, "bookmark_id": bookmarkID, "changed_at": changed, "fields": fields, "before": oldObj, "after": newObj})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"since": time.Now().UTC().Add(-age), "count": len(result), "changes": result}, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "History window (for example 24h, 7d, 4w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func changedFields(before, after map[string]any) []string {
	keys := map[string]struct{}{}
	for key := range before {
		keys[key] = struct{}{}
	}
	for key := range after {
		keys[key] = struct{}{}
	}
	var changed []string
	for _, key := range sortedKeys(keys) {
		left, _ := json.Marshal(before[key])
		right, _ := json.Marshal(after[key])
		if string(left) != string(right) {
			changed = append(changed, key)
		}
	}
	return changed
}
