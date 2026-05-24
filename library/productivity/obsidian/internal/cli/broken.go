// Copyright 2026 drdriftwood. Licensed under Apache-2.0. See LICENSE.

// `obsidian-pp-cli broken` — list unresolved wikilinks with their source
// note context. Builds on the live `obsidian unresolved` subcommand by
// joining against the mirror so each unresolved target carries the path
// (and title, when available) of the note that pointed at it.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
)

func newBrokenCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "broken",
		Short: "Unresolved wikilinks with their source-note context",
		Long: `For every wikilink in the local mirror that points at a non-existent
note, emit (source_path, source_title, target). The official
'obsidian unresolved' subcommand returns just the target path; this
Tier-3 view adds the source context so a triage agent can decide
whether to create the missing note or fix the link.`,
		Example: `  obsidian-pp-cli broken
  obsidian-pp-cli broken --json --limit 200`,
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

			// Wikilink resolution matches the orphans command: target_path
			// may equal the full path, the path with `.md` stripped, the
			// title, OR the basename with both folder prefix and `.md`
			// stripped. The basename clause matters in vaults with nested
			// notes — `[[foo]]` written in any folder resolves to
			// `notes/foo.md` if that's the only note named `foo`. Without
			// it, broken would flag every short-form wikilink to a nested
			// note as broken and `health.integrity` would be artificially
			// low.
			rows, err := db.DB().Query(`
				SELECT n.path AS source_path, n.title AS source_title, l.target_path
				FROM obsidian_links l
				JOIN notes n ON n.id = l.source_id
				WHERE l.link_type = 'wikilink'
				  AND NOT EXISTS (
					SELECT 1 FROM notes t
					WHERE t.path = l.target_path
					   OR replace(t.path, '.md', '') = l.target_path
					   OR replace(replace(t.path, rtrim(t.path, replace(t.path, '/', '')), ''), '.md', '') = l.target_path
					   OR t.title = l.target_path
				  )
				ORDER BY l.target_path, n.path
				LIMIT ?
			`, limit)
			if err != nil {
				return fmt.Errorf("querying broken links: %w", err)
			}
			defer rows.Close()

			type brokenLink struct {
				SourcePath  string `json:"source_path"`
				SourceTitle string `json:"source_title"`
				Target      string `json:"target"`
			}
			var items []brokenLink
			for rows.Next() {
				var b brokenLink
				if err := rows.Scan(&b.SourcePath, &b.SourceTitle, &b.Target); err != nil {
					continue
				}
				items = append(items, b)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating broken links: %w", err)
			}

			emitStalenessWarning(cmd, db)

			if flags.asJSON {
				out, _ := json.MarshalIndent(map[string]any{
					"count": len(items),
					"items": items,
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No broken wikilinks.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d broken wikilinks:\n\n", len(items))
			fmt.Fprintf(cmd.OutOrStdout(), "%-40s -> %s\n", "SOURCE", "MISSING TARGET")
			for _, b := range items {
				src := b.SourcePath
				if len(src) > 40 {
					src = src[:37] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s -> %s\n", src, b.Target)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/obsidian-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum broken links to show")
	return cmd
}
