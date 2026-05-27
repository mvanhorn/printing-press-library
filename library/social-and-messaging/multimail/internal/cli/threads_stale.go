package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/multimail/internal/store"
)

type staleThreadRow struct {
	ThreadID     string `json:"thread_id"`
	Mailbox      string `json:"mailbox"`
	MailboxID    string `json:"mailbox_id"`
	Subject      string `json:"subject"`
	LastReplyAt  string `json:"last_reply_at"`
	StaleDays    int    `json:"stale_days"`
	MessageCount int    `json:"message_count"`
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

When the threads table has data (from CLI thread operations), it uses
that directly. Otherwise, it derives threads from synced emails by
grouping on thread_id — any email chain with 2+ messages is a thread.

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

			var results []staleThreadRow

			// Check if the threads table has data
			var threadCount int
			countRow := db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM threads`)
			_ = countRow.Scan(&threadCount)

			if threadCount > 0 {
				// Use threads table directly
				results, err = staleFromThreadsTable(cmd, db, cutoff, mbNames)
			} else {
				// PATCH: Derive threads from mailboxes_emails parent_id
				// grouping. The API has no list-threads endpoint, so
				// sync --full cannot populate the threads table. Fall
				// back to treating shared parent_id groups as threads.
				results, err = staleFromEmailParentGroups(cmd, db, cutoff, mbNames)
			}
			if err != nil {
				return err
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

// staleFromThreadsTable uses the dedicated threads table (populated by CLI thread operations).
func staleFromThreadsTable(cmd *cobra.Command, db *store.Store, cutoff time.Time, mbNames map[string]string) ([]staleThreadRow, error) {
	threadRows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT id, mailboxes_id, data FROM threads`)
	if err != nil {
		return nil, fmt.Errorf("querying threads: %w", err)
	}
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

		lastTime, perr := time.Parse(time.RFC3339, lastActivity)
		if perr != nil {
			lastTime, perr = time.Parse(time.RFC3339Nano, lastActivity)
			if perr != nil {
				continue
			}
		}
		if lastTime.After(cutoff) {
			continue
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
			if n, ok := v.(float64); ok {
				msgCount = int(n)
			}
		}
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
	threadRows.Close()
	return results, nil
}

// staleFromEmailParentGroups derives threads from mailboxes_emails by grouping
// on thread_id from the API response. Any thread_id with 2+ emails is treated
// as a conversation thread. Uses json_extract(data, '$.thread_id') — not the
// parent_id column, which stores the FK to the parent mailbox.
func staleFromEmailParentGroups(cmd *cobra.Command, db *store.Store, cutoff time.Time, mbNames map[string]string) ([]staleThreadRow, error) {
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT
			json_extract(data, '$.thread_id'),
			mailboxes_id,
			COUNT(*) as msg_count,
			MAX(COALESCE(json_extract(data, '$.received_at'), json_extract(data, '$.created_at'), synced_at)) as last_ts,
			-- Pick subject from any email in the thread
			COALESCE(json_extract(data, '$.subject'), '')
		FROM mailboxes_emails
		WHERE json_extract(data, '$.thread_id') IS NOT NULL
		AND json_extract(data, '$.thread_id') != ''
		GROUP BY json_extract(data, '$.thread_id'), mailboxes_id
		HAVING COUNT(*) >= 2`)
	if err != nil {
		return nil, fmt.Errorf("querying email threads: %w", err)
	}
	var results []staleThreadRow
	for rows.Next() {
		var parentID, mailboxID, lastTS, subject string
		var msgCount int
		if rows.Scan(&parentID, &mailboxID, &msgCount, &lastTS, &subject) != nil {
			continue
		}
		if lastTS == "" {
			continue
		}

		lastTime, perr := time.Parse(time.RFC3339, lastTS)
		if perr != nil {
			lastTime, perr = time.Parse(time.RFC3339Nano, lastTS)
			if perr != nil {
				continue
			}
		}
		if lastTime.After(cutoff) {
			continue
		}

		staleDays := int(time.Since(lastTime).Hours() / 24)

		mbName := mbNames[mailboxID]
		if mbName == "" {
			mbName = mailboxID
		}

		results = append(results, staleThreadRow{
			ThreadID:     parentID,
			Mailbox:      mbName,
			MailboxID:    mailboxID,
			Subject:      subject,
			LastReplyAt:  lastTS,
			StaleDays:    staleDays,
			MessageCount: msgCount,
		})
	}
	rows.Close()
	return results, nil
}
