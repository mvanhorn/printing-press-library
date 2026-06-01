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

func newNovelTrustTimelineCmd(flags *rootFlags) *cobra.Command {
	var flagMailbox string

	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Per-mailbox chronological history of every oversight mode change with timestamps and who triggered it.",
		Long: `Trust timeline queries audit-log events filtered by oversight mode changes
to produce a chronological trust progression for a mailbox.

Use this command for viewing the historical progression of oversight mode
changes. Do NOT use this command for current-state fleet view; use
'trust status' instead.

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

			hintIfUnsynced(cmd, db, "audit-log")
			hintIfStale(cmd, db, "audit-log", flags.maxAge)

			sqlDB := db.DB()

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

			type timelineEntry struct {
				Timestamp  string `json:"timestamp"`
				MailboxID  string `json:"mailbox_id"`
				Address    string `json:"address,omitempty"`
				Action     string `json:"action"`
				OldMode    string `json:"old_mode,omitempty"`
				NewMode    string `json:"new_mode,omitempty"`
				ActorKeyID string `json:"actor_key_id,omitempty"`
			}

			var entries []timelineEntry
			auditRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'audit-log'`)
			if err != nil {
				return fmt.Errorf("querying audit-log: %w", err)
			}
			for auditRows.Next() {
				var raw string
				if auditRows.Scan(&raw) != nil {
					break
				}
				var entry map[string]any
				if json.Unmarshal([]byte(raw), &entry) != nil {
					continue
				}
				action, _ := entry["action"].(string)
				// Include mode changes, upgrades, and related trust actions
				isTrustAction := action == "oversight_mode_change" ||
					action == "mode_change" ||
					action == "upgrade" ||
					action == "request_upgrade" ||
					action == "trust_upgrade" ||
					action == "mailbox_update"
				if !isTrustAction {
					continue
				}

				mailboxID := ""
				oldMode := ""
				newMode := ""
				if meta, ok := entry["metadata"].(map[string]any); ok {
					mailboxID, _ = meta["mailbox_id"].(string)
					oldMode, _ = meta["old_mode"].(string)
					newMode, _ = meta["new_mode"].(string)
					if newMode == "" {
						newMode, _ = meta["oversight_mode"].(string)
					}
				}
				if mailboxID == "" {
					mailboxID, _ = entry["resource_id"].(string)
				}

				// For mailbox_update, only include if oversight_mode changed
				if action == "mailbox_update" && newMode == "" {
					continue
				}

				// Filter by mailbox if specified
				if flagMailbox != "" && mailboxID != flagMailbox {
					// Also check address
					if addr := mailboxNames[mailboxID]; addr != flagMailbox {
						continue
					}
				}

				createdAt := ""
				if ts, ok := parseNumericTime(entry["created_at"]); ok {
					createdAt = ts.Format(time.RFC3339)
				} else if s, ok := entry["created_at"].(string); ok {
					createdAt = s
				}

				actorKeyID, _ := entry["actor_key_id"].(string)

				entries = append(entries, timelineEntry{
					Timestamp:  createdAt,
					MailboxID:  mailboxID,
					Address:    mailboxNames[mailboxID],
					Action:     action,
					OldMode:    oldMode,
					NewMode:    newMode,
					ActorKeyID: actorKeyID,
				})
			}
			auditRows.Close()

			// Sort chronologically; entries with no parseable timestamp sort last.
			sort.Slice(entries, func(i, j int) bool {
				ti, tj := entries[i].Timestamp, entries[j].Timestamp
				if ti == "" {
					return false
				}
				if tj == "" {
					return true
				}
				return ti < tj
			})

			output := map[string]any{
				"events":       entries,
				"total":        len(entries),
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}
			if flagMailbox != "" {
				output["mailbox_filter"] = flagMailbox
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}
	cmd.Flags().StringVar(&flagMailbox, "mailbox", "", "Filter by mailbox ID or address")
	return cmd
}
