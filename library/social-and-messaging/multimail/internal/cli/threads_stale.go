package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"multimail-pp-cli/internal/store"
)

type staleThreadRow struct {
	ThreadID    string `json:"thread_id"`
	Mailbox     string `json:"mailbox"`
	MailboxID   string `json:"mailbox_id"`
	Subject     string `json:"subject"`
	LastReplyAt string `json:"last_reply_at"`
	StaleDays   int    `json:"stale_days"`
	MessageCount int   `json:"message_count"`
}

func newThreadsStaleCmd(flags *rootFlags) *cobra.Command {
	var (
		days   int
		limit  int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List conversation threads with no reply in N days — surfaces dropped conversations",
		Long: `Threads stale finds conversation threads that have gone silent. It
scans the local SQLite cache for threads whose last message is older
than the threshold, revealing dropped conversations that may need
follow-up.

Requires synced data (run 'multimail-pp-cli sync --full' first).`,
		Example: strings.Trim(`
  multimail-pp-cli threads stale --json
  multimail-pp-cli threads stale --days 3 --json
  multimail-pp-cli threads stale --days 14 --limit 20 --json --select subject,mailbox,stale_days`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("multimail-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days)

			// Build mailbox name lookup
			mbNames := map[string]string{}
			mbRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, COALESCE(json_extract(data, '$.address'), id)
				FROM resources WHERE resource_type = 'mailboxes'`)
			if err != nil {
				return fmt.Errorf("querying mailboxes: %w", err)
			}
			for mbRows.Next() {
				var id, name string
				if mbRows.Scan(&id, &name) == nil {
					mbNames[id] = name
				}
			}
			mbRows.Close()

			if len(mbNames) == 0 {
				return fmt.Errorf("no mailboxes in local cache — run 'multimail-pp-cli sync --full' first")
			}

			// Query threads and compute staleness
			threadRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, mailboxes_id, data FROM threads`)
			if err != nil {
				return fmt.Errorf("querying threads: %w", err)
			}
			defer threadRows.Close()

			var results []staleThreadRow
			for threadRows.Next() {
				var id, mailboxID string
				var dataBytes []byte
				if err := threadRows.Scan(&id, &mailboxID, &dataBytes); err != nil {
					continue
				}

				var threadData map[string]interface{}
				if json.Unmarshal(dataBytes, &threadData) != nil {
					continue
				}

				// Extract last activity timestamp
				var lastActivity string
				for _, key := range []string{"last_reply_at", "last_message_at", "updated_at", "last_activity_at"} {
					if v, ok := threadData[key]; ok {
						if s, ok := v.(string); ok && s != "" {
							lastActivity = s
							break
						}
					}
				}
				if lastActivity == "" {
					continue
				}

				lastTime, err := time.Parse(time.RFC3339, lastActivity)
				if err != nil {
					// Try RFC3339Nano
					lastTime, err = time.Parse(time.RFC3339Nano, lastActivity)
					if err != nil {
						continue
					}
				}

				if lastTime.After(cutoff) {
					continue // not stale
				}

				staleDays := int(time.Since(lastTime).Hours() / 24)

				subject := ""
				if v, ok := threadData["subject"]; ok {
					if s, ok := v.(string); ok {
						subject = s
					}
				}

				var msgCount int
				if v, ok := threadData["message_count"]; ok {
					switch n := v.(type) {
					case float64:
						msgCount = int(n)
					}
				}
				// Fallback: count emails with this thread as parent
				if msgCount == 0 {
					countRow := db.DB().QueryRowContext(cmd.Context(),
						`SELECT COUNT(*) FROM mailboxes_emails
						WHERE mailboxes_id = ? AND parent_id = ?`, mailboxID, id)
					_ = countRow.Scan(&msgCount)
				}

				mbName := mbNames[mailboxID]
				if mbName == "" {
					mbName = mailboxID
				}

				results = append(results, staleThreadRow{
					ThreadID:     id,
					Mailbox:      mbName,
					MailboxID:    mailboxID,
					Subject:      subject,
					LastReplyAt:  lastActivity,
					StaleDays:    staleDays,
					MessageCount: msgCount,
				})
			}

			// Sort by stale_days descending — most stale first
			sort.Slice(results, func(i, j int) bool {
				return results[i].StaleDays > results[j].StaleDays
			})

			// Apply limit
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Consider threads stale after this many days without reply")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum threads to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
