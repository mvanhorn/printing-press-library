// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/artyfacts/internal/store"
	"github.com/spf13/cobra"
)

func newArtifactsStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDBPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Show artifacts modified remotely since last sync",
		Long: `Compares local store timestamps against the live API to identify
artifacts that have been updated by other agents or users since your
last 'sync'. These are candidates for re-sync.`,
		Example:     "  artyfacts-pp-cli artifacts stale\n  artyfacts-pp-cli artifacts stale --json",
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
				return fmt.Errorf("opening store: %w\nRun 'sync' first to populate the local cache.", err)
			}
			defer db.Close()

			// Fetch live artifact list (metadata only — no sections needed)
			data, err := c.Get("/artifacts", map[string]string{"limit": "500"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse response (actual API: {items:[...]})
			var batch []map[string]any
			if err := json.Unmarshal(data, &batch); err != nil {
				var wrapper struct {
					Items []map[string]any `json:"items"`
				}
				if err2 := json.Unmarshal(data, &wrapper); err2 != nil {
					return fmt.Errorf("parsing response: %w", err2)
				}
				batch = wrapper.Items
			}

			remoteUpdates := make(map[string]string, len(batch))
			for _, a := range batch {
				id, _ := a["id"].(string)
				updatedAt, _ := a["updated_at"].(string)
				if id != "" {
					remoteUpdates[id] = updatedAt
				}
			}

			stale, err := db.StaleArtifacts(remoteUpdates)
			if err != nil {
				return err
			}

			if flags.asJSON {
				b, _ := json.Marshal(stale)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}

			if len(stale) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "All synced artifacts are up to date.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d artifact(s) stale:\n\n", len(stale))
			for _, s := range stale {
				if s.Reason == "not-synced" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  [not in local store]\n", s.ID)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  local=%s  remote=%s\n",
						s.ID, s.LocalUpdatedAt, s.RemoteUpdatedAt)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nRun 'sync' to update the local store.\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDBPath, "db", "", "Custom SQLite database path")
	return cmd
}
