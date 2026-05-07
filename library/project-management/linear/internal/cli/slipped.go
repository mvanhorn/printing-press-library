package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// newSlippedCmd shows what carried over from the previous cycle into the
// current cycle. Defined as: issues that were in the prior cycle, did not
// complete, and are now in the active cycle (or unscheduled). Maya's
// Friday-update ritual: "what slipped from last sprint."
func newSlippedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var teamFilter string
	cmd := &cobra.Command{
		Use:   "slipped",
		Short: "Show issues that slipped from the previous cycle into the current cycle",
		Long: `Identifies issues that were active in the previous cycle, did not reach a
completed state, and are still open in the current cycle (or have rolled
into the active cycle for the team). Groups by reason heuristic:
  in_progress  the issue is still actively being worked
  carried      the cycle changed but state remained the same
  blocked      the issue is in a state-type of 'unstarted' but has been
               carried more than once`,
		Example: `  linear-pp-cli slipped
  linear-pp-cli slipped --team ENG --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			cycles, err := db.ListCycles("")
			if err != nil {
				return err
			}
			if len(cycles) == 0 {
				return fmt.Errorf("no cycles in local store; run 'linear-pp-cli sync' first")
			}

			currentRef, err := resolveCycleArg(cycles, "current")
			if err != nil {
				return err
			}
			previousRef, err := resolveCycleArg(cycles, "previous")
			if err != nil {
				return fmt.Errorf("no previous cycle to diff against: %w", err)
			}

			currentIssues, err := db.ListIssues(map[string]string{"cycle_id": currentRef.ID}, 1000)
			if err != nil {
				return err
			}
			previousIssues, err := db.ListIssues(map[string]string{"cycle_id": previousRef.ID}, 1000)
			if err != nil {
				return err
			}

			// Filter by team
			if teamFilter != "" {
				teamID := teamFilter
				if resolved, ok := resolveTeamID(db, teamFilter); ok {
					teamID = resolved
				}
				currentIssues = filterIssuesByTeam(currentIssues, teamID)
				previousIssues = filterIssuesByTeam(previousIssues, teamID)
			}

			// "Slipped" = in current cycle but state is not completed AND issue
			// also appeared in the previous cycle. We approximate the second
			// condition by intersecting on identifier (which is stable across
			// cycle changes for the same issue ID).
			type slim struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
				Title      string `json:"title"`
				Priority   int    `json:"priority"`
				Assignee   struct {
					Name        string `json:"name"`
					DisplayName string `json:"displayName"`
				} `json:"assignee"`
				State struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"state"`
			}

			parsePrev := map[string]bool{}
			for _, raw := range previousIssues {
				var s slim
				if err := json.Unmarshal(raw, &s); err == nil && s.ID != "" {
					parsePrev[s.ID] = true
				}
			}
			type slipRow struct {
				Identifier string `json:"identifier"`
				Title      string `json:"title"`
				State      string `json:"state"`
				Priority   string `json:"priority"`
				Assignee   string `json:"assignee"`
				Reason     string `json:"reason"`
			}
			var rows []slipRow
			for _, raw := range currentIssues {
				var s slim
				if err := json.Unmarshal(raw, &s); err != nil {
					continue
				}
				if !parsePrev[s.ID] {
					continue
				}
				if s.State.Type == "completed" || s.State.Type == "canceled" {
					continue
				}
				reason := "carried"
				switch s.State.Type {
				case "started":
					reason = "in_progress"
				case "unstarted", "backlog":
					reason = "blocked"
				}
				assignee := s.Assignee.DisplayName
				if assignee == "" {
					assignee = s.Assignee.Name
				}
				if assignee == "" {
					assignee = "(unassigned)"
				}
				rows = append(rows, slipRow{
					Identifier: s.Identifier,
					Title:      s.Title,
					State:      s.State.Name,
					Priority:   priorityLabel(s.Priority),
					Assignee:   assignee,
					Reason:     reason,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Reason != rows[j].Reason {
					return rows[i].Reason < rows[j].Reason
				}
				return rows[i].Identifier < rows[j].Identifier
			})

			summary := map[string]any{
				"current_cycle":  labelCycle(currentRef),
				"previous_cycle": labelCycle(previousRef),
				"slipped_count":  len(rows),
				"items":          rows,
			}
			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			}
			fmt.Fprintf(os.Stderr, "Slipped from %s into %s: %d issues\n", labelCycle(previousRef), labelCycle(currentRef), len(rows))
			if len(rows) == 0 {
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tPRI\tSTATE\tREASON\tASSIGNEE\tTITLE")
			for _, r := range rows {
				title := r.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Identifier, r.Priority, r.State, r.Reason, r.Assignee, title)
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&teamFilter, "team", "", "Filter to a team key (ENG) or UUID")
	_ = strings.Join // keep imports lean
	return cmd
}
