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

// pp:data-source live
// today resolves the viewer through a live GraphQL query on every run, then
// filters the locally synced issue rows for that assignee. It cannot run
// without credentials, so it is not a local-only surface.
func newTodayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var jsonOut bool
	var stateFlag string
	cmd := &cobra.Command{
		Use:         "today",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Show your issues for today across all teams",
		Long: `Display all issues assigned to you, sorted by priority. Requires a prior sync.

--state resolves through the same state groups as 'issues list' and defaults to
the 'active' group. Run 'groups list' to see how active is defined here, or
'groups check active' to see exactly which of your workflow states it hits.`,
		Example: `  linear-pp-cli today
  linear-pp-cli today --json
  linear-pp-cli today --state started`,
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

			set, err := resolveStateSet(flags, "", stateFlag)
			if err != nil {
				return err
			}

			// Get viewer ID from config
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var viewer struct {
				Viewer struct {
					ID string `json:"id"`
				} `json:"viewer"`
			}
			if err := c.QueryInto(`query { viewer { id } }`, nil, &viewer); err != nil {
				return fmt.Errorf("fetching viewer: %w", err)
			}

			issues, err := db.ListIssues(map[string]string{
				"assignee_id": viewer.Viewer.ID,
			}, 200)
			if err != nil {
				return err
			}

			type issueRow struct {
				Identifier string `json:"identifier"`
				Title      string `json:"title"`
				Priority   int    `json:"priority"`
				State      struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"state"`
				Team struct {
					Key string `json:"key"`
				} `json:"team"`
				DueDate string `json:"dueDate"`
			}

			var active []issueRow
			for _, raw := range issues {
				var row issueRow
				json.Unmarshal(raw, &row)
				if set.Matches(row.State.Type, row.State.Name) {
					active = append(active, row)
				}
			}

			sort.Slice(active, func(i, j int) bool {
				if active[i].Priority != active[j].Priority {
					return active[i].Priority < active[j].Priority
				}
				return active[i].Identifier < active[j].Identifier
			})

			if len(active) == 0 {
				if !hintIfUnsynced(cmd, db, "issues") {
					hintIfStale(cmd, db, "issues", flags.maxAge)
					if wantJSON {
						enc := json.NewEncoder(os.Stdout)
						enc.SetIndent("", "  ")
						return enc.Encode(active)
					}
					fmt.Printf("No issues assigned to you in state group %q. Nice!\n", set.Group.Name)
				} else if wantJSON {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(active)
				}
				return nil
			}
			hintIfStale(cmd, db, "issues", flags.maxAge)

			if wantJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(active)
			}

			fmt.Printf("%-12s %-4s %-15s %-10s %s\n", "ID", "PRI", "STATE", "TEAM", "TITLE")
			fmt.Println(strings.Repeat("-", 80))
			for _, row := range active {
				pri := priorityLabel(row.Priority)
				title := row.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				fmt.Printf("%-12s %-4s %-15s %-10s %s\n", row.Identifier, pri, row.State.Name, row.Team.Key, title)
			}
			fmt.Fprintf(os.Stderr, "\n%d issues in state group %q\n", len(active), set.Group.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON (alias for the root --json flag)")
	cmd.Flags().StringVar(&stateFlag, "state", "active", stateGroupFlagUsage)
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func priorityLabel(p int) string {
	switch p {
	case 0:
		return "---"
	case 1:
		return "URG"
	case 2:
		return "HI"
	case 3:
		return "MED"
	case 4:
		return "LOW"
	default:
		return fmt.Sprintf("%d", p)
	}
}
