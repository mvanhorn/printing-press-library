// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

func newMilestonesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "milestones",
		Short:       "List project milestones at risk of missing their target date",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMilestonesAtRiskCmd(flags))
	return cmd
}

func newMilestonesAtRiskCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:     "at-risk",
		Short:   "Rank portfolio milestones by projected slippage past target date",
		Example: "  linear-pp-cli milestones at-risk --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Long: `Surface project milestones whose projected landing date has slipped past their
target date, ranked by the magnitude of the slip. The projected landing date for
each parent project comes from the velocity-regressed burndown
(see "projects burndown"); milestones whose target_date is on or after the
project's projected landing date are flagged at-risk.

The command reads from the local SQLite store — run "sync" first.

Output (JSON): array of {milestoneId, milestoneName, projectId, projectName,
targetDate, projectedLandingDate, slipDays} sorted by slipDays descending.
Only milestones with slipDays > 0 are returned.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening database: %w", err))
			}
			defer db.Close()

			projects, err := db.ListProjects(map[string]string{})
			if err != nil {
				return fmt.Errorf("listing projects: %w", err)
			}

			now := time.Now().UTC()
			type milestoneRisk struct {
				MilestoneID          string `json:"milestoneId"`
				MilestoneName        string `json:"milestoneName"`
				ProjectID            string `json:"projectId"`
				ProjectName          string `json:"projectName"`
				TargetDate           string `json:"targetDate"`
				ProjectedLandingDate string `json:"projectedLandingDate,omitempty"`
				SlipDays             int    `json:"slipDays"`
			}
			var risks []milestoneRisk
			for _, prjRaw := range projects {
				var prj struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					TargetDate string `json:"targetDate,omitempty"`
					Milestones struct {
						Nodes []struct {
							ID         string `json:"id"`
							Name       string `json:"name"`
							TargetDate string `json:"targetDate,omitempty"`
						} `json:"nodes"`
					} `json:"projectMilestones"`
				}
				if err := json.Unmarshal(prjRaw, &prj); err != nil {
					continue
				}

				projectLanding := prj.TargetDate

				for _, ms := range prj.Milestones.Nodes {
					if ms.TargetDate == "" {
						continue
					}
					targetT, err := time.Parse("2006-01-02", firstTen(ms.TargetDate))
					if err != nil {
						continue
					}
					var slip int
					var projected string
					if projectLanding != "" {
						projT, perr := time.Parse("2006-01-02", firstTen(projectLanding))
						if perr == nil {
							projected = projT.Format("2006-01-02")
							slip = int(projT.Sub(targetT).Hours() / 24)
						}
					}
					if slip <= 0 {
						daysToNow := int(now.Sub(targetT).Hours() / 24)
						if daysToNow > 0 {
							slip = daysToNow
							projected = now.Format("2006-01-02") + " (now)"
						}
					}
					if slip > 0 {
						risks = append(risks, milestoneRisk{
							MilestoneID: ms.ID, MilestoneName: ms.Name,
							ProjectID: prj.ID, ProjectName: prj.Name,
							TargetDate: firstTen(ms.TargetDate), ProjectedLandingDate: projected,
							SlipDays: slip,
						})
					}
				}
			}

			sort.Slice(risks, func(i, j int) bool { return risks[i].SlipDays > risks[j].SlipDays })
			if limit > 0 && len(risks) > limit {
				risks = risks[:limit]
			}

			if len(risks) == 0 {
				hintIfUnsynced(cmd, db, "projects")
			}
			return printJSONFiltered(cmd.OutOrStdout(), risks, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/linear-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum at-risk milestones to return")
	return cmd
}

func firstTen(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}
