// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented RunE body.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelTriageCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var dbPath string

	type triageRow struct {
		ID           string   `json:"id"`
		TicketNumber string   `json:"ticketNumber"`
		Subject      string   `json:"subject"`
		Status       string   `json:"status"`
		Priority     string   `json:"priority"`
		AssigneeID   string   `json:"assigneeId"`
		DueDate      string   `json:"dueDate"`
		Score        float64  `json:"score"`
		Reasons      []string `json:"reasons"`
	}

	cmd := &cobra.Command{
		Use:         "triage",
		Short:       "One ranked queue merging unassigned, overdue, and high-priority tickets with a priority score.",
		Example:     "  zoho-desk-pp-cli triage --limit 20 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
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
			scored := make([]triageRow, 0)
			for _, t := range tickets {
				status := str(t, "status")
				if isClosedStatus(status) {
					continue
				}
				priority := str(t, "priority")
				assignee := str(t, "assigneeId")
				dueStr := str(t, "dueDate")

				score := 0.0
				reasons := make([]string, 0, 3)
				if assignee == "" {
					score += 3
					reasons = append(reasons, "unassigned")
				}
				if due, ok := parseZohoTime(dueStr); ok && due.Before(now) {
					score += 3
					reasons = append(reasons, "overdue")
				}
				score += float64(priorityWeight(priority))
				if priority != "" {
					reasons = append(reasons, strings.ToLower(priority)+"-priority")
				}
				if ct, ok := parseZohoTime(str(t, "createdTime")); ok {
					ageDays := now.Sub(ct).Hours() / 24
					if ageDays > 5 {
						ageDays = 5
					}
					if ageDays > 0 {
						score += ageDays
					}
				}

				scored = append(scored, triageRow{
					ID:           str(t, "id"),
					TicketNumber: str(t, "ticketNumber"),
					Subject:      str(t, "subject"),
					Status:       status,
					Priority:     priority,
					AssigneeID:   assignee,
					DueDate:      dueStr,
					Score:        round1(score),
					Reasons:      reasons,
				})
			}
			sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
			if flagLimit > 0 && len(scored) > flagLimit {
				scored = scored[:flagLimit]
			}

			view := struct {
				Count          int         `json:"count"`
				ScannedTickets int         `json:"scanned_tickets"`
				Tickets        []triageRow `json:"tickets"`
			}{
				Count:          len(scored),
				ScannedTickets: len(tickets),
				Tickets:        scored,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum tickets to return, highest score first")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
