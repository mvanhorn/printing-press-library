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
func newNovelSlaRadarCmd(flags *rootFlags) *cobra.Command {
	var flagWithin string
	var dbPath string

	type slaRow struct {
		ID           string  `json:"id"`
		TicketNumber string  `json:"ticketNumber"`
		Subject      string  `json:"subject"`
		Status       string  `json:"status"`
		Priority     string  `json:"priority"`
		AssigneeID   string  `json:"assigneeId"`
		DueDate      string  `json:"dueDate"`
		HoursToDue   float64 `json:"hoursToDue"`
	}

	cmd := &cobra.Command{
		Use:         "sla-radar",
		Short:       "See which open tickets will breach SLA in the next N hours, ranked by time-to-due, before they breach.",
		Example:     "  zoho-desk-pp-cli sla-radar --within 2h --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			within, err := cliutil.ParseDurationLoose(flagWithin)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --within %q: %w", flagWithin, err))
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

			now := time.Now()
			deadline := now.Add(within)
			out := make([]slaRow, 0)
			for _, t := range tickets {
				status := str(t, "status")
				if isClosedStatus(status) {
					continue
				}
				due, ok := parseZohoTime(str(t, "dueDate"))
				if !ok || due.Before(now) || due.After(deadline) {
					continue
				}
				out = append(out, slaRow{
					ID:           str(t, "id"),
					TicketNumber: str(t, "ticketNumber"),
					Subject:      str(t, "subject"),
					Status:       status,
					Priority:     str(t, "priority"),
					AssigneeID:   str(t, "assigneeId"),
					DueDate:      str(t, "dueDate"),
					HoursToDue:   round1(due.Sub(now).Hours()),
				})
			}
			sort.SliceStable(out, func(i, j int) bool {
				ti, _ := parseZohoTime(out[i].DueDate)
				tj, _ := parseZohoTime(out[j].DueDate)
				return ti.Before(tj)
			})

			view := struct {
				Within         string   `json:"within"`
				Count          int      `json:"count"`
				ScannedTickets int      `json:"scanned_tickets"`
				Tickets        []slaRow `json:"tickets"`
			}{
				Within:         flagWithin,
				Count:          len(out),
				ScannedTickets: len(tickets),
				Tickets:        out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagWithin, "within", "24h", "Time window to scan for upcoming SLA breaches (e.g. 2h, 24h, 1d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
