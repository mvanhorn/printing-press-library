// bookmarks find — keyword/author search over the locally synced bookmark set.
//
// X makes it trivial to bookmark and hard to retrieve: the web UI has no
// bookmark search, and the v2 bookmarks endpoint returns posts only in
// reverse-bookmark order with no per-bookmark timestamp. This command turns
// the local store into the missing search layer — sync bookmarks once, then
// query by keyword and/or author offline as many times as you like without
// re-spending PPU read credits.
//
// It reads the typed `bookmarks` table directly (synced rows are NOT FTS-indexed
// and are unreachable via `search`), matching on the post text and, when the
// sync requested author_id, on the author. Author handles resolve against the
// synced `users` table; a numeric id is matched directly.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
)

func newNovelBookmarksFindCmd(flags *rootFlags) *cobra.Command {
	var from string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "find [query]",
		Short: "Search your synced bookmarks by keyword and/or author — offline, no API re-spend",
		Long: "Search the bookmarks you've synced into the local store. X has no bookmark search and\n" +
			"its API returns no bookmark timestamp, so bookmarks become a write-only graveyard;\n" +
			"this rebuilds the missing retrieval layer from the local store.\n\n" +
			"Matches the post text (a default bookmark field) and, when present, the author. Results\n" +
			"are ordered by post date, newest first — note that's when the post was written, not when\n" +
			"you bookmarked it, which the X API does not expose.\n\n" +
			"Populate the store first:\n" +
			"  x-twitter-pp-cli users bookmarks get-users <your-id>   # confirm access\n" +
			"  x-twitter-pp-cli sync --resources bookmarks\n\n" +
			"To enable --from, sync bookmarks with the author field and sync users for handle lookup:\n" +
			"  x-twitter-pp-cli sync --resources bookmarks --param tweet.fields=author_id,created_at\n" +
			"  x-twitter-pp-cli sync --resources users",
		Example: "  x-twitter-pp-cli users bookmarks find \"rust async\"\n" +
			"  x-twitter-pp-cli users bookmarks find --from @karpathy\n" +
			"  x-twitter-pp-cli users bookmarks find \"llm\" --from elonmusk --limit 20 --agent",
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// A query or author that matches nothing is a valid empty result from
			// the local store, not an error, so skip dogfood's error-path probe.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			// Need at least one filter; bare invocation is a help request, not a
			// "dump every bookmark" request.
			if query == "" && from == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("x-twitter-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'x-twitter-pp-cli sync --resources bookmarks' first to populate the local store.", err)
			}
			defer db.Close()

			maybeEmitSyncHints(cmd, db, "bookmarks", flags.maxAge)

			authorID := ""
			if from != "" {
				resolved, ok := resolveBookmarkAuthor(cmd, db, from)
				if !ok {
					// Couldn't resolve the requested author to an id, so the filter
					// can't match anything. Emit an empty result rather than
					// silently dropping the filter and returning unrelated posts.
					return printJSONFiltered(cmd.OutOrStdout(), []json.RawMessage{}, flags)
				}
				authorID = resolved
			}

			sqlStr, sqlArgs := buildBookmarkFindQuery(query, authorID, limit)
			rows, err := db.DB().QueryContext(cmd.Context(), sqlStr, sqlArgs...)
			if err != nil {
				return fmt.Errorf("querying bookmarks: %w", err)
			}
			defer rows.Close()

			results := make([]json.RawMessage, 0)
			for rows.Next() {
				var data sql.NullString
				if err := rows.Scan(&data); err != nil {
					continue
				}
				if data.Valid && data.String != "" {
					results = append(results, json.RawMessage(data.String))
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading bookmark rows: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Filter by author (@handle resolved via synced users, or a numeric author id)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database (defaults to the standard cache location)")
	return cmd
}

// buildBookmarkFindQuery is kept pure so the predicate/argument pairing is
// unit-testable without a live store. An empty query or authorID drops that
// predicate; ordering is by post date (newest first) because the X API exposes
// no bookmark timestamp.
func buildBookmarkFindQuery(query, authorID string, limit int) (string, []any) {
	var where []string
	var args []any
	if query != "" {
		where = append(where, "lower(json_extract(data, '$.text')) LIKE ?")
		args = append(args, "%"+strings.ToLower(query)+"%")
	}
	if authorID != "" {
		where = append(where, "json_extract(data, '$.author_id') = ?")
		args = append(args, authorID)
	}
	sqlStr := `SELECT data FROM bookmarks`
	if len(where) > 0 {
		sqlStr += " WHERE " + strings.Join(where, " AND ")
	}
	sqlStr += " ORDER BY json_extract(data, '$.created_at') DESC, id DESC LIMIT ?"
	args = append(args, limit)
	return sqlStr, args
}

// resolveBookmarkAuthor accepts either a numeric author id (used directly) or an
// @handle (leading @ stripped, looked up against the synced users table). Returns
// ok=false when a handle can't be resolved, so the caller short-circuits to an
// empty result instead of returning unfiltered bookmarks.
func resolveBookmarkAuthor(cmd *cobra.Command, db *store.Store, from string) (string, bool) {
	handle := strings.TrimPrefix(strings.TrimSpace(from), "@")
	if handle == "" {
		return "", false
	}
	if isAllDigits(handle) {
		return handle, true
	}
	var id sql.NullString
	err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT id FROM users WHERE lower(username) = lower(?) LIMIT 1`, handle).Scan(&id)
	if err != nil || !id.Valid || id.String == "" {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Couldn't resolve author %q from the local store. Sync users (x-twitter-pp-cli sync --resources users) or pass a numeric author id.\n", from)
		return "", false
	}
	return id.String, true
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
