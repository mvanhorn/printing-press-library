// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/greenhouse/internal/store"
)

// newSyncCmd fetches every resource for a board and persists it to the
// local store under a board-scoped key (jobs:stripe), so local reads
// stay per-company and work offline. The scaffold referenced `sync` in
// missing-store guidance, MCP error text, and docs, but generated no
// command; this wires the real one.
func newSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sync <board_token>",
		Short:        "Fetch jobs, departments, and offices for a company and store them locally",
		Example:      "  greenhouse-pp-cli sync stripe",
		Annotations:  map[string]string{"mcp:local-write": "true"},
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			token := args[0]
			resources := []struct {
				name         string
				pathTemplate string
			}{
				{"jobs", "/{board_token}/jobs"},
				{"departments", "/{board_token}/departments"},
				{"offices", "/{board_token}/offices"},
			}

			// Fetch live first (strategy "live" skips the unscoped
			// write-through), collecting what will be persisted.
			type synced struct {
				name  string
				items []json.RawMessage
			}
			var results []synced
			for _, r := range resources {
				path := replacePathParam(r.pathTemplate, "board_token", token)
				data, _, err := resolveReadWithStrategyAndResponsePath(
					cmd.Context(), c, flags, "live", r.name, true, path, nil, nil, r.name, cmd.ErrOrStderr())
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var items []json.RawMessage
				if json.Unmarshal(data, &items) == nil {
					kept := items[:0]
					for _, it := range items {
						trimmed := strings.TrimSpace(string(it))
						if trimmed != "" && trimmed != "null" && trimmed != "[]" && trimmed != "{}" {
							kept = append(kept, it)
						}
					}
					items = kept
				}
				results = append(results, synced{r.name, items})
			}

			// Persist under board-scoped keys and verify durability, so
			// a failed store write never masquerades as a successful
			// sync.
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("greenhouse-pp-cli"))
			if err != nil {
				return fmt.Errorf("sync: open local store: %w", err)
			}
			defer db.Close()

			total := 0
			for _, r := range results {
				scoped := r.name + ":" + token
				stored, skipped, err := db.UpsertBatch(scoped, r.items)
				if err != nil {
					return fmt.Errorf("sync %s: persist: %w", r.name, err)
				}
				// The upsert counts are the source of truth: pre-existing
				// rows for this board must not mask a zero-persistence
				// failure on a fresh fetch.
				if len(r.items) > 0 && stored == 0 {
					return fmt.Errorf("sync %s: fetched %d records but none persisted (%d skipped); the sync did not persist", r.name, len(r.items), skipped)
				}
				readback, err := db.List(scoped, 0)
				if err != nil {
					return fmt.Errorf("sync %s: verify local write: %w", r.name, err)
				}
				if len(r.items) > 0 && len(readback) == 0 {
					return fmt.Errorf("sync %s: fetched %d records but the local store holds none; the sync did not persist", r.name, len(r.items))
				}
				// Record the sync marker only after the write and the
				// verification both succeeded, using the persisted count.
				if err := db.SaveSyncState(scoped, "", stored); err != nil {
					return fmt.Errorf("sync %s: record sync state: %w", r.name, err)
				}
				total += stored
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d records for board %q\n", total, token)
			return nil
		},
	}
	return cmd
}
