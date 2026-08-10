// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/job-boards/lever/internal/store"
)

// newSyncCmd fetches every open posting for a company and persists it to
// the local store under a company-scoped key (postings:leverdemo), so
// local reads stay per-company and work offline. The scaffold referenced
// `sync` in missing-store guidance, MCP error text, and docs, but
// generated no command; this wires the real one.
func newSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sync <company>",
		Short:        "Fetch all open postings for a company and store them locally",
		Example:      "  lever-pp-cli sync leverdemo",
		Annotations:  map[string]string{"mcp:local-write": "true"},
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			company := args[0]
			path := replacePathParam("/postings/{company}", "company", company)
			params := map[string]string{"mode": "json"}

			// Fetch live first (strategy "live" skips the unscoped
			// write-through), collecting what will be persisted.
			data, _, err := resolveReadWithStrategyAndResponsePath(
				cmd.Context(), c, flags, "live", "postings", true, path, params, nil, "", cmd.ErrOrStderr())
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

			// Persist under a company-scoped key and verify durability, so
			// a failed store write never masquerades as a successful sync.
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("lever-pp-cli"))
			if err != nil {
				return fmt.Errorf("sync: open local store: %w", err)
			}
			defer db.Close()

			scoped := "postings:" + company
			stored, skipped, err := db.UpsertBatch(scoped, items)
			if err != nil {
				return fmt.Errorf("sync postings: persist: %w", err)
			}
			// The upsert count is the source of truth: pre-existing rows
			// for this company must not mask a zero-persistence failure on
			// a fresh fetch.
			if len(items) > 0 && stored == 0 {
				return fmt.Errorf("sync postings: fetched %d records but none persisted (%d skipped); the sync did not persist", len(items), skipped)
			}
			readback, err := db.List(scoped, 0)
			if err != nil {
				return fmt.Errorf("sync postings: verify local write: %w", err)
			}
			if len(items) > 0 && len(readback) == 0 {
				return fmt.Errorf("sync postings: fetched %d records but the local store holds none; the sync did not persist", len(items))
			}
			// Record the sync marker only after the write and the
			// verification both succeeded, using the persisted count.
			if err := db.SaveSyncState(scoped, "", stored); err != nil {
				return fmt.Errorf("sync postings: record sync state: %w", err)
			}
			if flags != nil && flags.asJSON {
				data, err := wrapAgentOutput(json.RawMessage("null"), map[string]any{
					"source":        "live",
					"resource_type": "postings",
					"company":       company,
					"stored":        stored,
					"skipped":       skipped,
					"synced_at":     time.Now().UTC().Format(time.RFC3339),
				})
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d records for company %q\n", stored, company)
			return nil
		},
	}
	return cmd
}
