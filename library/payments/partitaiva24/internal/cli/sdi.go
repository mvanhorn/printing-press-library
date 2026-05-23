// Copyright 2026 giuseppe-bisemi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: hand-authored SDI acknowledgement watcher.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type sdiStuckInvoice struct {
	ID                string `json:"id"`
	Number            string `json:"number"`
	Date              string `json:"date"`
	Customer          string `json:"customer"`
	Status            string `json:"status"`
	DaysSinceTransmit int    `json:"days_since_transmit"`
}

type sdiWatchReport struct {
	Threshold         string            `json:"threshold"`
	CandidatesChecked int               `json:"candidates_checked"`
	Stuck             []sdiStuckInvoice `json:"stuck"`
}

func newSdiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdi",
		Short: "Monitor SDI invoice delivery",
		Long:  "Inspect synced electronic invoices and live SDI notifications.",
		Example: `  partitaiva24-pp-cli sdi watch --older-than 7d
  partitaiva24-pp-cli sdi watch --older-than 72h --limit 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newSdiWatchCmd(flags))
	return cmd
}

func newSdiWatchCmd(flags *rootFlags) *cobra.Command {
	olderThan := "7d"
	limit := 50
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Find paid e-invoices without SDI acknowledgement",
		Long:  "Read older paid electronic invoices from the local store and check live SDI notification status.",
		Example: `  partitaiva24-pp-cli sdi watch --older-than 7d
  partitaiva24-pp-cli sdi watch --older-than 14d --limit 25`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dur, err := parseDurationDays(olderThan)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-dur).Format("2006-01-02")
			s, err := openCLIStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT id, number, date, status, COALESCE(json_extract(data, '$.to.companyname'), '') FROM invoices WHERE date < ? AND paid = 1 AND e_invoice = 1 ORDER BY date LIMIT ?`, cutoff, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report := sdiWatchReport{Threshold: olderThan}
			for rows.Next() {
				var id, number, dateStr, status, customer sql.NullString
				if err := rows.Scan(&id, &number, &dateStr, &status, &customer); err != nil {
					return err
				}
				data, err := c.Get(fmt.Sprintf("/user/invoices/%s/s_d_i_notification", nullableString(id)), nil)
				report.CandidatesChecked++
				if err != nil || sdiPending(data) {
					d, _ := parseYMD(nullableString(dateStr))
					report.Stuck = append(report.Stuck, sdiStuckInvoice{
						ID: nullableString(id), Number: nullableString(number), Date: nullableString(dateStr),
						Customer: nullableString(customer), Status: nullableString(status),
						DaysSinceTransmit: int(time.Since(d).Hours() / 24),
					})
				}
				time.Sleep(200 * time.Millisecond)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", olderThan, "Threshold duration such as 7d, 72h, or 168h")
	cmd.Flags().IntVar(&limit, "limit", limit, "Maximum invoices to check")
	return cmd
}

func sdiPending(data json.RawMessage) bool {
	if len(data) == 0 || string(data) == "null" || string(data) == "[]" || string(data) == "{}" {
		return true
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "pending") || strings.Contains(lower, "attesa")
}
