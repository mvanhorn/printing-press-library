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

func newNovelThreadsStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagMailbox string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List conversation threads with no reply in N days — surfaces dropped conversations.",
		Long: `Stale thread detection performs a time-windowed join of synced threads and
emails. A thread is stale when the last activity is older than the
threshold and has unanswered inbound messages. The API has no inactive
threads query — this analysis is only possible against synced data.

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

			hintIfUnsynced(cmd, db, "threads")
			hintIfStale(cmd, db, "threads", flags.maxAge)

			sqlDB := db.DB()

			// Load threads from both domain table and generic resources
			type threadInfo struct {
				ThreadID       string   `json:"thread_id"`
				MailboxID      string   `json:"mailbox_id,omitempty"`
				MailboxAddress string   `json:"mailbox_address,omitempty"`
				LastActivity   string   `json:"last_activity"`
				LastActivityTs time.Time `json:"-"`
				MessageCount   int      `json:"message_count"`
				HasUnanswered  bool     `json:"has_unanswered_inbound"`
				Participants   []string `json:"participants,omitempty"`
				StaleDays      int      `json:"stale_days"`
				Subject        string   `json:"subject,omitempty"`
			}

			// Build mailbox name map
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

			cutoff := time.Now().AddDate(0, 0, -flagDays)
			var staleThreads []threadInfo

			// Try threads from resources table (ThreadResponse shape)
			tRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'threads'`)
			if err == nil {
				for tRows.Next() {
					var raw string
					if tRows.Scan(&raw) != nil {
						break
					}
					var t map[string]any
					if json.Unmarshal([]byte(raw), &t) != nil {
						continue
					}

					threadID, _ := t["thread_id"].(string)
					if threadID == "" {
						threadID, _ = t["id"].(string)
					}
					mailboxID, _ := t["mailboxes_id"].(string)
					if mailboxID == "" {
						mailboxID, _ = t["mailbox_id"].(string)
					}

					if flagMailbox != "" && mailboxID != flagMailbox {
						continue
					}

					lastActivity, lastTs := parseThreadLastActivity(t)
					if lastTs.IsZero() {
						continue
					}

					// Only stale threads
					if lastTs.After(cutoff) {
						continue
					}

					msgCount := 0
					if mc, ok := t["message_count"].(float64); ok {
						msgCount = int(mc)
					}

					hasUnanswered, _ := t["has_unanswered_inbound"].(bool)

					var participants []string
					if ps, ok := t["participants"].([]any); ok {
						for _, p := range ps {
							if s, ok := p.(string); ok {
								participants = append(participants, s)
							}
						}
					}

					staleDays := int(time.Since(lastTs).Hours() / 24)

					staleThreads = append(staleThreads, threadInfo{
						ThreadID:       threadID,
						MailboxID:      mailboxID,
						MailboxAddress: mailboxNames[mailboxID],
						LastActivity:   lastActivity,
						LastActivityTs: lastTs,
						MessageCount:   msgCount,
						HasUnanswered:  hasUnanswered,
						Participants:   participants,
						StaleDays:      staleDays,
					})
				}
				tRows.Close()
			}

			// Also check threads in the domain table
			dtRows, err := sqlDB.QueryContext(ctx, `SELECT "id", "mailboxes_id", "data" FROM "threads"`)
			if err == nil {
				seen := map[string]bool{}
				for _, st := range staleThreads {
					seen[st.ThreadID] = true
				}
				for dtRows.Next() {
					var id, mbID, raw string
					if dtRows.Scan(&id, &mbID, &raw) != nil {
						break
					}
					if seen[id] {
						continue
					}
					if flagMailbox != "" && mbID != flagMailbox {
						continue
					}
					var t map[string]any
					if json.Unmarshal([]byte(raw), &t) != nil {
						continue
					}

					lastActivity, lastTs := parseThreadLastActivity(t)
					if lastTs.IsZero() || lastTs.After(cutoff) {
						continue
					}

					msgCount := 0
					if mc, ok := t["message_count"].(float64); ok {
						msgCount = int(mc)
					}
					hasUnanswered, _ := t["has_unanswered_inbound"].(bool)
					staleDays := int(time.Since(lastTs).Hours() / 24)

					staleThreads = append(staleThreads, threadInfo{
						ThreadID:       id,
						MailboxID:      mbID,
						MailboxAddress: mailboxNames[mbID],
						LastActivity:   lastActivity,
						LastActivityTs: lastTs,
						MessageCount:   msgCount,
						HasUnanswered:  hasUnanswered,
						StaleDays:      staleDays,
					})
				}
				dtRows.Close()
			}

			// Sort by staleness (oldest first)
			sort.Slice(staleThreads, func(i, j int) bool {
				return staleThreads[i].LastActivityTs.Before(staleThreads[j].LastActivityTs)
			})

			output := map[string]any{
				"threshold_days": flagDays,
				"stale_threads":  staleThreads,
				"total":          len(staleThreads),
				"generated_at":   time.Now().UTC().Format(time.RFC3339),
			}
			if flagMailbox != "" {
				output["mailbox_filter"] = flagMailbox
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 7, "Number of days of inactivity to consider a thread stale")
	cmd.Flags().StringVar(&flagMailbox, "mailbox", "", "Filter by mailbox ID")
	return cmd
}

func parseThreadLastActivity(t map[string]any) (string, time.Time) {
	// Try last_activity field first
	if ts, ok := parseNumericTime(t["last_activity"]); ok {
		return ts.Format(time.RFC3339), ts
	}
	// Try updated_at
	if ts, ok := parseNumericTime(t["updated_at"]); ok {
		return ts.Format(time.RFC3339), ts
	}
	// Try created_at as fallback
	if ts, ok := parseNumericTime(t["created_at"]); ok {
		return ts.Format(time.RFC3339), ts
	}
	return "", time.Time{}
}
