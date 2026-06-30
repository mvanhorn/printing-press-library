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
func newNovelBreachHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var dbPath string

	type groupRow struct {
		Key        string  `json:"key"`
		Name       string  `json:"name"`
		Breaches   int     `json:"breaches"`
		Total      int     `json:"total"`
		BreachRate float64 `json:"breachRate"`
	}

	cmd := &cobra.Command{
		Use:         "breach-history",
		Short:       "Who breached SLA and how often, grouped by agent or department.",
		Example:     "  zoho-desk-pp-cli breach-history --by agent --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if flagBy != "agent" && flagBy != "department" {
				return usageErr(fmt.Errorf("invalid --by %q: must be 'agent' or 'department'", flagBy))
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

			var names map[string]string
			keyField := "assigneeId"
			if flagBy == "department" {
				keyField = "departmentId"
				names = departmentNames(cmd.Context(), db)
			} else {
				names = agentNames(cmd.Context(), db)
			}

			now := time.Now()
			totals := map[string]int{}
			breaches := map[string]int{}
			totalBreaches := 0
			for _, t := range tickets {
				key := str(t, keyField)
				totals[key]++

				due, ok := parseZohoTime(str(t, "dueDate"))
				if !ok || !due.Before(now) {
					continue
				}
				breached := false
				if isClosedStatus(str(t, "status")) {
					if ct, ok := parseZohoTime(str(t, "closedTime")); ok && ct.After(due) {
						breached = true
					}
				} else {
					breached = true
				}
				if breached {
					breaches[key]++
					totalBreaches++
				}
			}

			groups := make([]groupRow, 0, len(breaches))
			for key, b := range breaches {
				name := names[key]
				if key == "" {
					if flagBy == "department" {
						name = "(none)"
					} else {
						name = "(unassigned)"
					}
				} else if name == "" {
					name = key
				}
				total := totals[key]
				rate := 0.0
				if total > 0 {
					rate = round2(float64(b) / float64(total))
				}
				groups = append(groups, groupRow{
					Key:        key,
					Name:       name,
					Breaches:   b,
					Total:      total,
					BreachRate: rate,
				})
			}
			sort.SliceStable(groups, func(i, j int) bool { return groups[i].Breaches > groups[j].Breaches })

			view := struct {
				By            string     `json:"by"`
				TotalBreaches int        `json:"totalBreaches"`
				Groups        []groupRow `json:"groups"`
			}{
				By:            flagBy,
				TotalBreaches: totalBreaches,
				Groups:        groups,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "agent", "Group breaches by 'agent' or 'department'")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
