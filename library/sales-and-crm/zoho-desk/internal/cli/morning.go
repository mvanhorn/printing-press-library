// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented RunE body.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelMorningCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	type breachRow struct {
		ID           string  `json:"id"`
		TicketNumber string  `json:"ticketNumber"`
		Subject      string  `json:"subject"`
		AssigneeID   string  `json:"assigneeId"`
		DueDate      string  `json:"dueDate"`
		HoursToDue   float64 `json:"hoursToDue"`
	}
	type overloadedRow struct {
		AgentID string  `json:"agentId"`
		Name    string  `json:"name"`
		Load    float64 `json:"load"`
	}

	cmd := &cobra.Command{
		Use:         "morning",
		Short:       "A single brief composing breach forecast, agent load, and overnight changes.",
		Example:     "  zoho-desk-pp-cli morning --json",
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
			names := agentNames(cmd.Context(), db)
			now := time.Now()

			// Breach forecast: open tickets due within the next 24h.
			breaches := make([]breachRow, 0)
			breachDeadline := now.Add(24 * time.Hour)
			for _, t := range tickets {
				if isClosedStatus(str(t, "status")) {
					continue
				}
				due, ok := parseZohoTime(str(t, "dueDate"))
				if !ok || due.Before(now) || due.After(breachDeadline) {
					continue
				}
				breaches = append(breaches, breachRow{
					ID:           str(t, "id"),
					TicketNumber: str(t, "ticketNumber"),
					Subject:      str(t, "subject"),
					AssigneeID:   str(t, "assigneeId"),
					DueDate:      str(t, "dueDate"),
					HoursToDue:   round1(due.Sub(now).Hours()),
				})
			}
			sort.SliceStable(breaches, func(i, j int) bool {
				ti, _ := parseZohoTime(breaches[i].DueDate)
				tj, _ := parseZohoTime(breaches[j].DueDate)
				return ti.Before(tj)
			})
			breachTop := breaches
			if len(breachTop) > 5 {
				breachTop = breachTop[:5]
			}

			// Agent load: open-ticket count per assignee.
			counts := map[string]int{}
			for _, t := range tickets {
				if isClosedStatus(str(t, "status")) {
					continue
				}
				counts[str(t, "assigneeId")]++
			}
			loadRows := make([]overloadedRow, 0, len(counts))
			loads := make([]float64, 0, len(counts))
			for aid, c := range counts {
				name := names[aid]
				if aid == "" {
					name = "(unassigned)"
				} else if name == "" {
					name = aid
				}
				loadRows = append(loadRows, overloadedRow{AgentID: aid, Name: name, Load: float64(c)})
				loads = append(loads, float64(c))
			}
			median := medianFloat(loads)
			sort.SliceStable(loadRows, func(i, j int) bool { return loadRows[i].Load > loadRows[j].Load })
			overloaded := loadRows
			if len(overloaded) > 5 {
				overloaded = overloaded[:5]
			}

			// Recent changes: tickets modified in the last 12h.
			recentCutoff := now.Add(-12 * time.Hour)
			recentCount := 0
			for _, t := range tickets {
				if mod, ok := parseZohoTime(str(t, "modifiedTime")); ok && !mod.Before(recentCutoff) {
					recentCount++
				}
			}

			view := struct {
				BreachForecast struct {
					Within string      `json:"within"`
					Count  int         `json:"count"`
					Top    []breachRow `json:"top"`
				} `json:"breachForecast"`
				AgentLoad struct {
					Median     float64         `json:"median"`
					Overloaded []overloadedRow `json:"overloaded"`
				} `json:"agentLoad"`
				RecentChanges struct {
					Since string `json:"since"`
					Count int    `json:"count"`
				} `json:"recentChanges"`
				ScannedTickets int `json:"scanned_tickets"`
			}{}
			view.BreachForecast.Within = "24h"
			view.BreachForecast.Count = len(breaches)
			view.BreachForecast.Top = breachTop
			view.AgentLoad.Median = round2(median)
			view.AgentLoad.Overloaded = overloaded
			view.RecentChanges.Since = "12h"
			view.RecentChanges.Count = recentCount
			view.ScannedTickets = len(tickets)

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
