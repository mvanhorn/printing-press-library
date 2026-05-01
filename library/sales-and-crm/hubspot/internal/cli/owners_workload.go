package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot-pp-cli/internal/store"
)

type ownerWorkloadRow struct {
	OwnerID      string  `json:"owner_id"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	OpenDeals    int     `json:"open_deals"`
	DealValue    float64 `json:"deal_value"`
	OpenTickets  int     `json:"open_tickets"`
	OverdueTasks int     `json:"overdue_tasks"`
	TotalLoad    int     `json:"total_load"`
}

func newOwnersWorkloadCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "workload",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show team member workload across deals, tickets, and tasks",
		Long:  "Cross-entity analysis showing which owners are overloaded with open deals, tickets, and overdue tasks.",
		Example: "  hubspot-pp-cli owners workload\n" +
			"  hubspot-pp-cli owners workload --limit 10 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("workload: %w", err)
			}
			defer s.Close()

			rows, err := buildWorkloadData(s, limit)
			if err != nil {
				return fmt.Errorf("workload: %w", err)
			}

			data, err := json.Marshal(rows)
			if err != nil {
				return fmt.Errorf("workload: %w", err)
			}
			if flags.compact {
				data = filterFields(data, "name,open_deals,open_tickets,overdue_tasks,total_load")
			}
			if flags.selectFields != "" {
				data = filterFields(data, flags.selectFields)
			}
			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return printOutput(cmd.OutOrStdout(), data, true)
			}
			return printWorkloadTable(cmd, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max owners to show")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func buildWorkloadData(s *store.Store, limit int) ([]ownerWorkloadRow, error) {
	type ownerInfo struct {
		name, email                     string
		openDeals, openTickets, overdue int
		dealValue                       float64
	}
	owners := map[string]*ownerInfo{}

	// Load owner names
	oRows, err := s.Query(`SELECT id, data FROM owners`)
	if err == nil {
		defer oRows.Close()
		for oRows.Next() {
			var id, raw string
			if err := oRows.Scan(&id, &raw); err != nil {
				continue
			}
			var o struct {
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
				Email     string `json:"email"`
			}
			if json.Unmarshal([]byte(raw), &o) != nil {
				continue
			}
			owners[id] = &ownerInfo{
				name:  strings.TrimSpace(o.FirstName + " " + o.LastName),
				email: o.Email,
			}
		}
	}

	// Count open deals per owner
	dRows, err := s.Query(`SELECT data FROM deals`)
	if err == nil {
		defer dRows.Close()
		for dRows.Next() {
			var raw string
			if err := dRows.Scan(&raw); err != nil {
				continue
			}
			var d struct {
				Properties map[string]string `json:"properties"`
			}
			if json.Unmarshal([]byte(raw), &d) != nil {
				continue
			}
			ownerID := d.Properties["hubspot_owner_id"]
			if ownerID == "" {
				continue
			}
			if owners[ownerID] == nil {
				owners[ownerID] = &ownerInfo{name: "Owner " + ownerID}
			}
			stage := d.Properties["dealstage"]
			if stage != "closedwon" && stage != "closedlost" {
				owners[ownerID].openDeals++
				if amt := d.Properties["amount"]; amt != "" {
					var v float64
					fmt.Sscanf(amt, "%f", &v)
					owners[ownerID].dealValue += v
				}
			}
		}
	}

	// Count open tickets per owner
	tRows, err := s.Query(`SELECT data FROM tickets`)
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var raw string
			if err := tRows.Scan(&raw); err != nil {
				continue
			}
			var t struct {
				Properties map[string]string `json:"properties"`
			}
			if json.Unmarshal([]byte(raw), &t) != nil {
				continue
			}
			ownerID := t.Properties["hubspot_owner_id"]
			if ownerID == "" {
				continue
			}
			if owners[ownerID] == nil {
				owners[ownerID] = &ownerInfo{name: "Owner " + ownerID}
			}
			owners[ownerID].openTickets++
		}
	}

	// Count overdue tasks per owner
	tkRows, err := s.Query(`SELECT data FROM tasks`)
	if err == nil {
		defer tkRows.Close()
		for tkRows.Next() {
			var raw string
			if err := tkRows.Scan(&raw); err != nil {
				continue
			}
			var tk struct {
				Properties map[string]string `json:"properties"`
			}
			if json.Unmarshal([]byte(raw), &tk) != nil {
				continue
			}
			ownerID := tk.Properties["hubspot_owner_id"]
			status := tk.Properties["hs_task_status"]
			if ownerID == "" || status == "COMPLETED" {
				continue
			}
			if owners[ownerID] == nil {
				owners[ownerID] = &ownerInfo{name: "Owner " + ownerID}
			}
			owners[ownerID].overdue++
		}
	}

	// Build result
	var result []ownerWorkloadRow
	for id, info := range owners {
		total := info.openDeals + info.openTickets + info.overdue
		result = append(result, ownerWorkloadRow{
			OwnerID:      id,
			Name:         info.name,
			Email:        info.email,
			OpenDeals:    info.openDeals,
			DealValue:    info.dealValue,
			OpenTickets:  info.openTickets,
			OverdueTasks: info.overdue,
			TotalLoad:    total,
		})
	}

	// Sort by total load descending
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].TotalLoad > result[i].TotalLoad {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func printWorkloadTable(cmd *cobra.Command, rows []ownerWorkloadRow) error {
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, strings.Join([]string{bold("NAME"), bold("DEALS"), bold("DEAL VALUE"), bold("TICKETS"), bold("OVERDUE TASKS"), bold("TOTAL")}, "\t"))
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%d\t$%.0f\t%d\t%d\t%d\n", r.Name, r.OpenDeals, r.DealValue, r.OpenTickets, r.OverdueTasks, r.TotalLoad)
	}
	return tw.Flush()
}
