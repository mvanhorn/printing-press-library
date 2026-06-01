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

func newNovelTrustStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Fleet-wide view of each mailbox's oversight mode, time-at-level, and upgrade eligibility.",
		Long: `Trust status joins synced mailboxes with audit-log events to show the current
oversight mode, how long each mailbox has been at its current level, and
whether it is eligible for an upgrade.

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

			hintIfUnsynced(cmd, db, "mailboxes")
			hintIfStale(cmd, db, "mailboxes", flags.maxAge)

			sqlDB := db.DB()

			// Build map of latest mode-change events per mailbox from audit log
			type modeChange struct {
				Timestamp time.Time
				NewMode   string
			}
			lastModeChange := map[string]modeChange{}
			auditRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'audit-log'`)
			if err == nil {
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
					if action != "oversight_mode_change" && action != "mode_change" &&
						action != "upgrade" && action != "request_upgrade" {
						continue
					}
					mailboxID := ""
					newMode := ""
					if meta, ok := entry["metadata"].(map[string]any); ok {
						mailboxID, _ = meta["mailbox_id"].(string)
						newMode, _ = meta["new_mode"].(string)
						if newMode == "" {
							newMode, _ = meta["oversight_mode"].(string)
						}
					}
					if mailboxID == "" {
						mailboxID, _ = entry["resource_id"].(string)
					}
					if mailboxID == "" {
						continue
					}
					ts, ok := parseNumericTime(entry["created_at"])
					if !ok {
						continue
					}
					if existing, exists := lastModeChange[mailboxID]; !exists || ts.After(existing.Timestamp) {
						lastModeChange[mailboxID] = modeChange{Timestamp: ts, NewMode: newMode}
					}
				}
				auditRows.Close()
			}

			// Trust ladder ordering for upgrade eligibility
			trustLadder := map[string]int{
				"gated_all":  0,
				"gated_send": 1,
				"monitored":  2,
				"autonomous": 3,
			}
			maxLevel := 3

			type trustRow struct {
				MailboxID      string `json:"mailbox_id"`
				Address        string `json:"address"`
				OversightMode  string `json:"oversight_mode"`
				TimeAtLevel    string `json:"time_at_level"`
				TimeAtLevelSec int64  `json:"time_at_level_seconds"`
				TrustLevel     int    `json:"trust_level"`
				CanUpgrade     bool   `json:"can_upgrade"`
				NextMode       string `json:"next_mode,omitempty"`
			}

			var results []trustRow
			mbRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'mailboxes'`)
			if err != nil {
				return fmt.Errorf("querying mailboxes: %w", err)
			}
			for mbRows.Next() {
				var raw string
				if mbRows.Scan(&raw) != nil {
					break
				}
				var m map[string]any
				if json.Unmarshal([]byte(raw), &m) != nil {
					continue
				}
				id, _ := m["id"].(string)
				addr, _ := m["address"].(string)
				mode, _ := m["oversight_mode"].(string)
				if mode == "" {
					mode = "gated_send" // default
				}

				level := trustLadder[mode]
				canUpgrade := level < maxLevel

				var timeAtLevel time.Duration
				if mc, ok := lastModeChange[id]; ok && mc.NewMode == mode {
					timeAtLevel = time.Since(mc.Timestamp)
				} else {
					// Fall back to created_at
					if ts, ok := parseNumericTime(m["created_at"]); ok {
						timeAtLevel = time.Since(ts)
					}
				}

				nextMode := ""
				if canUpgrade {
					for name, lvl := range trustLadder {
						if lvl == level+1 {
							nextMode = name
							break
						}
					}
				}

				results = append(results, trustRow{
					MailboxID:      id,
					Address:        addr,
					OversightMode:  mode,
					TimeAtLevel:    formatDuration(timeAtLevel),
					TimeAtLevelSec: int64(timeAtLevel.Seconds()),
					TrustLevel:     level,
					CanUpgrade:     canUpgrade,
					NextMode:       nextMode,
				})
			}
			mbRows.Close()

			sort.Slice(results, func(i, j int) bool {
				return results[i].TrustLevel > results[j].TrustLevel
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

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd", days)
}
