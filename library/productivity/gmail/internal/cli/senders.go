// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: sender leaderboard. Gmail has no sender aggregation anywhere;
// this is a GROUP BY over the local mirror that would cost O(mailbox) live
// fetches. Detail mode (--sender) shows one address's history.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type senderRow struct {
	Sender      string  `json:"sender"`
	Count       int     `json:"count"`
	Unread      int     `json:"unread"`
	TotalBytes  int64   `json:"total_bytes"`
	FirstSeen   string  `json:"first_seen"`
	LastSeen    string  `json:"last_seen"`
	UnreadRatio float64 `json:"unread_ratio"`
}

type senderMonthRow struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

func newNovelSendersCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sender string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "senders",
		Short: "Who fills your mailbox: per-sender count, unread, size, first/last seen",
		Long: `Aggregates the local mailbox mirror by sender. The live API cannot
answer this at all; run "gmail-pp-cli pull" first to build the mirror.
Pass --sender for one address's monthly history.`,
		Example: strings.Trim(`
  gmail-pp-cli senders --limit 20
  gmail-pp-cli senders --limit 20 --agent
  gmail-pp-cli senders --sender billing@stripe.com`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "aggregate the local mirror by sender")
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "messages") {
				hintIfStale(cmd, db, "messages", flags.maxAge)
			}

			if sender != "" {
				rows, err := db.DB().QueryContext(cmd.Context(), `
					SELECT COALESCE(substr(json_extract(data,'$.internal_ts'),1,7),'') AS month,
					       COUNT(*)
					FROM messages
					WHERE lower(json_extract(data,'$.from_email')) = lower(?)
					GROUP BY month ORDER BY month DESC LIMIT 36`, sender)
				if err != nil {
					return fmt.Errorf("querying sender history: %w", err)
				}
				defer rows.Close()
				var months []senderMonthRow
				for rows.Next() {
					var m senderMonthRow
					if err := rows.Scan(&m.Month, &m.Count); err != nil {
						return fmt.Errorf("scanning sender history: %w", err)
					}
					months = append(months, m)
				}
				if err := rows.Err(); err != nil {
					return err
				}
				out := map[string]any{"sender": sender, "months": months}
				if wantsJSONOutput(cmd, flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				if len(months) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "no local messages from %s (run: gmail-pp-cli pull)\n", sender)
					return nil
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "MONTH\tMESSAGES")
				for _, m := range months {
					fmt.Fprintf(tw, "%s\t%d\n", m.Month, m.Count)
				}
				tw.Flush()
				return nil
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(json_extract(data,'$.from_email'),'(unknown)') AS sender,
				       COUNT(*) AS cnt,
				       SUM(CASE WHEN json_extract(data,'$.unread') THEN 1 ELSE 0 END) AS unread,
				       COALESCE(SUM(size_estimate),0) AS bytes,
				       COALESCE(MIN(json_extract(data,'$.internal_ts')),'') AS first_seen,
				       COALESCE(MAX(json_extract(data,'$.internal_ts')),'') AS last_seen
				FROM messages
				GROUP BY sender
				ORDER BY cnt DESC
				LIMIT ?`, limit)
			if err != nil {
				return fmt.Errorf("querying senders: %w", err)
			}
			defer rows.Close()
			var out []senderRow
			for rows.Next() {
				var r senderRow
				if err := rows.Scan(&r.Sender, &r.Count, &r.Unread, &r.TotalBytes, &r.FirstSeen, &r.LastSeen); err != nil {
					return fmt.Errorf("scanning sender row: %w", err)
				}
				if r.Count > 0 {
					r.UnreadRatio = float64(r.Unread) / float64(r.Count)
				}
				// internal_ts is RFC3339; keep the date component only.
				// truncateCell would ellipsize it into "2026-08-0…".
				r.FirstSeen = dateOnly(r.FirstSeen)
				r.LastSeen = dateOnly(r.LastSeen)
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if wantsJSONOutput(cmd, flags) {
				if out == nil {
					out = []senderRow{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "local mirror is empty; run: gmail-pp-cli pull --since 30d")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "COUNT\tUNREAD\tSIZE\tFIRST\tLAST\tSENDER")
			for _, r := range out {
				fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\t%s\n", r.Count, r.Unread, humanBytes(r.TotalBytes), r.FirstSeen, r.LastSeen, r.Sender)
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of senders to show")
	cmd.Flags().StringVar(&sender, "sender", "", "Show monthly history for one address instead of the leaderboard")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}

// dateOnly keeps the YYYY-MM-DD prefix of an RFC3339 timestamp.
func dateOnly(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// humanBytes renders a byte count compactly (KB/MB/GB).
func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
