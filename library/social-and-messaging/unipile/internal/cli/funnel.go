// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type funnelView struct {
	Window          string  `json:"window"`
	Since           string  `json:"since"`
	InvitesSent     int     `json:"invitations_sent"`
	InvitesReceived int     `json:"invitations_received"`
	Accepted        int     `json:"connections_accepted"`
	Replied         int     `json:"connections_that_replied"`
	AcceptedPerSent float64 `json:"accepted_per_sent_pct"`
	ReplyRatePct    float64 `json:"reply_rate_pct"`
	CohortCaveat    string  `json:"cohort_caveat,omitempty"`
	Note            string  `json:"note,omitempty"`
}

func newNovelFunnelCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		weeks  int
	)
	cmd := &cobra.Command{
		Use:   "funnel",
		Short: "Invitation funnel: sent, accepted, replied, with conversion rates",
		Long: strings.Trim(`
Joins synced invitation history, connections, and conversations to report
outreach conversion over a window. Three sources the API never returns together.

Connections accepted counts every connection formed inside the window, whichever
side started it. Invitations sent counts only what you sent inside the window, so
the accepted-per-sent ratio is indicative, not a true cohort conversion: a
connection formed today may answer an invitation you sent last month, and
invitations other people sent you count as accepted without ever being sent.

Reply rate is the share of those new connections who have since sent you at
least one message, which is a true cohort measure.

Use this command to judge outreach copy before scaling volume. Do NOT use it
for per-person state; use 'contact'.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli funnel --weeks 4 --agent
  unipile-pp-cli funnel --weeks 1
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--weeks=4",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "funnel")
			}
			if weeks <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--weeks must be positive"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			now := time.Now().UTC()
			cutoff := now.AddDate(0, 0, -7*weeks)
			view := funnelView{Window: fmt.Sprintf("%dw", weeks), Since: cutoff.Format(time.RFC3339)}

			db, ok, err := novelStore(cmd, flags, dbPath, view)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			rows, err := db.QueryContext(ctx, `
				SELECT COALESCE(json_extract(data,'$.parsed_datetime'),'')
				FROM resources WHERE resource_type = 'users-invite-sent'`)
			if err != nil {
				return fmt.Errorf("reading sent invitations: %w", err)
			}
			for rows.Next() {
				var ts sql.NullString
				if err := rows.Scan(&ts); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning invitation: %w", err)
				}
				if when, perr := time.Parse(time.RFC3339, ts.String); perr == nil && when.After(cutoff) {
					view.InvitesSent++
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating invitations: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing invitation rows: %w", err)
			}

			rRows, err := db.QueryContext(ctx, `
				SELECT COALESCE(json_extract(data,'$.parsed_datetime'),'')
				FROM resources WHERE resource_type = 'users-invite-received'`)
			if err == nil {
				for rRows.Next() {
					var ts sql.NullString
					if serr := rRows.Scan(&ts); serr != nil {
						_ = rRows.Close()
						return fmt.Errorf("scanning received invitation: %w", serr)
					}
					if when, perr := time.Parse(time.RFC3339, ts.String); perr == nil && when.After(cutoff) {
						view.InvitesReceived++
					}
				}
				if rerr := rRows.Err(); rerr != nil {
					_ = rRows.Close()
					return fmt.Errorf("iterating received invitations: %w", rerr)
				}
				if cerr := rRows.Close(); cerr != nil {
					return fmt.Errorf("closing received invitation rows: %w", cerr)
				}
			}

			relations, err := loadRelations(ctx, db)
			if err != nil {
				return err
			}
			chats, err := loadChats(ctx, db, false)
			if err != nil {
				return err
			}
			_, inbound, _, err := loadLastMessages(ctx, db)
			if err != nil {
				return err
			}
			repliedBy := make(map[string]bool, len(chats))
			for _, c := range chats {
				if c.AttendeeID != "" && inbound[c.ID] > 0 {
					repliedBy[c.AttendeeID] = true
				}
			}
			for _, r := range relations {
				when, perr := time.Parse(time.RFC3339, r.CreatedAt)
				if perr != nil || !when.After(cutoff) {
					continue
				}
				view.Accepted++
				if repliedBy[r.MemberID] {
					view.Replied++
				}
			}
			if view.InvitesSent > 0 {
				view.AcceptedPerSent = round1(100 * float64(view.Accepted) / float64(view.InvitesSent))
			}
			if view.Accepted > view.InvitesSent {
				view.CohortCaveat = "more connections were formed than invitations you sent in this window: the surplus comes from invitations you sent earlier and from invitations other people sent you. Treat accepted-per-sent as indicative, not a cohort conversion rate."
			}
			if view.Accepted > 0 {
				view.ReplyRatePct = round1(100 * float64(view.Replied) / float64(view.Accepted))
			}
			if view.InvitesSent == 0 && view.Accepted == 0 {
				view.Note = "no invitation or connection activity in the window; run 'unipile-pp-cli sync --resources users-invite-sent,users-relations'"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "STAGE\tCOUNT\tRATE\n")
			fmt.Fprintf(tw, "invitations sent\t%d\t-\n", view.InvitesSent)
			fmt.Fprintf(tw, "invitations received\t%d\t-\n", view.InvitesReceived)
			fmt.Fprintf(tw, "connections accepted\t%d\t%.1f%% of sent\n", view.Accepted, view.AcceptedPerSent)
			fmt.Fprintf(tw, "of those, replied\t%d\t%.1f%%\n", view.Replied, view.ReplyRatePct)
			if err := tw.Flush(); err != nil {
				return err
			}
			if view.CohortCaveat != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nnote: %s\n", view.CohortCaveat)
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().IntVar(&weeks, "weeks", 4, "window size in weeks")
	return cmd
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
