// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"judgementtw-pp-cli/internal/extract"
	"judgementtw-pp-cli/internal/judicial"
	"judgementtw-pp-cli/internal/source/fjud"
)

// newPurgeCmd builds 'purge orphans' — the privacy-purge novel feature.
func newPurgeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Privacy compliance: remove judgments the Judicial Yuan has taken offline",
	}
	cmd.AddCommand(newPurgeOrphansCmd(flags))
	return cmd
}

func newPurgeOrphansCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "orphans",
		Short: "Re-check synced JIDs; delete any the website has removed (privacy)",
		Long: `Walks the locally-synced judgment IDs (optionally limited to those updated
after --since) and re-fetches each via the website. When a judgment returns
'查無資料' — the Judicial Yuan removed it for privacy — the local row is
deleted and an entry is added to change_log so the operator has an audit trail.

Required by the Judicial Yuan terms of use.`,
		Example: `  judgementtw-pp-cli purge orphans --since 115/01/01 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			var sinceDate time.Time
			if since != "" {
				sinceDate, err = extract.ParseDate(since)
				if err != nil {
					return usageErr(err)
				}
			}
			ids, err := listJudgmentIDs(ctx, db, "", "", 0)
			if err != nil {
				return err
			}
			c := fjudClient(flags)
			result := struct {
				Checked int      `json:"checked"`
				Removed int      `json:"removed"`
				Errors  int      `json:"errors"`
				Purged  []string `json:"purged_jids"`
			}{Purged: []string{}}
			for _, jid := range ids {
				if !sinceDate.IsZero() {
					p, err := extract.Parse(jid)
					if err == nil {
						jdate, _ := time.Parse("20060102", p.JDate)
						if jdate.Before(sinceDate) {
							continue
						}
					}
				}
				result.Checked++
				_, err := c.GetJudgment(ctx, jid, false)
				if errors.Is(err, fjud.ErrNotFound) {
					_, _ = db.ExecContext(ctx, `DELETE FROM judgments WHERE id = ?`, jid)
					_, _ = db.ExecContext(ctx, `DELETE FROM citations WHERE jid = ?`, jid)
					_, _ = db.ExecContext(ctx, `DELETE FROM sentences WHERE jid = ?`, jid)
					_, _ = db.ExecContext(ctx, `DELETE FROM jid_refs WHERE from_jid = ? OR to_jid = ?`, jid, jid)
					_ = judicial.LogEvent(ctx, db, "purged", jid, "purge orphans: 查無資料")
					result.Removed++
					result.Purged = append(result.Purged, jid)
					continue
				}
				if err != nil {
					result.Errors++
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: re-fetch %s failed: %v\n", jid, err)
					continue
				}
			}
			return emitJSON(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only re-check judgments dated on or after this date (民國 115/1/1 or 2026-01-01)")
	return cmd
}

// newDoctorWindowCmd builds 'doctor window' — the service-window reporter
// novel feature. Registered as a subcommand of `doctor` from root.go.
func newDoctorWindowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "window",
		Short: "Report whether the official open-data API service window is currently open (00:00–06:00 Asia/Taipei)",
		Long: `The official Judicial Yuan open-data API (data.judicial.gov.tw) only serves
between 00:00 and 06:00 Taipei time. This CLI defaults to website scraping
(which is 24/7), but if you later flip on API support, run 'doctor window'
to know whether the API is reachable now.`,
		Example:     `  judgementtw-pp-cli doctor window --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			now := extract.TaipeiTime()
			open := extract.IsAPIServiceWindow(now)
			payload := map[string]any{
				"taipei_time":               now.Format(time.RFC3339),
				"api_window_open":           open,
				"window_start":              "00:00 Asia/Taipei",
				"window_end":                "06:00 Asia/Taipei",
				"seconds_until_next_window": extract.SecondsUntilNextWindow(now),
			}
			return emitJSON(cmd.OutOrStdout(), payload, flags)
		},
	}
	return cmd
}
