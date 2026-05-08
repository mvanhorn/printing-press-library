// Compound command: stale thread detection.
// Hand-built transcendence feature — not generated from OpenAPI.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"multimail-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

type staleThread struct {
	ThreadID      string `json:"thread_id"`
	Subject       string `json:"subject"`
	From          string `json:"from"`
	LastActivity  string `json:"last_activity"`
	StaleDays     int    `json:"stale_days"`
	MessageCount  int    `json:"message_count"`
	MailboxID     string `json:"mailbox_id,omitempty"`
}

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var days int
	var mailboxID string
	var limit int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find threads that have gone unanswered past a threshold",
		Long: `Detect conversation threads where the last message is inbound
and older than a configurable threshold. Surfaces the emails you're
dropping the ball on.

Requires synced data — run 'multimail-pp-cli sync' first.`,
		Example: `  # Threads unanswered for 3+ days
  multimail-pp-cli stale --days 3

  # Stale threads in a specific mailbox
  multimail-pp-cli stale --days 2 --mailbox 01ABC123

  # JSON output for automation
  multimail-pp-cli stale --days 3 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("multimail-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'multimail-pp-cli sync' first.", err)
			}
			defer db.Close()

			threads, err := findStaleThreads(db, days, mailboxID, limit)
			if err != nil {
				return err
			}

			jsonMode := flags.asJSON || !isTerminal(cmd.OutOrStdout())
			if jsonMode {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if flags.compact {
					type compactStale struct {
						ThreadID  string `json:"thread_id"`
						Subject   string `json:"subject"`
						StaleDays int    `json:"stale_days"`
					}
					compact := make([]compactStale, len(threads))
					for i, t := range threads {
						compact[i] = compactStale{t.ThreadID, t.Subject, t.StaleDays}
					}
					return enc.Encode(compact)
				}
				// Ensure non-nil slice for consistent JSON output
				if threads == nil {
					threads = []staleThread{}
				}
				return enc.Encode(threads)
			}

			if len(threads) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale threads found. All caught up!")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d stale thread(s) (unanswered > %d days):\n\n", len(threads), days)
			for _, t := range threads {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", t.ThreadID, t.Subject)
				fmt.Fprintf(cmd.OutOrStdout(), "    From: %s | %d days stale | %d messages\n\n", t.From, t.StaleDays, t.MessageCount)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 3, "Threshold in days for staleness")
	cmd.Flags().StringVar(&mailboxID, "mailbox", "", "Filter to specific mailbox ID")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum stale threads to return")

	return cmd
}

func findStaleThreads(db *store.Store, days int, mailboxFilter string, limit int) ([]staleThread, error) {
	sqlDB := db.DB()
	threshold := time.Now().AddDate(0, 0, -days)

	// Strategy: Look at threads where the last email is inbound and older than threshold.
	query := `SELECT id, data FROM threads`
	var args []any
	if mailboxFilter != "" {
		query += ` WHERE mailboxes_id = ?`
		args = append(args, mailboxFilter)
	}

	rows, err := sqlDB.Query(query, args...)
	if err != nil {
		// If threads table has no data, fall back to emails-based detection
		return findStaleFromEmails(db, days, mailboxFilter, limit)
	}
	defer rows.Close()

	var results []staleThread
	for rows.Next() {
		var id string
		var dataStr string
		if err := rows.Scan(&id, &dataStr); err != nil {
			continue
		}

		var thread map[string]any
		if err := json.Unmarshal([]byte(dataStr), &thread); err != nil {
			continue
		}

		// Check if thread has unanswered inbound
		hasUnanswered, _ := thread["has_unanswered_inbound"].(bool)
		if !hasUnanswered {
			continue
		}

		// Parse last activity
		lastActivityStr, _ := thread["last_activity"].(string)
		if lastActivityStr == "" {
			continue
		}
		lastActivity, err := time.Parse(time.RFC3339, lastActivityStr)
		if err != nil {
			continue
		}

		if lastActivity.After(threshold) {
			continue
		}

		staleDays := int(time.Since(lastActivity).Hours() / 24)
		threadID, _ := thread["thread_id"].(string)
		if threadID == "" {
			threadID = id
		}

		// Get subject and from from the thread's email data
		subject := "unknown"
		from := "unknown"
		messageCount := 0
		if emails, ok := thread["emails"].([]any); ok {
			messageCount = len(emails)
			if len(emails) > 0 {
				if lastEmail, ok := emails[len(emails)-1].(map[string]any); ok {
					if s, ok := lastEmail["subject"].(string); ok {
						subject = s
					}
					if f, ok := lastEmail["from"].(string); ok {
						from = f
					}
				}
			}
		}
		if mc, ok := thread["message_count"].(float64); ok && messageCount == 0 {
			messageCount = int(mc)
		}

		// Extract participants for subject fallback
		if subject == "unknown" {
			if parts, ok := thread["participants"].([]any); ok && len(parts) > 0 {
				if p, ok := parts[0].(string); ok {
					from = p
				}
			}
		}

		results = append(results, staleThread{
			ThreadID:     threadID,
			Subject:      subject,
			From:         from,
			LastActivity: lastActivityStr,
			StaleDays:    staleDays,
			MessageCount: messageCount,
		})
	}

	// Sort by staleness descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].StaleDays > results[j].StaleDays
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// findStaleFromEmails is a fallback when threads table is empty.
// Groups emails by thread_id and finds unanswered inbound threads.
func findStaleFromEmails(db *store.Store, days int, mailboxFilter string, limit int) ([]staleThread, error) {
	sqlDB := db.DB()
	threshold := time.Now().AddDate(0, 0, -days)

	query := `SELECT id, data FROM emails ORDER BY json_extract(data, '$.received_at') DESC`
	rows, err := sqlDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying emails: %w", err)
	}
	defer rows.Close()

	type threadInfo struct {
		subject      string
		from         string
		lastActivity time.Time
		count        int
		lastDir      string
		mailboxID    string
	}
	threads := make(map[string]*threadInfo)

	for rows.Next() {
		var id string
		var dataStr string
		if err := rows.Scan(&id, &dataStr); err != nil {
			continue
		}

		var email map[string]any
		if err := json.Unmarshal([]byte(dataStr), &email); err != nil {
			continue
		}

		threadID, _ := email["thread_id"].(string)
		if threadID == "" {
			threadID = id // standalone email
		}

		direction, _ := email["direction"].(string)
		subject, _ := email["subject"].(string)
		from, _ := email["from"].(string)
		receivedAt, _ := email["received_at"].(string)
		mbID, _ := email["mailbox_id"].(string)

		t, _ := time.Parse(time.RFC3339, receivedAt)

		info, exists := threads[threadID]
		if !exists {
			info = &threadInfo{}
			threads[threadID] = info
		}
		info.count++
		if mbID != "" {
			info.mailboxID = mbID
		}
		if t.After(info.lastActivity) {
			info.lastActivity = t
			info.lastDir = direction
			info.subject = subject
			info.from = from
		}
	}

	var results []staleThread
	for threadID, info := range threads {
		if info.lastDir != "inbound" {
			continue
		}
		if info.lastActivity.After(threshold) {
			continue
		}
		if mailboxFilter != "" && info.mailboxID != mailboxFilter {
			continue
		}

		results = append(results, staleThread{
			ThreadID:     threadID,
			Subject:      info.subject,
			From:         info.from,
			LastActivity: info.lastActivity.Format(time.RFC3339),
			StaleDays:    int(time.Since(info.lastActivity).Hours() / 24),
			MessageCount: info.count,
			MailboxID:    info.mailboxID,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].StaleDays > results[j].StaleDays
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}
