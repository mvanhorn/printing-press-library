// Copyright 2026 drdriftwood. Licensed under Apache-2.0. See LICENSE.

// `obsidian-pp-cli stale` — find notes that haven't been modified in N
// days but still have incoming wikilinks (so they're not orphans, just
// stale). Replaces the generic Press "items not updated in N days" logic
// with obsidian-correct semantics querying notes.modified_at.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string
	var limit int
	var includeOrphans bool

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Notes not modified in N days that still have incoming links",
		Long: `Find notes whose modified_at is older than N days AND have at least one
incoming wikilink. The incoming-link gate is the differentiator vs.
'orphans': stale-but-linked notes are likely outdated references —
candidates for review or update.

Pass --include-orphans to also include notes with zero incoming links
(equivalent to 'orphans' filtered by age).`,
		Example: `  # Default: 30 days, linked-but-stale
  obsidian-pp-cli stale

  # 7-day window
  obsidian-pp-cli stale --days 7

  # Include orphaned stale notes
  obsidian-pp-cli stale --days 90 --include-orphans

  # JSON output
  obsidian-pp-cli stale --json`,
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

			cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

			dbi := db.DB()
			var q string
			// Incoming-link resolution mirrors `pm_orphans` / `broken` /
			// `health` — match a note via full path, basename with
			// folder prefix + `.md` stripped, OR frontmatter title — so
			// the "still being linked" gate stays coherent with the other
			// Tier-3 counts. The expression appears three times below;
			// SQLite has no parameterized subquery reuse, so the cost is
			// repetition.
			if includeOrphans {
				q = `
					SELECT n.path, n.title, n.modified_at, n.word_count,
					       COALESCE((SELECT COUNT(*) FROM obsidian_links l
					                 WHERE l.target_path = n.path
					                    OR l.target_path = replace(replace(n.path, rtrim(n.path, replace(n.path, '/', '')), ''), '.md', '')
					                    OR l.target_path = n.title), 0) AS incoming
					FROM notes n
					WHERE n.modified_at < ?
					ORDER BY n.modified_at ASC
					LIMIT ?`
			} else {
				q = `
					SELECT n.path, n.title, n.modified_at, n.word_count,
					       (SELECT COUNT(*) FROM obsidian_links l
					        WHERE l.target_path = n.path
					           OR l.target_path = replace(replace(n.path, rtrim(n.path, replace(n.path, '/', '')), ''), '.md', '')
					           OR l.target_path = n.title) AS incoming
					FROM notes n
					WHERE n.modified_at < ?
					  AND EXISTS (
					        SELECT 1 FROM obsidian_links l
					        WHERE l.target_path = n.path
					           OR l.target_path = replace(replace(n.path, rtrim(n.path, replace(n.path, '/', '')), ''), '.md', '')
					           OR l.target_path = n.title)
					ORDER BY n.modified_at ASC
					LIMIT ?`
			}
			rows, err := dbi.Query(q, cutoff, limit)
			if err != nil {
				return fmt.Errorf("querying stale notes: %w", err)
			}
			defer rows.Close()

			type staleNote struct {
				Path       string `json:"path"`
				Title      string `json:"title"`
				ModifiedAt string `json:"modified_at"`
				WordCount  int    `json:"word_count"`
				Incoming   int    `json:"incoming_links"`
				DaysSince  int    `json:"days_since"`
			}

			now := time.Now()
			var items []staleNote
			for rows.Next() {
				var n staleNote
				if err := rows.Scan(&n.Path, &n.Title, &n.ModifiedAt, &n.WordCount, &n.Incoming); err != nil {
					continue
				}
				if t, err := time.Parse(time.RFC3339, n.ModifiedAt); err == nil {
					n.DaysSince = int(now.Sub(t).Hours() / 24)
				}
				items = append(items, n)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating stale notes: %w", err)
			}

			emitStalenessWarning(cmd, db)

			if flags.asJSON {
				out, _ := json.MarshalIndent(map[string]any{
					"days":            days,
					"include_orphans": includeOrphans,
					"count":           len(items),
					"items":           items,
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale notes found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d stale notes (no edits in %d+ days, with incoming links):\n\n", len(items), days)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-6s %-4s %s\n", "PATH", "DAYS", "IN", "TITLE")
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-6s %-4s %s\n", "----", "----", "--", "-----")
			for _, n := range items {
				p := n.Path
				if len(p) > 30 {
					p = p[:27] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-6d %-4d %s\n", p, n.DaysSince, n.Incoming, n.Title)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 30, "Days without modification to consider stale")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/obsidian-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum notes to show")
	cmd.Flags().BoolVar(&includeOrphans, "include-orphans", false, "Also include stale notes with zero incoming links")
	return cmd
}
