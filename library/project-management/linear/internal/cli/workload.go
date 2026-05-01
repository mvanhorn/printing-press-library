package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newWorkloadCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var jsonOut bool
	var teamFilter string
	cmd := &cobra.Command{
		Use:   "workload",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show issue and estimate distribution per team member",
		Long:  "Analyze workload balance across team members, including issue counts and total estimates.",
		Example: `  linear-pp-cli workload
  linear-pp-cli workload --team ENG
  linear-pp-cli workload --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			filter := map[string]string{}
			if teamFilter != "" {
				filter["team_id"] = teamFilter
			}
			issues, err := db.ListIssues(filter, 5000)
			if err != nil {
				return err
			}

			type memberLoad struct {
				Name     string  `json:"name"`
				Issues   int     `json:"issues"`
				Estimate float64 `json:"estimate"`
				InProg   int     `json:"inProgress"`
			}

			loads := map[string]*memberLoad{}
			for _, raw := range issues {
				var row struct {
					Estimate float64               `json:"estimate"`
					State    struct{ Type string } `json:"state"`
					Assignee *struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"assignee"`
				}
				json.Unmarshal(raw, &row)
				if row.State.Type == "completed" || row.State.Type == "canceled" {
					continue
				}
				if row.Assignee == nil {
					continue
				}
				ml, ok := loads[row.Assignee.ID]
				if !ok {
					ml = &memberLoad{Name: row.Assignee.Name}
					loads[row.Assignee.ID] = ml
				}
				ml.Issues++
				ml.Estimate += row.Estimate
				if row.State.Type == "started" {
					ml.InProg++
				}
			}

			sorted := make([]*memberLoad, 0, len(loads))
			for _, ml := range loads {
				sorted = append(sorted, ml)
			}
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].Estimate > sorted[j].Estimate })

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(sorted)
			}

			if len(sorted) == 0 {
				fmt.Println("No active issues with assignees found.")
				return nil
			}

			fmt.Printf("%-25s %-8s %-10s %-10s\n", "MEMBER", "ISSUES", "ESTIMATE", "IN PROG")
			fmt.Println(strings.Repeat("-", 60))
			for _, ml := range sorted {
				fmt.Printf("%-25s %-8d %-10.1f %-10d\n", ml.Name, ml.Issues, ml.Estimate, ml.InProg)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&teamFilter, "team", "", "Filter by team key or ID")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
