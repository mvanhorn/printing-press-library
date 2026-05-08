// Compound command: inbox health score.
// Hand-built transcendence feature — not generated from OpenAPI.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"multimail-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

type healthScore struct {
	Overall         int              `json:"overall"`
	Grade           string           `json:"grade"`
	Components      healthComponents `json:"components"`
	Mailbox         string           `json:"mailbox,omitempty"`
	SyncedAt        string           `json:"synced_at,omitempty"`
	Recommendation  string           `json:"recommendation"`
}

type healthComponents struct {
	UnreadRatio    componentScore `json:"unread_ratio"`
	BounceRate     componentScore `json:"bounce_rate"`
	QuotaHeadroom  componentScore `json:"quota_headroom"`
	OversightQueue componentScore `json:"oversight_queue"`
}

type componentScore struct {
	Score   int     `json:"score"`
	Max     int     `json:"max"`
	Value   float64 `json:"value"`
	Detail  string  `json:"detail"`
}

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var mailboxID string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Composite inbox health score from cached data",
		Long: `Single-number composite score combining unread ratio, bounce rate,
quota headroom, and oversight queue depth. Instantly tells you if an
inbox needs attention.

Score range: 0-100 (A: 90+, B: 75-89, C: 60-74, D: 40-59, F: <40).
Requires synced data — run 'multimail-pp-cli sync' first.`,
		Example: `  # Overall health across all mailboxes
  multimail-pp-cli health

  # Health for a specific mailbox
  multimail-pp-cli health --mailbox 01ABC123

  # JSON output for automation
  multimail-pp-cli health --json`,
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

			result, err := computeHealth(db, mailboxID)
			if err != nil {
				return err
			}

			jsonMode := flags.asJSON || !isTerminal(cmd.OutOrStdout())
			if jsonMode {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			// Human-friendly output
			fmt.Fprintf(cmd.OutOrStdout(), "Inbox Health: %d/100 (%s)\n\n", result.Overall, result.Grade)
			printComponent(cmd, "Unread Ratio", result.Components.UnreadRatio)
			printComponent(cmd, "Bounce Rate", result.Components.BounceRate)
			printComponent(cmd, "Quota Headroom", result.Components.QuotaHeadroom)
			printComponent(cmd, "Oversight Queue", result.Components.OversightQueue)
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", result.Recommendation)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&mailboxID, "mailbox", "", "Filter to specific mailbox ID")

	return cmd
}

func printComponent(cmd *cobra.Command, name string, c componentScore) {
	fmt.Fprintf(cmd.OutOrStdout(), "  %-18s %2d/%d  %s\n", name, c.Score, c.Max, c.Detail)
}

func computeHealth(db *store.Store, mailboxFilter string) (*healthScore, error) {
	sqlDB := db.DB()

	// 1. Unread ratio (0-25 points)
	// Count inbound emails that haven't been replied to
	var totalInbound, unansweredInbound int
	inboundQuery := `SELECT COUNT(*) FROM emails WHERE json_extract(data, '$.direction') = 'inbound'`
	unansweredQuery := `SELECT COUNT(*) FROM emails WHERE json_extract(data, '$.direction') = 'inbound' AND json_extract(data, '$.status') NOT IN ('replied', 'spam_flagged', 'spam_quarantined')`

	if mailboxFilter != "" {
		inboundQuery += ` AND parent_id = ?`
		unansweredQuery += ` AND parent_id = ?`
		sqlDB.QueryRow(inboundQuery, mailboxFilter).Scan(&totalInbound)
		sqlDB.QueryRow(unansweredQuery, mailboxFilter).Scan(&unansweredInbound)
	} else {
		sqlDB.QueryRow(inboundQuery).Scan(&totalInbound)
		sqlDB.QueryRow(unansweredQuery).Scan(&unansweredInbound)
	}

	unreadRatio := 0.0
	if totalInbound > 0 {
		unreadRatio = float64(unansweredInbound) / float64(totalInbound)
	}
	unreadScore := int(math.Round(25 * (1 - unreadRatio)))
	unreadDetail := fmt.Sprintf("%.0f%% unanswered (%d/%d inbound)", unreadRatio*100, unansweredInbound, totalInbound)

	// 2. Bounce rate (0-25 points)
	var totalSent, bounced int
	sentQuery := `SELECT COUNT(*) FROM emails WHERE json_extract(data, '$.direction') = 'outbound'`
	bouncedQuery := `SELECT COUNT(*) FROM emails WHERE json_extract(data, '$.bounced_at') IS NOT NULL`

	if mailboxFilter != "" {
		sentQuery += ` AND parent_id = ?`
		bouncedQuery += ` AND parent_id = ?`
		sqlDB.QueryRow(sentQuery, mailboxFilter).Scan(&totalSent)
		sqlDB.QueryRow(bouncedQuery, mailboxFilter).Scan(&bounced)
	} else {
		sqlDB.QueryRow(sentQuery).Scan(&totalSent)
		sqlDB.QueryRow(bouncedQuery).Scan(&bounced)
	}

	bounceRate := 0.0
	if totalSent > 0 {
		bounceRate = float64(bounced) / float64(totalSent)
	}
	// Bounce rate scoring: 0% = 25, 5% = 20, 10% = 15, 20%+ = 0
	bounceScore := int(math.Round(25 * math.Max(0, 1-bounceRate*5)))
	bounceDetail := fmt.Sprintf("%.1f%% bounce rate (%d/%d sent)", bounceRate*100, bounced, totalSent)

	// 3. Quota headroom (0-25 points)
	var sentThisMonth, monthlyQuota int
	row := sqlDB.QueryRow(`SELECT json_extract(data, '$.emails_sent_this_month'), json_extract(data, '$.monthly_email_quota') FROM account LIMIT 1`)
	if err := row.Scan(&sentThisMonth, &monthlyQuota); err != nil {
		sentThisMonth = 0
		monthlyQuota = 0
	}

	quotaUsed := 0.0
	if monthlyQuota > 0 {
		quotaUsed = float64(sentThisMonth) / float64(monthlyQuota)
	}
	quotaScore := 25
	quotaDetail := "no quota data"
	if monthlyQuota > 0 {
		quotaScore = int(math.Round(25 * math.Max(0, 1-quotaUsed)))
		remaining := monthlyQuota - sentThisMonth
		quotaDetail = fmt.Sprintf("%.0f%% used (%d/%d, %d remaining)", quotaUsed*100, sentThisMonth, monthlyQuota, remaining)
	}

	// 4. Oversight queue depth (0-25 points)
	var pendingOversight int
	sqlDB.QueryRow(`SELECT COUNT(*) FROM oversight WHERE json_extract(data, '$.action') IS NULL OR json_extract(data, '$.action') = 'pending'`).Scan(&pendingOversight)
	oversightScore := 25
	oversightDetail := "no pending items"
	if pendingOversight > 0 {
		// Score degrades: 1-2 pending = 20, 3-5 = 15, 6-10 = 10, 10+ = 5, 20+ = 0
		switch {
		case pendingOversight <= 2:
			oversightScore = 20
		case pendingOversight <= 5:
			oversightScore = 15
		case pendingOversight <= 10:
			oversightScore = 10
		case pendingOversight <= 20:
			oversightScore = 5
		default:
			oversightScore = 0
		}
		oversightDetail = fmt.Sprintf("%d emails pending oversight", pendingOversight)
	}

	overall := unreadScore + bounceScore + quotaScore + oversightScore
	grade := "F"
	switch {
	case overall >= 90:
		grade = "A"
	case overall >= 75:
		grade = "B"
	case overall >= 60:
		grade = "C"
	case overall >= 40:
		grade = "D"
	}

	recommendation := recommendAction(unreadScore, bounceScore, quotaScore, oversightScore, pendingOversight)

	// Get sync time
	syncedAt := ""
	if sa := db.GetLastSyncedAt("emails"); sa != "" {
		syncedAt = sa
	}

	return &healthScore{
		Overall: overall,
		Grade:   grade,
		Mailbox: mailboxFilter,
		SyncedAt: syncedAt,
		Recommendation: recommendation,
		Components: healthComponents{
			UnreadRatio:    componentScore{Score: unreadScore, Max: 25, Value: unreadRatio, Detail: unreadDetail},
			BounceRate:     componentScore{Score: bounceScore, Max: 25, Value: bounceRate, Detail: bounceDetail},
			QuotaHeadroom:  componentScore{Score: quotaScore, Max: 25, Value: quotaUsed, Detail: quotaDetail},
			OversightQueue: componentScore{Score: oversightScore, Max: 25, Value: float64(pendingOversight), Detail: oversightDetail},
		},
	}, nil
}

func recommendAction(unread, bounce, quota, oversight, pendingCount int) string {
	// Return the most actionable recommendation
	if oversight < 15 && pendingCount > 0 {
		return fmt.Sprintf("Action needed: %d emails awaiting oversight decision. Run 'mm oversight list' to review.", pendingCount)
	}
	if quota < 15 {
		return "Warning: quota running low. Consider upgrading your plan or reducing send volume."
	}
	if bounce < 15 {
		return "Warning: high bounce rate. Check recipient addresses and domain verification status."
	}
	if unread < 15 {
		return "Many unanswered inbound emails. Run 'mm stale' to see which threads need attention."
	}
	return "Inbox is healthy. No immediate action needed."
}
