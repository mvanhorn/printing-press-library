// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelInboxReviewCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var resume bool
	var dbPath string
	cmd := &cobra.Command{
		Use: "review", Short: "Create or resume a bounded Unsorted review session",
		Example:     "  raindrop-pp-cli inbox review --limit 10 --resume",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, path, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var sessionID int64
			if resume {
				_ = db.DB().QueryRowContext(cmd.Context(), `SELECT id FROM inbox_sessions WHERE status='open' ORDER BY updated_at DESC LIMIT 1`).Scan(&sessionID)
			}
			if sessionID == 0 {
				res, err := db.DB().ExecContext(cmd.Context(), `INSERT INTO inbox_sessions(status) VALUES('open')`)
				if err != nil {
					return err
				}
				sessionID, err = res.LastInsertId()
				if err != nil {
					return err
				}
			}
			bookmarks, err := loadLocalBookmarks(db)
			if err != nil {
				return err
			}
			added := 0
			for _, bookmark := range bookmarks {
				if bookmark.CollectionID != -1 || added >= limit {
					continue
				}
				res, err := db.DB().ExecContext(cmd.Context(), `INSERT OR IGNORE INTO inbox_items(session_id,bookmark_id) VALUES(?,?)`, sessionID, bookmark.ID)
				if err != nil {
					return err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					added++
				}
			}
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT i.bookmark_id,i.state,r.data FROM inbox_items i LEFT JOIN raindrops r ON r.id=i.bookmark_id WHERE i.session_id=? ORDER BY i.updated_at,i.bookmark_id LIMIT ?`, sessionID, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			var items []map[string]any
			for rows.Next() {
				var id, state string
				var raw []byte
				if err := rows.Scan(&id, &state, &raw); err != nil {
					return err
				}
				var bookmark any
				_ = json.Unmarshal(raw, &bookmark)
				items = append(items, map[string]any{"bookmark_id": id, "state": state, "bookmark": bookmark})
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"session_id": sessionID, "database": path, "added": added, "items": items}, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum bookmarks in this review batch")
	cmd.Flags().BoolVar(&resume, "resume", false, "Resume the newest open review session")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newInboxApplyCmd(flags *rootFlags) *cobra.Command {
	var bookmarkID, tagsRaw, dbPath string
	var collectionID int64
	var skip bool
	cmd := &cobra.Command{
		Use: "apply <session-id>", Short: "Apply one staged inbox decision and record confirmation", Example: "  raindrop-pp-cli inbox apply session-1 --dry-run --agent", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid session id"))
			}
			if bookmarkID == "" {
				return usageErr(fmt.Errorf("--bookmark is required"))
			}
			db, _, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			decision := map[string]any{"skip": skip, "collection_id": collectionID}
			if tagsRaw != "" {
				for _, tag := range strings.Split(tagsRaw, ",") {
					if tag = strings.TrimSpace(tag); tag != "" {
						decision["tags"] = appendString(decision["tags"], tag)
					}
				}
			}
			encoded, _ := json.Marshal(decision)
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"session_id": sessionID, "bookmark_id": bookmarkID, "decision": decision, "dry_run": true}, flags)
			}
			state := "skipped"
			if !skip {
				body := map[string]any{}
				if collectionID != 0 {
					body["collection"] = map[string]any{"$id": collectionID}
				}
				if tags, ok := decision["tags"].([]string); ok {
					body["tags"] = tags
				}
				if len(body) == 0 {
					return usageErr(fmt.Errorf("provide --collection-id, --tags, or --skip"))
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				if _, _, err := c.PutWithParams(cmd.Context(), "/raindrop/"+bookmarkID, nil, body); err != nil {
					return classifyAPIError(err, flags)
				}
				state = "applied"
			}
			res, err := db.DB().ExecContext(cmd.Context(), `UPDATE inbox_items SET state=?,decision=?,updated_at=? WHERE session_id=? AND bookmark_id=?`, state, string(encoded), time.Now().UTC(), sessionID, bookmarkID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return fmt.Errorf("bookmark %s is not in session %d", bookmarkID, sessionID)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"session_id": sessionID, "bookmark_id": bookmarkID, "state": state}, flags)
		},
	}
	cmd.Flags().StringVar(&bookmarkID, "bookmark", "", "Bookmark ID to decide")
	cmd.Flags().Int64Var(&collectionID, "collection-id", 0, "Destination collection ID")
	cmd.Flags().StringVar(&tagsRaw, "tags", "", "Comma-separated replacement tags")
	cmd.Flags().BoolVar(&skip, "skip", false, "Mark item skipped without API mutation")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func appendString(v any, value string) []string {
	if existing, ok := v.([]string); ok {
		return append(existing, value)
	}
	return []string{value}
}
