// Compound command: oversight dashboard.
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

type oversightSummary struct {
	PendingCount     int              `json:"pending_count"`
	TotalDecisions   int              `json:"total_decisions"`
	ApprovalRate     float64          `json:"approval_rate"`
	RejectionRate    float64          `json:"rejection_rate"`
	AvgDecisionTime  string           `json:"avg_decision_time"`
	AvgDecisionSecs  float64          `json:"avg_decision_seconds"`
	MostGatedSenders []gatedSender    `json:"most_gated_senders,omitempty"`
	SyncedAt         string           `json:"synced_at,omitempty"`
}

type gatedSender struct {
	Sender string `json:"sender"`
	Count  int    `json:"count"`
}

func newOversightSummaryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Oversight dashboard: pending count, decision velocity, approval rate",
		Long: `See pending approval count, average decision time, approval rate, and
most-gated senders across all mailboxes. The operator's command center.

Requires synced data — run 'multimail-pp-cli sync' first.`,
		Example: `  # Full oversight dashboard
  multimail-pp-cli oversight summary

  # JSON for automation
  multimail-pp-cli oversight summary --json

  # Key metrics only
  multimail-pp-cli oversight summary --json --select pending_count,avg_decision_time,approval_rate`,
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

			summary, err := computeOversightSummary(db)
			if err != nil {
				return err
			}

			jsonMode := flags.asJSON || !isTerminal(cmd.OutOrStdout())
			if jsonMode {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Oversight Dashboard")
			fmt.Fprintln(cmd.OutOrStdout(), "===================")
			fmt.Fprintf(cmd.OutOrStdout(), "Pending:          %d\n", summary.PendingCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Total Decisions:  %d\n", summary.TotalDecisions)
			fmt.Fprintf(cmd.OutOrStdout(), "Approval Rate:    %.1f%%\n", summary.ApprovalRate*100)
			fmt.Fprintf(cmd.OutOrStdout(), "Rejection Rate:   %.1f%%\n", summary.RejectionRate*100)
			fmt.Fprintf(cmd.OutOrStdout(), "Avg Decision Time: %s\n", summary.AvgDecisionTime)

			if len(summary.MostGatedSenders) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nMost-Gated Senders:")
				for _, s := range summary.MostGatedSenders {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s (%d)\n", s.Sender, s.Count)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func computeOversightSummary(db *store.Store) (*oversightSummary, error) {
	sqlDB := db.DB()

	// Count pending
	var pendingCount int
	sqlDB.QueryRow(`SELECT COUNT(*) FROM oversight WHERE json_extract(data, '$.action') IS NULL OR json_extract(data, '$.action') = ''`).Scan(&pendingCount)

	// Count approved and rejected from audit log
	var approved, rejected int
	sqlDB.QueryRow(`SELECT COUNT(*) FROM oversight WHERE json_extract(data, '$.action') = 'approve'`).Scan(&approved)
	sqlDB.QueryRow(`SELECT COUNT(*) FROM oversight WHERE json_extract(data, '$.action') = 'reject'`).Scan(&rejected)

	totalDecisions := approved + rejected
	approvalRate := 0.0
	rejectionRate := 0.0
	if totalDecisions > 0 {
		approvalRate = float64(approved) / float64(totalDecisions)
		rejectionRate = float64(rejected) / float64(totalDecisions)
	}

	// Average decision time from audit log
	avgDecisionSecs := 0.0
	avgDecisionTime := "N/A"

	// Query audit entries for oversight decisions to compute decision velocity
	rows, err := sqlDB.Query(`SELECT data FROM resources WHERE resource_type = 'audit-log' ORDER BY synced_at DESC LIMIT 500`)
	if err == nil {
		defer rows.Close()
		var totalTime float64
		var decisionCount int

		for rows.Next() {
			var dataStr string
			if err := rows.Scan(&dataStr); err != nil {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(dataStr), &entry); err != nil {
				continue
			}
			action, _ := entry["action"].(string)
			if action != "oversight.approve" && action != "oversight.reject" {
				continue
			}
			createdStr, _ := entry["created_at"].(string)
			if createdStr == "" {
				continue
			}
			created, err := time.Parse(time.RFC3339, createdStr)
			if err != nil {
				continue
			}

			// Look up the email's received_at from the metadata
			if meta, ok := entry["metadata"].(map[string]any); ok {
				if emailTime, ok := meta["email_received_at"].(string); ok {
					received, err := time.Parse(time.RFC3339, emailTime)
					if err == nil {
						dt := created.Sub(received).Seconds()
						if dt > 0 {
							totalTime += dt
							decisionCount++
						}
					}
				}
			}
		}

		if decisionCount > 0 {
			avgDecisionSecs = totalTime / float64(decisionCount)
			avgDecisionTime = formatDuration(time.Duration(avgDecisionSecs) * time.Second)
		}
	}

	// Most-gated senders: find which senders appear most in oversight
	senderCounts := make(map[string]int)
	senderRows, err := sqlDB.Query(`SELECT data FROM oversight`)
	if err == nil {
		defer senderRows.Close()
		for senderRows.Next() {
			var dataStr string
			if err := senderRows.Scan(&dataStr); err != nil {
				continue
			}
			var item map[string]any
			if err := json.Unmarshal([]byte(dataStr), &item); err != nil {
				continue
			}
			// Try to get the from address from the gated email
			emailID, _ := item["email_id"].(string)
			if emailID != "" {
				emailData, err := db.Get("emails", emailID)
				if err == nil && emailData != nil {
					var email map[string]any
					if json.Unmarshal(emailData, &email) == nil {
						if from, ok := email["from"].(string); ok {
							senderCounts[from]++
						}
					}
				}
			}
		}
	}

	var topSenders []gatedSender
	for sender, count := range senderCounts {
		topSenders = append(topSenders, gatedSender{Sender: sender, Count: count})
	}
	sort.Slice(topSenders, func(i, j int) bool {
		return topSenders[i].Count > topSenders[j].Count
	})
	if len(topSenders) > 5 {
		topSenders = topSenders[:5]
	}

	syncedAt := db.GetLastSyncedAt("oversight")

	return &oversightSummary{
		PendingCount:     pendingCount,
		TotalDecisions:   totalDecisions,
		ApprovalRate:     approvalRate,
		RejectionRate:    rejectionRate,
		AvgDecisionTime:  avgDecisionTime,
		AvgDecisionSecs:  avgDecisionSecs,
		MostGatedSenders: topSenders,
		SyncedAt:         syncedAt,
	}, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}
