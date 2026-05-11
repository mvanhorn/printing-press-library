// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/artyfacts/internal/store"
	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var flagFull bool
	var flagType string
	var flagDBPath string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull all artifacts and sections into the local SQLite store",
		Long: `Fetch all artifacts and their sections from the Artyfacts API and store
them locally in SQLite. Subsequent searches and reads use this local cache.

The store lives at ~/.local/share/artyfacts-pp-cli/store.db by default.
Sync is incremental by default — only artifacts updated since the last sync
are fetched. Use --full to force a complete refresh.`,
		Example:     "  artyfacts-pp-cli sync\n  artyfacts-pp-cli sync --full\n  artyfacts-pp-cli sync --type doc",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			start := time.Now()
			fmt.Fprintf(os.Stderr, "Syncing to %s...\n", dbPath)

			// Fetch org context
			orgData, orgErr := c.Get("/org/context", nil)
			if orgErr == nil {
				_ = db.UpsertOrgContext(orgData)
			}

			// Build artifact list params
			params := map[string]string{"limit": "200"}
			if flagType != "" {
				params["type"] = flagType
			}

			// Fetch all artifacts (paginate by fetching multiple pages if needed)
			var allArtifacts []map[string]any
			offset := 0
			for {
				params["limit"] = "200"
				if offset > 0 {
					params["offset"] = fmt.Sprintf("%d", offset)
				}
				data, err := c.Get("/artifacts", params)
				if err != nil {
					return fmt.Errorf("fetching artifacts: %w", err)
				}

				// Artyfacts API returns {items:[...], count:N, offset:N, limit:N}
				var batch []map[string]any
				if err := json.Unmarshal(data, &batch); err != nil {
					// Try paginated object wrapper (actual API shape: {items:[...]})
					var wrapper struct {
						Items []map[string]any `json:"items"`
					}
					if err2 := json.Unmarshal(data, &wrapper); err2 != nil {
						return fmt.Errorf("parsing artifacts response: %w", err2)
					}
					batch = wrapper.Items
				}

				if len(batch) == 0 {
					break
				}
				allArtifacts = append(allArtifacts, batch...)

				// If we got fewer than the page size, we're done
				if len(batch) < 200 {
					break
				}
				offset += len(batch)
			}

			artifactCount := 0
			sectionCount := 0

			for _, art := range allArtifacts {
				artID, _ := art["id"].(string)
				if artID == "" {
					continue
				}

				// Skip if not modified since last sync (incremental mode)
				if !flagFull {
					lastSync, _ := db.LastSyncTime()
					if !lastSync.IsZero() {
						updatedAt, _ := art["updated_at"].(string)
						if updatedAt != "" {
							t, parseErr := time.Parse(time.RFC3339, updatedAt)
							if parseErr == nil && t.Before(lastSync) {
								continue
							}
						}
					}
				}

				if err := db.UpsertArtifact(art); err != nil {
					fmt.Fprintf(os.Stderr, "warning: upsert artifact %s: %v\n", artID, err)
					continue
				}
				artifactCount++

				// Fetch sections for this artifact
				secData, secErr := c.Get(fmt.Sprintf("/artifacts/%s/sections", artID), nil)
				if secErr != nil {
					fmt.Fprintf(os.Stderr, "warning: fetching sections for %s: %v\n", artID, secErr)
					continue
				}
				// Actual API: {"sections": [...]} wrapper
				var sections []map[string]any
				if err := json.Unmarshal(secData, &sections); err != nil {
					var wrapper struct {
						Sections []map[string]any `json:"sections"`
					}
					if err2 := json.Unmarshal(secData, &wrapper); err2 == nil {
						sections = wrapper.Sections
					}
				}
				for _, sec := range sections {
					if err := db.UpsertSection(sec, artID); err != nil {
						fmt.Fprintf(os.Stderr, "warning: upsert section: %v\n", err)
					} else {
						sectionCount++
					}
				}
			}

			duration := time.Since(start)
			mode := "incremental"
			if flagFull {
				mode = "full"
			}
			_ = db.LogSync(mode, artifactCount, sectionCount, duration)

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(map[string]any{
					"artifacts": artifactCount,
					"sections":  sectionCount,
					"duration":  duration.String(),
					"mode":      mode,
					"store":     dbPath,
				}), flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d artifacts, %d sections (%s, %s)\n",
				artifactCount, sectionCount, duration.Round(time.Millisecond), mode)
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagFull, "full", false, "Force a complete refresh (ignore incremental state)")
	cmd.Flags().StringVar(&flagType, "type", "", "Only sync artifacts of this type (doc, spec, research, etc.)")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Custom SQLite database path (default: ~/.local/share/artyfacts-pp-cli/store.db)")

	return cmd
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
