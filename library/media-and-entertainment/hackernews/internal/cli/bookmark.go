package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/store"
)

// bookmark stores user-curated favorites in the local SQLite database.
// Decoupled from HN account favorites — works without auth, exports as
// JSON, and survives across sessions because it lives in the same store
// file as the rest of the synced data.

func ensureBookmarksTable(db *store.Store) error {
	_, err := db.DB().Exec(`CREATE TABLE IF NOT EXISTS bookmarks (
		id TEXT PRIMARY KEY,
		title TEXT,
		url TEXT,
		note TEXT,
		added_at TEXT NOT NULL
	)`)
	return err
}

func newBookmarkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bookmark",
		Short: "Manage local bookmarks (add, list, rm)",
		Long:  "Local-only bookmarks for HN items. Stored in the SQLite database alongside synced data.",
	}
	cmd.AddCommand(newBookmarkAddCmd(flags))
	cmd.AddCommand(newBookmarkListCmd(flags))
	cmd.AddCommand(newBookmarkRmCmd(flags))
	return cmd
}

func newBookmarkAddCmd(flags *rootFlags) *cobra.Command {
	var note string
	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add an item to local bookmarks",
		Example: strings.Trim(`
  hackernews-pp-cli bookmark add 12345678
  hackernews-pp-cli bookmark add 12345678 --note "great rust thread"
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			db, err := store.Open(defaultDBPath("hackernews-pp-cli"))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			if err := ensureBookmarksTable(db); err != nil {
				return apiErr(err)
			}

			// Try to fetch title/url from API for context.
			c, err := flags.newClient()
			if err == nil {
				if data, getErr := c.Get("/item/"+id+".json", nil); getErr == nil {
					obj := map[string]any{}
					_ = json.Unmarshal(data, &obj)
					title, _ := obj["title"].(string)
					url, _ := obj["url"].(string)
					_, ierr := db.DB().Exec(
						`INSERT OR REPLACE INTO bookmarks(id, title, url, note, added_at) VALUES (?, ?, ?, ?, ?)`,
						id, title, url, note, time.Now().UTC().Format(time.RFC3339),
					)
					if ierr != nil {
						return apiErr(ierr)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "bookmarked: %s — %s\n", id, title)
					return nil
				}
			}
			// Fallback: insert without metadata.
			_, ierr := db.DB().Exec(
				`INSERT OR REPLACE INTO bookmarks(id, title, url, note, added_at) VALUES (?, ?, ?, ?, ?)`,
				id, "", "", note, time.Now().UTC().Format(time.RFC3339),
			)
			if ierr != nil {
				return apiErr(ierr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "bookmarked: %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Optional note to attach to the bookmark")
	return cmd
}

type bookmarkRow struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Note    string `json:"note"`
	AddedAt string `json:"added_at"`
}

func newBookmarkListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List local bookmarks",
		Example: strings.Trim(`
  hackernews-pp-cli bookmark list
  hackernews-pp-cli bookmark list --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.Open(defaultDBPath("hackernews-pp-cli"))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			if err := ensureBookmarksTable(db); err != nil {
				return apiErr(err)
			}
			rows, err := db.DB().Query(`SELECT id, title, url, note, added_at FROM bookmarks ORDER BY added_at DESC`)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			out := []bookmarkRow{}
			for rows.Next() {
				var b bookmarkRow
				if err := rows.Scan(&b.ID, &b.Title, &b.URL, &b.Note, &b.AddedAt); err != nil {
					return apiErr(err)
				}
				out = append(out, b)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no bookmarks")
				return nil
			}
			tableRows := make([][]string, 0, len(out))
			for _, b := range out {
				tableRows = append(tableRows, []string{b.ID, b.AddedAt, truncateAtRune(b.Title, 60), b.URL})
			}
			return flags.printTable(cmd, []string{"ID", "ADDED", "TITLE", "URL"}, tableRows)
		},
	}
	return cmd
}

func newBookmarkRmCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a bookmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			db, err := store.Open(defaultDBPath("hackernews-pp-cli"))
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			if err := ensureBookmarksTable(db); err != nil {
				return apiErr(err)
			}
			res, err := db.DB().Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
			if err != nil {
				return apiErr(err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no such bookmark: %s\n", id)
				return notFoundErr(fmt.Errorf("bookmark %s not found", id))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", id)
			return nil
		},
	}
	return cmd
}
