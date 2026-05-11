// Copyright 2026 bryanaaron. Licensed under Apache-2.0.
// Hand-authored Phase 3 novel sync subcommands for Instagram.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"instagram-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// resolveUserID fetches the numeric Instagram user ID for a given username.
// Calls /api/v1/users/web_profile_info/?username=<username>.
func resolveUserID(ctx context.Context, flags *rootFlags, username string) (string, error) {
	c, err := flags.newClient()
	if err != nil {
		return "", err
	}
	data, err := c.Get("/api/v1/users/web_profile_info/", map[string]string{"username": username})
	if err != nil {
		return "", fmt.Errorf("profile lookup for %q: %w", username, err)
	}
	var resp struct {
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parse profile response: %w", err)
	}
	if resp.Data.User.ID == "" {
		return "", fmt.Errorf("user %q not found or not accessible", username)
	}
	return resp.Data.User.ID, nil
}

// ensureFollowersSnapshotTable creates the followers_snapshots table if it does not exist.
// Called at the start of any command that reads or writes snapshots.
func ensureFollowersSnapshotTable(db *store.Store) error {
	_, err := db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS followers_snapshots (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			username    TEXT NOT NULL,
			pk          TEXT NOT NULL,
			uname       TEXT NOT NULL,
			full_name   TEXT NOT NULL DEFAULT '',
			snapshot_ts INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.DB().Exec(`CREATE INDEX IF NOT EXISTS idx_fs_username_ts ON followers_snapshots(username, snapshot_ts)`)
	return err
}

// paginateList fetches all pages from an endpoint that uses next_max_id cursor pagination.
// It calls GET path with params, extracts the items array via itemsKey, and accumulates.
func paginateList(ctx context.Context, flags *rootFlags, path string, params map[string]string, itemsKey string, maxItems int) ([]json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	var all []json.RawMessage
	cursorParam := "max_id"
	cursor := ""
	for {
		p := make(map[string]string, len(params)+1)
		for k, v := range params {
			p[k] = v
		}
		if cursor != "" {
			p[cursorParam] = cursor
		}
		data, err := c.Get(path, p)
		if err != nil {
			return all, err
		}
		var page map[string]json.RawMessage
		if err := json.Unmarshal(data, &page); err != nil {
			return all, fmt.Errorf("parse page: %w", err)
		}
		// Extract items
		rawItems, ok := page[itemsKey]
		if !ok {
			// Try "users" as fallback key for followers/following responses
			rawItems, ok = page["users"]
			if !ok {
				break
			}
		}
		var items []json.RawMessage
		if err := json.Unmarshal(rawItems, &items); err != nil || len(items) == 0 {
			break
		}
		all = append(all, items...)
		if maxItems > 0 && len(all) >= maxItems {
			break
		}
		// Advance cursor
		var nextID json.RawMessage
		if nid, ok := page["next_max_id"]; ok {
			nextID = nid
		} else if bi, ok := page["big_list"]; ok {
			_ = bi
			// big_list=true means more pages; fall through to cursor check below
		}
		if nextID == nil {
			break
		}
		s := strings.Trim(string(nextID), `"`)
		if s == "" || s == "null" {
			break
		}
		cursor = s
	}
	return all, nil
}

// AddSyncNovelSubcommands registers the Instagram-specific sync subcommands
// on an existing sync *cobra.Command.
func AddSyncNovelSubcommands(syncCmd *cobra.Command, flags *rootFlags) {
	syncCmd.AddCommand(newSyncFollowersCmd(flags))
	syncCmd.AddCommand(newSyncFollowingCmd(flags))
	syncCmd.AddCommand(newSyncMediaCmd(flags))
	syncCmd.AddCommand(newSyncCommentsCmd(flags))
}

func newSyncFollowersCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxItems int
	cmd := &cobra.Command{
		Use:   "followers <username>",
		Short: "Sync followers list to SQLite (required for followers diff and comments strangers)",
		Args:  cobra.ExactArgs(1),
		Example: `  instagram-pp-cli sync followers johndoe
  instagram-pp-cli sync followers johndoe --max-items 2000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			if err := ensureFollowersSnapshotTable(db); err != nil {
				return fmt.Errorf("prepare snapshot table: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Resolving user ID for %q...\n", username)
			userID, err := resolveUserID(cmd.Context(), flags, username)
			if err != nil {
				return err
			}

			path := strings.ReplaceAll("/api/v1/friendships/{user_id}/followers/", "{user_id}", url.PathEscape(userID))
			fmt.Fprintf(cmd.OutOrStdout(), "Fetching followers (this may take a while for large accounts)...\n")

			followers, err := paginateList(cmd.Context(), flags, path, nil, "users", maxItems)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: pagination stopped early: %v\n", err)
			}

			// Store in generic resources table
			if _, _, err := db.UpsertBatch("followers", followers); err != nil {
				return fmt.Errorf("store followers: %w", err)
			}

			// Write snapshot for followers-diff
			ts := time.Now().Unix()
			tx, err := db.DB().BeginTx(cmd.Context(), nil)
			if err != nil {
				return err
			}
			for _, raw := range followers {
				var f struct {
					PK       string `json:"pk"`
					Username string `json:"username"`
					FullName string `json:"full_name"`
				}
				if json.Unmarshal(raw, &f) != nil || f.PK == "" {
					continue
				}
				if _, err := tx.Exec(
					`INSERT INTO followers_snapshots (username, pk, uname, full_name, snapshot_ts) VALUES (?,?,?,?,?)`,
					username, f.PK, f.Username, f.FullName, ts,
				); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("write snapshot: %w", err)
				}
			}
			if err := tx.Commit(); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d followers for @%s (snapshot written at %s).\n",
				len(followers), username, time.Unix(ts, 0).Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/instagram-pp-cli/data.db)")
	cmd.Flags().IntVar(&maxItems, "max-items", 0, "Max followers to fetch (0 = unlimited)")
	return cmd
}

func newSyncFollowingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxItems int
	cmd := &cobra.Command{
		Use:   "following <username>",
		Short: "Sync following list to SQLite (required for following non-mutual)",
		Args:  cobra.ExactArgs(1),
		Example: `  instagram-pp-cli sync following johndoe`,
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			userID, err := resolveUserID(cmd.Context(), flags, username)
			if err != nil {
				return err
			}

			path := strings.ReplaceAll("/api/v1/friendships/{user_id}/following/", "{user_id}", url.PathEscape(userID))
			fmt.Fprintf(cmd.OutOrStdout(), "Fetching following list for @%s...\n", username)

			following, err := paginateList(cmd.Context(), flags, path, nil, "users", maxItems)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: pagination stopped early: %v\n", err)
			}

			if _, _, err := db.UpsertBatch("following", following); err != nil {
				return fmt.Errorf("store following: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d following for @%s.\n", len(following), username)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/instagram-pp-cli/data.db)")
	cmd.Flags().IntVar(&maxItems, "max-items", 0, "Max following to fetch (0 = unlimited)")
	return cmd
}

func newSyncMediaCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxItems int
	cmd := &cobra.Command{
		Use:   "media <username>",
		Short: "Sync posts to SQLite (required for posts engagement and type-compare)",
		Args:  cobra.ExactArgs(1),
		Example: `  instagram-pp-cli sync media johndoe
  instagram-pp-cli sync media johndoe --max-items 500`,
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			userID, err := resolveUserID(cmd.Context(), flags, username)
			if err != nil {
				return err
			}

			path := strings.ReplaceAll("/api/v1/feed/user/{user_id}/", "{user_id}", url.PathEscape(userID))
			fmt.Fprintf(cmd.OutOrStdout(), "Fetching posts for @%s...\n", username)

			items, err := paginateList(cmd.Context(), flags, path, nil, "items", maxItems)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: pagination stopped early: %v\n", err)
			}

			if _, _, err := db.UpsertBatch("media", items); err != nil {
				return fmt.Errorf("store media: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d posts for @%s.\n", len(items), username)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/instagram-pp-cli/data.db)")
	cmd.Flags().IntVar(&maxItems, "max-items", 0, "Max posts to fetch (0 = unlimited)")
	return cmd
}

func newSyncCommentsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "comments <media-id>",
		Short: "Sync comments on a post to SQLite (required for top-commenters and strangers)",
		Args:  cobra.ExactArgs(1),
		Example: `  instagram-pp-cli sync comments 1234567890123456789`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mediaID := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			path := strings.ReplaceAll("/api/v1/media/{media_id}/comments/", "{media_id}", url.PathEscape(mediaID))
			fmt.Fprintf(cmd.OutOrStdout(), "Fetching comments for media %s...\n", mediaID)

			comments, err := paginateList(cmd.Context(), flags, path, nil, "comments", 0)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: pagination stopped early: %v\n", err)
			}

			if _, _, err := db.UpsertBatch("comments", comments); err != nil {
				return fmt.Errorf("store comments: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d comments for media %s.\n", len(comments), mediaID)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/instagram-pp-cli/data.db)")
	return cmd
}

// ensureDMTable creates the dm_messages FTS table if absent.
func ensureDMTable(db *store.Store) error {
	// Store DMs as a plain table (not FTS5 virtual) for cross-compat, with a separate FTS5 index
	_, err := db.DB().Exec(`
		CREATE TABLE IF NOT EXISTS dm_messages (
			id         TEXT PRIMARY KEY,
			thread_id  TEXT NOT NULL,
			sender_pk  TEXT NOT NULL,
			sender     TEXT NOT NULL DEFAULT '',
			body       TEXT NOT NULL DEFAULT '',
			sent_at    INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.DB().Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS dm_messages_fts USING fts5(
			id UNINDEXED,
			thread_id UNINDEXED,
			sender,
			body,
			content='dm_messages',
			content_rowid='rowid',
			tokenize='porter unicode61'
		)
	`)
	if err != nil {
		// FTS5 table may already exist; ignore error
		return nil
	}
	// Trigger to keep FTS in sync
	_, _ = db.DB().Exec(`
		CREATE TRIGGER IF NOT EXISTS dm_messages_ai AFTER INSERT ON dm_messages BEGIN
			INSERT INTO dm_messages_fts(rowid, id, thread_id, sender, body)
			VALUES (new.rowid, new.id, new.thread_id, new.sender, new.body);
		END
	`)
	return nil
}

// parseIntJSON extracts an integer from raw JSON (handles both number and string-encoded integer).
func parseIntJSON(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	// Try number first
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	// Try string-encoded integer
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

// StoreDMMessage writes a DM message record to dm_messages.
func StoreDMMessage(db *sql.DB, id, threadID, senderPK, sender, body string, sentAt int64) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO dm_messages (id, thread_id, sender_pk, sender, body, sent_at)
		VALUES (?,?,?,?,?,?)
	`, id, threadID, senderPK, sender, body, sentAt)
	return err
}
