// Copyright 2026 drdriftwood. Licensed under Apache-2.0. See LICENSE.

// `obsidian-pp-cli orphans` — list vault notes that have no incoming
// wikilinks. Replaces the Press's generic "items missing assignee/project"
// scanner with obsidian-correct semantics.
//
// Tier-3 enhancement over the live `obsidian orphans` subcommand: results
// come from the local mirror so they can be ranked by note age (newest
// orphans first by default, oldest with --oldest), and a note's metadata
// (title, modified_at, word_count) is included for triage. Falls back to
// the live subprocess when the mirror is empty.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/obsidian"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
)

func newOrphansCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var oldest bool

	cmd := &cobra.Command{
		Use:   "orphans",
		Short: "Vault notes with no incoming wikilinks (mirror-ranked)",
		Long: `Find notes with no incoming wikilinks. Results are pulled from the local
mirror so they can be ranked by note age (newest first by default;
--oldest reverses) and accompanied by triage metadata (title, mtime,
word_count). Run 'obsidian-pp-cli sync' first to populate the mirror.

When the mirror is empty this command falls back to the live
'obsidian orphans' subprocess, which returns just paths.`,
		Example: `  # Recently-created notes that nobody has linked to yet
  obsidian-pp-cli orphans

  # Oldest unloved notes (triage candidates)
  obsidian-pp-cli orphans --oldest

  # JSON output, top 20
  obsidian-pp-cli orphans --json --limit 20`,
		Annotations: map[string]string{
			"mcp:read-only":        "true",
			"pp:typed-exit-codes":  "0,3,5",
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

			items, hasMirror, err := mirrorOrphans(db, limit, oldest)
			if err != nil {
				return err
			}
			if !hasMirror {
				return fallbackLiveOrphans(cmd, flags, limit)
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
				fmt.Fprintln(cmd.OutOrStdout(), "No orphan notes found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d orphan notes:\n\n", len(items))
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-6s %s\n", "PATH", "MODIFIED", "WORDS", "TITLE")
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-6s %s\n", "----", "--------", "-----", "-----")
			for _, it := range items {
				p := it["path"].(string)
				if len(p) > 30 {
					p = p[:27] + "..."
				}
				modShort := ""
				if m, ok := it["modified_at"].(string); ok && len(m) >= 10 {
					modShort = m[:10]
				}
				title, _ := it["title"].(string)
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-12s %-6v %s\n", p, modShort, it["word_count"], title)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/obsidian-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum orphans to show")
	cmd.Flags().BoolVar(&oldest, "oldest", false, "Sort oldest first (default: newest first)")
	return cmd
}

// mirrorOrphans returns orphan notes from the local mirror. The second
// return is false when no notes exist in the mirror yet — callers can
// fall back to the live subprocess in that case.
func mirrorOrphans(db *store.Store, limit int, oldest bool) ([]map[string]any, bool, error) {
	dbi := db.DB()
	// Empty mirror -> caller falls back to live.
	var noteCount int
	if err := dbi.QueryRow(`SELECT COUNT(*) FROM notes`).Scan(&noteCount); err != nil {
		return nil, false, fmt.Errorf("counting notes: %w", err)
	}
	if noteCount == 0 {
		return nil, false, nil
	}

	// A note is orphaned when no obsidian_links row references its path
	// or its basename-as-target (Obsidian wikilinks rarely include the
	// .md extension or the folder prefix). The basename match below is a
	// best-effort approximation; it can over-attribute when two distinct
	// notes share a basename, but that's preferable to false-orphaning.
	order := "modified_at DESC"
	if oldest {
		order = "modified_at ASC"
	}
	// Resolution mirrors broken/health/stale: full path, basename with
	// folder prefix + `.md` stripped, or frontmatter title. The title
	// clause matters for vaults that wikilink by display name rather
	// than filename; without it those notes would falsely flag here.
	q := fmt.Sprintf(`
		SELECT n.path, n.title, n.modified_at, n.word_count
		FROM notes n
		LEFT JOIN obsidian_links l
		       ON l.target_path = n.path
		       OR l.target_path = replace(replace(n.path, rtrim(n.path, replace(n.path, '/', '')), ''), '.md', '')
		       OR l.target_path = n.title
		WHERE l.target_path IS NULL
		ORDER BY %s
		LIMIT ?
	`, order)
	rows, err := dbi.Query(q, limit)
	if err != nil {
		return nil, true, fmt.Errorf("querying orphans: %w", err)
	}
	defer rows.Close()

	var items []map[string]any
	for rows.Next() {
		var path, title, modified string
		var words int
		if err := rows.Scan(&path, &title, &modified, &words); err != nil {
			return nil, true, err
		}
		items = append(items, map[string]any{
			"path":        path,
			"title":       title,
			"modified_at": modified,
			"word_count":  words,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, true, fmt.Errorf("iterating orphans: %w", err)
	}
	if items == nil {
		// Stable empty slice so JSON renders [] not null.
		items = []map[string]any{}
	}
	return items, true, nil
}

// fallbackLiveOrphans dispatches to `obsidian orphans` when the mirror
// is unpopulated. Returns just paths — no rich metadata.
func fallbackLiveOrphans(cmd *cobra.Command, flags *rootFlags, limit int) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, err := c.Get("/orphans", map[string]string{})
	if err != nil {
		return classifyAPIError(err, flags)
	}
	var paths []string
	if err := json.Unmarshal(data, &paths); err != nil {
		return fmt.Errorf("parsing live orphans: %w", err)
	}
	if limit > 0 && len(paths) > limit {
		paths = paths[:limit]
	}
	if flags.asJSON {
		out, _ := json.MarshalIndent(map[string]any{
			"count":  len(paths),
			"items":  paths,
			"source": "live",
		}, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "no local mirror — falling back to live `obsidian orphans` (run sync for richer metadata)")
	if len(paths) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No orphan notes found.")
		return nil
	}
	for _, p := range paths {
		fmt.Fprintln(cmd.OutOrStdout(), p)
	}
	return nil
}

// emitStalenessWarning prints a stderr hint when the mirror hasn't been
// synced in over 24h. The message varies based on whether Obsidian is
// currently reachable — if it is, suggest running sync; if it isn't,
// remind the operator the data may be stale.
//
// Safe to call with a nil store (no-op); a nil store is a common path
// when a command opened the database read-only and we want a best-effort
// staleness check rather than another open.
func emitStalenessWarning(cmd *cobra.Command, db *store.Store) {
	if db == nil {
		return
	}
	sqlDB := db.DB()
	if sqlDB == nil {
		return
	}
	t, ok := readLastSync(sqlDB)
	if !ok {
		return
	}
	age := time.Since(t)
	if age < 24*time.Hour {
		return
	}
	hours := int(age.Hours())
	if obsidian.IsRunning() {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"note: mirror last synced %dh ago — run `obsidian-pp-cli sync` to refresh.\n", hours)
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"warning: results may be stale (mirror last synced %dh ago; open Obsidian and run sync to refresh).\n", hours)
}
