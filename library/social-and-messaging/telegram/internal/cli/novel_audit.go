// `audit` — novel #4. Roll-up of outbound activity from the local store.

package cli

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAuditCmd(flags *rootFlags) *cobra.Command {
	var (
		since  string
		chatID string
	)
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Roll-up of outbound activity from the local store (no API call)",
		Long:  "Counts messages sent in a time window, broken down by chat and media type. Useful as a daily ops check or as the first step of a delivery investigation.",
		Example: strings.Trim(`
  telegram-pp-cli audit --since today --json
  telegram-pp-cli audit --since 7d --chat 1234567
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,10",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"audit": map[string]any{}}, flags)
			}
			s, err := openNovelStore(cmd.Context())
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			report, err := buildAuditReport(cmd.Context(), s.DB(), since, chatID)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "today", "Time window: today | 1h | 6h | 1d | 7d | 30d")
	cmd.Flags().StringVar(&chatID, "chat", "", "Filter to a single chat ID or @username")
	return cmd
}

type auditReport struct {
	Since       string          `json:"since"`
	Window      string          `json:"window"`
	Totals      auditTotals     `json:"totals"`
	ByChat      []auditChatRow  `json:"by_chat"`
	ByMediaType []auditMediaRow `json:"by_media_type"`
	Errors      []auditErrorRow `json:"errors,omitempty"`
}

type auditTotals struct {
	Sent        int    `json:"sent"`
	Received    int    `json:"received"`
	LastSentAt  string `json:"last_sent_at,omitempty"`
	UniqueChats int    `json:"unique_chats"`
}

type auditChatRow struct {
	ChatID string `json:"chat_id"`
	Sent   int    `json:"sent"`
}

type auditMediaRow struct {
	MediaType string `json:"media_type"`
	Sent      int    `json:"sent"`
}

type auditErrorRow struct {
	ChatID string `json:"chat_id"`
	Error  string `json:"error"`
	Date   int64  `json:"date"`
}

func buildAuditReport(ctx context.Context, db *sql.DB, since, chatID string) (auditReport, error) {
	// Initialize slices empty so JSON marshals as [] (not null) on the
	// empty-result path — jq/agent consumers iterate cleanly.
	report := auditReport{
		Since:       since,
		Window:      since,
		ByChat:      []auditChatRow{},
		ByMediaType: []auditMediaRow{},
	}
	cutoff, err := parseSince(since)
	if err != nil {
		return report, err
	}
	cutoffUnix := cutoff.Unix()

	chatWhere := ""
	args := []any{cutoffUnix}
	if chatID != "" {
		chatWhere = " AND chat_id = ?"
		args = append(args, chatID)
	}

	// Totals
	if err := db.QueryRowContext(ctx,
		`SELECT
            COALESCE(SUM(direction = 'outbound'), 0),
            COALESCE(SUM(direction = 'inbound'),  0),
            COUNT(DISTINCT chat_id)
         FROM telegram_messages
         WHERE date >= ?`+chatWhere,
		args...,
	).Scan(&report.Totals.Sent, &report.Totals.Received, &report.Totals.UniqueChats); err != nil {
		return report, err
	}
	// last sent
	var lastSent sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT MAX(date) FROM telegram_messages WHERE direction='outbound' AND date >= ?`+chatWhere,
		args...,
	).Scan(&lastSent); err == nil && lastSent.Valid {
		report.Totals.LastSentAt = isoFromUnix(lastSent.Int64)
	}

	// By chat
	rows, err := db.QueryContext(ctx,
		`SELECT chat_id, COUNT(*) AS sent
         FROM telegram_messages
         WHERE direction='outbound' AND date >= ?`+chatWhere+`
         GROUP BY chat_id ORDER BY sent DESC LIMIT 50`,
		args...,
	)
	if err != nil {
		return report, err
	}
	for rows.Next() {
		var r auditChatRow
		if err := rows.Scan(&r.ChatID, &r.Sent); err != nil {
			rows.Close()
			return report, err
		}
		report.ByChat = append(report.ByChat, r)
	}
	rows.Close()

	// By media type
	rows2, err := db.QueryContext(ctx,
		`SELECT COALESCE(NULLIF(media_type,''),'text') AS media_type, COUNT(*) AS sent
         FROM telegram_messages
         WHERE direction='outbound' AND date >= ?`+chatWhere+`
         GROUP BY media_type ORDER BY sent DESC`,
		args...,
	)
	if err != nil {
		return report, err
	}
	for rows2.Next() {
		var r auditMediaRow
		if err := rows2.Scan(&r.MediaType, &r.Sent); err != nil {
			rows2.Close()
			return report, err
		}
		report.ByMediaType = append(report.ByMediaType, r)
	}
	rows2.Close()

	// Errors (recorded with non-empty error column; populated if/when send
	// failures get persisted with error metadata).
	rows3, err := db.QueryContext(ctx,
		`SELECT chat_id, COALESCE(error,''), date FROM telegram_messages
         WHERE error IS NOT NULL AND error != '' AND date >= ?`+chatWhere+`
         ORDER BY date DESC LIMIT 20`,
		args...,
	)
	if err == nil {
		for rows3.Next() {
			var r auditErrorRow
			if err := rows3.Scan(&r.ChatID, &r.Error, &r.Date); err != nil {
				break
			}
			report.Errors = append(report.Errors, r)
		}
		rows3.Close()
	}

	return report, nil
}

func isoFromUnix(u int64) string {
	return time.Unix(u, 0).UTC().Format(time.RFC3339)
}
