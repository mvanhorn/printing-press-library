// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `grep` — find clips by remembered lyric/prompt/tag/title phrases via the
// local FTS5 full-text index. Reads the local SQLite store only; no network
// and no auth. Read-only.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

func newSunoGrepCmd(flags *rootFlags) *cobra.Command {
	var (
		limit  int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "grep <phrase>",
		Short: "Full-text search local clips by lyric/prompt/tag/title phrase",
		Long: "Find clips by remembered lyric/prompt phrases via local full-text match.\n\n" +
			"Searches the local FTS5 index over your synced clips (title, tags, and lyrics/prompt). " +
			"Do NOT use for live server-side title search; use 'search' instead.",
		Example:     "  suno-pp-cli grep \"midnight drive\"\n  suno-pp-cli grep synthwave --limit 5 --json",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a search phrase is required: grep \"<phrase>\""))
			}
			phrase := args[0]

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'suno-pp-cli sync' first.", err)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "clips")
			hintIfStale(cmd, db, "clips", flags.maxAge)

			if limit <= 0 {
				limit = 50
			}

			results, err := grepClips(db, phrase, limit)
			if err != nil {
				return fmt.Errorf("searching local clips: %w", err)
			}
			grepIDs := make([]string, len(results))
			for i, r := range results {
				grepIDs[i] = r.ID
			}
			attachWorkspaceColumn(db, grepIDs, func(i int, label string) {
				results[i].Workspace = label
			}, len(results))
			if perr := printJSONFiltered(cmd.OutOrStdout(), results, flags); perr != nil {
				return perr
			}
			// Like Unix grep, signal "no matches" with a non-zero exit (3 =
			// not found) so callers and pipelines can branch on it. The result
			// payload (an empty list) is still written to stdout above.
			if len(results) == 0 {
				return notFoundErr(fmt.Errorf("no clips matched %q in the local store", phrase))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of matching clips to return")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("suno-pp-cli"), "Path to local SQLite store")
	return cmd
}

// grepMatch is one FTS5 hit for the grep command.
type grepMatch struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Tags      string `json:"tags"`
	Snippet   string `json:"snippet"`
	Workspace string `json:"workspace,omitempty"`
}

// grepClips runs an FTS5 MATCH over the clips_fts index (title, tags, prompt) and
// joins back to the clips table for the id. snippet() highlights the match.
func grepClips(db *store.Store, phrase string, limit int) ([]grepMatch, error) {
	out := make([]grepMatch, 0)
	rows, err := db.DB().Query(
		`SELECT c.id,
		        COALESCE(c.title, ''),
		        COALESCE(c.tags, ''),
		        snippet(clips_fts, -1, '[', ']', '…', 12)
		   FROM clips_fts
		   JOIN clips c ON c.rowid = clips_fts.rowid
		  WHERE clips_fts MATCH ?
		  ORDER BY rank
		  LIMIT ?`,
		phrase, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, tags string
		var snippet sql.NullString
		if err := rows.Scan(&id, &title, &tags, &snippet); err != nil {
			return nil, err
		}
		out = append(out, grepMatch{
			ID:      id,
			Title:   title,
			Tags:    tags,
			Snippet: snippet.String,
		})
	}
	return out, rows.Err()
}
