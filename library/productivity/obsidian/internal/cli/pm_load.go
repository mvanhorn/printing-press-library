// Copyright 2026 Angelo Pullen and contributors. Licensed under Apache-2.0. See LICENSE.

// The generic Printing Press emits a `load` command for SaaS APIs that
// shows workload distribution per assignee. That semantic doesn't apply
// to Obsidian (notes have no assignees), so the command is repurposed as
// a thin convenience wrapper that prints vault-level totals from the
// local mirror — same intent (a "load report") with vault-correct units.
//
// `hotspots` is the proper Tier-3 analytic for "where is attention
// concentrated"; this command just reports raw counts so operators have
// a quick "how full is the mirror" answer.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
	"github.com/spf13/cobra"
)

func newLoadCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "load",
		Short: "Print vault totals from the local mirror",
		Long: `Show how much of the vault the local mirror currently covers: total
notes synced, distinct tags, link counts, last sync time. Useful as a
quick sanity check before running Tier-3 analytics (health, broken,
decay, hotspots, reconcile) that depend on a populated mirror.`,
		Example: `  obsidian-pp-cli load
  obsidian-pp-cli load --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("obsidian-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureObsidianSchema(); err != nil {
				return err
			}

			dbi := db.DB()
			counts := map[string]any{}
			var n int
			_ = dbi.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&n)
			counts["notes"] = n
			_ = dbi.QueryRow(`SELECT COUNT(DISTINCT tag) FROM obsidian_tags`).Scan(&n)
			counts["distinct_tags"] = n
			_ = dbi.QueryRow(`SELECT COUNT(*) FROM obsidian_links WHERE link_type='wikilink'`).Scan(&n)
			counts["wikilinks"] = n
			_ = dbi.QueryRow(`SELECT COUNT(*) FROM obsidian_links WHERE link_type='embed'`).Scan(&n)
			counts["embeds"] = n
			_ = dbi.QueryRow(`SELECT COUNT(*) FROM obsidian_links WHERE link_type='external'`).Scan(&n)
			counts["external_links"] = n

			var vaultPath, lastSync string
			_ = dbi.QueryRow(`SELECT vault_path, COALESCE(last_sync_at,'') FROM vault_sync_state WHERE id=1`).Scan(&vaultPath, &lastSync)
			counts["vault_path"] = vaultPath
			counts["last_sync_at"] = lastSync

			if flags.asJSON {
				out, _ := json.MarshalIndent(counts, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"vault:         %s\nlast sync:     %s\nnotes:         %d\ndistinct tags: %d\nwikilinks:     %d\nembeds:        %d\nexternal:      %d\n",
				vaultPath, lastSync, counts["notes"], counts["distinct_tags"],
				counts["wikilinks"], counts["embeds"], counts["external_links"])
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/obsidian-pp-cli/data.db)")
	return cmd
}
