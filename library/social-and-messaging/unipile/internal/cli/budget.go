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

// Conservative LinkedIn caps published in Unipile's "Provider Limits and
// Restrictions" guide. Unipile enforces none of these; LinkedIn enforces all of
// them and answers with 422/429/500 or an account restriction.
const (
	defaultInviteDailyCap  = 100
	defaultInviteWeeklyCap = 200
)

type budgetLine struct {
	Metric    string `json:"metric"`
	Window    string `json:"window"`
	Used      int    `json:"used"`
	Cap       int    `json:"cap"`
	Remaining int    `json:"remaining"`
	Status    string `json:"status"`
}

type budgetView struct {
	Account        string       `json:"account,omitempty"`
	AccountType    string       `json:"account_type,omitempty"`
	GeneratedAt    string       `json:"generated_at"`
	Lines          []budgetLine `json:"lines"`
	Caveat         string       `json:"caveat"`
	MirrorSyncedAt string       `json:"mirror_synced_at,omitempty"`
	Note           string       `json:"note,omitempty"`
}

func newNovelBudgetCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		dailyCap  int
		weeklyCap int
	)
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "LinkedIn invitation headroom left today and this week, per account",
		Long: strings.Trim(`
Counts the invitations already sent from the local mirror and subtracts them
from LinkedIn's published caps, so you can see how much outreach headroom is
left before LinkedIn starts answering with 422/429/500.

Unipile enforces no limits of its own ("we don't enforce any limits on our
side") - LinkedIn does, silently. Counts come from synced invitation history,
so they include invitations sent from the LinkedIn UI as well as from this CLI.

Use this command before any bulk invitation run. Do NOT treat it as LinkedIn's
own counter; it reports what your synced history shows.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli budget --agent
  unipile-pp-cli budget --daily-cap 80 --weekly-cap 200
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--daily-cap=100",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "budget")
			}
			if dailyCap <= 0 || weeklyCap <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--daily-cap and --weekly-cap must be positive"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := budgetView{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Lines:       make([]budgetLine, 0),
				Caveat:      "LinkedIn's real caps vary by account age, subscription, and connection count. These are Unipile's conservative published recommendations, not a guarantee.",
			}
			db, ok, err := novelStore(cmd, flags, dbPath, view)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			now := time.Now().UTC()
			dayCutoff := now.Add(-24 * time.Hour)
			weekCutoff := now.Add(-7 * 24 * time.Hour)

			rows, err := db.QueryContext(ctx, `
				SELECT COALESCE(json_extract(data,'$.parsed_datetime'),'')
				FROM resources WHERE resource_type = 'users-invite-sent'`)
			if err != nil {
				return fmt.Errorf("reading sent invitations: %w", err)
			}
			var day, week, total int
			for rows.Next() {
				var ts sql.NullString
				if err := rows.Scan(&ts); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning invitation: %w", err)
				}
				total++
				when, perr := time.Parse(time.RFC3339, ts.String)
				if perr != nil {
					continue
				}
				if when.After(dayCutoff) {
					day++
				}
				if when.After(weekCutoff) {
					week++
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating invitations: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing invitation rows: %w", err)
			}

			var acctName, acctType sql.NullString
			if err := db.QueryRowContext(ctx, `SELECT COALESCE(name,''), COALESCE(type,'') FROM accounts LIMIT 1`).Scan(&acctName, &acctType); err == nil {
				view.Account = acctName.String
				view.AccountType = acctType.String
			}
			var syncedAt sql.NullString
			if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(last_synced_at),'') FROM sync_state WHERE resource_type LIKE 'users-invite%'`).Scan(&syncedAt); err == nil {
				view.MirrorSyncedAt = syncedAt.String
			}

			view.Lines = append(view.Lines,
				budgetLine{Metric: "invitations", Window: "24h", Used: day, Cap: dailyCap, Remaining: max(0, dailyCap-day), Status: budgetStatus(day, dailyCap)},
				budgetLine{Metric: "invitations", Window: "7d", Used: week, Cap: weeklyCap, Remaining: max(0, weeklyCap-week), Status: budgetStatus(week, weeklyCap)},
			)
			if total == 0 {
				view.Note = "no invitation history in the local mirror; run 'unipile-pp-cli sync --resources users-invite-sent' for accurate counts"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Account != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n\n", view.Account, view.AccountType)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "METRIC\tWINDOW\tUSED\tCAP\tREMAINING\tSTATUS")
			for _, l := range view.Lines {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%s\n", l.Metric, l.Window, l.Used, l.Cap, l.Remaining, l.Status)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Caveat)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().IntVar(&dailyCap, "daily-cap", defaultInviteDailyCap, "LinkedIn invitations considered safe per 24h for this account")
	cmd.Flags().IntVar(&weeklyCap, "weekly-cap", defaultInviteWeeklyCap, "LinkedIn invitations considered safe per 7 days for this account")
	return cmd
}

func budgetStatus(used, cap int) string {
	switch {
	case used >= cap:
		return "exhausted"
	case float64(used) >= 0.8*float64(cap):
		return "near-limit"
	default:
		return "ok"
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
