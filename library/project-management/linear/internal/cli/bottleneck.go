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

// pp:data-source computed
// bottleneck never calls the API: it aggregates the locally synced issue rows
// into per-assignee load counts and an overload alert, so what it emits is
// derived arithmetic rather than the stored rows themselves.
func newBottleneckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var jsonOut bool
	var teamFilter string
	var stateFlag string
	cmd := &cobra.Command{
		Use:         "bottleneck",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Find overloaded team members and blocked issues",
		Long: `Analyze issue distribution per assignee to find bottlenecks. Shows who has too
many open issues and which issues are blocking others.

What counts as open is the 'active' state group by default. Redefine it in
groups.toml, or pass --state to count a different group for this run.`,
		Example: `  linear-pp-cli bottleneck
  linear-pp-cli bottleneck --team ENG
  linear-pp-cli bottleneck --state started --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			// The local --json flag shadows the root persistent one, so a
			// caller who passes --json ahead of the subcommand, or who uses
			// --agent, sets flags.asJSON and never sets jsonOut. Honor both.
			wantJSON := jsonOut || flags.asJSON

			set, err := resolveStateSet(flags, teamKeyForGroups(db, teamFilter), stateFlag)
			if err != nil {
				return err
			}

			filter := map[string]string{}
			if teamFilter != "" {
				filter["team_id"] = teamFilter
			}
			issues, err := db.ListIssues(filter, 5000)
			if err != nil {
				return err
			}

			type assigneeLoad struct {
				Name   string `json:"name"`
				ID     string `json:"id"`
				Active int    `json:"active"`
				Urgent int    `json:"urgent"`
				High   int    `json:"high"`
			}

			loads := map[string]*assigneeLoad{}
			for _, raw := range issues {
				var row struct {
					Priority int `json:"priority"`
					State    struct {
						Name string `json:"name"`
						Type string `json:"type"`
					} `json:"state"`
					Assignee *struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"assignee"`
				}
				json.Unmarshal(raw, &row)
				if !set.Matches(row.State.Type, row.State.Name) {
					continue
				}
				if row.Assignee == nil {
					continue
				}
				al, ok := loads[row.Assignee.ID]
				if !ok {
					al = &assigneeLoad{Name: row.Assignee.Name, ID: row.Assignee.ID}
					loads[row.Assignee.ID] = al
				}
				al.Active++
				if row.Priority == 1 {
					al.Urgent++
				} else if row.Priority == 2 {
					al.High++
				}
			}

			sorted := make([]*assigneeLoad, 0, len(loads))
			for _, al := range loads {
				sorted = append(sorted, al)
			}
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].Active > sorted[j].Active
			})

			if len(sorted) == 0 {
				hintIfUnsynced(cmd, db, "issues")
			} else {
				hintIfStale(cmd, db, "issues", flags.maxAge)
			}

			if wantJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(sorted)
			}

			if len(sorted) == 0 {
				fmt.Printf("No issues with assignees in state group %q.\n", set.Group.Name)
				return nil
			}

			fmt.Printf("%-25s %-8s %-8s %-8s %s\n", "ASSIGNEE", "ACTIVE", "URGENT", "HIGH", "ALERT")
			fmt.Println(strings.Repeat("-", 70))
			for _, al := range sorted {
				alert := ""
				if al.Active > 15 {
					alert = "OVERLOADED"
				} else if al.Urgent > 3 {
					alert = "TOO MANY URGENT"
				}
				fmt.Printf("%-25s %-8d %-8d %-8d %s\n", al.Name, al.Active, al.Urgent, al.High, alert)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&teamFilter, "team", "", "Filter by team key or ID")
	cmd.Flags().StringVar(&stateFlag, "state", "active", stateGroupFlagUsage)
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON (alias for the root --json flag)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
