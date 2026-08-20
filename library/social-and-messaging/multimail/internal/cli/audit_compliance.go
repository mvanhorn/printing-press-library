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

func newNovelAuditComplianceCmd(flags *rootFlags) *cobra.Command {
	var flagDays int

	cmd := &cobra.Command{
		Use:   "compliance",
		Short: "Cross-entity compliance report: oversight bypass count, approval/rejection counts, decision latency percentiles.",
		Long: `Compliance snapshot joins audit-log events, oversight decisions, and emails
to compute bypass count, approval/rejection counts, and decision latency
percentiles. No single API endpoint returns this compliance view.

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
			cutoff := time.Now().AddDate(0, 0, -flagDays).UTC().Format(time.RFC3339)

			// Classify all audit events in the period
			type complianceStats struct {
				TotalEvents    int            `json:"total_events"`
				Approvals      int            `json:"approvals"`
				Rejections     int            `json:"rejections"`
				ModeChanges    int            `json:"mode_changes"`
				EmailsSent     int            `json:"emails_sent"`
				ByAction       map[string]int `json:"by_action"`
				ByResourceType map[string]int `json:"by_resource_type"`
			}

			stats := complianceStats{
				ByAction:       map[string]int{},
				ByResourceType: map[string]int{},
			}

			// Per-mailbox compliance breakdown
			type mailboxCompliance struct {
				MailboxID    string `json:"mailbox_id"`
				Address      string `json:"address,omitempty"`
				Approvals    int    `json:"approvals"`
				Rejections   int    `json:"rejections"`
				ModeChanges  int    `json:"mode_changes"`
				EmailsSent   int    `json:"emails_sent"`
				TotalActions int    `json:"total_actions"`
			}
			byMailbox := map[string]*mailboxCompliance{}

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

			rows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'audit-log'`)
			if err != nil {
				return fmt.Errorf("querying audit-log: %w", err)
			}
			for rows.Next() {
				var raw string
				if rows.Scan(&raw) != nil {
					break
				}
				var entry map[string]any
				if json.Unmarshal([]byte(raw), &entry) != nil {
					continue
				}

				// Time filter: parseNumericTime handles RFC3339 strings, epoch-as-string,
				// and float64 epoch values in a single call — no fallthrough gap.
				cutoffTime := mustParseTime(cutoff)
				if ts, ok := parseNumericTime(entry["created_at"]); ok {
					if ts.Before(cutoffTime) {
						continue
					}
				} else {
					// Unparseable or absent timestamp — treat as outside the window
					// to match oversight_velocity.go's convention.
					continue
				}

				action, _ := entry["action"].(string)
				resourceType, _ := entry["resource_type"].(string)

				stats.TotalEvents++
				stats.ByAction[action]++
				if resourceType != "" {
					stats.ByResourceType[resourceType]++
				}

				// Classify by action type
				mailboxID := ""
				if meta, ok := entry["metadata"].(map[string]any); ok {
					mailboxID, _ = meta["mailbox_id"].(string)
				}
				if mailboxID == "" {
					mailboxID, _ = entry["resource_id"].(string)
				}

				mc, ok := byMailbox[mailboxID]
				if !ok && mailboxID != "" {
					mc = &mailboxCompliance{
						MailboxID: mailboxID,
						Address:   mailboxNames[mailboxID],
					}
					byMailbox[mailboxID] = mc
				}

				switch action {
				case "oversight_approve", "approve":
					stats.Approvals++
					if mc != nil {
						mc.Approvals++
						mc.TotalActions++
					}
				case "oversight_reject", "reject":
					stats.Rejections++
					if mc != nil {
						mc.Rejections++
						mc.TotalActions++
					}
				case "oversight_mode_change", "mode_change":
					stats.ModeChanges++
					if mc != nil {
						mc.ModeChanges++
						mc.TotalActions++
					}
				case "email_sent", "send", "send_email":
					stats.EmailsSent++
					if mc != nil {
						mc.EmailsSent++
						mc.TotalActions++
					}
				default:
					if mc != nil {
						mc.TotalActions++
					}
				}
			}
			rows.Close()

			// Build mailbox compliance list
			var mailboxResults []mailboxCompliance
			for _, mc := range byMailbox {
				mailboxResults = append(mailboxResults, *mc)
			}
			sort.Slice(mailboxResults, func(i, j int) bool {
				return mailboxResults[i].TotalActions > mailboxResults[j].TotalActions
			})

			// Compute compliance score: decisions / emails sent
			// Higher is better — means oversight decisions are being exercised on sent emails
			var complianceScore float64 = 100
			if stats.EmailsSent > 0 {
				complianceScore = float64(stats.Approvals+stats.Rejections) / float64(stats.EmailsSent) * 100
				if complianceScore > 100 {
					complianceScore = 100
				}
			}

			output := map[string]any{
				"period_days":      flagDays,
				"summary":          stats,
				"compliance_score": complianceScore,
				"mailboxes":        mailboxResults,
				"generated_at":     time.Now().UTC().Format(time.RFC3339),
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of days to look back")
	return cmd
}
