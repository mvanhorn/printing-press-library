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
func newNovelAgentLoadCmd(flags *rootFlags) *cobra.Command {
	var flagWeighted bool
	var dbPath string

	type agentRow struct {
		AgentID     string  `json:"agentId"`
		Name        string  `json:"name"`
		OpenTickets int     `json:"openTickets"`
		Load        float64 `json:"load"`
		AboveMedian bool    `json:"aboveMedian"`
	}

	cmd := &cobra.Command{
		Use:         "agent-load",
		Short:       "See which agents are actually overloaded right now, weighting open tickets by priority and age versus the team median.",
		Example:     "  zoho-desk-pp-cli agent-load --weighted --json",
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

			type acc struct {
				count    int
				weighted float64
			}
			groups := map[string]*acc{}
			now := time.Now()
			for _, t := range tickets {
				if isClosedStatus(str(t, "status")) {
					continue
				}
				aid := str(t, "assigneeId")
				a := groups[aid]
				if a == nil {
					a = &acc{}
					groups[aid] = a
				}
				a.count++
				ageDays := 0.0
				if ct, ok := parseZohoTime(str(t, "createdTime")); ok {
					if d := now.Sub(ct).Hours() / 24; d > 0 {
						ageDays = d
					}
				}
				a.weighted += float64(priorityWeight(str(t, "priority"))) * (1 + ageDays/7)
			}

			rows := make([]agentRow, 0, len(groups))
			loads := make([]float64, 0, len(groups))
			for aid, a := range groups {
				load := float64(a.count)
				if flagWeighted {
					load = round2(a.weighted)
				}
				name := names[aid]
				if aid == "" {
					name = "(unassigned)"
				} else if name == "" {
					name = aid
				}
				rows = append(rows, agentRow{
					AgentID:     aid,
					Name:        name,
					OpenTickets: a.count,
					Load:        load,
				})
				loads = append(loads, load)
			}

			median := round2(medianFloat(loads))
			for i := range rows {
				rows[i].AboveMedian = rows[i].Load > median
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Load > rows[j].Load })

			view := struct {
				Weighted       bool       `json:"weighted"`
				Median         float64    `json:"median"`
				Count          int        `json:"count"`
				ScannedTickets int        `json:"scanned_tickets"`
				Agents         []agentRow `json:"agents"`
			}{
				Weighted:       flagWeighted,
				Median:         median,
				Count:          len(rows),
				ScannedTickets: len(tickets),
				Agents:         rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().BoolVar(&flagWeighted, "weighted", false, "Weight open tickets by priority and age instead of a plain count")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
