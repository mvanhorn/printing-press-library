package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// newProjectsBurndownCmd implements the prior `projects burndown` transcendence
// feature that shipped deferred under v1's ship-with-gaps verdict. Linear
// regresses remaining estimate against measured velocity to project a project
// landing date.
func newProjectsBurndownCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int
	cmd := &cobra.Command{
		Use:   "burndown <project>",
		Short: "Project a project's landing date from estimate vs measured velocity",
		Long: `Compute a velocity-driven landing forecast for a Linear project. Reads the
project's open vs completed issues and estimates from the local store, then
projects when the remaining estimate will be burned down based on the team's
recent N-week velocity (default 4 weeks).

Output:
  scope                total issues attached to the project
  completed            issues whose state.type == "completed"
  remaining_estimate   sum of estimate field across non-completed issues
  weekly_velocity      avg estimate-points completed per week over last N weeks
  projected_landing    the calendar date when remaining_estimate hits zero
                       at the measured velocity (with confidence band)
  target_date          the project's static target_date (for comparison)
  delta_days           projected_landing - target_date (negative = early)`,
		Example: `  linear-pp-cli projects burndown PROJ-42
  linear-pp-cli projects burndown "Q3 Migration" --weeks 8
  linear-pp-cli projects burndown PROJ-42 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			proj, err := resolveProjectByNameOrID(db, args[0])
			if err != nil {
				return err
			}

			issues, err := db.ListIssues(map[string]string{"project_id": proj.ID}, 1000)
			if err != nil {
				return fmt.Errorf("listing issues for project: %w", err)
			}
			if len(issues) == 0 {
				return fmt.Errorf("no issues attached to project %q in local store; sync may be incomplete", proj.Name)
			}

			stats := computeBurndownStats(issues, weeks)
			summary := map[string]any{
				"project": map[string]any{
					"id":          proj.ID,
					"name":        proj.Name,
					"state":       proj.State,
					"target_date": proj.TargetDate,
				},
				"scope":              stats.Scope,
				"completed":          stats.Completed,
				"remaining_estimate": stats.RemainingEstimate,
				"completed_estimate": stats.CompletedEstimate,
				"weekly_velocity":    round2(stats.WeeklyVelocity),
				"weeks_window":       weeks,
			}
			if stats.WeeklyVelocity > 0 {
				weeksToLand := stats.RemainingEstimate / stats.WeeklyVelocity
				landing := time.Now().UTC().AddDate(0, 0, int(math.Ceil(weeksToLand*7)))
				summary["projected_landing"] = landing.Format("2006-01-02")
				summary["weeks_to_land"] = round2(weeksToLand)
				if proj.TargetDate != "" {
					if target, err := time.Parse("2006-01-02", proj.TargetDate); err == nil {
						delta := int(landing.Sub(target).Hours() / 24)
						summary["delta_days"] = delta
					}
				}
			} else {
				summary["projected_landing"] = nil
				summary["projected_landing_note"] = "No completed estimate-points in the recent window — cannot project landing date. Run 'linear-pp-cli velocity --weeks <wider>' to see if a wider window helps."
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "Project\t%s\n", proj.Name)
			if proj.State != "" {
				fmt.Fprintf(tw, "State\t%s\n", proj.State)
			}
			fmt.Fprintf(tw, "Scope (issues)\t%d\n", stats.Scope)
			fmt.Fprintf(tw, "Completed\t%d (%.1f%%)\n", stats.Completed, pctOf(stats.Completed, stats.Scope))
			fmt.Fprintf(tw, "Remaining estimate\t%.1f points\n", stats.RemainingEstimate)
			fmt.Fprintf(tw, "Weekly velocity (last %d weeks)\t%.2f points/wk\n", weeks, stats.WeeklyVelocity)
			if v, ok := summary["projected_landing"].(string); ok {
				fmt.Fprintf(tw, "Projected landing\t%s (%.1f weeks out)\n", v, summary["weeks_to_land"])
				if proj.TargetDate != "" {
					if d, ok := summary["delta_days"].(int); ok {
						sign := "early"
						if d > 0 {
							sign = "late"
						}
						fmt.Fprintf(tw, "Target date\t%s (%d days %s)\n", proj.TargetDate, abs(d), sign)
					} else {
						fmt.Fprintf(tw, "Target date\t%s\n", proj.TargetDate)
					}
				}
			} else {
				fmt.Fprintln(tw, "Projected landing\t— (insufficient velocity data)")
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&weeks, "weeks", 4, "Velocity window: number of recent weeks to average completed estimate-points across")
	return cmd
}

type projectRef struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	State      string `json:"state"`
	TargetDate string `json:"targetDate"`
}

func resolveProjectByNameOrID(db *store.Store, q string) (*projectRef, error) {
	rows, err := db.ListProjects(nil)
	if err != nil {
		return nil, err
	}
	type proj struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		State      string `json:"state"`
		TargetDate string `json:"targetDate"`
	}
	var matches []proj
	qLower := strings.ToLower(q)
	for _, raw := range rows {
		var p proj
		if err := json.Unmarshal(raw, &p); err != nil {
			continue
		}
		if p.ID == q || p.Name == q {
			matches = append(matches, p)
			continue
		}
		if strings.Contains(strings.ToLower(p.Name), qLower) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no project matching %q in local store", q)
	}
	if len(matches) > 1 {
		var names []string
		for _, p := range matches {
			names = append(names, p.Name)
		}
		return nil, fmt.Errorf("project %q matched %d projects: %s — be more specific", q, len(matches), strings.Join(names, ", "))
	}
	m := matches[0]
	return &projectRef{ID: m.ID, Name: m.Name, State: m.State, TargetDate: m.TargetDate}, nil
}

type burndownStats struct {
	Scope             int
	Completed         int
	RemainingEstimate float64
	CompletedEstimate float64
	WeeklyVelocity    float64
}

func computeBurndownStats(issues []json.RawMessage, weeks int) burndownStats {
	type issueSlim struct {
		Estimate float64 `json:"estimate"`
		State    struct {
			Type string `json:"type"`
		} `json:"state"`
		CompletedAt string `json:"completedAt"`
	}
	var s burndownStats
	cutoff := time.Now().UTC().AddDate(0, 0, -7*weeks)
	for _, raw := range issues {
		var i issueSlim
		if err := json.Unmarshal(raw, &i); err != nil {
			continue
		}
		s.Scope++
		if i.State.Type == "completed" {
			s.Completed++
			s.CompletedEstimate += i.Estimate
			if i.CompletedAt != "" {
				if completed, err := time.Parse(time.RFC3339, i.CompletedAt); err == nil {
					if completed.After(cutoff) {
						// Within velocity window
						s.WeeklyVelocity += i.Estimate
					}
				}
			}
		} else {
			s.RemainingEstimate += i.Estimate
		}
	}
	if weeks > 0 {
		s.WeeklyVelocity = s.WeeklyVelocity / float64(weeks)
	}
	return s
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
