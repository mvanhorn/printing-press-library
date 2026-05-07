package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var jsonOut bool
	var days int
	var teamFilter string
	cmd := &cobra.Command{
		Use:         "stale",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Find issues not updated in N days",
		Long:        "Scan locally synced issues for staleness. Groups by team and project. Requires a prior sync.",
		Example: `  linear-pp-cli stale --days 30
  linear-pp-cli stale --days 14 --team ENG
  linear-pp-cli stale --json`,
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

			cutoff := time.Now().AddDate(0, 0, -days)
			type staleIssue struct {
				Identifier string                      `json:"identifier"`
				Title      string                      `json:"title"`
				UpdatedAt  string                      `json:"updatedAt"`
				DaysSince  int                         `json:"daysSince"`
				State      struct{ Name, Type string } `json:"state"`
				Team       struct{ Key string }        `json:"team"`
				Project    *struct{ Name string }      `json:"project"`
				Assignee   *struct{ Name string }      `json:"assignee"`
			}

			staleIssues := make([]staleIssue, 0)
			for _, raw := range issues {
				var row staleIssue
				json.Unmarshal(raw, &row)
				if row.State.Type == "completed" || row.State.Type == "canceled" {
					continue
				}
				updated, err := time.Parse(time.RFC3339, row.UpdatedAt)
				if err != nil {
					continue
				}
				if updated.Before(cutoff) {
					row.DaysSince = int(time.Since(updated).Hours() / 24)
					staleIssues = append(staleIssues, row)
				}
			}

			sort.Slice(staleIssues, func(i, j int) bool {
				return staleIssues[i].DaysSince > staleIssues[j].DaysSince
			})

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(staleIssues)
			}

			if len(staleIssues) == 0 {
				fmt.Printf("No issues stale for more than %d days.\n", days)
				return nil
			}

			fmt.Printf("%-12s %-5s %-10s %-15s %s\n", "ID", "DAYS", "TEAM", "STATE", "TITLE")
			fmt.Println(strings.Repeat("-", 80))
			for _, row := range staleIssues {
				title := row.Title
				if len(title) > 35 {
					title = title[:32] + "..."
				}
				fmt.Printf("%-12s %-5d %-10s %-15s %s\n", row.Identifier, row.DaysSince, row.Team.Key, row.State.Name, title)
			}
			fmt.Fprintf(os.Stderr, "\n%d stale issues (>%d days)\n", len(staleIssues), days)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Consider issues stale after this many days without updates")
	cmd.Flags().StringVar(&teamFilter, "team", "", "Filter by team key or ID")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
