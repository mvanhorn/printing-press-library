package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"multimail-pp-cli/internal/store"
)

type inboxHealthRow struct {
	Mailbox        string  `json:"mailbox"`
	MailboxID      string  `json:"mailbox_id"`
	TotalEmails    int     `json:"total_emails"`
	UnreadCount    int     `json:"unread_count"`
	OldestUnread   string  `json:"oldest_unread"`
	ReplyRate      float64 `json:"reply_rate"`
	ThreadCount    int     `json:"thread_count"`
	AvgThreadDepth float64 `json:"avg_thread_depth"`
}

func newInboxHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Per-mailbox health snapshot: unread count, oldest unread age, reply rate, and thread depth",
		Long: `Inbox health shows a per-mailbox health snapshot from synced data.
It reports unread count, oldest unread email age, reply rate, and
average thread depth — metrics that reveal whether a mailbox is
keeping up with its volume.

Reply rate is derived from synced emails: outbound emails with a
parent_id are counted as replies to existing conversations. Thread
metrics are derived from emails grouped by parent_id, since the API
has no list-threads endpoint. Both rely on synced email data only.

Requires synced data (run 'multimail-pp-cli sync --full' first).`,
		Example: strings.Trim(`
  multimail-pp-cli inbox health --json
  multimail-pp-cli inbox health --json --select mailbox,unread_count,reply_rate`, "\n"),
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

			// Get mailbox info
			type mbInfo struct {
				ID   string
				Name string
			}
			var mailboxes []mbInfo
			mbRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, COALESCE(json_extract(data, '$.address'), id)
				FROM resources WHERE resource_type = 'mailboxes'`)
			if err != nil {
				return fmt.Errorf("querying mailboxes: %w", err)
			}
			for mbRows.Next() {
				var m mbInfo
				if err := mbRows.Scan(&m.ID, &m.Name); err != nil {
					continue
				}
				mailboxes = append(mailboxes, m)
			}
			mbRows.Close()

			if len(mailboxes) == 0 {
				return fmt.Errorf("no mailboxes in local cache — run 'multimail-pp-cli sync --full' first")
			}

			var results []inboxHealthRow
			for _, mb := range mailboxes {
				// Total emails for this mailbox
				var totalEmails int
				totalRow := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM mailboxes_emails WHERE mailboxes_id = ?`, mb.ID)
				_ = totalRow.Scan(&totalEmails)

				// Unread count — check whether the 'read' field exists in any
				// email for this mailbox; only fall back to 'is_read' when the
				// primary field is absent entirely (not when the count is zero,
				// which is a legitimate "all read" state).
				var unreadCount int
				var hasReadField int
				readFieldCheck := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM mailboxes_emails
					WHERE mailboxes_id = ?
					AND json_extract(data, '$.read') IS NOT NULL`, mb.ID)
				_ = readFieldCheck.Scan(&hasReadField)

				if hasReadField > 0 {
					// Primary field exists — count unread via 'read'
					unreadRow := db.DB().QueryRowContext(cmd.Context(),
						`SELECT COUNT(*) FROM mailboxes_emails
						WHERE mailboxes_id = ?
						AND json_extract(data, '$.read') = 0`, mb.ID)
					_ = unreadRow.Scan(&unreadCount)
				} else {
					// Fallback: 'read' field absent — try 'is_read'
					unreadFallback := db.DB().QueryRowContext(cmd.Context(),
						`SELECT COUNT(*) FROM mailboxes_emails
						WHERE mailboxes_id = ?
						AND (json_extract(data, '$.is_read') = 0
						  OR json_extract(data, '$.is_read') = 'false')`, mb.ID)
					_ = unreadFallback.Scan(&unreadCount)
				}

				// Oldest unread email age
				var oldestUnread string
				oldestRow := db.DB().QueryRowContext(cmd.Context(),
					`SELECT MIN(COALESCE(json_extract(data, '$.received_at'), json_extract(data, '$.created_at'), synced_at))
					FROM mailboxes_emails
					WHERE mailboxes_id = ?
					AND (json_extract(data, '$.read') = 0
					  OR json_extract(data, '$.is_read') = 0
					  OR json_extract(data, '$.is_read') = 'false')`, mb.ID)
				var oldestTS string
				if oldestRow.Scan(&oldestTS) == nil && oldestTS != "" {
					if t, err := time.Parse(time.RFC3339, oldestTS); err == nil {
						d := time.Since(t)
						switch {
						case d < time.Hour:
							oldestUnread = fmt.Sprintf("%dm", int(d.Minutes()))
						case d < 24*time.Hour:
							oldestUnread = fmt.Sprintf("%dh", int(d.Hours()))
						case d < 30*24*time.Hour:
							oldestUnread = fmt.Sprintf("%dd", int(d.Hours()/24))
						default:
							oldestUnread = fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
						}
					}
				}
				if oldestUnread == "" {
					oldestUnread = "—"
				}

				// PATCH: Derive reply rate from mailboxes_emails.
				// The reply endpoint is POST-only (no list GET), so sync
				// cannot populate the reply table. Instead, count outbound
				// emails with a parent_id — these are replies to existing
				// conversations.
				var replyCount int
				replyRow := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM mailboxes_emails
					WHERE mailboxes_id = ?
					AND parent_id IS NOT NULL AND parent_id != ''
					AND (json_extract(data, '$.direction') IN ('outbound', 'sent')
					  OR LOWER(COALESCE(json_extract(data, '$.from'), '')) = LOWER(?))`,
					mb.ID, mb.Name)
				_ = replyRow.Scan(&replyCount)

				var replyRate float64
				if totalEmails > 0 {
					replyRate = float64(replyCount) / float64(totalEmails) * 100
				}

				// PATCH: Derive thread count and average depth from
				// mailboxes_emails parent_id grouping. The API has no
				// list-threads endpoint (only GET single thread by ID),
				// so sync --full cannot populate the threads table.
				var threadCount int
				var avgDepth float64
				threadRows, terr := db.DB().QueryContext(cmd.Context(),
					`SELECT COUNT(*) FROM mailboxes_emails
					WHERE mailboxes_id = ? AND parent_id IS NOT NULL AND parent_id != ''
					GROUP BY parent_id
					HAVING COUNT(*) >= 2`, mb.ID)
				if terr == nil {
					var totalDepth int
					for threadRows.Next() {
						var cnt int
						if threadRows.Scan(&cnt) == nil {
							threadCount++
							totalDepth += cnt
						}
					}
					threadRows.Close()
					if threadCount > 0 {
						avgDepth = float64(totalDepth) / float64(threadCount)
					}
				}

				results = append(results, inboxHealthRow{
					Mailbox:        mb.Name,
					MailboxID:      mb.ID,
					TotalEmails:    totalEmails,
					UnreadCount:    unreadCount,
					OldestUnread:   oldestUnread,
					ReplyRate:      replyRate,
					ThreadCount:    threadCount,
					AvgThreadDepth: avgDepth,
				})
			}

			// Sort by unread count descending — most backlogged first
			sort.Slice(results, func(i, j int) bool {
				return results[i].UnreadCount > results[j].UnreadCount
			})

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
