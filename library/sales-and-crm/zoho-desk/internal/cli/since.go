// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented RunE body.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	type sinceRow struct {
		ID           string `json:"id"`
		TicketNumber string `json:"ticketNumber"`
		Subject      string `json:"subject"`
		Status       string `json:"status"`
		Priority     string `json:"priority"`
		AssigneeID   string `json:"assigneeId"`
		ModifiedTime string `json:"modifiedTime"`
	}

	cmd := &cobra.Command{
		Use:         "since <duration>",
		Short:       "Every ticket modified within a time window, most recent first.",
		Example:     "  zoho-desk-pp-cli since 12h --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a <duration> argument is required (e.g. 12h, 7d)"))
			}
			dur, err := cliutil.ParseDurationLoose(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid duration %q: %w", args[0], err))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zoho-desk-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zoho-desk-pp-cli sync' first.", err)
			}
			defer db.Close()

			tickets, err := loadTickets(cmd.Context(), db)
			if err != nil {
				return fmt.Errorf("reading tickets: %w", err)
			}

			cutoff := time.Now().Add(-dur)
			out := make([]sinceRow, 0)
			for _, t := range tickets {
				mod, ok := parseZohoTime(str(t, "modifiedTime"))
				if !ok || mod.Before(cutoff) {
					continue
				}
				out = append(out, sinceRow{
					ID:           str(t, "id"),
					TicketNumber: str(t, "ticketNumber"),
					Subject:      str(t, "subject"),
					Status:       str(t, "status"),
					Priority:     str(t, "priority"),
					AssigneeID:   str(t, "assigneeId"),
					ModifiedTime: str(t, "modifiedTime"),
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				ti, _ := parseZohoTime(out[i].ModifiedTime)
				tj, _ := parseZohoTime(out[j].ModifiedTime)
				return ti.After(tj)
			})

			view := struct {
				Since          string     `json:"since"`
				Count          int        `json:"count"`
				ScannedTickets int        `json:"scanned_tickets"`
				Tickets        []sinceRow `json:"tickets"`
			}{
				Since:          args[0],
				Count:          len(out),
				ScannedTickets: len(tickets),
				Tickets:        out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
