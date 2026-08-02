// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: follow-up tracker. Classifies every thread by who had the
// last word (a sendAs identity vs anyone else) and ages it. Via the live API
// this is a per-thread fetch storm at 20 quota units per message; locally it
// is one window query.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

type followupRow struct {
	ThreadID  string `json:"thread_id"`
	LastFrom  string `json:"last_from"`
	Subject   string `json:"subject"`
	LastAt    string `json:"last_at"`
	AgeDays   int    `json:"age_days"`
	Direction string `json:"direction"` // in: you owe a reply; out: they owe you
	MessageID string `json:"last_message_id"`
}

// selfAddresses derives the mailbox's own identities from messages that carry
// the SENT system label in the local mirror.
func selfAddresses(cmd *cobra.Command, db *store.Store) (map[string]bool, error) {
	rows, err := db.DB().QueryContext(cmd.Context(), `
		SELECT DISTINCT lower(json_extract(data,'$.from_email'))
		FROM messages
		WHERE EXISTS (
			SELECT 1 FROM json_each(json_extract(messages.data,'$.labelIds'))
			WHERE json_each.value = 'SENT'
		)`)
	if err != nil {
		return nil, fmt.Errorf("deriving own addresses: %w", err)
	}
	defer rows.Close()
	self := map[string]bool{}
	for rows.Next() {
		var addr sql.NullString
		if err := rows.Scan(&addr); err != nil {
			continue
		}
		if addr.Valid && addr.String != "" {
			self[addr.String] = true
		}
	}
	return self, rows.Err()
}

func newNovelFollowupsCmd(flags *rootFlags) *cobra.Command {
	var direction string
	var days int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "followups",
		Short: "Threads awaiting a reply: yours to send (--direction in) or theirs (--direction out)",
		Long: `Finds conversations where the last message has sat unanswered for N days.

  --direction out  your message was last; they never replied (outreach chasing)
  --direction in   their message was last; you owe the reply

Requires a populated local mirror (gmail-pp-cli pull) that includes your
sent mail.`,
		Example: strings.Trim(`
  gmail-pp-cli followups --direction out --days 3
  gmail-pp-cli followups --direction out --days 3 --agent
  gmail-pp-cli followups --direction in --days 2`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "classify threads by who owes a reply")
			}
			if direction != "in" && direction != "out" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--direction must be \"in\" or \"out\""))
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "messages") {
				hintIfStale(cmd, db, "messages", flags.maxAge)
			}
			self, err := selfAddresses(cmd, db)
			if err != nil {
				return err
			}
			if len(self) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: no sent mail in the local mirror; run gmail-pp-cli pull --query \"in:sent OR in:inbox\" so both directions are visible")
			}
			// Latest message per thread.
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT thread_id, id,
				       COALESCE(lower(json_extract(data,'$.from_email')),'') AS from_email,
				       COALESCE(json_extract(data,'$.from'),'') AS from_disp,
				       COALESCE(json_extract(data,'$.subject'),'') AS subject,
				       COALESCE(json_extract(data,'$.internal_ts'),'') AS ts,
				       COALESCE(json_extract(data,'$.labelIds'),'[]') AS labels
				FROM (
					SELECT *, ROW_NUMBER() OVER (
						PARTITION BY thread_id
						ORDER BY json_extract(data,'$.internal_ts') DESC
					) AS rn
					FROM messages
					WHERE thread_id IS NOT NULL AND thread_id != ''
				)
				WHERE rn = 1`)
			if err != nil {
				return fmt.Errorf("querying thread heads: %w", err)
			}
			defer rows.Close()
			cutoff := time.Now().AddDate(0, 0, -days)
			var out []followupRow
			for rows.Next() {
				var threadID, msgID, fromEmail, fromDisp, subject, ts, labels string
				if err := rows.Scan(&threadID, &msgID, &fromEmail, &fromDisp, &subject, &ts, &labels); err != nil {
					return fmt.Errorf("scanning thread head: %w", err)
				}
				if ts == "" {
					continue
				}
				lastAt, err := time.Parse(time.RFC3339, ts)
				if err != nil || lastAt.After(cutoff) {
					continue
				}
				// Ignore drafts, spam, and trash heads.
				if strings.Contains(labels, `"DRAFT"`) || strings.Contains(labels, `"SPAM"`) || strings.Contains(labels, `"TRASH"`) {
					continue
				}
				fromSelf := self[fromEmail]
				switch direction {
				case "out":
					if !fromSelf {
						continue
					}
				case "in":
					if fromSelf {
						continue
					}
					// Only count threads still sitting in the inbox as owed replies.
					if !strings.Contains(labels, `"INBOX"`) {
						continue
					}
				}
				out = append(out, followupRow{
					ThreadID:  threadID,
					LastFrom:  fromDisp,
					Subject:   subject,
					LastAt:    lastAt.Local().Format("2006-01-02"),
					AgeDays:   int(time.Since(lastAt).Hours() / 24),
					Direction: direction,
					MessageID: msgID,
				})
				if len(out) >= limit {
					break
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if wantsJSONOutput(cmd, flags) {
				if out == nil {
					out = []followupRow{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no %s-direction follow-ups older than %d day(s)\n", direction, days)
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "AGE\tLAST\tFROM\tSUBJECT\tREPLY WITH")
			for _, r := range out {
				fmt.Fprintf(tw, "%dd\t%s\t%s\t%s\tgmail-pp-cli reply %s\n",
					r.AgeDays, r.LastAt, truncateCell(r.LastFrom, 28), truncateCell(r.Subject, 44), r.MessageID)
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&direction, "direction", "out", "Which side owes the reply: in (you) or out (them)")
	cmd.Flags().IntVar(&days, "days", 3, "Minimum age in days of the unanswered message")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum threads to list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}
