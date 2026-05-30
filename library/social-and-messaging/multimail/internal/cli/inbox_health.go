// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for CLI Printing Press.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelInboxHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Per-mailbox health snapshot: unread count, oldest unread age, reply rate, and thread depth.",
		Long: `Inbox health aggregates synced emails per mailbox to compute unread count,
oldest unread age, reply rate, and average thread depth. No single API
endpoint returns this cross-mailbox view.

Run 'multimail-pp-cli sync' first to populate local data.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "multimail-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return fmt.Errorf("no local data. Run 'multimail-pp-cli sync' first")
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "mailboxes_emails")
			hintIfStale(cmd, db, "mailboxes_emails", flags.maxAge)

			sqlDB := db.DB()

			// Load mailbox names
			mailboxNames := map[string]string{}
			mbRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'mailboxes'`)
			if err == nil {
				for mbRows.Next() {
					var raw string
					if mbRows.Scan(&raw) != nil {
						break
					}
					var m map[string]any
					if json.Unmarshal([]byte(raw), &m) == nil {
						id, _ := m["id"].(string)
						addr, _ := m["address"].(string)
						if id != "" {
							mailboxNames[id] = addr
						}
					}
				}
				mbRows.Close()
			}

			// Aggregate email stats per mailbox
			type mailboxHealth struct {
				MailboxID       string  `json:"mailbox_id"`
				Address         string  `json:"address,omitempty"`
				TotalEmails     int     `json:"total_emails"`
				InboundCount    int     `json:"inbound_count"`
				OutboundCount   int     `json:"outbound_count"`
				UnreadCount     int     `json:"unread_count,omitempty"`
				OldestUnread    string  `json:"oldest_unread,omitempty"`
				OldestUnreadAge string  `json:"oldest_unread_age,omitempty"`
				ReplyRate       float64 `json:"reply_rate_pct"`
				UniqueThreads   int     `json:"unique_threads"`
			}

			byMailbox := map[string]*mailboxHealth{}
			threadsByMailbox := map[string]map[string]bool{} // mailbox -> set of thread_ids

			// Query mailbox emails
			rows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'mailboxes_emails'`)
			if err != nil {
				return fmt.Errorf("querying emails: %w", err)
			}
			for rows.Next() {
				var raw string
				if rows.Scan(&raw) != nil {
					break
				}
				var e map[string]any
				if json.Unmarshal([]byte(raw), &e) != nil {
					continue
				}
				mailboxID, _ := e["mailboxes_id"].(string)
				if mailboxID == "" {
					mailboxID, _ = e["mailbox_id"].(string)
				}
				if mailboxID == "" {
					continue
				}

				mh, ok := byMailbox[mailboxID]
				if !ok {
					mh = &mailboxHealth{
						MailboxID: mailboxID,
						Address:   mailboxNames[mailboxID],
					}
					byMailbox[mailboxID] = mh
					threadsByMailbox[mailboxID] = map[string]bool{}
				}

				mh.TotalEmails++
				dir, _ := e["direction"].(string)
				status, _ := e["status"].(string)
				if dir == "inbound" || dir == "received" {
					mh.InboundCount++
				} else {
					mh.OutboundCount++
				}

				// Track unread (inbound + not read)
				if (dir == "inbound" || dir == "received") && status != "read" {
					mh.UnreadCount++
					if ts, ok := parseNumericTime(e["received_at"]); ok {
						tsStr := ts.Format(time.RFC3339)
						if mh.OldestUnread == "" || tsStr < mh.OldestUnread {
							mh.OldestUnread = tsStr
						}
					}
				}

				// Track threads
				if tid, ok := e["thread_id"].(string); ok && tid != "" {
					threadsByMailbox[mailboxID][tid] = true
				}
			}
			rows.Close()

			// Also check generic emails resource for additional data
			rows2, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'emails'`)
			if err == nil {
				for rows2.Next() {
					var raw string
					if rows2.Scan(&raw) != nil {
						break
					}
					var e map[string]any
					if json.Unmarshal([]byte(raw), &e) != nil {
						continue
					}
					// emails without specific mailbox_id go to "unknown"
					mailboxID := "unknown"
					dir, _ := e["direction"].(string)

					mh, ok := byMailbox[mailboxID]
					if !ok {
						mh = &mailboxHealth{MailboxID: mailboxID}
						byMailbox[mailboxID] = mh
						threadsByMailbox[mailboxID] = map[string]bool{}
					}

					mh.TotalEmails++
					if dir == "inbound" || dir == "received" {
						mh.InboundCount++
						status, _ := e["status"].(string)
						if status != "read" {
							mh.UnreadCount++
							if ts, ok := parseNumericTime(e["received_at"]); ok {
								tsStr := ts.Format(time.RFC3339)
								if mh.OldestUnread == "" || tsStr < mh.OldestUnread {
									mh.OldestUnread = tsStr
								}
							}
						}
					} else {
						mh.OutboundCount++
					}
					if tid, ok := e["thread_id"].(string); ok && tid != "" {
						threadsByMailbox[mailboxID][tid] = true
					}
				}
				rows2.Close()
			}

			// Compute derived stats
			var results []mailboxHealth
			for mbID, mh := range byMailbox {
				mh.UniqueThreads = len(threadsByMailbox[mbID])
				if mh.InboundCount > 0 && mh.OutboundCount > 0 {
					mh.ReplyRate = float64(mh.OutboundCount) / float64(mh.InboundCount) * 100
					if mh.ReplyRate > 100 {
						mh.ReplyRate = 100
					}
				}
				if mh.OldestUnread != "" {
					if ts, err := time.Parse(time.RFC3339, mh.OldestUnread); err == nil {
						mh.OldestUnreadAge = formatDuration(time.Since(ts))
					}
				}
				if mh.MailboxID == "unknown" && mh.TotalEmails == 0 {
					continue
				}
				results = append(results, *mh)
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].TotalEmails > results[j].TotalEmails
			})

			output := map[string]any{
				"mailboxes":    results,
				"total":        len(results),
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}
	return cmd
}
