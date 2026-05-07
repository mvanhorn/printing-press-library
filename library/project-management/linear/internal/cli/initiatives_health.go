package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// newInitiativesHealthCmd shows a portfolio rollup: each initiative with its
// child projects, milestone target-vs-projected dates, and risk flags. Priya's
// Tuesday portfolio review.
func newInitiativesHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Rolled-up portfolio view per initiative: project progress, milestone risk, slippage",
		Long: `Compute a portfolio health snapshot per initiative. For each initiative:

  projects_total        count of projects under this initiative
  projects_on_track     projects with no late milestones
  projects_at_risk      projects with at least one milestone whose targetDate
                        falls within --warn-days but is not yet done
  projects_slipped      projects with at least one milestone whose targetDate
                        is in the past but not done

The data comes from a single live GraphQL query — no sync required.`,
		Example: `  linear-pp-cli initiatives health
  linear-pp-cli initiatives health --warn-days 14 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			warnDays, _ := cmd.Flags().GetInt("warn-days")

			// Linear caps GraphQL complexity (~1000 points for personal API keys).
			// Initiatives → projects nested twice deep is borderline; we leave
			// project milestones out and derive risk from project.targetDate
			// + project.progress instead. v1 retro #6.
			// Linear caps GraphQL complexity (~1000 points for personal API
			// keys). Initiatives × projects nested inflates fast; we trim
			// both pages so the budget stays under the cap. v1 retro #6.
			const q = `query InitiativeHealth {
				initiatives(first: 20) {
					nodes {
						id
						name
						status
						projects(first: 25) {
							nodes {
								id
								name
								state
								progress
								targetDate
							}
						}
					}
				}
			}`
			var resp struct {
				Initiatives struct {
					Nodes []struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						Status   string `json:"status"`
						Projects struct {
							Nodes []struct {
								ID         string  `json:"id"`
								Name       string  `json:"name"`
								State      string  `json:"state"`
								Progress   float64 `json:"progress"`
								TargetDate string  `json:"targetDate"`
							} `json:"nodes"`
						} `json:"projects"`
					} `json:"nodes"`
				} `json:"initiatives"`
			}
			if err := c.QueryInto(q, nil, &resp); err != nil {
				return fmt.Errorf("initiatives health query: %w", err)
			}

			now := time.Now().UTC()
			warnCutoff := now.AddDate(0, 0, warnDays)

			type projHealth struct {
				ID                string  `json:"id"`
				Name              string  `json:"name"`
				State             string  `json:"state"`
				Progress          float64 `json:"progress"`
				TargetDate        string  `json:"target_date"`
				MilestonesAt      int     `json:"milestones_at_risk"`
				MilestonesSlipped int     `json:"milestones_slipped"`
			}
			type initRow struct {
				ID              string       `json:"id"`
				Name            string       `json:"name"`
				Status          string       `json:"status"`
				ProjectsTotal   int          `json:"projects_total"`
				ProjectsOnTrack int          `json:"projects_on_track"`
				ProjectsAtRisk  int          `json:"projects_at_risk"`
				ProjectsSlipped int          `json:"projects_slipped"`
				HealthScore     float64      `json:"health_score"`
				Projects        []projHealth `json:"projects"`
			}

			var rows []initRow
			for _, ini := range resp.Initiatives.Nodes {
				row := initRow{
					ID:     ini.ID,
					Name:   ini.Name,
					Status: ini.Status,
				}
				for _, p := range ini.Projects.Nodes {
					ph := projHealth{
						ID:         p.ID,
						Name:       p.Name,
						State:      p.State,
						Progress:   p.Progress,
						TargetDate: p.TargetDate,
					}
					// Risk is derived from project.targetDate + progress: if the
					// target is in the past and progress < 100% we slip; if
					// target is within --warn-days and progress < 80% we are
					// at-risk; otherwise on-track.
					if p.TargetDate != "" {
						if target, err := time.Parse("2006-01-02", p.TargetDate); err == nil {
							if target.Before(now) && p.Progress < 1.0 {
								ph.MilestonesSlipped++
							} else if target.Before(warnCutoff) && p.Progress < 0.8 {
								ph.MilestonesAt++
							}
						}
					}
					row.ProjectsTotal++
					switch {
					case ph.MilestonesSlipped > 0:
						row.ProjectsSlipped++
					case ph.MilestonesAt > 0:
						row.ProjectsAtRisk++
					default:
						row.ProjectsOnTrack++
					}
					row.Projects = append(row.Projects, ph)
				}
				if row.ProjectsTotal > 0 {
					row.HealthScore = float64(row.ProjectsOnTrack) / float64(row.ProjectsTotal)
				}
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].HealthScore < rows[j].HealthScore
			})

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"warn_days_threshold": warnDays,
					"initiatives_count":   len(rows),
					"items":               rows,
				})
			}

			if len(rows) == 0 {
				fmt.Println("No initiatives found in your workspace.")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "INITIATIVE\tSTATUS\tPROJECTS\tON_TRACK\tAT_RISK\tSLIPPED\tHEALTH%")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%d\t%.0f%%\n",
					truncateText(r.Name, 30), r.Status, r.ProjectsTotal, r.ProjectsOnTrack, r.ProjectsAtRisk, r.ProjectsSlipped, r.HealthScore*100)
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().Int("warn-days", 14, "Milestones with targetDate within this many days are flagged at-risk")
	return cmd
}

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
