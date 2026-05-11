// Copyright 2026 bryanaaron. Licensed under Apache-2.0.
// Hand-authored Phase 3 novel analytics: dm search via FTS5.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"instagram-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// newDMCmd returns the `dm` top-level command covering DM analytics.
func newDMCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dm",
		Short: "Direct message analytics (run sync dm first to populate; Instagram has no DM search)",
	}
	cmd.AddCommand(newDMSearchCmd(flags))
	cmd.AddCommand(newDMSyncCmd(flags))
	return cmd
}

func newDMSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across synced DMs (Instagram has zero DM search on any surface)",
		Long: `Searches the local dm_messages table using SQLite FTS5 (full-text search).
Instagram's web app, mobile app, and API all lack DM search. This is the only
way to find old messages by keyword.

Run 'instagram-pp-cli dm sync' first to populate the DM store. The sync
command reads DM threads from the Instagram web API and writes them to
SQLite.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		Example: `  instagram-pp-cli dm search "invoice"
  instagram-pp-cli dm search "meeting next week" --limit 20 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			if err := ensureDMTable(db); err != nil {
				return fmt.Errorf("prepare dm table: %w", err)
			}

			// Check if dm_messages has any rows before running FTS query
			var rowCount int
			_ = db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM dm_messages`).Scan(&rowCount)
			if rowCount == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No DMs found in store. Run: instagram-pp-cli dm sync")
				return nil
			}

			// FTS5 query: match against dm_messages_fts
			ftsQuery := `
				SELECT
					m.id, m.thread_id, m.sender, m.body, m.sent_at
				FROM dm_messages_fts fts
				JOIN dm_messages m ON m.rowid = fts.rowid
				WHERE dm_messages_fts MATCH ?
				ORDER BY m.sent_at DESC
			`
			if limit > 0 {
				ftsQuery += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), ftsQuery, query)
			if err != nil {
				// FTS table may not exist yet
				if strings.Contains(err.Error(), "no such table") {
					fmt.Fprintln(cmd.OutOrStdout(), "DM FTS index not ready. Run: instagram-pp-cli dm sync")
					return nil
				}
				return fmt.Errorf("search: %w", err)
			}
			defer rows.Close()

			type msgRow struct {
				ID       string `json:"id"`
				ThreadID string `json:"thread_id"`
				Sender   string `json:"sender"`
				Body     string `json:"body"`
				SentAt   string `json:"sent_at"`
			}
			var msgs []msgRow
			for rows.Next() {
				var m msgRow
				var sentAt int64
				if err := rows.Scan(&m.ID, &m.ThreadID, &m.Sender, &m.Body, &sentAt); err == nil {
					if sentAt > 0 {
						m.SentAt = time.Unix(sentAt, 0).Format("2006-01-02 15:04")
					}
					msgs = append(msgs, m)
				}
			}

			if flags.asJSON {
				raw, _ := json.MarshalIndent(map[string]any{
					"query": query,
					"count": len(msgs),
					"messages": func() any { if msgs == nil { return []msgRow{} }; return msgs }(),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "DM search results for %q: %d\n\n", query, len(msgs))
			if len(msgs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No messages match that query.")
				return nil
			}

			tableRows := make([][]string, len(msgs))
			for i, m := range msgs {
				body := m.Body
				if len(body) > 60 {
					body = body[:60] + "…"
				}
				tableRows[i] = []string{m.SentAt, m.Sender, body, m.ThreadID}
			}
			return flags.printTable(cmd, []string{"DATE", "SENDER", "MESSAGE", "THREAD"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results to show (0 = unlimited)")
	return cmd
}

// newDMSyncCmd syncs DM inbox to the local store.
// Instagram DM API is /api/v1/direct_v2/inbox/ — not in the sniffed spec but the
// endpoint is documented in the private API. We attempt it; if it returns 403,
// we surface a clear error.
func newDMSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync DM inbox to SQLite for offline search",
		Long: `Fetches the DM inbox from Instagram's private API and writes messages to the
local dm_messages SQLite table. Required before 'dm search' works.

Note: The DM inbox endpoint (/api/v1/direct_v2/inbox/) may return 403 for
some accounts depending on session type. If it fails, re-import your Chrome
session with 'auth login --chrome' and retry.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer db.Close()

			if err := ensureDMTable(db); err != nil {
				return fmt.Errorf("prepare dm table: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Fetching DM inbox...")

			// Paginate DM inbox
			cursor := ""
			totalMsgs := 0
			for {
				params := map[string]string{"visual_message_return_type": "unseen", "thread_message_limit": "10"}
				if cursor != "" {
					params["cursor"] = cursor
				}
				data, err := c.Get("/api/v1/direct_v2/inbox/", params)
				if err != nil {
					return fmt.Errorf("fetch inbox: %w", err)
				}

				var inbox struct {
					Inbox struct {
						Threads []struct {
							ThreadID string `json:"thread_id"`
							Items    []struct {
								ItemID    string          `json:"item_id"`
								UserID    string          `json:"user_id"`
								Text      string          `json:"text"`
								Timestamp int64           `json:"timestamp"`
								Link      json.RawMessage `json:"link"`
							} `json:"items"`
							Users []struct {
								PK       string `json:"pk"`
								Username string `json:"username"`
							} `json:"users"`
						} `json:"threads"`
						OldestCursor string `json:"oldest_cursor"`
						HasOlder     bool   `json:"has_older"`
					} `json:"inbox"`
				}
				if err := json.Unmarshal(data, &inbox); err != nil {
					return fmt.Errorf("parse inbox: %w", err)
				}

				// Build sender map from thread users
				for _, thread := range inbox.Inbox.Threads {
					senderMap := map[string]string{}
					for _, u := range thread.Users {
						senderMap[u.PK] = u.Username
					}
					for _, item := range thread.Items {
						if item.Text == "" {
							continue
						}
						sender := senderMap[item.UserID]
						if sender == "" {
							sender = item.UserID
						}
						// DM timestamp is microseconds on Instagram
						sentAt := item.Timestamp / 1_000_000
						if err := StoreDMMessage(db.DB(), item.ItemID, thread.ThreadID, item.UserID, sender, item.Text, sentAt); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: store message %s: %v\n", item.ItemID, err)
							continue
						}
						totalMsgs++
					}
				}

				if !inbox.Inbox.HasOlder || inbox.Inbox.OldestCursor == "" {
					break
				}
				cursor = inbox.Inbox.OldestCursor
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d DM messages. Run 'instagram-pp-cli dm search <query>' to search.\n", totalMsgs)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
